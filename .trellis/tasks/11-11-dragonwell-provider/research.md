# Research: Alibaba Dragonwell provider 接入前字段级事实调研

- Query: 新增 Dragonwell provider（仅 Standard 风味）前的 repo 路由 / 资产命名 / 版本格式 / GA 过滤 / 校验旁路 / 保留策略调研
- Scope: external（GitHub API 实测，`gh api` 已认证）+ 内部对照（jvm provider 先例）
- Date: 2026-11-12
- 数据来源：`gh api`（api.github.com，2026-11-12 实测）+ release 资产原文（curl）

## 结论速览

1. **全家族已迁至 `dragonwell-project`**——不止 21，8/11/17/25 也全在 `dragonwell-project/dragonwell{8,11,17,21,25}`；`alibaba/dragonwell{8,11,17}` 是 GitHub 转移重定向（API 返回新 full_name），`alibaba/dragonwell21` 直接 404。路由表只有一张：`dragonwell-project/dragonwell<major>`。
2. **意外：dragonwell25 线已存在且有 Standard GA**（`dragonwell-standard-25.0.4.0.4+7_jdk-25.0.4-ga`，含 x64_windows zip）——majors 不是 {8,11,17,21}，是 **{8,11,17,21,25}**。
3. 四+一线 latest 全部有 `Alibaba_Dragonwell_Standard_*_x64_windows.zip`；无任何 aarch64_windows 资产（21 线 latest 穷举验证，平台仅 linux 系 + x64_windows）。
4. **`.prerelease` 字段完全不可用**（extended 与 2019 年 preview 也都是 false）；GA 过滤只能靠 tag：前缀 `dragonwell-standard-` + 尾缀 `-ga`。
5. **11 线资产名不可由 tag 推导**（tag `11.0.32.28` → 资产 `11.0.32.28.9`，多一个 OpenJDK build 号）；必须在 release 资产列表里按后缀 `_x64_windows.zip` 匹配，不能拼名。
6. `.sha256.txt` 是 `hex␣␣内部构建路径` 两列（路径不是资产名），取第一列即可；`.zip.json` 旁路是 Adoptium 风格元数据，**自带 sha256 字段**，可一次拿到校验和+版本。
7. 历史全保留（8=50 / 11=47 / 17=20 / 21=21 / 25=6 条 release，2019 年至今无删除，2022 年老 release 资产仍在），`?per_page=100` 一页拉全。
8. `/releases/latest` 在 5 个仓全部返回 Standard（即使 extended 更晚 created/published），但机制未明文，**建议 Available 走每仓 1 次 `/releases?per_page=100` + tag 过滤**（预算同为 5 请求，且同时喂 ListVersions/Resolve/LatestPatch）。
9. ReleaseName 建议用 dragonwell 侧版本（tag 去前缀去 `_jdk-...` 尾巴）：`21.0.12.0.12+8` / `11.0.32.28` / `8.30.29` / `25.0.4.0.4+7`——组内唯一、`available -a` 可复制；tag 可逆推（8 线除外，`8.30.29` 反推不出 `jdk8u502`）。

## 逐题事实

### Q1 repo 路由表（两 org × 四线，共 8 条查询）

| 查询 | 结果 | 说明 |
|---|---|---|
| `repos/dragonwell-project/dragonwell8` | `dragonwell-project/dragonwell8 archived=false`（desc=Alibaba Dragonwell8 JDK） | 存在 |
| `repos/dragonwell-project/dragonwell11` | `dragonwell-project/dragonwell11 archived=false` | 存在 |
| `repos/dragonwell-project/dragonwell17` | `dragonwell-project/dragonwell17 archived=false` | 存在 |
| `repos/dragonwell-project/dragonwell21` | `dragonwell-project/dragonwell21 archived=false` | 存在 |
| `repos/alibaba/dragonwell8` | 返回 `dragonwell-project/dragonwell8 archived=false` | **重定向**（GitHub transfer redirect，API 跟随） |
| `repos/alibaba/dragonwell11` | 返回 `dragonwell-project/dragonwell11 archived=false` | 重定向 |
| `repos/alibaba/dragonwell17` | 返回 `dragonwell-project/dragonwell17 archived=false` | **17 线也已迁走**（背景假设"8/11/17 在 alibaba"已过时） |
| `repos/alibaba/dragonwell21` | `404 Not Found` | 连重定向都没有，旧地址彻底失效 |

