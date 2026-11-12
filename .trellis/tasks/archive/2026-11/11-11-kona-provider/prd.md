# 接入 Tencent Kona provider

## Goal

新增 `kona` 发行版 provider：腾讯 KonaJDK（HotSpot 下游，保守跟随上游，面向大数据/云场景调优），数据源为官方 `Tencent/TencentKona-*` GitHub releases。仅 Windows x64。

## 已知事实（2026-11-12 子代理调研实测，详见本目录 research.md）

- **repo 布局：每个大版本一个仓库**：`Tencent/TencentKona-{8,11,17,21,25}`（全 LTS 线）。
- **SHA256 官方索引已找到**：`https://tencent.github.io/konajdk/releases/kona-v1.json`（腾讯官方 GitHub Pages，`actions/setup-java` 同源消费），按文件给 64-hex SHA256，子代理实测下载一版逐字节校验一致。**索引滞后 1~2 个季度**（每线只保留最近 8 期）→ 最新版本仍只有 `.md5`。
- **校验和策略（逐版本取最强，2026-11-11 用户确认原则）**：索引命中走 SHA256；未命中走 `.md5` 旁路（BSD 格式 `MD5 (file) = <32hex>`，`internal/jdk` 仍需 md5 算法扩展）。release 正文/wiki/README 均无 SHA256。
- **tag 三套模板按线不同**：17/21/25 `TencentKona-<ver>`、11 `kona<ver>`、8 `<ver>-GA`（资产前缀 `TencentKona8`）；`.b1`/`.b2` build 后缀只在资产名且不稳定（有 `.b2` 重建反例）→ **资产必须从 release 列表按 `jdk_windows-x86_64_signed.zip` 后缀挑，tag 推不出资产名**。
- 四段版本 `21.0.13.0.3` 与 `openjdk version` 输出逐字相等（`+1-LTS` 是 build 串）；**2026-10 周期起 21/17/25 三线同时从三段切五段**——比较器须与段数无关。
- Windows 矩阵：五线 latest 全有 `windows-x86_64` signed zip；106 期 release 零 `windows-aarch64`。
- GA 过滤不能靠 `.prerelease`（全 false），靠命名排除 `Fiber-*` 等变体。
- 历史 release 全量保留（2019-11 至今），Available = 5 发请求（每仓一发 `?per_page=100`）。
- ReleaseName 建议：tag 派生版本号、不含 `.bN`（`21.0.13.0.3` / `11.0.33` / `8.0.28`）——复制可装、组内唯一、tag 可逆推。

## Requirements

- `internal/provider/kona/` 新增适配器，嵌入 `Base`，实现四入口 + `Configurable`，`init()` 自注册。
- major 清单写死 `8 / 11 / 17 / 21 / 25`（全 LTS，LTS 标记恒 true）。
- **校验和实现（逐版本取最强，调研已定源）**：
  1. SHA256 优先：查询官方索引 `kona-v1.json`（按文件名匹配），命中则用 SHA256 校验；
  2. 未命中（索引滞后的最新版本）→ 下载 GitHub release 的 `.md5` 旁路（BSD 格式 `MD5 (file) = <32hex>`）校验——需扩展 `Asset.ChecksumAlgo` 支持 `md5`（`internal/jdk` 校验层加算法，与 liberica 的 `sha1` 先例同构）；
  3. provider 注释与 CHANGELOG 明示校验来源与"逐版本取最强"策略。
- ReleaseName 规范 design.md 定案（候选：`21.0.13.0.3`，`.b1` 是否入名以"复制可安装 + 组内唯一"为准）。
- 版本比较适配同 dragonwell（局部比较器优先，不动全局语义）。
- `arch=aarch64` 统一无 Windows ARM64 错误。
- 中文注释与 CLI 文案。

## Out of Scope

- 非线内大版本；JRE/其他平台；镜像源；MD5 之外任何"无校验"方案（禁止跳过完整性校验）。

## Acceptance Criteria

- [ ] 校验和方案在 design.md 记录依据（SHA256 索引 URL + 滞后性结论 + 用户"逐版本取最强"确认）；`internal/jdk` 的 md5 算法扩展有单测
- [ ] 单测：三套 tag 模板解析、资产后缀挑选（含 `.bN` 后缀变体）、段数无关版本比较、LTS 标记、aarch64 守卫、分页解析、索引命中/未命中两态校验路由
- [ ] `jvm available kona` / `-a` / `--json` 正确且缓存生效
- [ ] `install kona@21` / `install kona@<完整版本>` → 校验和验证 → `use` / `exec kona@21 -- java -version` → `uninstall` 全链路人工验证
- [ ] `outdated` / `update kona@21` 工作
- [ ] `make fmt` / `make vet` / `make test` / `make test-race` 全绿
