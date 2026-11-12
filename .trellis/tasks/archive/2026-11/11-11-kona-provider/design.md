# Design: Tencent Kona provider

> 事实依据见 research.md；本文件只记决策与结构。

## 1. 模块边界

- 新包 `internal/provider/kona/`，单文件 `kona.go` + `kona_test.go`。
- 嵌入 `provider.Base`，`Name()="kona"`、`DisplayName()`，四入口 + `init()` 注册。
- `Configurable.Configure(arch, mirror)`：只消费 arch；MirrorURL 恒空。
- **本任务额外修改 `internal/jdk`**：`fileHash` 增加 md5 算法分支（kona 独占，无并行冲突）。

## 2. 数据源与端点

- **版本发现**：`https://api.github.com/repos/Tencent/TencentKona-{8,11,17,21,25}/releases?per_page=100`（每仓一发，五仓 ≤35 期单页全量）。
- **校验和源 A（SHA256）**：`https://tencent.github.io/konajdk/releases/kona-v1.json`（官方 GitHub Pages，actions/setup-java 同源；只作校验和源，**不作版本发现源**——滞后 1~2 季度）。进程内 `sync.Once` 拉一次，失败静默记 miss（索引是增强不是依赖）。
- **校验和源 B（MD5 兜底）**：`<zip browser_download_url> + ".md5"`（BSD 格式 `MD5 (file) = <32hex>`，正则 `=\s*([0-9a-fA-F]{32})`）。**旁路缺失 → 报错中止，禁止跳过校验**（11 线漏挂旁路实证，解析要容错而非假定存在）。
- majors 写死 `{8, 11, 17, 21, 25}` 全 LTS；`Available()` 静态。
- 四入口共用 `fetchFiltered(major)` + 进程内缓存（dragonwell 同构）。

## 3. 解析契约

- **tag → 版本，三套线内模板**：17/21/25 去前缀 `TencentKona-`；11 去前缀 `kona`；8 去前缀（无）+ 去后缀 `-GA`（早期 `v` 前缀一并剥）。ReleaseName 不含 `.bN`（资产 build 后缀不进版本号）：`21.0.13.0.3` / `17.0.21.0.3` / `25.0.5.0.3` / `11.0.33` / `8.0.28`。
- **排除规则**：tag 或资产名含 `fiber`；tag 含 `vector-api`；tag 去装饰后版本段数 < 2（`TencentKona-21-ga` → "21" 单段，弃）；release 无合格 Windows 资产（旧期/EA 天然过滤）。
- **资产挑选**：assets 内后缀 `jdk_windows-x86_64_signed.zip` 且不含 `fiber`（容忍 `b1_jdk`/`b1-jdk` 分隔符混用与 `.b1`/`.b2` 后缀差异）。**禁止由 tag 拼资产名**（`.b2` 重建反例）。
- **Resolve 双分支**（version-parsing.md）：`IsPureMajor` → LatestPatch；否则 ReleaseName 精确匹配。
- **版本比较**：`app.CompareVersions` 直接复用（3/4/5 段混排按段比较天然正确，research Q8 实证）。新形态登记 `internal/app/version_test.go`。
- major 提取 `app.MajorOfVersion`（`8.0.28` → 8、`21.0.13.0.3` → 21 均兼容）。

## 4. 校验和（两源混合，逐版本取最强——用户 2026-11-11 拍板）

Resolve 命中资产后：
1. 查 kona-v1.json：`files[]` 按 `version == ReleaseName && filename == 资产名` 匹配 → `Checksum` = 索引 hex（小写），`ChecksumAlgo="sha256"`；
2. 未命中 → 拉 `.md5` 旁路解析 32-hex → `ChecksumAlgo="md5"`；
3. `.md5` 也缺失/解析失败 → 返回错误（不装）。
- `internal/jdk/jdk.go` 的 `fileHash` 加 `case "md5"` → `md5.New()`（`crypto/md5`），注释同步「按厂商供给取最强」；`Install` 与 `cacheHit` 两条消费路径自动生效（`Asset.ChecksumAlgo` 已参数化）。
- provider 注释与 CHANGELOG 措辞（CHANGELOG 归收尾）注明两源依据。

## 5. 架构守卫

`arch == "aarch64"` 四入口统一报「Kona 不提供 Windows ARM64 构建，请改用 temurin 或 microsoft」（106 期 release 零 windows-aarch64 实证）。

## 6. 测试策略

- 表驱动：三套 tag 模板解析（含 `TencentKona-21-ga` 弃单段、`v8.0.0-GA` 早期、fiber/vector 排除）、资产挑选（`.b2`、`b1-jdk` 连字符、fiber 资产、无 windows 旧期）、md5 BSD 格式解析、kona-v1.json 命中/未命中路由、`.md5` 缺失报错、aarch64 守卫、3/4/5 段混排排序、索引拉取失败静默降级。
- `internal/app/version_test.go` 登记：`21.0.13.0.3`、`17.0.4.1`、`11.0.33`、`8.0.28`。
- `internal/jdk`：`fileHash` md5 用例（临时文件 + 已知 md5 值）。

## 7. 权衡记录

- 索引不整文件持久缓存：CLI 单命令进程生命周期短，`sync.Once` 够用；落盘缓存收益低且引入失效问题。
- MD5 兜底而非硬性只认 SHA256：用户拍板「跟厂商供给取最强」；新版本大概率走 MD5 分支（索引欠 2 轮）。
- `TencentKona-21-ga`（→21.0.0）弃：单段版本破坏目录/比较语义，且首期无实际使用价值。
