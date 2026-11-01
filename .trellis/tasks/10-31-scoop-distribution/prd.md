# Scoop 分发接入（子任务 3/3）

> **状态：挂起（2026-11-01）**——Extras 的 Package Request issue 有 required 知名度门槛（GitHub 项目 ≥100 stars / 50 forks），本仓库当前 4 stars / 1 forks，如实提交必被拒。用户拍板：等知名度达标后再提，期间不建自建 bucket。本地工作已全部完成（artifacts/jvm.json 已验证），重启时直接从 implement.md 第 3 步（提 PR）开始。

> 顺序约束：**依赖 `10-31-goreleaser-migration` 完成**（manifest 的 URL/hash 指向稳定产线）；与 `10-31-winget-automation` 无依赖，可并行。父任务：`10-31-release-automation`。

## Goal

向 ScoopInstaller/Extras 提交一次性 manifest（`checkver`/`autoupdate` 字段齐全），之后每次发版的 manifest 更新由 Extras 官方 Excavator 自动跟进——发版侧**零 CI 成本、零后续动作**。

## 已确认事实

- 分发范围已拍板（2026-10-31）：**只提 Extras，不建自建 bucket**。被拒/搁置的 trade-off 用户已知情；若发生，回退路径为另行建自建 bucket（不在本任务树范围）。
- `bucket/jvm.json` 在 Extras 未被占用（2026-10-31 查重 404）。
- Extras 只能 PR 不能 push；manifest 带 `checkver`+`autoupdate` 后由官方 Excavator（每 4h）自动更新版本/URL/hash 并提交，无需发版侧介入。
- manifest 技术形态：
  - 消费**便携 zip**（不用 setup.exe，portable 惯例）；
  - `architecture` 映射：`64bit` → `jvm-windows-amd64.zip`、`arm64` → `jvm-windows-arm64.zip`；
  - `bin: jvm.exe`（scoop 自动 shim）；
  - `checkver: github`（BaixuanZhu/JVM）+ `autoupdate`：URL 拼接 release 资产直链，hash 从 `checksums.txt` 提取；
  - jvm 首次运行自举（写用户 PATH + shell 集成）在 scoop 场景照常工作，无需 manifest 特殊处理。

## Requirements

1. 编写 Extras 规范的 `jvm.json` manifest（含 `checkver`/`autoupdate`/`description`/`homepage`/`license` 等完整字段）；
2. PR 提交前本地验证：直接以本地 manifest 文件执行 `scoop install`（免 bucket）确认可装、hash 与 release 实际资产一致、`autoupdate` 演练（`scoop checkup`/手动 `scoop update` 验证 hash 提取）；
3. 向 ScoopInstaller/Extras 提交 PR，跟进 review 意见修改直至合并（人工环节，无法自动化）；
4. 合并后：README 安装章节补 scoop 方式（用户可见变更）；AGENTS.md 发布章节记录「scoop 分发已由 Extras Excavator 自动跟进，勿在 CI 加 scoop job」；CHANGELOG.md 记录。

## Acceptance Criteria

- [ ] manifest 通过 Extras 的 CI 校验（schema、hash、au 规范）；
- [ ] PR 合并进 ScoopInstaller/Extras master；
- [ ] `scoop install jvm`（Extras 为默认 bucket）安装成功，新终端 `jvm version` 可用；
- [ ] 下一次发版后 Excavator 自动更新 manifest（发版后 4h~1 天窗口内观察确认），`scoop update; scoop status` 检测到新版本、`scoop update jvm` 升级成功；
- [ ] README / AGENTS.md / CHANGELOG.md 已同步。

## Out of Scope

- 自建 bucket（仅作 Extras 被拒后的回退路径，另行立项）；
- WinGet（子任务 2）、Chocolatey 等其它渠道；
- 改动 jvm 运行时以适配 scoop（无证据需要）。

## Open Questions

- 无（分发范围已拍板）。
