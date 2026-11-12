# Design: IBM Semeru provider

> 事实依据见 research.md（含实测样例与 URL 原文）；本文件只记决策与结构。

## 1. 模块边界

- 新包 `internal/provider/semeru/`，单文件 `semeru.go` + `semeru_test.go`（先例：corretto/zulu 单文件）。
- 嵌入 `provider.Base`，实现 `Name()="semeru"` / `DisplayName()` / `Available` / `Resolve` / `LatestPatch` / `ListVersions`，`init()` 调 `provider.Register`。
- 实现 `Configurable.Configure(arch, mirror)`：只消费 arch；mirror 忽略（无国内镜像，直连 GitHub，temurin-ea 先例），`Asset.MirrorURL` 恒空。

## 2. 数据源与端点

- 基址 `https://api.github.com/repos/ibmruntimes/`，repo = `semeru{major}-binaries`。
- 活跃 major 写死 `{8, 11, 17, 21, 25, 26}`；LTS = `{8, 11, 17, 21, 25}`（26 为当期短期版，IBM 下载页原文佐证）。27 出 GA 后手动加一行。
- `Available()` 返回静态列表（零请求，zulu `activeVersions` 先例）；表格行内 latest 由 cmd 层 `patchOf` → `LatestPatch` 逐条查询（10 分钟 TTL 缓存），**不在 Available 里打网络**。
- `LatestPatch(major)`：`GET /repos/.../releases/latest`（1 请求；latest 端点自动跳过 prerelease）。**防御**：latest release 无合格资产（如极端情况返回 certified）时回退 `ListVersions` 取首条。
- `ListVersions(major)` / `Resolve` 精确路径：`GET /releases?per_page=100`（全历史在架；单页足够，>100 条再加分页，research 实测最大 68）。
- 请求经 `app.HTTPGetJSON`，UA 用 `app.UserAgent()`；403/429 限流时错误文案提示「GitHub API 限流，稍后再试或 jvm available --refresh 绕过缓存前请等待」。

## 3. 解析契约

- **ReleaseName = tag 去 `jdk-` 前缀原样**（候选 A）：新格式 `21.0.13.0`、旧格式 `21.0.5+11_openj9-0.48.0`、8 线旧格式去 `jdk` 得 `8u402-b06_openj9-0.43.0`、8 线新格式 `8.0.492.0`。信息无损，release 页可直读。
- **Resolve 双分支**（version-parsing.md 规范）：`IsPureMajor` → LatestPatch；否则列表内精确匹配 ReleaseName；**短形匹配**：用户输入 `21.0.5+11`（省略 `_openj9-...` 尾巴）按「去掉尾部后的 ReleaseName 前缀」匹配，组内唯一命中才接受，多命中/零命中报错。
- **GA 过滤**：`prerelease == false` 为主 + tag 尾缀排除 `-m1|-m1a|-m2|-m2a|-rc`（防漏标）；certified 靠资产前缀天然排除。
- **资产挑选**：release.assets 内前缀 `ibm-semeru-open-jdk_x64_windows_` + 后缀 `.zip`（排除 msi/jre/debugimage/testimage/x86-32/certified）。**禁止由 tag 拼资产名**（过渡期 tag≠资产实证）。
- **版本比较**：直接 `app.CompareVersions`（按非数字切段，四段/`+build`/混合长度全兼容，version-parsing.md 禁止 provider 局部比较器）。新形态登记进 `internal/app/version_test.go` 注册表。
- **major 提取**：`app.MajorOfVersion`（`8u402-b06...` → 8 已兼容）。

## 4. 校验和

- 下载 URL 与校验和同源 release：资产 `browser_download_url` 原样入 `Asset.ZipURL`。
- 校验：同 release 内资产 `<zip名>.sha256.txt`（sha256sum 双列格式，`strings.Fields()[0]` 取第一列），填 `Asset.Checksum`（小写）+ `ChecksumAlgo` 留空（默认 sha256）。microsoft `.sha256sum.txt` 同款先例。
- `.zip.json` 旁路不接（信息更全但 30KB，校验和场景 `.sha256.txt` 足够；列为未来增强）。

## 5. 架构守卫

`arch == "aarch64"` 时四入口统一返回「Semeru (OpenJ9) 不提供 Windows ARM64 构建，请改用 temurin 或 microsoft」——corretto/graalvm 的 `errNoWindowsARM64` 同款模式（包内私有哨兵错误 + 各入口 `checkArch()`）。

## 6. 错误与缓存兼容

- `~/.jvm/available-cache.json`：distro 键自动生效，无需改 `cmd/available_cache.go`（验收项）。
- `outdated`/`update`：`LatestPatch` 契约满足即自动生效。
- 查询错误带 `(windows/x64)` 上下文（temurin 先例）。

## 7. 测试策略

- 表驱动单测（fixture JSON，不引 httptest 也可：解析函数收 `[]byte`）：三代 tag 解析（旧/过渡/新 + 8 线三杂例）、ReleaseName 归一、短形匹配唯一/多命中、GA 过滤（milestone/certified/正常）、资产挑选、sha256.txt 双列解析、aarch64 守卫、比较器排序（新旧混排）。
- `internal/app/version_test.go` 登记：`21.0.13.0`、`21.0.5+11_openj9-0.48.0`、`8u402-b06_openj9-0.43.0`、`8.0.492.0`。

## 8. 权衡记录

- Available 静态 vs 14 仓探测：选静态——LTS 集合稳定、省 14 请求/次；代价是新线（27+）需手动维护（graalvm `cpuReleases` 同类代价）。
- ReleaseName 候选 A vs B：A 含 `+`（temurin 目录先例），B 与资产名同源；选 A 因 release 页可直读、语义与 JVM `openjdk version` 输出更接近。
- 校验走 `.sha256.txt` vs `.zip.json`：选前者（百字节级、与 microsoft 先例同构）。
