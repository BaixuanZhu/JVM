# Implement: Tencent Kona provider

> 执行前读本目录 prd.md / design.md / research.md 与 `.trellis/spec/backend/version-parsing.md`、`query-cache.md`。

## 硬边界（并行协作约束）

- **只允许新建/修改**：`internal/provider/kona/**`、`internal/jdk/jdk.go` 与 `internal/jdk/jdk_test.go`（仅 fileHash 的 md5 分支）、`internal/app/version_test.go`（仅追加本任务新形态用例）。
- **禁止触碰**：`main.go`、`CHANGELOG.md`、`AGENTS.md`、`README*`、`docs/`、`internal/shell/*`、`scripts/*`、其他 provider 目录。
- 注释与 CLI 文案中文；公共符号文档注释；不 commit。

## Checklist

1. [ ] `internal/jdk/jdk.go`：`fileHash` 加 `case "md5", "md5-sum"` → `md5.New()`（import `crypto/md5`），函数注释更新为「sha256（默认）/ sha1 / md5——按厂商供给取最强」
2. [ ] `internal/jdk/jdk_test.go`：fileHash md5 用例（临时文件写已知内容，断言 md5 hex）
3. [ ] `internal/provider/kona/kona.go`：
   - 常量（apiBase / repo 模板 / majors 全 LTS / 索引 URL / 资产后缀）+ GitHub release 与 kona-v1.json 结构体
   - `checkArch()` aarch64 哨兵守卫
   - `fetchFiltered(major)`：`/releases?per_page=100` → 三套模板解析 + fiber/vector/单段排除 + 必须含合格资产 + 进程内缓存
   - 资产挑选（后缀 `jdk_windows-x86_64_signed.zip` 且不含 fiber）
   - 校验路由 `resolveChecksum(release, asset)`：kona-v1.json（sync.Once，失败记 miss）命中 → sha256；否则 `.md5` 旁路（BSD 正则）→ md5；旁路缺失 → error
   - 四入口（Available 静态 / ListVersions 降序 / LatestPatch 首条 / Resolve 双分支精确匹配 + 校验和填充）；`init()` 注册；`Configure` 只存 arch
4. [ ] `internal/provider/kona/kona_test.go`：design §6 全部表驱动用例（内嵌 fixture JSON）
5. [ ] `internal/app/version_test.go`：登记 `21.0.13.0.3` / `17.0.4.1` / `11.0.33` / `8.0.28`（仅追加）
6. [ ] 验证：`make fmt && make vet && make test && make test-race` 全绿
7. [ ] 用 `gh api repos/Tencent/TencentKona-21/releases?per_page=100` 抓真实 JSON 灌进解析函数对拍（`TencentKona-21-ga` 单段、17.0.4.1 的 `.b2`、fiber 期三个陷阱样本必测）；`curl -sL https://tencent.github.io/konajdk/releases/kona-v1.json` 拉真实索引验证匹配结构

## 验收对照

单测/编译项本任务内完成；CLI 全链路（含真实 SHA256/MD5 两分支安装）由 orchestrator 在收尾注册后验证。

## 回滚点

`git checkout -- internal/provider/kona internal/jdk internal/app/version_test.go` 即回滚。
