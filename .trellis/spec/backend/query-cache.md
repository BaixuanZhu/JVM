# Query Cache Guidelines

> `~/.jvm/available-cache.json`（cmd/available_cache.go）的缓存契约。
> 2026-11-11 子任务 latestpatch-cache-unify 建立：LatestPatch 查询统一入口 + 逐条缓存。

---

## 为什么有这份规范

LatestPatch(distro, major) 曾被 available/outdated/update 四个调用点各自直查网络，
且 outdated 与 update --all 存在整段同构复制块。收敛后形成以下**必须维持**的契约，
违反任意一条都会重新制造"多份实现互相漂移"或缓存错位 bug。

## 硬性契约

1. **LatestPatch 一律经 `cmd.patchOf(p, major, useCache)`**（available_cache.go）。
   业务代码禁止绕过它直调 `p.LatestPatch`（仓内 grep `\.LatestPatch(` 的合法位置
   仅 patchOf 内部与 provider 包自身的接口实现/内部复用）。
   - useCache=false 表示跳过读（available `-r` / queryLatestPatches 已串行预读过），
     **回填照做**——刷新后缓存正是最新结果，不回填会把陈旧条目留在原地。
2. **缓存 key 恒为 `provider.Name()@major`**。目录名解析出的 distro 与
   `provider.Name()` 一一对应（registry 按 Name 索引；旧无前缀目录归 temurin；
   temurin-ea 靠 SplitDistro 贪心拆分）。新增 provider 时必须保持
   `Get(distro).Name() == distro`，否则读键（queryLatestPatches 用目录 distro）与
   写键（patchOf 用 Name()）错位。
3. **缓存文件三类条目、互不混淆**：
   - `Tables[distro]`：表格**骨架**（Available 的 major+LTS 清单），整表存取，
     无 failed 概念；
   - `Patches["distro@major"]`：LatestPatch 逐条结果（表格行与 outdated/update 共用）；
   - `Groups["distro@major"]`：ListVersions 全量子版本（availableGroups 专用）。
   每条目独立时间戳（RFC3339），TTL 10 分钟，**failed 不落盘**，arch 不匹配整体失效
   （mirror 不参与键）。
4. **并发**：`mutateCache` 的读-改-写临界区由 `cacheMu` 串行化（逐条化后多个
   goroutine 并发回填会互相覆盖）；读路径 `loadCacheFile` 无锁，撕裂 JSON 靠
   Unmarshal 失败→零值→miss 自愈——"缓存只加速，绝不拦截"。
5. **缓存里禁止放本地状态**（ListLocal 结果是运行时旁路）。available 已装标记等
   本地叠加一律渲染时现算，否则本地装/卸会被 TTL 固化成脏数据。

## 兼容性约定

- 缓存结构演进靠"旧读新/新读旧都降级为 miss"保证安全：json.Unmarshal 忽略未知
  字段、缺字段当空 map。**不写迁移代码**，不为旧格式做特判。
- `--json` 输出路径与缓存无关（DTO 永不包含缓存来源信息）；缓存说明行
  （cacheNoticeLine）只出现在人类输出，且仅在**零网络请求**（全部条目命中）时附。

## 已定决策（勿重开）

- outdated/update 读 TTL 缓存（2026-11-11 用户批准）；**不给 outdated 加
  `--refresh`**——需要新鲜数据走 `available -r` 或等 TTL 过期。
- availableTable 整表命中判据 = 骨架命中 + 全行 Patches 命中；任一 miss 时骨架
  重取轻量的 Available、patch 由 patchOf 增量复用。
