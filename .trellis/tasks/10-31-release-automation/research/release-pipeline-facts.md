# 发版管线迁移研究结论（父任务共享）

> 来源：2026-10-31 规划会话的 web 调研 + repo 证据核对。三个子任务共用，各自 design.md 引用本文，不重复展开。

## GoReleaser 能力边界

- **NSIS 支持存在且开源可用**（非 Pro 专属）：文档 goreleaser.com/customization/package/nsis，v2.2 引入、v2.14 博客再次确认。但 goreleaser 用**自己生成的 NSIS 脚本模板**，不支持复用项目自有 `.nsi`。
- 本项目 `installer/jvm.nsi` 的定制逻辑 goreleaser 模板覆盖不了（repo 证据）：MUI2 组件选择页、SecConfig 安装后跑 `jvm.exe version` 触发自举、卸载时询问删 `~/.jvm`、`InstallDirRegKey` 复用目录、HKCU Uninstall 全套条目 → **决策：混合模式，NSIS 留 Makefile**。
- 生命周期顺序（对本设计关键）：`--rm-dist` 清理 → `before.hooks` → builds → archives → checksums → release（extra_files 在此阶段解析 glob）。因此 **before hook 里跑 `make installer` 产出的 setup.exe 不会被后续清理，且早于 extra_files 收集**——这是 NSIS 桥接的落点。
- `{{ .Version }}` = 去 `v` 前缀的 tag，与 Makefile `git describe | sed 's/^v//'` 语义一致；`checksums.name_template` 可产出 `checksums.txt`（默认 `<project>_checksums.txt` 必须改）；`extra_files` 默认**不进** checksums。
- archives `name_template: "jvm-windows-{{ .Arch }}"`：goreleaser v2 已移除 replacements 机制，`.Arch` 原样输出 `amd64`/`arm64`，满足资产名契约。
- zip 内是否包顶层目录不影响消费者：upgrade.go 用 `filepath.Base(f.Name)=="jvm.exe"` 匹配，install.ps1 用 `$entry.Name`（basename）匹配，均容错。

## Scoop / Extras 机制

- ScoopInstaller/Extras 无 push 权限，只有 PR 通道；收录有 maintainer 裁量门槛（知名度、发版稳定性），全新仓库可能被拒/搁置。
- manifest 带 `checkver` + `autoupdate` 后，Extras 官方 **Excavator** action（ScoopInstaller/GithubActions 仓库，每 4h）自动检测新 GitHub release、更新 version/URL/hash 并提交 → 发版侧零成本。**不要**在发版 CI 里再开 scoop PR job（与 Excavator 重复，社区不鼓励 bot PR 噪音）。
- `bucket/jvm.json` 在 Extras 未占用（2026-10-31 查重 404）。
- 参考：github.com/ScoopInstaller/GithubActions、github.com/ScoopInstaller/Scoop/wiki/Buckets。

## WinGet 机制

- 首次必须人工提 New Package PR（一次性）；后续自动化标准做法：`vedantmgoyal2009/winget-releaser` action（Marketplace "WinGet Releaser"），`on: release: types: [published]` 触发，底层 `microsoft/winget-create update` 自动改 manifest + 提交 PR。
- **需要 classic PAT（public_repo scope）**存 repo secret（默认 `GITHUB_TOKEN` 开不了跨仓库 PR）。
- 兼容性核对：NSIS per-user（HKCU 免管理员）→ `Scope: user`；NSIS 原生 `/S` 静默；`QuietUninstallString` 已写（jvm.nsi:82）；arm64 setup.exe 的 x86 stub 无碍——winget `Architecture` 字段指目标架构；无代码签名不阻塞收录（SmartScreen 提示维持现状）。

## 本仓库的资产契约（三消费者矩阵）

| 资产 | upgrade.go:48 | install.ps1:108-110 | winget/scoop manifest |
|---|---|---|---|
| `jvm-windows-{amd64,arm64}.zip` | ✅ 精确名匹配 | ✅ `releases/latest/download/` | ✅（scoop 用；winget 用 setup.exe） |
| `checksums.txt` | ✅ 逐行按文件名取 hash | ✅ 同左 | scoop autoupdate 的 hash 提取源 |
| `jvm-windows-{arch}-setup.exe` | — | — | ✅ winget InstallerUrl |
| `install.ps1` | — | （自身） | — |

checksums 解析（upgrade.go `parseChecksum` ≡ install.ps1 `Get-ExpectedChecksum`）：逐行两字段、按文件名匹配、支持 `*` 前缀 binary 格式——**多行不破坏兼容**。
