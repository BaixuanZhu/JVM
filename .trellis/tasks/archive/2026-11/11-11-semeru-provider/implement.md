# Implement: IBM Semeru provider

> 执行前读本目录 prd.md / design.md / research.md 与 `.trellis/spec/backend/version-parsing.md`、`query-cache.md`。

## 硬边界（并行协作约束）

- **只允许新建/修改**：`internal/provider/semeru/**`、`internal/app/version_test.go`（仅追加本任务的新形态用例）。
- **禁止触碰**：`main.go`（注册 import 由收尾统一加）、`CHANGELOG.md`、`AGENTS.md`、`README*`、`docs/`、`internal/shell/*`（补全 token）、`scripts/*`、`internal/jdk/**`、其他 provider 目录。
- 注释与 CLI 文案中文；公共符号带文档注释；不 commit。

## Checklist

1. [ ] `internal/provider/semeru/semeru.go`：
   - 常量（apiBase / repo 模板 / 资产前后缀 / 活跃 major 表 + LTS 集）与 GitHub release JSON 结构体（tag_name / prerelease / assets[{name, browser_download_url}]）
   - `checkArch()` aarch64 哨兵守卫（corretto/graalvm 同款，报错文案建议 temurin/microsoft）
   - `Available()` 静态列表；`LatestPatch`（latest 端点 + 无资产回退列表）；`ListVersions`（per_page=100 降序）；`Resolve` 双分支 + 短形匹配（`_openj9-` 尾巴省略，唯一命中才接受）
   - tag → ReleaseName（去 `jdk-` 前缀；8 线 `jdk8u` 去 `jdk`）；GA 过滤（prerelease + 尾缀黑名单）；资产挑选（前缀+后缀）；`.sha256.txt` 双列解析
   - `init()` 注册；`Configure(arch, mirror)` 只存 arch
2. [ ] `internal/provider/semeru/semeru_test.go`：design §7 全部表驱动用例（fixture 用 `[]byte` 内嵌 JSON，不引网络）
3. [ ] `internal/app/version_test.go`：注册表追加 `21.0.13.0` / `21.0.5+11_openj9-0.48.0` / `8u402-b06_openj9-0.43.0` / `8.0.492.0`（仅追加行，勿动他人用例）
4. [ ] 验证：`make fmt && make vet && make test && make test-race`（在仓库根目录跑，全绿才算完）
5. [ ] 开发机人工冒烟（不写注册无法用 CLI，可写临时 main 或用 `go run` 小驱动验证解析函数；全链路冒烟归收尾）：至少用 `gh api` 抓一份真实 release JSON 灌进解析函数对拍一遍

## 验收对照

prd.md Acceptance Criteria 中单测/编译项在本任务内完成；CLI 全链路项（available/install/use/exec/outdated/update）在收尾注册后由 orchestrator 验证。

## 回滚点

单包自包含，`git checkout -- internal/provider/semeru internal/app/version_test.go` 即回滚，无跨模块副作用。