结论：**全部四线现居 `dragonwell-project`**，路由表 = `dragonwell-project/dragonwell{major}`，major ∈ {8,11,17,21,25}。

### Q2 每线 Windows x64 资产（各仓 `/releases/latest` 实测）

| 线 | latest tag | published | Windows 资产原文 |
|---|---|---|---|
| 21 | `dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga` | 2026-09-01 | `Alibaba_Dragonwell_Standard_21.0.12.0.12.8_x64_windows.zip`（+ `.sha256.txt` / `.zip.json` / `-sbom.json`） |
| 17 | `dragonwell-standard-17.0.20.0.21+8_jdk-17.0.20-ga` | 2026-09-01 | `Alibaba_Dragonwell_Standard_17.0.20.0.21.8_x64_windows.zip`（同上三旁路） |
| 11 | `dragonwell-standard-11.0.32.28_jdk-11.0.32-ga` | 2026-09-20 | `Alibaba_Dragonwell_Standard_11.0.32.28.9_x64_windows.zip`（**注意多出 `.9`**） |
| 8 | `dragonwell-standard-8.30.29_jdk8u502-ga` | 2026-09-18 | `Alibaba_Dragonwell_Standard_8.30.29_x64_windows.zip`（与 tag 版本号逐字一致，无后缀） |
| 25 | `dragonwell-standard-25.0.4.0.4+7_jdk-25.0.4-ga` | —（见 Q7） | `Alibaba_Dragonwell_Standard_25.0.4.0.4.7_x64_windows.zip` + **额外 `..._x64_windows-fix.zip`** |

- 8 线与 21 线差异：8 线无 `+build` 后缀、无第四五段版本，资产名 = tag 版本原样；21 线 `+8` 变 `.8` 拼进资产名。
- **aarch64_windows：预期无，验证无**。21 线 latest 资产全集（穷举）：`aarch64_linux` / `riscv64_linux` / `x64_alpine-linux` / `x64_linux` / `x64_windows` 五平台 × (zip/.sha256.txt/.zip.json/-sbom.json)，无任何 windows-arm 资产；8/11/17 latest 同样只有 x64_windows。
- 部分历史 release 另有 `.zip.sig`（如 11.0.28、8.26.25、17.0.16），21 latest 没有——sig 不是稳定契约。

### Q3 版本号格式逐线归纳（每线 ≥3 历史 tag 实测）

**21 线**（`dragonwell-project/dragonwell21`，21 条 release 全列）：

```
dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga   2026-09-01
dragonwell-standard-21.0.11.0.11+10_jdk-21.0.11-ga  2026-05-18
dragonwell-standard-21.0.10.0.10+7_jdk-21.0.10-ga   2026-02-28
dragonwell-standard-21.0.4.0.4+7_jdk-21.0.4-ga      2024-08-29
dragonwell-standard-21.0.1.0.1+12_jdk-21.0.1-ga     2023-11-24
```
模式：`dragonwell-standard-<A.B.C.0.D+Z>_jdk-<A.B.C>-ga`——dragonwell 侧五段（第 4 段恒 0，第 5 段 = dragonwell update）+ `+Z`（OpenJDK build）；OpenJDK 侧 = 前三段。Extended 同版本并行（`dragonwell-extended-21.0.12.0.12+8_...`）。

**17 线**（20 条，**无任何 extended**，全 Standard）：

```
dragonwell-standard-17.0.20.0.21+8_jdk-17.0.20-ga   2026-09-01
dragonwell-standard-17.0.19.0.20+10_jdk-17.0.19-ga  2026-05-06
dragonwell-standard-17.0.16.0.17+8_jdk-17.0.16-ga   2025-09-05
dragonwell-standard-17.0.8.0.8+7_jdk-17.0.8-ga      2023-08-08
dragonwell-17.0.3.0.3+7_jdk-17.0.3-ga               2022-05-11  ← 风味拆分前的裸前缀
dragonwell-17.0.1.0.1+12_jdk-17.0.1-ga              2021-11-15  ← 同上
```
2022-08（17.0.4）起才有 `dragonwell-standard-` 前缀；更早是裸 `dragonwell-`。

**11 线**（47 条）：

