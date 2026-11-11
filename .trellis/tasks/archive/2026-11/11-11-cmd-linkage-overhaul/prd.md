# 命令面联动补全: LatestPatch 缓存统一 + available 已装标记 + 缓存清理联动 + 提示一致化

## Goal

2026-11-11 对 jvm 命令面做了一次"可联动未联动 / 可简单做复杂了"的审视, 发现的缺口
经用户逐项确认后立项收口。命令间既有闭环 (outdated→update 提示、uninstall 自动回退、
exec 未装提示 install、install 命中下载缓存) 不动, 本任务只补缺口:

1. LatestPatch 查询在 available/outdated/update 三命令四个调用点各自直查网络,
   available 的 10 分钟 TTL 缓存旁置不用; 且 outdated 与 update --all 存在整段
   同构的并发查询复制块 (做复杂了)。
2. available 只列远端可装版本, 不显示本地已装状态, 用户看不出自己落后 (sdkman
   用 `*` 标已装的先例)。
3. update 收敛到新 patch 后, 旧 patch 的缓存 zip 是语义上永不再用的死重;
   uninstall 删版本目录后对应缓存 zip 静默留存, 无提示也无法按单文件清理。
4. pin 不校验未装版本 (写完到 use 才失败); use 未装版本的错误提示与 exec/auto
   不一致 (后者给完整 install 命令, 前者只让用户去看 list)。

## 任务地图 (4 个子任务)

| 子任务 | 交付 | 优先级 |
|---|---|---|
| 11-11-latestpatch-cache-unify | 共享 LatestPatch 查询 + distro@major 逐条缓存, 四调用点复用 | P2, **先行** |
| 11-11-available-installed-mark | available 表格/分组/--json 标记本地已装 | P2, 在 unify 之后 |
| 11-11-cache-cleanup-on-update-uninstall | update 清旧 zip; uninstall 询问清缓存 | P3, 独立 |
| 11-11-pin-check-use-hint | pin 未装校验; use 未装提示统一 | P3, 独立 |

顺序约束 (非依赖系统, 子任务 prd 重复声明):

- latestpatch-cache-unify 先行: 动 availableTable/outdated/update 的公共查询路径
  与缓存结构, 是其余子任务的落点地基。
- available-installed-mark 在其后: 与 unify 同在 cmd.go 的 available 渲染区域叠改,
  串行做避免同文件冲突。
- cache-cleanup 与 pin-check 互相独立、与上两个也不重叠 (分别动 update 执行段 +
  Uninstall / Pin + Use), 可穿插或并行。

## 跨子任务验收

- [x] `make fmt` / `make vet` / `make test` 全绿 (每子任务收口时各自保证, 父任务
      归档前整仓再跑一遍 + `go test -race ./cmd/` 全绿, 2026-11-11)。
- [x] CHANGELOG.md `[Unreleased]` 记录全部用户可见变更: 缓存复用 (变更)、available
      已装列与 --json installed (新增)、uninstall 询问清缓存 (新增) + update 顺带
      清理 (变更)、pin/use 提示 (变更)。
- [x] AGENTS.md 同步: available 缓存描述 (逐条粒度 + outdated/update 复用 + 已装
      标记)、update/uninstall 缓存 zip 行为、internal/jdk 下载缓存段的命名约定
      双向指针。
- [x] scripts/integration-test.sh 评估并改造: 15 段重构为缓存 zip 生命周期闭环
      (uninstall -y 保留 → 重装命中缓存 → update 顺带清旧 zip 断言), 新增
      `echo y |` 交互式 uninstall 用例 (第二问读 EOF 默认 N → 保留提示)。
- [x] 最终集成审视: 子任务4 质检在 1-3 已合入 + 4 未提交的合并态上以真实二进制
      实跑 (pin/use/exec 三命令、stderr diff 逐字节比对), 即合并过检。

## 收口记录 (2026-11-11)

四个子任务全部实施-质检-提交-归档, feat 提交: 7e53c0c (LatestPatch 缓存统一) /
18e0aa2 (available 已装标记) / c5cb724 (缓存清理联动) / 250a392 (pin/use 提示统一)。
spec 沉淀: .trellis/spec/backend/query-cache.md (patchOf 唯一入口等 5 条硬性契约)。
质检共抓出 1 个 P0 (集成脚本 15 段前提失效) 已返工, 其余 P2 均已当场补齐或记录。

## 范围外 (明确不做)

- outdated 不加 `--refresh` flag: TTL 10 分钟复用已获批准, 需要新鲜数据走
  `available -r` 或等 TTL 过期, 不为低频场景加参数面。
- available 不做"落后"箭头/比较标记: 判断落后是 outdated 的职责, available 只
  客观展示已装与最新。
- 不改 exec/auto 已有的提示控制流 (use 只是向它们的既有格式对齐)。
- 缓存 zip 不做自动按龄淘汰 (`cache clean --older-than` 已覆盖)。
- CMD (cmd.exe) 的 shell 集成/自动切换仍不支持 (既有已知限制)。

## Notes

- 决策记录 (2026-11-11, 用户拍板):
  - outdated 在 10 分钟 TTL 内读缓存 —— 批准;
  - uninstall 缓存 zip 处理采用**交互询问** (y/N), `-y` 跳过询问时默认保留并打印
    提示 (保守值, 防脚本场景静默删大文件; 子任务 PRD 评审时可再确认);
  - update 清理旧 patch zip 直接删, 不询问 (死重无歧义)。
- 父任务不直接承载实现, 全部交付在四个子任务; 父任务负责源需求、地图、跨子验收
  与最终集成审视。
