# LatestPatch 查询统一与 distro@major 逐条缓存, outdated/update 复用

## Goal

LatestPatch(distro, major) 的结果被四个调用点各自直查网络, available 的 10 分钟
TTL 缓存 (整表粒度) 旁置不用; 且 outdated 与 update --all 存在整段同构的并发查询
复制块。本任务抽共享查询层 + 把缓存粒度改为 distro@major 逐条, 四个调用点读写
同一份缓存, 同时消除复制块。

## Requirements

### R1 消除复制块: 共享查询函数

- cmd 包新增共享查询层, 替换两处整段同构的并发查询块:
  - `cmd/outdated.go` Outdated() 内的并发块 (~L76-92);
  - `cmd/update.go` updateAll() 内的并发块 (~L140-156, 注释自承"与 Outdated 同款")。
- `groupInstalled` / `outdatedRow` 已共享, 查询层同样放 cmd 包, 输入
  `[]installedGroup`, 输出 `[]outdatedRow` (miss 组并发直查的既有行为不变)。

### R2 缓存逐条化: availableCache 结构改造

- `cmd/available_cache.go`:
  - `Tables` 条目保留 `Available()` 产物 (major + LTS 清单, 表格形态的骨架),
    `cachedRow` 去掉 `latest` 字段;
  - 新增 `Patches map[string]cachedPatch`, key `"distro@major"`, 值为
    `{latest, saved_at}` —— LatestPatch 结果的逐条条目, 与分组缓存 `Groups`
    (ListVersions 产物) 同键风格、各自独立 map。
- 旧缓存文件 (含 `Tables.latest` 的历史结构): 新代码 unmarshal 后 `Patches`
  为空 → 全部当 miss 重查回填, **不写迁移代码**。

### R3 四个调用点统一读写缓存

- 共享查询层内部: 逐组读 `Patches` 条目 (arch 键校验 + TTL 内命中) → miss 组
  并发 `p.LatestPatch` → 成功逐条回填, failed 不落盘。
- 各调用点:
  - `availableTable` (cmd/cmd.go): 每 major 的 latest 查询改走同一逐条路径;
    `-r/--refresh` 语义保持 (绕过缓存直查);
  - `Outdated`: 删复制块, 改调共享层 (**TTL 内读缓存**, 用户 2026-11-11 批准);
  - `Update` 单组: 执行前 `p.LatestPatch(major)` 改走逐条缓存路径 (TTL 内读);
  - `updateAll`: 删复制块, 改调共享层。

### R4 输出行为

- outdated 人类输出: 全部组命中缓存 (本次零网络请求) 时附一行缓存说明
  (对齐 available 的 `cacheNoticeLine` 风格); 部分命中不附 (避免噪音)。
- `--json` 路径不受影响: stdout 纯净、DTO 结构不变 (outdated/available 均无
  新字段)。

### R5 文档同步

- AGENTS.md: `available` 缓存描述改为逐条粒度 + outdated/update 复用的事实。

## 约束

- arch 参与键校验保留 (x64/aarch64 版本集合不同, 整体失效); failed 不落盘;
  缓存读写任何失败静默降级直查 (缓存只加速不拦截) —— 三条既有原则不动。
- `Groups` (ListVersions 分组缓存) 不动。
- provider 接口零改动 (`LatestPatch` 签名不变)。
- 兼容性: 新缓存文件被旧版本 jvm 读 → 旧代码找不到 `Tables.latest` 当 miss
  重查, 降级安全 (与旧文件读新代码对称)。

## Acceptance Criteria

- [ ] cmd 包内不再存在 outdated 与 update --all 两处同构并发查询块 (单一实现,
      `grep -c "LatestPatch" cmd/*.go` 调用点收敛到共享层内部)。
- [ ] fake provider 计数单测: outdated 对同组二次调用 TTL 内零 `LatestPatch`
      网络请求; TTL 过期 / 换 arch 后重新查询并回填。
- [ ] available 表格行为不回归: 首查并发 + 成功才回填、二次命中打缓存说明行、
      `-r` 绕过 (既有表测适配新缓存结构后全绿)。
- [ ] update 单组 / --all 走缓存路径, 既有 `planUpdate`/`buildUpdatePlan`/
      `execUpdatePlan` 测试不回归。
- [ ] 旧格式缓存文件存在时不报错, 全 miss 重查回填。
- [ ] outdated --json 输出与改造前逐字节等价 (DTO 不变)。
- [ ] `make fmt` / `make vet` / `make test` 全绿; AGENTS.md 同步。

## Notes

- 复杂任务: 技术设计见 design.md, 执行清单见 implement.md。
- 顺序: 本任务在父任务地图中**先行**, available-installed-mark 在其后。
- 决策依据: TTL 10 分钟对季度 patch 场景足够安全 (available 缓存注释已论证
  "照缓存装"都可用, "照缓存升级检查"风险更低)。
