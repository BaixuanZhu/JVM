# 新增发行版 provider：Dragonwell / Kona / Semeru（父任务）

## Goal

为 jvm 新增 Alibaba Dragonwell、Tencent Kona、IBM Semeru 三个发行版 provider，全部走各发行版官方 GitHub releases 端点（不经 scoop bucket 等第三方聚合源）。父任务持有需求源、子任务地图、跨子任务验收与最终集成收尾，本身不做 provider 实现。

## 背景与动机

- 现状盘点：temurin / zulu / liberica 的版本列表已经完整（含非 LTS 全 patch）；microsoft（仅 LTS 11/17/21/25）、corretto（latest_links 索引只保留支持中的线）、graalvm（只接 CPU LTS 21/25）的"不全"是官方产品线决定的，无更多数据源可挖。
- 本次扩展的增量在**厂商维度**：国产双雄（dragonwell / kona，国内用户下载链路友好）+ OpenJ9 生态唯一选择（semeru）。
- 用户已确认的取舍：
  - **有官方端点最好**——三家全部具备官方 GitHub releases，无需第三方中转。
  - **ARM64 无关紧要**——三家官方均无 Windows ARM64 构建；ARM64 环境下查询/安装入口统一报错提示即可（"不显示"），不作为阻塞项。

## 子任务地图

| 子任务 | 优先级 | 数据源 | 校验和 | 关键风险 |
|---|---|---|---|---|
| 11-11-semeru-provider | P1（建议首先激活） | `ibmruntimes/semeru{8,11,16..25}-binaries` | `.sha256.txt` 旁路 ✓ | tag/asset 版本格式需精确研究 |
| 11-11-dragonwell-provider | P2 | `alibaba/dragonwell{8,11,17}` + `dragonwell-project/dragonwell21` | `.sha256.txt` 旁路 ✓ | 四段版本号、repo 分属两个 org |
| 11-11-kona-provider | P2 | `Tencent/TencentKona-{8,11,17,21,25}` | GitHub release 仅 `.md5` ⚠ | 无官方 SHA256 时需用户决策（接 MD5 / 放弃） |

三个子任务相互独立、可并行或任意顺序推进；建议顺序 semeru → dragonwell → kona（发布基建亲和度递减）。

## 跨子任务共同约束

1. **架构语义（用户明确）**：三家仅支持 `x64`。`arch=aarch64` 时 `Available` / `Resolve` / `LatestPatch` / `ListVersions` 各入口统一返回"无 Windows ARM64 构建"类错误并建议改用 temurin/microsoft——沿用 corretto / graalvm 的 `errNoWindowsARM64` 先例，不引入新机制。补全脚本的 distro 列表不按 arch 过滤（现状即如此，与 corretto/graalvm 行为一致）。
2. **官方端点原则**：元数据与下载全部直连官方 GitHub releases / 官方 CDN，禁止引入 scoop bucket 等社区聚合源作为数据链路（信任链必须厂商一手）。
3. **完整性校验（用户 2026-11-11 确认的原则）**：跟随厂商官方供给，**取其最强算法（SHA256 > SHA1 > MD5）**，校验失败即中止。Kona 官方仅提供 MD5，降级已获批；所用算法与来源须在 provider 注释与 CHANGELOG 中明示。
4. **命名**：distro 名 `semeru` / `dragonwell` / `kona`（均无连字符，`SplitDistro` 无需改动）；版本目录命名 `{distro}-{ReleaseName}`，各子任务 design 阶段定义 ReleaseName 规范。
5. **命令面自动生效**：新 provider 注册后 `available`（含缓存/`--json`/`-a` 分组）、`install`（含本地 zip 双参数）、`use`/`pin`/`exec`/`outdated`/`update`/`uninstall`/`cache` 全链路应无改动即工作——这是验收重点，出现需要改 cmd 层的需求时回到父任务重新评估。

## 父任务收尾职责（全部子任务归档后）

- [x] 补全 token：`jvm-completion` v8 → v9（distro 列表新增 semeru/dragonwell/kona）
- [x] shell 集成脚本：确认 `CandidateFiles`/钩子无涉及 distro 的内容需同步（核实无）
- [x] `AGENTS.md`：发行版清单、provider 目录列表（新增三条适配器条目）、Security considerations 网络端点与校验模型（「按厂商供给取最强（SHA256 > SHA1 > MD5，Kona 为 MD5 先例）」）全部更新
- [x] README / docs：中英 README 发行版表 + tagline + 设计决策计数、docs/commands 中英 install 示例
- [x] `CHANGELOG.md` Unreleased 段落已写
- [x] `scripts/integration-test.sh`：6l–6za 用例（available×3 / install×4 含 kona 双分支校验 / exec 厂商串断言 / 五段版本 junction / uninstall×4）
- [x] 最终集成验收：CI 集成测试 run 43137274415 全绿（真实下载安装 + 校验 + exec + 卸载），替代本地人工冒烟（本机 GitHub API 配额受限，迁移到 CI 属更优解）

## Acceptance Criteria（父任务）

- [x] 三个子任务全部完成并归档
- [x] `make fmt` / `make vet` / `make test` / `make test-race` 全绿
- [x] 跨子任务收尾清单逐项勾完
- [x] 三个新 distro 的全链路验证通过（CI 集成测试真实下载验证，见上）

## Notes

- 调研快照（2026-11-11，gh api 实测）已沉淀在各子任务 prd 的"已知事实"节，子任务 design 阶段复核后引用。