```
dragonwell-standard-11.0.32.28_jdk-11.0.32-ga   2026-09-20
dragonwell-extended-11.0.32.29_jdk-11.0.32-ga   2026-09-22  ← extended 的 dragonwell update +1（29≠28，与 8/21/25 线不同！）
dragonwell-standard-11.0.31.27_jdk-11.0.31-ga   2026-05-15
dragonwell-standard-11.0.28.24_jdk-11.0.28-ga   2025-09-05
dragonwell-11.0.15.11_jdk-11.0.15-ga            2022-08-01  ← 拆分前裸前缀
dragonwell_11.0.7.2_GA                           2020-04-27  ← 下划线古董格式
dragonwell_11.0.5.1-preview                      2019-12-30  ← preview 但 prerelease=false
dragonwell-11.0.9.4_jdk-11.0.9+0                 2020-12-11  ← 尾巴不是 -ga 的变体
```
模式：四段、tag **不含 build 号**；OpenJDK 侧 = 前三段；资产名第五段 = OpenJDK build（`.json` 旁路实证：`"version": "11.0.32.28+9", "build": 9, "semver": "11.0.32+2809"`，故 tag `11.0.32.28` → 资产 `11.0.32.28.9`）。

**8 线**（50 条）：

```
dragonwell-standard-8.30.29_jdk8u502-ga   2026-09-18
dragonwell-extended-8.30.29_jdk8u502-ga   2026-09-21  ← 同版本号，extended 稍后发
dragonwell-standard-8.29.28_jdk8u492-ga   2026-05-15
dragonwell-standard-8.28.27_jdk8u482-ga   2026-03-04
dragonwell-8.11.12_jdk8u332-ga            2022-05-07  ← 拆分前裸前缀
dragonwell-8.3.3-GA                        2020-04-09  ← 古董
v8.0.0-GA                                  2019-06-25  ← 更古董（v 前缀）
```
模式：`dragonwell-standard-8.<dwupdate>.<build>_jdk8u<CPU>-ga`；**OpenJDK 侧是 `jdk8u502` 形式，CPU 号无法从 `8.30.29` 推出**（8.30.29→502、8.29.28→492、8.28.27→482，各差 10，但这是 Oracle CPU 序列，不是本地可算的）。8 线资产名 = tag 版本原样无后缀。

**25 线**（6 条）：`dragonwell-standard-25.0.4.0.4+7_jdk-25.0.4-ga` / `25.0.3.0.3+9` / `25.0.2.0.2+10` / `25.0.0.0.1+36`（无 25.0.1，直接 25.0.0→25.0.2）。

**Standard vs Extended 前缀差异确认**：`dragonwell-standard-` vs `dragonwell-extended-`，逐字比较；两者 `.prerelease` 均 false。

### Q4 GA / 风味过滤

| 证据 | 实测 |
|---|---|
| `.prerelease` 字段 | extended、preview、古董全部 `prerelease=false`（含 `dragonwell_11.0.5.1-preview`）→ **字段不可用** |
| tag 前缀 | `dragonwell-standard-` 精确区分风味（Extended 全部 `dragonwell-extended-`） |
| tag 尾巴 | GA 统一以 `-ga` 结尾；存在反例 `dragonwell-11.0.9.4_jdk-11.0.9+0`（非 ga 尾巴，会被尾缀过滤排除） |
| 拆分前裸前缀 | `dragonwell-11.0.15.11_...` / `dragonwell-8.11.12_...` / `dragonwell-17.0.1.0.1+12_...`（2022-08 前）→ 被前缀过滤天然排除 |

**建议过滤规则：`^dragonwell-standard-` 前缀 && 以 `-ga` 结尾**，其余（extended / preview / `+0` / 古董 / 裸前缀）全排除。

### Q5 `.sha256.txt` 与 `.json` 旁路（21 线原文）

`.sha256.txt` 原文（curl 实取）：

```
9b6e4aa9a58d41d0e8b0ab699b0cfe11f5f1b264bb9042ea525cf4a35204925c  target/windows/x64/dragonwell/OpenJDK21U-jdk_x64_windows_dragonwell_2026-09-01-07-46.zip
```

- **两列 sha256sum 格式**：hex + 两个空格 + 文件名；但文件名是**内部构建路径**（`target/windows/...`），**不是 GitHub 资产名**——解析取第一列（`strings.Fields()[0]`），不要按文件名匹配。
- 对照验证：该 hex 与 `.zip.json` 里 `"sha256"` 字段一致，也与 11 线旁路格式一致。

