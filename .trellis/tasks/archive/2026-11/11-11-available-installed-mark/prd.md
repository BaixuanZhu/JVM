# available 标记本地已装版本 (表格/分组/--json)

## Goal

`jvm available` 只列远端可装版本, 用户看不出"哪些已装、装的是哪个 patch" —— 得
知道再跑 outdated。本地数据现成 (`junction.ListLocal` 一把), 展示层叠加即可让
available 直接暴露升级信息 (sdkman `sdk list java` 用 `*` 标已装的先例)。

## Requirements

### R1 表格形态: 增加"已装"列

- 三列 (大版本/最新版本/类型) 扩为四列, 新增**已装**列: 该 (distro, major)
  本地语义最新的已装版本号 (如 `21.0.5+11`), 未装显示 `-`。
- 列宽按 `runeWidth` 计算的既有对齐逻辑沿用; 已装版本号均为 ASCII, 不引入新的
  宽度问题。

### R2 分组形态 (-a / --major): 已装子版本行标记

- 已装的子版本行加 `(已装)` 后缀; 与 `← 最新` 标记共存, 最新且已装时两者并列
  (如 `21.0.12+8  ← 最新 (已装)`)。

### R3 --json 增字段 (追加, 不破坏既有字段)

- `rowJSON` 增 `installed string`: 该 (distro, major) 本地最新已装版本号,
  未装为空串。
- `groupJSON` 增 `installed []string`: 该组已装子版本列表 (降序, 无则 `[]`)。
- 消费方可据此判断"已装的是什么、列表里哪些已装"。

### R4 本地信息不进缓存

- 缓存条目仍为纯远端数据 (Patches/Tables/Groups 均不含本地信息); 缓存命中
  路径在渲染时运行时 `ListLocal` 叠加标记 —— 本地装/卸即时反映, 不受 TTL 影响。

## 约束

- 顺序: 在 latestpatch-cache-unify 合入后实施 (同在 cmd.go 的 available
  渲染区域叠改, 串行避免冲突; 父任务地图已声明)。
- 不做"落后"箭头/版本比较标记 (outdated 职责, 父任务范围外)。
- 旧无前缀目录 (无 distro 前缀) 由 `SplitDistro` 归为 temurin, 与 `list`
  展示口径一致。
- distro 无任何已装版本时该 distro 全表无已装标记 (跨 distro 不显示)。

## Acceptance Criteria

- [ ] 表格: 已装 major 行"已装"列显示本地最新版本号, 未装行显示 `-`, 四列对齐。
- [ ] 分组: 已装子版本行带 `(已装)` 标记, 与 `← 最新` 并存正确。
- [ ] `--json`: 两形态新字段存在且值正确 (`json.Unmarshal` 断言, 含未装空值
      规范化: installed 为 `""` / `[]` 而非 null)。
- [ ] 缓存命中路径同样显示已装标记 (表测: 先查询落缓存, 改动本地目录后缓存
      命中路径标记随之变化)。
- [ ] `make fmt` / `make vet` / `make test` 全绿; CHANGELOG 记录 (含 --json
      字段新增)。

## Notes

- PRD-only (轻量): 展示层叠加, 无控制流/缓存结构变化。
- 实现提示: `ListLocal` 一次 → 按 (distro, major) 建映射 (组内首个即最新,
  降序输入), 表格/分组/--json 三渲染共用该映射。
