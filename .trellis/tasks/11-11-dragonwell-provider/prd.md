# 接入 Alibaba Dragonwell provider

## Goal

新增 `dragonwell` 发行版 provider：Alibaba Dragonwell **Standard 风味**（跟随上游 OpenJDK 的标准构建），数据源为官方 GitHub releases。仅 Windows x64。

## 已知事实（2026-11-12 子代理调研实测，详见本目录 research.md）

- **全家已迁 `dragonwell-project` org**：8 / 11 / 17 / 21 均在 `dragonwell-project/dragonwell<major>`（alibaba 旧仓只剩转移重定向，`alibaba/dragonwell21` 404）。路由表单 org 一张即可。
- **25 线已存在**：`dragonwell-project/dragonwell25` 已有 4 个 Standard GA（`25.0.4.0.4+7` 起），含 `x64_windows` zip。
- **版本号为自家格式，逐线不同**：21/25 线 `21.0.12.0.12+8`（asset 名 `+` 变 `.`）；11 线 tag `11.0.32.28` → 资产却带 OpenJDK build 段 `11.0.32.28.9`（**资产名不可由 tag 推导**，须在 release 资产列表按 `_x64_windows.zip` 后缀匹配）；8 线资产 = tag 版本原样（`8.30.29`）。
- **GA 过滤靠命名**：`.prerelease` 字段不可用（extended 也是 false）；过滤 = tag 前缀 `dragonwell-standard-` + 尾缀 `-ga`（顺带排除 `-fix.zip` 变体）。`/releases/latest` 端点行为黑盒，不依赖。
- **校验和齐全**：每资产有旁路 `.sha256.txt`（格式 `hex + 两空格 + 内部构建路径`，取第一列）+ Adoptium 风格 `.zip.json`（自带 sha256 字段，可一次拿全）。
- Windows 仅 `x64_windows`；历史 release 全保留（2019 至今无删），每仓 1 发 `?per_page=100` 即可覆盖四入口。
- Standard 与 Extended 两种风味（Extended 叠加阿里私有增强）；本次只接 **Standard**，Extended 后续可按 temurin-ea 先例作为第二 provider 变体加（不在本任务范围）。

## Requirements

- `internal/provider/dragonwell/` 新增适配器，嵌入 `Base`，实现四入口 + `Configurable`，`init()` 自注册。
- major 清单写死 `8 / 11 / 17 / 21 / 25`（全 LTS，LTS 标记恒 true；25 线调研确认已有 Standard GA，纳入；新线出现时随版本升级维护，graalvm `cpuReleases` 先例）。
- **repo 路由表**：major → `dragonwell-project/dragonwell<major>` 单 org 写死（design.md 记录 2026 年从 alibaba org 整体迁出的背景）。
- **版本号适配（本任务核心）**：
  - ReleaseName 规范在 design.md 定案（候选：完整四段 `21.0.12.0.12+8`，tag 可逆推、`available -a` 复制可安装；8 系格式抽样后定）。
  - `CompareVersions` / `MajorOf` 对四段版本的处理需验证，必要时 provider 内局部比较器（zulu `compareJavaVersion` 先例），**不改 `internal/app` 全局比较语义**。
  - tag → 版本、asset 名 → 版本的双向解析各有格式，表驱动单测覆盖。
- `Resolve` 精确/大版本两态；`ListVersions(major)` 分页遍历该仓库全部 GA。
- 校验：`.sha256.txt` 旁路 SHA256。
- `arch=aarch64` 统一无 Windows ARM64 错误（corretto/graalvm 先例）。
- 中文注释与 CLI 文案。

## Out of Scope

- Extended 风味；dragonwell 25（不存在）；Linux/macOS；镜像源。

## Acceptance Criteria

- [ ] 单测：tag/asset 版本解析（含 `+`↔`.` 差异、8 系特例）、四段版本比较、LTS 标记、aarch64 守卫、分页解析
- [ ] `jvm available dragonwell` / `-a` / `--json` 正确且缓存生效
- [ ] `install dragonwell@21` / `install dragonwell@<完整版本>` → SHA256 校验 → `use` / `exec dragonwell@21 -- java -version` → `uninstall` 全链路人工验证
- [ ] `outdated` / `update dragonwell@21` 工作
- [ ] 版本目录名与 junction 匹配、`uninstall --all` 组语义在四段版本下正确
- [ ] `make fmt` / `make vet` / `make test` / `make test-race` 全绿
