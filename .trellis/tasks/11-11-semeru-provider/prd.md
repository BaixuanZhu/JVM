# 接入 IBM Semeru provider（OpenJ9）

## Goal

新增 `semeru` 发行版 provider：IBM Semeru（OpenJDK 类库 + Eclipse OpenJ9 虚拟机），数据源为 IBM 官方 `ibmruntimes/semeru*-binaries` GitHub releases。仅 Windows x64。

## 已知事实（2026-11-12 子代理调研实测，详见本目录 research.md）

- **仓库布局：每个大版本一个 binaries 仓库**，组织内共 14 个 open GA 线仓（`semeru8/11/16~25/26/27-binaries`；另有 `-certified` / `-ea` 变体仓，本次**只接 open GA**）。26 仓已存在；27 仓也已建（27 非 LTS，LTS 节奏 21→25→29）。
- **tag/资产命名 2026-04 CPU 起切换新格式，新旧并存必须都解析**：旧 `jdk-21.0.5+11_openj9-0.48.0` → 新 `jdk-21.0.12.0`（四段点分，第 4 段是 OpenJ9 refresh）；2026-02 过渡期存在 tag 旧、资产名已新的混合特例。8 线旧格式 `jdk8u402-b06_openj9-0.43.0`、新格式归一 `jdk-8.0.492.0`。
- Windows x64 资产映射：`ibm-semeru-open-jdk_x64_windows_<版本串>.zip`；**无 aarch64_windows**（多仓全量资产验证）。
- GA 过滤：`.prerelease` 布尔可靠；11 仓历史混入 2 枚 `-certified` GA tag，用资产前缀 `ibm-semeru-open-jdk_x64_windows_` 天然排除。
- **校验与元数据一次拿全**：旁路 `.zip.json`（Adoptium 风格，约 30KB）自带 sha256 字段与结构化版本，优于单独抓 `.sha256.txt`（后者是 `<hash><两空格><文件名>` 格式非裸 hex）。
- 历史 release 不删（8 仓 62 条回溯 2021-07），ListVersions 可全量分页。
- GitHub API 未认证限流 60 req/h：Available 若打全部 14 仓各一发共 14 请求；**建议活跃线收窄为 `8/11/17/21/25/26`（6 仓）**，26 为当期短期版、27 出 GA 后再纳入（zulu `activeVersions` 先例）。

## Requirements

- `internal/provider/semeru/` 新增适配器，嵌入 `Base`，实现 `Available` / `Resolve` / `LatestPatch` / `ListVersions`，`init()` 自注册；实现 `Configurable` 接收 arch。
- `Available` 返回支持的大版本列表（LTS 标记：8/11/17/21/25 为 LTS，其余 false）；major 清单写死活跃线 `8/11/17/21/25/26`（先例：graalvm `cpuReleases` / zulu `activeVersions`；26 为当期短期版，27 出 GA 后纳入）。
- `Resolve`：纯大版本号 → 组内最新 GA；完整版本号 → 精确匹配 release tag/asset（`ParseVersionSpec` / `IsPureMajor` 复用）。
- `ListVersions(major)`：该 major 仓库的全部 GA release（分页遍历，降序）。
- 完整性校验：下载后按 `.zip.sha256.txt` 旁路 SHA256 校验（`Asset.ChecksumAlgo` 默认 sha256 路径）。
- ReleaseName / 版本目录命名：`semeru-{版本}`，具体规范（含 OpenJ9 build 段是否入名）在 design.md 定案，须保证用户从 `available -a` 复制可直接 `install`。
- `arch=aarch64` 时四个入口统一返回无 Windows ARM64 错误（建议改用 temurin/microsoft），对齐 corretto/graalvm 先例。
- 中文注释与 CLI 文案；公共符号文档注释齐全（项目规范）。

## Out of Scope

- certified / EA 变体；JRE image；Linux/macOS 平台；镜像源（直连 GitHub，国内可达性同 upgrade 自更新现状，不额外处理）。

## Acceptance Criteria

- [ ] 单测：tag/asset 版本解析、LTS 标记、`Resolve` 精确/大版本两态、aarch64 守卫、release 分页解析（fixture/httptest 表驱动，先例：现有各 provider 子包测试）
- [ ] `jvm available semeru` / `available semeru -a` / `--json` 输出正确且缓存生效（二次执行零网络）
- [ ] `jvm install semeru@21`（大版本取最新）与 `install semeru@<完整版本>` → 校验和验证 → `use` / `exec semeru@21 -- java -version`（确认输出含 OpenJ9）→ `uninstall` 全链路人工验证通过
- [ ] `outdated` / `update semeru@21` 对新 distro 工作（LatestPatch 契约满足）
- [ ] `make fmt` / `make vet` / `make test` / `make test-race` 全绿
- [ ] distro 名进补全列表（`provider.All()` 自动生效，无需改码——验证即可；token 递增归父任务收尾）

## Notes

- 建议首先激活：发布基建与 temurin 最像（旁路 sha256 + json 元数据 + 每版本一仓的确定性布局），接入成本三家里最低。
- 版本线含非 LTS（16~20/22/24），是本次三个新 distro 中唯一能扩"非 LTS 覆盖"的。
