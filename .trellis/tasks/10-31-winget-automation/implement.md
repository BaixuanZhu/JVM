# Implement: WinGet 自动提交

> 轻量偏中任务：唯一代码改动是 release workflow 加一个 winget job；其余是一次性人工环节（manifest 首提 PR、PAT 配置）。无 design.md——技术形态已全部定死在 prd.md（双 installer manifest / user scope / winget-releaser / continue-on-error 语义）。

## 前置条件（依赖顺序）

- [x] 子任务 1（goreleaser-migration）完成，v0.16.0 资产产线稳定（2026-11-01 首次真实发版验证通过）
- [ ] **用户外部动作**：创建 classic PAT（public_repo scope）存为 repo secret `WINGET_TOKEN`——没有它 winget job 无法实现，但**不阻塞 manifest 首提**（首提只要 GitHub 网页 PR）

## 执行清单（有序）

1. [x] 首提 manifest（已完成，artifacts/winget-pkgs/ 下三文件多文件格式——**单文件 singleton 的 Installers maxItems=1 装不下双架构，winget-pkgs 也只收多文件**；winget validate 绿、sha256 本地实算、URL 200、License MIT 已查证；提交 PR 时去掉中文注释减少 review 摩擦）：
   - 版本锚定 **v0.16.0**（已是 latest，资产 URL 稳定）；
   - 两个 Installer 条目：x64/arm64、`InstallerType: nullsoft`、`Scope: user`、`InstallModes: silent`（NSIS `/S`）；
   - InstallerUrl = `https://github.com/BaixuanZhu/JVM/releases/download/v0.16.0/jvm-windows-{amd64,arm64}-setup.exe`；
   - **InstallerSha256 注意：setup.exe 不在 checksums.txt 里**（那里只有两个 zip）——从 release 下载 setup.exe ×2 本地 `sha256sum` 计算；
   - ReleaseNotesUrl 指向 Release；Publisher/PublisherUrl/Author/Description/Homepage/License（MIT？以仓库 LICENSE 为准）补齐；
   - 用 `winget validate` / `winget-pkgs` 的 CI 校验器本地过一遍（本机有 winget 可直接验）。
2. [ ] 人工提 New Package PR 到 microsoft/winget-pkgs（模板 Step 列表如实勾选），跟进 review 至合并——**异步长杆项，尽早提交**，等待期并行做第 3 步。
3. [ ] release workflow 加 winget job（在 `.github/workflows/release.yml` 或独立 `winget.yml`，倾向独立文件——主发版 workflow 保持单一职责）：
   - 触发 `on: release published`（types: [published]），与 tag 主流程解耦；
   - `vedantmgoyal2009/winget-releaser` action，参数：identifier `BaixuanZhu.JVM`、token `${{ secrets.WINGET_TOKEN }}`、双架构（action 需要跑在能访问 installer 的 runner 上，windows-latest 稳妥）；
   - `continue-on-error: true` + 失败时 GitHub Actions 自动邮件告警——主发版不可被 winget 拖挂（验收第 4 条）；
   - 首提 PR 未合并前该 job 会失败（manifest 不存在）——**job 先落地但加注释标明生效条件，或等合并后再提交**（实现时按首提进度定，倾向后者：避免每次发版固定红一个 job）。
4. [ ] AGENTS.md 发布章节：winget 分发说明（PAT 的 scope/轮换、winget-releaser 失败不影响主发版）；README 安装章节补 `winget install BaixuanZhu.JVM`；CHANGELOG.md 记录。

## 验收对照（prd.md Acceptance Criteria 的验证路径）

- 首提合并 + `winget search` → 本机即可验；
- `winget install BaixuanZhu.JVM` 静默装 + `jvm version` → 本机（注意装的是 release 版，自举语义正常触发；装完 `winget uninstall` 清理）；
- 下次发版自动 PR → 留给父任务集成验收窗口（v0.17.0）；
- winget job 失败不影响主发版 → continue-on-error 语义 + 代码 review。

## 风险与回滚

- winget-pkgs review 可能要求改 Publisher/命名/静默开关细节——manifest 是外部仓库文件，改动不影响本仓库；本仓库回滚 = 删 winget job。
- PAT 泄露面：public_repo scope 最小化，只存 repo secret，轮换 = GitHub 设置重新生成 + 更新 secret。
