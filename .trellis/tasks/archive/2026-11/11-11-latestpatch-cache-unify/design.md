# LatestPatch 查询统一与逐条缓存 — 技术设计

## 背景与动机

四个调用点、同一数据、三种行为 (cmd 包, 2026-11-11 现状):

| 调用点 | 位置 | LatestPatch 来源 | 缓存 |
|---|---|---|---|
| available 表格 | cmd.go availableTable | 并发 `p.LatestPatch(r.Major)` | 整表读写 |
| outdated | outdated.go Outdated | 并发 `p.LatestPatch(g.major)` | 无 |
| update 单组 | update.go Update | 单次 `p.LatestPatch(major)` | 无 |
| update --all | update.go updateAll | 并发 (复制块) | 无 |

典型流 `available → outdated → update --all` 同一批查询打 3 遍网络。整表粒度
(TTL 内全有或全无) 恰好让只要几个组的 outdated 无法复用, 是结构性的粒度错配。

## 设计

### 1. 缓存结构 (available_cache.go)

```go
type availableCache struct {
    Arch    string                  `json:"arch"`
    Tables  map[string]cachedTable  `json:"tables"`  // key: distro; 只存 Available() 骨架
    Patches map[string]cachedPatch  `json:"patches"` // key: "distro@major"; 新增
    Groups  map[string]cachedGroup  `json:"groups"`  // 不动
}

// cachedRow 去掉 Latest, 只剩骨架 (major + LTS)
type cachedRow struct {
    Major int  `json:"major"`
    LTS   bool `json:"lts"`
}

type cachedPatch struct {
    SavedAt string `json:"saved_at"`
    Latest  string `json:"latest"`
}
```

- 新增 `loadPatchCache(distro, major) (string, bool)` / `savePatchCache(distro,
  major, latest)`, 与 loadTableCache/saveTableCache 同款 TTL + arch 判定
  (cacheFresh / cacheArchKey 复用)。
- `groupCacheKey` 已拼 `"distro@major"`, Patches 与 Groups 共用该拼 key 函数。
- `mutateCache` 初始化补 `Patches` 非 nil map。

### 2. 共享查询层: 两层结构 (推荐 b 案)

可选方案:
- a) 一个 `queryLatestPatches(groups) []outdatedRow` 通吃: availableTable 需把
  `[]app.Release` 转成 `[]installedGroup` 再把结果转回 `[]availableRow`, 两头
  互转别扭;
- b) **两层**: 单条层 `patchOf(distro, major string/int, useCache bool) (string, error)`
  负责读缓存→miss 直查→回填 (缓存的唯一交汇点); 并发外壳两处各自组装行类型。

采用 b:

```go
// available_cache.go 或 outdated.go:
// patchOf 返回该 (distro, major) 的最新 patch 版本号; useCache=false 绕过
// (available -r)。查询失败返回 error (failed 语义由调用方落到行上)。
func patchOf(p provider.Provider, major int, useCache bool) (string, error)

// outdated.go:
// queryLatestPatches 并发检查全部组 (读 Patches 缓存, miss 组直查并回填),
// 返回行序与输入一致; 全部命中缓存时第二返回值为 true (供调用方打说明行)。
func queryLatestPatches(groups []installedGroup) (rows []outdatedRow, allCached bool)
```

`queryLatestPatches` 就是 Outdated/updateAll 现在的并发块收敛 (outdatedRow 输出),
内部并发调 `patchOf`。availableTable 不用它 (行类型不同), 直接并发调 `patchOf`。

### 3. 各调用点改造

- **availableTable** (cmd.go):
  - `Available()` 结果 (major+LTS) 仍整表缓存: `saveTableCache` 存骨架行
    (全部 LatestPatch 成功才落缓存的原判据改为: 骨架表照存, Patches 逐条按
    成败落 —— 骨架与 patch 是两类数据, 骨架无 failed 概念);
  - 行内 latest 改为并发 `patchOf(p, r.Major, !opts.Refresh)`, 失败行
    `failed=true` 照旧;
  - 缓存命中判据变化: 原整表命中 → 现在"骨架表命中 + 全部行 Patches 命中"
    才打 `cacheNoticeLine` 并跳过网络; 任一 miss 走查询路径 (行为与原先
    "整表 TTL"对用户等价: 10 分钟内二次查询同样零网络)。
- **Outdated** (outdated.go): 删并发块 → `queryLatestPatches(groups)`;
  `allCached` 时人类输出末尾附缓存说明行; `--json` 忽略 allCached。
- **Update** 单组 (update.go): `p.LatestPatch(major)` → `patchOf(p, major, true)`;
  错误文案不变。
- **updateAll** (update.go): 删并发块 → `queryLatestPatches(groups)`; 后续
  分拣/计划/执行零改动。

### 4. 行为变化面 (对外)

- outdated: TTL 内可能读缓存 (批准); 人类输出多一行缓存说明 (仅全命中时);
  `--json` 不变。
- update: latest 可能来自 TTL 内缓存 —— 与 available 同级风险, 可接受。
- available 表格: 语义等价 (整表 TTL → 骨架 + 逐行混合 TTL, 用户视角相同);
  输出不变。

### 5. 明确不做

- 旧 `Tables.latest` 数据迁移 (自然 miss 重查, 一次代价)。
- outdated `--refresh` flag (父任务范围外已定)。
- Groups 分组缓存、provider 接口、`--json` DTO: 零改动。

## 兼容性

- 旧缓存文件 → 新代码: `Patches` 空 map, 全 miss, 重查回填; `Tables` 里多余的
  `latest` 字段被 json.Unmarshal 忽略 (Go 忽略未知字段), 骨架照常命中。
- 新缓存文件 → 旧代码: `cachedRow` 缺 `latest` → 旧代码当空串, 表格全 N/A,
  整表判据不落盘 → 下次重查自愈。降级安全 (仅在用户降级 jvm 版本时短暂出现)。
- 网络协议、注册表、junction: 零接触。

## 测试策略

- `available_cache_test.go`: 改造既有表测 + 新增 Patches 读写 / TTL 过期 /
  arch 失效 / 旧文件 (无 Patches、Tables 带 latest) 全 miss 断言。
- `outdated_test.go`: fake provider 计数器 (Available/LatestPatch 调用次数) ——
  同组二次调用 TTL 内 `LatestPatch` 零调用; 拨旧 `saved_at` 后重查;
  allCached 说明行出现/不出现。
- `cmd_test.go` availableTable 既有缓存路径断言适配。
- `update_test.go`: 既有 plan 测试不动即过 (LatestPatch mock 走 patchOf 后
  仍被调用); updateAll 路径 smoke。
- 手测 (dev build): `available temurin` 两次 (第二次缓存说明行)、`outdated`
  两次、`update --all` 干跑确认提示。

## 回滚

单人直提 main, 本任务一个 commit 收口; 出问题 `git revert` 单 commit 即回滚,
缓存文件多出的 `patches` 字段对旧代码无害 (见兼容性)。
