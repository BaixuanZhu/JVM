# 实施清单

前置: `python ./.trellis/scripts/task.py start 11-11-latestpatch-cache-unify` 后
才动手 (父任务地图: 本任务先行)。

## A. 缓存结构逐条化 (available_cache.go)

- [x] A1 `cachedRow` 去 `Latest`; 新增 `cachedPatch` + `Patches` map +
      `loadPatchCache` / `savePatchCache`; `mutateCache` 初始化补 nil map。
- [x] A2 `saveTableCache` 改存骨架行 (不再含 latest); 表测同步改造。
- [x] A3 表测: Patches 读写 / TTL 过期 (拨旧 saved_at) / arch 失效 /
      旧文件兼容 (无 Patches 且 Tables 带 latest → 全 miss、骨架仍命中)。
- 验证: `go test ./cmd/ -run 'TestAvailableCache|TestCache' -v` → 全绿。

## B. 共享查询层

- [x] B1 `patchOf(p, major, useCache)`: 读缓存 → miss `p.LatestPatch` →
      成功回填; 失败返回 error 不落盘。
- [x] B2 `queryLatestPatches(groups) ([]outdatedRow, allCached)`: 并发外壳
      (原 outdated.go 并发块平移), 内部 `patchOf`。
- 验证: `go vet ./cmd/` 通过; 新函数注释齐 (中文, 遵循注释规则)。

## C. 调用点收敛

- [x] C1 `availableTable`: 行 latest 改并发 `patchOf(..., !opts.Refresh)`;
      整表命中判据 = 骨架命中 + 全行 Patches 命中 (说明行行为不变);
      全部行成功才回填的旧判据废除 (骨架与 patch 分开落)。
- [x] C2 `Outdated`: 删复制块 → `queryLatestPatches`; allCached 时人类输出
      附缓存说明行; --json 不变。
- [x] C3 `Update` 单组: `LatestPatch` → `patchOf(p, major, true)`。
- [x] C4 `updateAll`: 删复制块 → `queryLatestPatches`; 分拣及以后零改动。
- 验证: `go test ./cmd/ -run 'TestAvailable|TestOutdated|TestUpdate' -v`。

## D. 网络计数测试 (fake provider)

- [x] D1 outdated 同组二次调用 TTL 内 `LatestPatch` 零调用;
      拨旧 saved_at 后重新调用且回填。
- [x] D2 update 单组在同场景下同样零网络 (走 patchOf)。
- 验证: `go test ./cmd/ -run 'TestOutdated.*Cache|TestUpdate.*Cache' -v`。

## E. 收尾 (review gate)

- [x] E1 `make fmt && make vet && make test` 全绿 (含 -race 抽查
      `go test -race ./cmd/`)。
- [x] E2 手测 dev build: available 两连击 (缓存说明行) / outdated 两连击 /
      update --all 幂等干跑; outdated --json 与改造前 diff 一致。
- [x] E3 AGENTS.md: available 缓存段落改为逐条粒度 + outdated/update 复用。
- [x] E4 CHANGELOG.md [Unreleased]: 变更 (outdated/update 复用查询缓存,
      二次查询零网络) 。
- [x] E5 提交: `feat(cmd): LatestPatch 查询统一为 distro@major 逐条缓存, outdated/update 复用`。

回滚点: A/B/C 各自可独立编译测试; 一个 commit 收口, revert 即回滚。
