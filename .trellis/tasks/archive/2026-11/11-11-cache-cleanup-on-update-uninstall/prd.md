# update 清理旧 patch 缓存 zip + uninstall 询问是否清缓存

## Goal

下载缓存 `{dataRoot}/cache` 的 zip 与版本目录生命周期脱节: update 收敛到新 patch
后, 旧 patch 的 zip 是语义上永不再用的死重 (update 语义下旧版不会再装), 却一直
占着数百 MB; uninstall 删版本目录后对应 zip 静默留存, 用户不知情, 且 `cache clean`
只有全清/按龄两档, 无法按单文件清理。

## Requirements

### R1 update 顺带清理旧 zip (直接删, 不询问)

- `execUpdatePlan` (cmd/update.go, 单组与 --all 共用) 删除旧版本目录时, 顺带
  删除对应缓存 zip `{CacheDir}/{目录名}.zip`。
- zip 删除失败宽容继续 (与目录删除同款"可能被进程占用"宽容), 汇总提示指向
  `jvm cache clean` 手动清理; 不影响升级成败判定。
- 新版 zip 天然留存 (下载即落缓存), 重装/回退新 patch 仍免下载。

### R2 uninstall 询问是否清缓存 (交互式, 用户指定)

- `Uninstall` 单版本与 `uninstallGroup` (--all) 删除版本目录完成后, 若存在对应
  缓存 zip:
  - 单版本: 询问 `是否同时删除缓存 zip? (temurin-21.0.5+11.zip, 182.3 MB) [y/N]`;
  - 组删除: 合并为**一次**询问, 列出全部 zip 与合计大小;
  - 答 y → 逐个删除 (失败宽容 + 提示); 答 N → 走 R4 保留提示。
- zip 不存在时: 不询问、不提示 (无噪音)。

### R3 -y 脚本路径: 跳过询问, 默认保留

- `-y/--yes` 跳过询问且**默认保留** zip, 打印 R4 保留提示 —— 脚本场景不静默删
  大文件 (保守默认; 评审如认为 -y 应连删, 改此处即可, 单点)。

### R4 保留分支提示

- 保留时打印一行: `安装包缓存已保留 (N 个, X MB), 重装免下载; jvm cache clean 可清理`
  —— 用户明确知道缓存还在、多大、怎么清。

## 约束

- 不改 `cache clean` 既有行为/参数 (全清 + `--older-than` 两档保持)。
- zip 路径拼装若 internal/jdk 已有 helper (cacheHit 同款约定) 优先复用, 避免
  目录名→文件名约定两处维护; 无则抽 cmd 包小 helper。
- uninstall 的询问发生在**目录删除成功之后** (先问后删会让取消语义混乱: 版本
  目录要删是已确认过的, 缓存去留是独立决定)。
- `jvm install <distro@版本> <zip>` 本地包安装不拷贝进缓存 (维持现状, 双占磁盘
  无意义, 不在本任务重开)。

## Acceptance Criteria

- [ ] update 升级后旧 patch 的目录与对应 zip 一并消失; zip 删除失败不影响升级
      结果与汇总语义。
- [ ] uninstall 存在缓存 zip 时出现 y/N 询问: y → zip 删除; N → 保留 + 提示。
- [ ] uninstall `-y` 不询问, 默认保留 + 打印提示。
- [ ] `--all` 组卸载一次询问覆盖多个 zip (列出清单与合计)。
- [ ] zip 不存在时 uninstall 行为与现状一致 (无询问无提示)。
- [ ] 表测 (临时目录): zip 存在性分支、删除调用、保留提示文案、-y 默认;
      `make fmt` / `make vet` / `make test` 全绿; CHANGELOG 记录。

## Notes

- 决策记录 (2026-11-11, 用户): uninstall 采用**询问式**清理; update 死重 zip
  直接删; `-y` 默认保留为本 PRD 选择的保守值, 评审时可定夺。
- 与 latestpatch-cache-unify 无代码重叠 (本任务动 execUpdatePlan 执行段与
  Uninstall/uninstallGroup, 对方动查询段), 但都在 update.go, 建议串行提交。
