# WinGet 自动提交（子任务 2/3）

> 顺序约束：**依赖 `10-31-goreleaser-migration` 完成**（manifest 指向的资产 URL 与 hash 依赖稳定产线）；与 `10-31-scoop-distribution` 无依赖，可并行。父任务：`10-31-release-automation`。

## Goal

让 jvm 可通过 `winget install` 安装、`winget upgrade` 升级：首次人工向 microsoft/winget-pkgs 提交 New Package manifest，之后每次发版由 release workflow 自动提交版本更新 PR（winget-releaser action）。

## 已确认事实

- 机制与 per-user 兼容性论证见父任务 `10-31-release-automation/prd.md` Background 节。
- 双 installer（amd64/arm64 setup.exe）进同一 manifest：`Architecture: x64/arm64`、`InstallerType: nullsoft`、`Scope: user`（HKCU 免管理员）；NSIS 原生支持 `/S` 静默；arm64 setup.exe 的 stub 是 x86 不影响——winget 的 Architecture 指目标架构。
- jvm.nsi 已写 `QuietUninstallString`（jvm.nsi:82），winget 卸载链路可用。
- 无代码签名：winget 不强制，收录 review 可过，用户侧有 SmartScreen 提示（父任务 Out of Scope 已排除签名）。

## Requirements

1. 生成首提 manifest（`winget-create` 或手写 YAML，PackageId 建议 `BaixuanZhu.JVM`）：
   - 两个 installer 条目（x64/arm64 nullsoft、user scope、`/S` 静默开关）；
   - InstallerUrl 指向 GitHub Release 资产；InstallerSha256 从 checksums 或本地计算；
   - ReleaseNotesUrl 指向 CHANGELOG/Release；
2. 人工提交 New Package PR 到 microsoft/winget-pkgs 并跟进合并（一次性，无法自动化）；
3. release workflow 新增 winget job：
   - 触发：release published（或 tag 流程末尾的独立 job）；
   - 使用 `vedantmgoyal2009/winget-releaser` action（底层 winget-create update）自动提交版本 PR；
   - **不阻塞主发版流程**：winget job 失败不影响 Release 资产已发布的事实（job 级 continue-on-error 或独立 workflow + 告警）；
   - 需 `WINGET_TOKEN` secret（classic PAT，public_repo scope）——**用户外部动作**，实现前须就位；
4. AGENTS.md 发布章节补 winget 说明（PAT 属 repo secret，轮换方式记录）；CHANGELOG.md 记录。

## Acceptance Criteria

- [ ] 首提 PR 已合并，`winget search` 能找到包（验证环境：任一台 Windows 机器）；
- [ ] `winget install BaixuanZhu.JVM` 静默安装成功，`jvm version` 可用（等 bootstrap 触发后新终端）；
- [ ] 下一次真实发版后，winget-pkgs 自动出现版本更新 PR（winget-releaser job 日志可证）；
- [ ] winget job 失败（可临时制造，如 token 失效演练）不影响 Release 主流程。

## Out of Scope

- 发布者验证（Publisher Verification）、代码签名；
- winget「便携 zip 包类型」manifest（选 installer 路线，便携归 scoop）；
- 多语言/多 Channel（如 EA 变体）的 winget 包装。

## Open Questions

- PackageId 最终取值（建议 `BaixuanZhu.JVM`，与 GitHub owner/repo 一致降低心智负担）——首提时确认即可，不阻塞规划。
