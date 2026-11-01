# 发版自动化: GoReleaser 迁移与包管理器生态接入（父任务）

## Goal

把发版管线从「手写 Makefile + release.yml 编排」迁移到 GoReleaser + GitHub Actions，并接入 WinGet / Scoop 两条包管理器分发线。本任务为父任务：持有需求源、子任务地图、顺序约束与跨子集成验收，自身无直接实现工作。

## Background（已确认事实，证据锚点）

- **资产名契约有三个存量消费者**，迁移不得破坏任何一方：
  - `internal/upgrade/upgrade.go:48` `expectedAssetName()` = `jvm-windows-{GOARCH}.zip`，精确名匹配 release asset，`jvm upgrade` 自更新全靠它；
  - `install.ps1:108-110` 走 `releases/latest/download/` 拉同名 zip + `checksums.txt`；
  - WinGet / Scoop manifest（本任务树新增的第四个消费者）也将指向这些 URL。
- **checksums.txt 格式契约**：GNU sha256sum 文本格式，逐行两字段（`<hash>  <filename>`），消费者按文件名逐行匹配（upgrade.go 的 `parseChecksum` 与 `install.ps1:66` `Get-ExpectedChecksum` 互为镜像）；**多出的行不破坏兼容**。当前内容只含两个便携 zip。
- **当前 release 资产集**（release.yml:119-125）：`jvm-windows-{amd64,arm64}-setup.exe`、`jvm-windows-{amd64,arm64}.zip`、`checksums.txt`、`install.ps1`。
- **NSIS 安装器有大量 goreleaser 模板无法覆盖的定制逻辑**（installer/jvm.nsi）：MUI2 组件选择页三 section、SecConfig 安装后跑一次 `jvm.exe version` 触发自举、卸载时询问是否删 `~/.jvm` 用户数据、`InstallDirRegKey` 升级复用目录、HKCU Uninstall 注册表全套。→ 决定采用**混合模式**：NSIS 留在 Makefile，goreleaser 只接管编译/zip/checksums/release。
- **单测 gate**：release.yml:30-33，红则不发版（安全敏感项目，发版不可逆）。
- **Release body 来源**：release.yml:86-112 awk 抽取 CHANGELOG.md 版本段落（AGENTS.md 硬约束，漏写导致 Release 页面无说明）。
- **版本注入语义**：Makefile:8,18 `git describe | sed 's/^v//'`；GoReleaser `{{ .Version }}` 同为去 `v` 的 tag，语义一致。Bootstrap 差异靠 ldflags（Makefile:21），goreleaser 只注入 `Version` 即等价于 `build-dist` 的自举语义。
- **Scoop 机制**：ScoopInstaller/Extras 只能 PR 不能 push；manifest 带 `checkver`+`autoupdate` 后由官方 Excavator（每 4h）自动跟进新版本，发版侧零成本；但 Extras 收录有 maintainer 裁量门槛（知名度/发版稳定性）。
- **WinGet 机制**：首次必须人工提 New Package PR（microsoft/winget-pkgs）；后续版本可用 winget-releaser action 在 release published 时自动提交 PR，需 classic PAT（public_repo scope）存 secret。本项目 NSIS 安装器 per-user（HKCU、免管理员）→ `Scope: user`，NSIS 原生支持 `/S` 静默，与 winget 兼容。

## 任务地图与顺序约束

| 顺序 | 子任务 | 交付物 | 依赖 |
|---|---|---|---|
| 1 | `10-31-goreleaser-migration` | GoReleaser 接管发版产线，资产完全兼容 | 无（是其余两个的基座） |
| 2a | `10-31-winget-automation` | winget-pkgs 首提 + 自动版本 PR | 子任务 1 完成（资产产线稳定） |
| 2b | `10-31-scoop-distribution` | 自建 bucket 自动 push（+ Extras 提交，范围待定） | 子任务 1 完成；与 2a 无依赖可并行 |

父子树不是依赖系统：本顺序必须同时写入各子任务 prd.md，执行时依次 `task.py start` 各子任务。

## 跨子任务验收标准（父任务集成验收）

在全部子任务完成后，以一次真实发版（打 tag vX.Y.Z）做端到端验证：

- [x] GoReleaser 产出全部 5 类资产，命名与 checksums.txt 格式与旧管线完全一致；
- [ ] 旧版本 `jvm upgrade` 能从新 release 自更新成功（资产名匹配 + SHA256 校验通过）；
- [x] `install.ps1` 从新 release 安装成功（含校验）；
- [ ] WinGet 侧出现自动版本更新 PR（首提已在早前人工完成的前提下）；
- [ ] Scoop bucket manifest 自动更新，`scoop update` 能检测到新版本；
- [x] AGENTS.md「Setup commands / 发布」章节同步改写，CHANGELOG.md 记录本次变更。

## Out of Scope

- 不改变任何 `jvm` 命令的运行时行为（`internal/` 代码零改动为理想情况）；
- 不做 winget 发布者验证（Publisher Verification，需组织账号与证书体系）；
- 不做代码签名（SmartScreen 提示维持现状）；
- 不迁移 CI 单测 workflow（ci.yml）与集成测试（integration-test.sh）的触发方式；
- 不做 Chocolatey / 其它包管理器。