`.zip.json` 旁路可用字段（21 线原文节选）：

```json
{"vendor":"Alibaba","os":"windows","arch":"x64","variant":"dragonwell",
 "version":{"major":21,"minor":0,"security":12,"patch":0,"version":"21.0.12.0","semver":"21.0.12+0","msi_product_version":"21.0.12.0","build":null},
 "scmRef":"dragonwell-standard-21.0.12.0.12+8_jdk-21.0.12-ga",
 "binary_type":"jdk","sha256":"9b6e...925c",
 "full_version_output":"openjdk version \"21.0.12.0.12\" 2026-07-21\r\nOpenJDK Runtime Environment (Alibaba Dragonwell Standard Edition)-21.0.12.0.12+8-GA (build 21.0.12.0.12)..."}
```

- `sha256` 直接可用（与 .sha256.txt 同值）；`scmRef` = tag 原文；`full_version_output` 可佐证 ReleaseName。11 线另有 `"version":"11.0.32.28+9","build":9,"semver":"11.0.32+2809"`。
- 两旁路并存，取 `.sha256.txt` 即可满足 jvm 现有 Asset 校验和契约（graalvm/microsoft 先例）；`.zip.json` 是可选增强。

### Q6 历史保留

| 仓 | release 总数（`?per_page=100` 实测 length） | 最早一条 | 旧资产抽查 |
|---|---|---|---|
| dragonwell8 | 50 | `v8.0.0-GA` 2019-06-25 | `dragonwell-standard-8.12.13_jdk8u345-ga`（2022-08）资产齐全（zip+sha256 都在） |
| dragonwell11 | 47 | `dragonwell_11.0.5.1-preview` 2019-12-30 | 11.0.28（2025）有 zip/.sha256/.json/.sig |
| dragonwell17 | 20 | `dragonwell-17.0.1.0.1+12_...` 2021-11-15 | 17.0.16 同上 |
| dragonwell21 | 21 | 21.0.1 2023-11-24 | latest 全套旁路 |
| dragonwell25 | 6 | 25.0.0.0.1+36 | — |

**无删除、无修剪**：全历史在架上，旧 patch 可装；`per_page=100` 当前一页拉全（最大仓 50 条）。

### Q7 API 预算与 majors 写死依据

- **dragonwell25 存在**：`gh api repos/dragonwell-project/dragonwell25` → 存在未归档，且已有 4 个 Standard GA（见 Q3）。org 仓列表（`orgs/dragonwell-project/repos`）中带 release 的主线仓仅 `dragonwell8/11/17/21/25`（其余是 `dragonwell8_hotspot` 等源码卫星仓与 `dragonwell-containers`/`criu` 等周边，无 release）。
- **majors 建议写死 {8,11,17,21,25}**，每 major 固定仓 `dragonwell-project/dragonwell<major>`——与 graalvm `cpuReleases` 写死先例同型。
- `/releases/latest` 实测 5 仓全部返回 Standard（8/11/17/21/25），但 21 线 extended 的 created_at（07:46:18）与 published_at（13:38）均晚于 standard（07:46:05 / 11:57），latest 仍返回 standard——说明 GitHub latest 排序并不简单按 created_at/published_at，**行为未契约化，存在某天返回 extended 的风险**。
- 预算：每仓 1 次 `/releases?per_page=100`（共 5 次）即可同时支撑 Available（每 major 取最新 Standard-GA）/ ListVersions（全部过滤后 tag）/ Resolve（列表内按 tag 匹配）/ LatestPatch（组内比较）。`/releases/latest` 可作快路径但必须校验 tag 前缀，不满足则回退列表。

### Q8 ReleaseName 候选评估

| 候选 | 样例 | 组内唯一 | tag 可逆推 | available -a 复制即装 | 评价 |
|---|---|---|---|---|---|
| A. dragonwell 侧（tag 去前缀去 `_jdk-...` 尾） | `21.0.12.0.12+8` / `11.0.32.28` / `8.30.29` | 是（update 号单调递增） | 17/21/25 是（`_jdk-<前三段>-ga`）；11 是（`_jdk-<前三段>-ga`）；**8 否**（推不出 jdk8u502） | 是 | **推荐**：身份即 dragonwell 版本，`java -version` 输出一致（`21.0.12.0.12`） |
| B. OpenJDK 侧（tag `_jdk-` 后、`-ga` 前） | `21.0.12` / `11.0.32` / `8u502` | 是（Standard 过滤后每 OpenJDK patch 恰一条） | 全部否（缺 dragonwell update + build） | 是 | 不推荐：丢风味身份、与 temurin/microsoft 版本号外观无差别易混淆、8 线形如 `8u502` 与项目版本比较器不兼容 |

