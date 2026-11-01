# Implement: Scoop 分发接入（Extras）

> 轻量任务：**本仓库零代码改动**，交付物是 ScoopInstaller/Extras 仓库的一个 manifest + 一次性人工 PR + 文档同步。PRD-only 本已合规，此清单为执行便利。

## 前置条件

- [x] 子任务 1 完成，v0.16.0 资产产线稳定（2026-11-01 验证通过）——manifest 直接锚定 v0.16.0。

## 执行清单（有序）

1. [ ] 编写 `bucket/jvm.json`（Extras 规范）：
   - `version: 0.16.0`、`description`（英文一句话）、`homepage: https://github.com/BaixuanZhu/JVM`、`license`（以仓库 LICENSE 为准，MIT?）、`url`/`hash` 双架构：
     - `architecture.64bit.url = .../releases/download/v0.16.0/jvm-windows-amd64.zip`（hash 取 checksums.txt 对应行）；
     - `architecture.arm64.url = ...jvm-windows-arm64.zip`；
   - `bin: jvm.exe`（scoop shim）；`checkver: { github: BaixuanZhu/JVM }`；
   - `autoupdate`：`url` 拼接 `$version`；**hash 从 checksums.txt 提取**——`hash.url` 指 checksums.txt、`regex` 按 `$basename` 匹配行（scoop 支持的占位符以 ScoopInstaller/Scoop wiki 的 autoupdate 文档为准，实现时核对——本机有 scoop，可实测）；
   - Extras 有 manifest 风格惯例（字段顺序、注释）——提交前对照同类型新合入的 Go CLI manifest 参考两三例。
2. [ ] 本地验证（本机 scoop 在 G:\Scoop）：
   - `scoop install <本地 jvm.json 路径>`（免 bucket 直接装 manifest 文件）装成功；
   - 装的是 v0.16.0、`jvm version` 正常、shim `jvm` 解析正确（自举会往用户 PATH 写 scoop apps 目录——本来就是 scoop 惯例，无碍）；
   - `scoop checkup` 无相关报错；验证完 `scoop uninstall jvm` 清理。
3. [ ] 向 ScoopInstaller/Extras 提 PR（fork + bucket/jvm.json 单文件），跟进 review 至合并——异步，尽早提交。
4. [ ] 合并后文档同步：README 安装章节补 scoop 方式；AGENTS.md 发布章节记录「scoop 由 Extras Excavator 自动跟进，**勿在 CI 加 scoop job**」；CHANGELOG.md 记录。

## 验收对照

- Extras CI 校验通过 → PR 的 checks；
- `scoop install jvm`（Extras 默认 bucket）→ 合并后本机验；
- Excavator 自动更新 → 留给父任务集成验收窗口（下次发版后 4h~1 天观察）。

## 风险与回滚

- Extras 收录有 maintainer 裁量（知名度/发版稳定性）——被拒或长期搁置的回退 = 另行立项自建 bucket（prd.md 已拍板不在本任务范围）；
- 本仓库改动仅文档，回滚 = revert 文档提交。
