# Implement: Alibaba Dragonwell provider

> 执行前读本目录 prd.md / design.md / research.md 与 `.trellis/spec/backend/version-parsing.md`、`query-cache.md`。

## 硬边界（并行协作约束）

- **只允许新建/修改**：`internal/provider/dragonwell/**`、`internal/app/version_test.go`（仅追加本任务新形态用例）。
- **禁止触碰**：`main.go`、`CHANGELOG.md`、`AGENTS.md`、`README*`、`docs/`、`internal/shell/*`、`scripts/*`、`internal/jdk/**`、其他 provider 目录。
- 注释与 CLI 文案中文；公共符号文档注释；不 commit。

## Checklist

1. [ ] `internal/provider/dragonwell/dragonwell.go`：
   - 常量（apiBase / repo 模板 / majors 表全 LTS / tag 前后缀 / 资产前后缀）+ GitHub release JSON 结构体
   - `checkArch()` aarch64 哨兵守卫
   - `fetchFiltered(major)`：`/releases?per_page=100` → tag 前后缀过滤 + 必须含合格 Windows 资产 + 进程内按 major 缓存（map，无 TTL）
   - tag → ReleaseName（`tag[len(prefix):strings.Index(tag, "_jdk-")]`；无 `_jdk-` 的已被过滤排除）
   - 资产挑选（前缀 + `_x64_windows.zip` 锚定后缀）；`.sha256.txt` 首列解析（第二列是内部路径，勿按名匹配）
   - 四入口（Available 静态 / ListVersions 降序 / LatestPatch 首条 / Resolve 双分支精确匹配）；`init()` 注册；`Configure` 只存 arch
2. [ ] `internal/provider/dragonwell/dragonwell_test.go`：design §6 全部表驱动用例（内嵌 fixture JSON）
3. [ ] `internal/app/version_test.go`：登记 `21.0.12.0.12+8` / `25.0.4.0.4+7` / `11.0.32.28` / `8.30.29`（仅追加）
4. [ ] 验证：`make fmt && make vet && make test && make test-race` 全绿
5. [ ] 用 `gh api repos/dragonwell-project/dragonwell21/releases?per_page=100` 抓真实 JSON 灌进解析函数对拍（11 线 `.9` build 段与 25 线 `-fix.zip` 两个陷阱样本必测）

## 验收对照

单测/编译项本任务内完成；CLI 全链路（available/install/use/exec/outdated/update）由 orchestrator 在收尾注册后验证。

## 回滚点

`git checkout -- internal/provider/dragonwell internal/app/version_test.go` 即回滚，无跨模块副作用。
