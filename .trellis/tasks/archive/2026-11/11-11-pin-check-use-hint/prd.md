# pin 未装校验提示 + use 未装错误提示统一

## Goal

两条提示一致性问题:

- (a) `jvm pin <版本>` 未装也静默写入 `.jvmrc`, 用户到 `use` 无参才失败, 多一轮
  往返 —— pin 时 ListLocal 就在手边。
- (b) 未装版本的错误提示三处不一致: `exec` (exec.go L89/L95) 与自动切换
  (auto.go resolveRcVersion) 给完整可复制的 `jvm install <distro>@<版本>` 命令;
  `use` 显式参数路径 (ResolveVersion 错误直出, 只让用户"运行 jvm list 查看") 与
  `use` 无参外来 rc 宽松路径 (resolveVersionLoose 错误直出) 都不给。

## Requirements

### R1 pin 有参时校验未装

- `Pin(arg)` 有参数时 (无参取 current 必已装, 不校验) 用 `ResolveVersion`
  (与 use 同规则: 纯大版本取组内最新 / 完整版本精确) 校验本地已装。
- 未装: 打印**警告行不阻断写入** —— 钉一个待装版本是合法场景 (CI 先 pin 后
  install), 警告内容含失败预告与安装命令, 如:
  `注意: corretto@21 未安装, .jvmrc 生效前需先 jvm install corretto@21`。
- 已装: 静默 (不加成功噪音)。

### R2 use 显式参数路径补提示

- `Use` 的 `ResolveVersion` 失败分支: 错误信息追加一行
  `先安装: jvm install <distro>@<version>` (原始输入回显), 与 exec 格式一致。

### R3 use 无参 loose 路径补提示

- `resolveVersionLoose` 失败分支 (Use 调用侧) 同样追加先安装提示, 对齐 exec 的
  loose 分支 (exec.go L89)。

### R4 提示模板统一

- 抽 cmd 包公共 helper (如 `installHint(distro, version string) string`),
  use/exec/auto 三处共用同一模板, 消灭各自手拼。

## 约束

- 不改 `internal/junction.ResolveVersion` 本体错误信息 (多消费方, junction 包
  不动)。
- 不改 auto 的控制流 (其提示已合规, 仅作为对齐基准换用公共 helper)。
- exec 的既有提示格式即目标格式 (用户已见过的文案, 不再造新格式)。
- Use 显式路径中"还没有安装任何版本"的既有提示已含 install 引导, 保持原样,
  只补"装了其他版本但没装这个"的分支。

## Acceptance Criteria

- [ ] pin 未装版本: `.jvmrc` 写入成功 + 警告行含 install 命令; 已装: 无警告。
- [ ] use 显式参数未装: 错误含 `先安装: jvm install ...` 且与 exec 同款格式。
- [ ] use 无参外来 rc (如 `.tool-versions` 半截版本) 宽松匹配失败: 同样含提示。
- [ ] exec/auto 输出与改造前一致 (只换 helper, 文案不变)。
- [ ] 表测: pin 校验两分支、三处错误信息断言、helper 模板; `make fmt` /
      `make vet` / `make test` 全绿; CHANGELOG (文案改进)。

## Notes

- PRD-only (轻量): 全部为提示层改动, 无控制流变化。
- 与其他子任务零代码重叠 (动 cmd.go 的 Pin/Use 与 rc.go), 可独立实施。