- ReleaseName = A 时目录名 `dragonwell-21.0.12.0.12+8`，major 分组 = 首段（21）；`+` 在 temurin 版本号已有先例（`21.0.5+11`），ParseVersionSpec/CompareVersions 无新需求。
- 8 线 tag 不可逆推的兜底：Resolve 统一走"列表内 tag 前缀匹配 `dragonwell-standard-{ReleaseName}_`"，五线同一条代码路径（1 请求 + 缓存），不为 17/21/25 单独做拼 tag 快路径。

## 对 design 的建议

1. **repo 路由表**：`map[major]repo = {8,11,17,21,25} → "dragonwell-project/dragonwell"+major`，写死（graalvm `cpuReleases` 先例）；不消费 alibaba 重定向。镜像：GitHub 直连，MirrorURL 留空（temurin-ea 先例）。
2. **ReleaseName 规范**：`tag[len("dragonwell-standard-") : idx("_jdk-")]`，即 dragonwell 侧版本原样（含 `+`）。示例：`21.0.12.0.12+8`、`25.0.4.0.4+7`、`11.0.32.28`、`8.30.29`。
3. **版本解析要点**：
   - Available/ListVersions：每仓 `GET /releases?per_page=100`（5 请求，全历史在架），过滤 `^dragonwell-standard-` && `-ga$`。
   - Resolve：同列表内找 tag == `dragonwell-standard-{spec}_` 前缀且后续到 `_jdk-` 前恰等于 spec（精确匹配，不做拼 URL）。
   - Windows 资产：release.assets 内后缀匹配 `Alibaba_Dragonwell_Standard_` 前缀 + `_x64_windows.zip` 结尾（排除 `-fix.zip`/`.sha256.txt`/`.zip.json`/`-sbom.json`/`.sig`）；**禁止由 tag 拼资产名**（11 线反例实证）。
   - 校验和：资产名 + `.sha256.txt` 旁路，取首列 hex；SHA256（`ChecksumAlgo` 默认）。
   - aarch64：无 aarch64_windows，`arch=aarch64` 时守卫报错并建议 temurin/microsoft（corretto/graalvm 先例）。
4. **风险清单**：
   - `/releases/latest` 风味不保真（机制未契约化）——不要依赖 latest 端点定 Standard。
   - 11 线 extended 与 standard 的 dragonwell 版本不同（11.0.32.29 vs .28）——前缀过滤必须在前，不能按"最新一条"取。
   - d25 出现 `_x64_windows-fix.zip` 资产——后缀匹配 `_x64_windows.zip` 恰好排除它，但正则要锚定结尾。
   - tag 含 `+`，API download URL 已是 `%2B` 编码（`browser_download_url` 原样使用即可，勿二次编解码）。
   - `.sha256.txt` 第二列是内部构建路径非资产名——解析只认第一列。
   - 旧格式 tag（裸前缀/下划线/v 前缀）被前缀过滤天然排除，无需特判。
   - 未来新 major（如 29）：需手动加仓（写死表维护，同 graalvm 跳号风险低——org 仓可枚举）。

## 未决问题

1. **是否接 25 线**：背景只说 8/11/17/21，但 25 线已有 4 个 GA 且命名同 21 线——MVP 含或不含需 design 拍板（事实层面零额外成本，路由表加一行）。
2. **拆分前裸前缀 release 是否纳入 ListVersions**（如 `dragonwell-17.0.1.0.1+12_...`，2022-08 前、语义上是 Standard 前身）：本调研默认排除（前缀过滤），如要纳入需额外的版本归一逻辑。
3. `.zip.json` 旁路是否利用（可一次取 sha256+元数据）：与 `.sha256.txt` 二选一即可，design 决定。
4. GitHub latest 端点为何返回 standard 而非更晚的 extended（排序机制黑盒）：不影响推荐方案（走列表），仅记录为未解释现象。
