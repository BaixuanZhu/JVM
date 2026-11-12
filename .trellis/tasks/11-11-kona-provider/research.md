# Research: Tencent Kona provider 接入前字段级事实调研

- Query: 为新增 `kona` provider 调研 TencentKona-{8,11,17,21,25} 五仓的 SHA256 供给 / tag 与资产命名 / Windows 矩阵 / GA 过滤 / .md5 格式 / 历史保留 / API 预算 / ReleaseName 候选
- Scope: external（GitHub API `gh api` 实测 + WebFetch 官网/wiki + 一次真实下载校验）
- Date: 2026-11-12
- 工具：本机 api.github.com 未认证 curl 限流，全部走 `gh api`（已认证）；raw 资产用 curl 直连 github.com（不受 api 限流影响）

## 结论速览

1. **SHA256：找到了（官方机器可读索引）**。`https://tencent.github.io/konajdk/releases/kona-v1.json`（腾讯官方 GitHub Pages 站，actions/setup-java 的 `KONA_RELEASES_URL` 同源消费）按 文件×版本 给出 64 位 hex **SHA256**。已实测验证：下载 8.0.26 Windows zip（128,630,523 B）计算 sha256 与索引值**逐字节一致**。
2. 但索引**滞后 1~2 个季度**（每线只保留最近 8 期、25 线 2 期；最新只到 8.0.26/11.0.31/17.0.19/21.0.11/25.0.3，站点 Last-Modified 2026-07-03）→ **当前最新 release（四段版本那批）仍只有 MD5**。校验方案应为"逐版本取最强：索引有则 SHA256，否则 .md5"。
3. release 正文、五仓 wiki、README 均无 SHA256 清单；kona.qq.com 本网络 DNS 不通（未验证；canonical 官网即上述 GitHub Pages 站）。
4. tag/资产命名五线三套模板：17/21/25 `TencentKona-<ver>`；11 `kona<ver>`；8 `<ver>-GA` + 资产前缀 `TencentKona8`、尾缀 `_8u<NNN>`。`.b1` 只出现在资产名（重建版 `.b2`），tag 可逆推版本、但 tag 推不出资产名 → 资产必须从 release 资产列表里按模式挑。
5. 五线 latest 全部有 `windows-x86_64_signed.zip`；全部 106 个 release 无任何 `windows-aarch64` 资产。
6. GA 过滤**不能靠 `.prerelease`**（五仓全部 106 release 均为 false），靠命名排除：`Fiber-*`/`*-fiber`/`*-vector-api`/无 windows signed 资产的旧期/`b1-ea`。
7. `.md5` 旁路是 **BSD 格式** `MD5 (文件名) = <32hex>`，非纯 hex，解析要按 `= ([0-9a-f]{32})` 取。
8. 历史全量保留（35/30/22/14/5 期，最早 2019-11），单页 `per_page=100` 即全量 → **Available = 5 发请求**（每仓 1 发）。
9. ReleaseName 建议：**取 tag 派生版本号、不含 `.bN`**（`21.0.13.0.3` / `11.0.33` / `8.0.28`），复制可装、组内唯一、tag 可逆推（17.0.4 与 17.0.4.1 靠 tag 后缀天然区分）。
10. 版本四段 `21.0.13.0.3` 与 JVM 版本串**完全相等**（`openjdk version "21.0.13.0.3"`，build `21.0.13.0.3+1-LTS`）；`.b1` 是资产名 build 后缀、`+1` 是 JVM build，都不进版本号。

## 逐题事实

### Q1 官方 SHA256 调研（最优先）

**结论：找到，来源为官方 GitHub Pages 索引 `kona-v1.json`；但覆盖滞后，最新 release 仍仅 MD5。**

| 查过位置 | 结果 |
|---|---|
| `https://tencent.github.io/konajdk/releases/kona-v1.json` | **有 SHA256**。每条文件 `{os, arch, filename, checksum}`，checksum 恒 64 位 hex（= SHA256）。actions/setup-java 的 KonaDistribution 以 `KONA_RELEASES_URL` 硬编码消费它并 `downloadAndVerify`，provenance 可靠 |
| 五仓全部 release 正文（`.body`，106 期） | 无。精确 grep `sha256\|sha-256\|checksum\|md5sum`（排除 java -version 里 "sharing" 的误匹配）→ **0 命中**；正文只有 `Update to <ver>` + java -version 块 |
| 五仓 wiki：21（空壳）、8 / 11 / 17 的 Installation Guide（英文+中文页）、25（仅链接） | 无任何校验和内容（8 线安装指南只教 `java -version` 验证） |
| 五仓 README | 只链 wiki 与 releases 页，无校验和 |
| `kona.qq.com` | 本机 DNS 解析超时（WebFetch TLS reset / web reader 500 / nslookup timeout），**无法验证**；搜索结果指向的 canonical 官网即 tencent.github.io/konajdk |

索引样例（`kona-v1.json` 的 `"8"[0]`，节选）：

```json
{
  "version": "8.0.26", "jdkVersion": "8u492", "latest": true,
  "baseUrl": "https://github.com/Tencent/TencentKona-8/releases/download/8.0.26-GA/",
  "files": [
    { "os": "windows", "arch": "x86_64",
      "filename": "TencentKona8.0.26.b1_jdk_windows-x86_64_8u492_signed.zip",
      "checksum": "37d3a7bee43435ebe034a7d38813c35fd38f64979aa82f5c053872d9beb056dd" },
    ...
  ]
}
```

**实测验证（2026-11-12）**：下载 `TencentKona8.0.26.b1_jdk_windows-x86_64_8u492_signed.zip`（Content-Length 128630523），`sha256 = 37d3a7bee43435ebe034a7d38813c35fd38f64979aa82f5c053872d9beb056dd` 与索引 **MATCH**。

索引覆盖矩阵（每线保留最近 8 期；`latest` 标记与 GitHub 实际 latest 对照）：

| 线 | 索引含版本数 | 索引 latest | GitHub 实际 latest（发布日） | 滞后 |
|---|---|---|---|---|
| 8 | 8（8.0.19~8.0.26） | 8.0.26 | 8.0.28（2026-11-05） | 2 期 |
| 11 | 8（11.0.24~11.0.31） | 11.0.31 | 11.0.33（2026-11-04） | 2 期 |
| 17 | 8（17.0.12~17.0.19） | 17.0.19 | 17.0.21.0.3（2026-10-30） | 2 期 |
| 21 | 8（21.0.4~21.0.11） | 21.0.11 | 21.0.13.0.3（2026-10-30） | 2 期 |
| 25 | 2（25.0.2~25.0.3） | 25.0.3 | 25.0.5.0.3（2026-10-30） | 2 期 |

站点 `Last-Modified: Fri, 03 Jul 2026 07:13:58 GMT`（含 2026-04 周期、缺 2026-07 与 2026-10 周期）→ 疑似按季度随 release 更新、当前欠 2 轮。索引里尚无任何四段版本。

### Q2 tag / 资产命名逐线归纳

各线 latest 与历史样本（`gh api` 实测原文）：

| 线 | tag 模板 | Windows 资产模板 | 实测样例（原文） |
|---|---|---|---|
| 21 | `TencentKona-<ver>` | `TencentKona-<ver>.bN_jdk_windows-x86_64_signed.zip` | tag `TencentKona-21.0.13.0.3`；资产 `TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip`（+`.md5`）。历史 `21.0.12`/`21.0.5` 同构；首期 tag `TencentKona-21-ga` → 资产却是 `TencentKona-21.0.0.b1_jdk_windows-x86_64_signed.zip`（tag≠版本） |
| 17 | 同 21 | 同 21 | tag `TencentKona-17.0.21.0.3`；`TencentKona-17.0.20`、`17.0.12` 同构。特例：`17.0.4.1` → 资产 `TencentKona-17.0.4.b2_...`；`17.0.0` → `TencentKona-17.0.0.b1-ea_jdk_windows-x86_64.zip`（ea、无 signed） |
| 25 | 同 21 | 同 21 | tag `TencentKona-25.0.5.0.3`；`25.0.4`/`25.0.1` 同构（共 5 期，无特例） |
| 11 | `kona<ver>` | `TencentKona-<ver>.bN_jdk_windows-x86_64_signed.zip` | tag `kona11.0.33`；资产 `TencentKona-11.0.33.b1_jdk_windows-x86_64_signed.zip`。历史 `kona11.0.16.1` → 资产 `TencentKona-11.0.16.b2_...`；fiber 变体 tag `kona11.0.15-fiber` → 资产 `TencentKona-11.0.15.b1_jdk_fiber_...`（另一期 `kona11.0.13-fiber` 资产用**连字符** `TencentKona-11.0.13.b1-jdk_fiber_...`）；`kona11.0.8-vector-api` 无 windows 资产；≤`kona11.0.12` 各期无 windows 资产 |
| 8 | `<ver>-GA`（早期 `v` 前缀、一期 `AARCH64_8u242`） | `TencentKona8.x.y.bN_jdk_windows-x86_64_8u<NNN>_signed.zip`（前缀无连字符、尾缀带 Oracle 更新号） | tag `8.0.28-GA`；资产 `TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip`。`8.0.11` → `.b2`；`Fiber-8.0.13-GA` → `TencentKona8.0.13.b1_jdk_fiber_windows-x86_64_8u362_signed.zip`；≤`8.0.7-GA` 各期无 windows 资产 |

要点归纳：

- **`.b1`/`.b2` 只在资产名，从不进 tag**；`.b1` 占绝对多数，但存在重建版 `.b2`（8.0.11、kona11.0.16.1、17.0.4.1 三例，均伴随 tag 端 `.1` 后缀）。⇒ **tag 无法可靠推出资产名**，必须按 tag 拉取 release 后在资产列表里模式匹配。
- 分隔符不稳定：windows 资产 `b1_jdk`（下划线）、linux 资产 `b1-jdk`（连字符）、个别 fiber 资产 `b1-jdk` 混用。匹配用宽松后缀 `jdk_windows-x86_64_signed.zip` 更稳。
- **四段版本与 JVM 版本串的关系**：`TencentKona-21.0.13.0.3` 的 release 正文：

  ```
  openjdk version "21.0.13.0.3" 2026-10-28 LTS
  OpenJDK Runtime Environment TencentKonaJDK (build 21.0.13.0.3+1-LTS)
  ```

  ⇒ tag 版本 == openjdk version（逐字相等），`+1-LTS` 是 build 串、`.b1` 是资产 build 后缀，三者互不掺和。旧周期（2026-07 及以前）是三段 `21.0.12`（java version 同为 `21.0.12`）；`.0.3` 第四五段是 2026-10 周期起的新方案（21.0.13.0.3 / 17.0.21.0.3 / 25.0.5.0.3 三线同时切换）。8 线不同：tag `8.0.28-GA` → java `1.8.0_512`、`(Tencent Kona 8.0.28)`；11 线 tag `kona11.0.33` → java `11.0.33`。
- 小怪例：`TencentKona-17.0.20` 的正文 java version 写的是 `17.0.19`（2026-04-24）——tag 与 JVM 串有历史性 1 期错位，以 tag 为准。

### Q3 Windows 资产矩阵（latest）

| 线 | latest tag | windows-x86_64 signed zip | windows-aarch64 |
|---|---|---|---|
| 8 | `8.0.28-GA` | 有 `TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip` + `.md5` | 无 |
| 11 | `kona11.0.33` | 有 `TencentKona-11.0.33.b1_jdk_windows-x86_64_signed.zip` + `.md5` | 无 |
| 17 | `TencentKona-17.0.21.0.3` | 有 `TencentKona-17.0.21.0.3.b1_jdk_windows-x86_64_signed.zip` + `.md5` | 无 |
| 21 | `TencentKona-21.0.13.0.3` | 有 `TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip` + `.md5` | 无 |
| 25 | `TencentKona-25.0.5.0.3` | 有 `TencentKona-25.0.5.0.3.b1_jdk_windows-x86_64_signed.zip` + `.md5` | 无 |

全历史扫描（106 期 × 全资产，按 `windows` 过滤）：**从未出现** `windows-aarch64` / msi / exe；Windows 资产只有 x86_64 signed zip 一种形态（唯一例外 17.0.0 的 `b1-ea` 无 signed zip，见 Q2）。签名与未签名差异：Windows 线一律 `_signed`（macOS 用 `_notarized`，Linux 无后缀），无需在 x64 间选择。

### Q4 GA 过滤

- **`.prerelease` 字段恒 false**：五仓全部 106 个 release 逐条核对均为 `prerelease=false`（含 Fiber/EA 变体）→ 该字段完全无区分度。
- 非 GA 混入物按 tag/资产命名区分：

| 类型 | tag 样例 | 处置 |
|---|---|---|
| Fiber 增强版 | `Fiber-8.0.13-GA`、`kona11.0.15-fiber`（资产含 `_jdk_fiber_`） | 排除（非主线 HotSpot 形态） |
| Vector API 试验版 | `kona11.0.8-vector-api` | 排除 |
| EA 首发版 | `TencentKona-17.0.0`（资产 `b1-ea`、无 signed） | 排除（无合格 windows 资产，资产模式天然过滤） |
| 无 Windows 资产的旧期 | 8 线 ≤`8.0.7-GA`、11 线 ≤`kona11.0.12`、`AARCH64_8u242` | 资产模式天然过滤 |
| tag 命名特例 | `TencentKona-21-ga`（→ 版本 21.0.0，但该期资产合格） | 可入表（21.0.0）或排除（无 所谓），解析需特判 |

### Q5 `.md5` 旁路内容格式

**BSD md5 格式**（非纯 hex、非 `md5sum` 格式）。两线实测原文（`cat -A` 确认行尾只有 `$` 即单个换行、无尾随空格）：

```
$ curl -sL https://github.com/Tencent/TencentKona-21/releases/download/TencentKona-21.0.13.0.3/TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip.md5
MD5 (TencentKona-21.0.13.0.3.b1_jdk_windows-x86_64_signed.zip) = 82a87aefb260ec0e672986bbc43abc08

$ curl -sL https://github.com/Tencent/TencentKona-8/releases/download/8.0.28-GA/TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip.md5
MD5 (TencentKona8.0.28.b1_jdk_windows-x86_64_8u512_signed.zip) = 2196083e32ab493c8035a13da2b50e82
```

解析：正则 `=\s*([0-9a-fA-F]{32})` 取哈希即可。旁路 URL 规则 = 资产 `browser_download_url` + `".md5"`（windows signed zip 全历史均有旁路；11 线 latest 的 **linux**-x86_64 资产恰好漏挂 `.md5`，说明旁路是手工挂的，解析要容错）。

### Q6 历史保留

| 仓 | 保留期数 | 最新发布 | 最早一期 | 结论 |
|---|---|---|---|---|
| TencentKona-8 | 35 | 2026-11-05 | `v8.0.0-GA`（2019-11-04） | 全量保留 |
| TencentKona-11 | 30 | 2026-11-04 | `kona11.0.9.1`（2021-03-08） | 全量保留（更早期或从未发 release，无从判断删除） |
| TencentKona-17 | 22 | 2026-10-30 | `TencentKona-17.0.0`（2021-11-04） | 全量保留 |
| TencentKona-21 | 14 | 2026-10-30 | `TencentKona-21-ga`（2024-09-05） | 全量保留 |
| TencentKona-25 | 5 | 2026-10-30 | `TencentKona-25.0.1`（2025-12-11） | 全量保留 |

无删档迹象；五仓均 `≤100` 期 → `releases?per_page=100` 单页拿全，无分页需求。

### Q7 API 预算

| 操作 | 请求 | 说明 |
|---|---|---|
| `Available()`（全表 + `-a` 分组） | **5 发** | `GET /repos/Tencent/TencentKona-{8,11,17,21,25}/releases?per_page=100` 各 1 发，单页全量；五仓 major 写死，无发现端点 |
| `LatestPatch(major)` | 1 发 / 线 | `GET /repos/.../releases/latest`（latest 恒为主线版——Fiber 都是被 newer 主线盖住的旧期，实测 latest 均正常）；或复用 Available 缓存零请求 |
| `Resolve(ver)` | 1 发 | `GET /repos/.../releases/tags/<tag>`（tag 按线内模板逆推） |
| 校验和 | +0~1 发 | 命中 kona-v1.json（整文件缓存）则 0 发；否则 +1 发 `.md5` 旁路（几十字节） |

### Q8 ReleaseName 候选评估

| 候选 | 复制可 install | 组内唯一 | tag 可逆推 | 评价 |
|---|---|---|---|---|
| **tag 派生版本号，不含 `.bN`**（`21.0.13.0.3` / `11.0.33` / `8.0.28` / `17.0.4.1`） | 是 | **是**（tag 唯一 → 派生唯一；`17.0.4` vs `17.0.4.1`、`11.0.16` vs `11.0.16.1` 天然区分） | 是（21/17/25: `TencentKona-<v>`；11: `kona<v>`；8: `<v>-GA`） | **推荐**。且 11/17/21/25 线与 `openjdk version` 逐字相等 |
| 资产名带 `.bN`（`21.0.13.0.3.b1`） | 是 | 是 | **否**（tag 无 `.bN`；`.b1`/`.b2` 无法从 tag 判断，三例 `.b2` 重建版反例） | 否决：违反可逆推 |
| JVM build 串（`21.0.13.0.3+1-LTS`） | 否（`+`/`LTS` 需再解析，且从 release 正文取、多一发请求） | 是 | 否 | 否决：成本高、8 线还是 `1.8.0_512` 异形 |

版本目录名示例：`kona-21.0.13.0.3`、`kona-8.0.28`。排序注意混合段数：`21.0.13.0.3` > `21.0.12`（第 3 段 13>12 即分出）；`17.0.4.1` > `17.0.4`——数值段比较天然正确，无需特判段数。

## 对 design 的建议

- **校验方案（两源混合，逐版本取最强）**：
  1. 安装时先查 kona-v1.json（整文件一次拉取、按 TTL/进程内缓存；URL 写常量）。版本+filename 命中 → `ChecksumAlgo="sha256"`，哈希取自索引；
  2. 未命中（新 release 滞后期）→ 拉 `<zip下载URL>.md5`（BSD 格式，`= ([0-9a-f]{32})` 解析），`ChecksumAlgo="md5"`（`internal/jdk` 需扩展 md5，与 liberica sha1 先例同构；**旁路缺失时按下载失败处理，禁止跳过校验**——11 线漏挂 linux 旁路说明要容错而不是假定存在）；
  3. 索引本身不作为版本发现源（滞后），只作校验和源；版本发现一律 GitHub releases API。
  4. 注释/CHANGELOG 措辞建议："Kona 校验跟随厂商官方供给取最强：官方 release 索引（konajdk GitHub Pages，actions/setup-java 同源）提供 SHA256 时用 SHA256，其余按官方 .md5 旁路 MD5"。依据：本文件 Q1（含 2026-11-12 实测验证记录）+ 用户 2026-11-11 决策。
- **ReleaseName**：tag 派生、去线内装饰（前缀 `TencentKona-`/`kona`、后缀 `-GA`、早期 `v`），fiber/vector 排除在外。特例表：`TencentKona-21-ga` → `21.0.0`（或直接被"资产需含 `jdk_windows-x86_64_signed.zip`"过滤掉，二选一在 design 定）。
- **解析要点**：资产匹配用后缀 `jdk_windows-x86_64_signed.zip` 且排除 `fiber`（容忍 `b1_jdk`/`b1-jdk` 混用与 `.b1`/`.b2`/`b1-ea` 差异）；tag → 版本用三套线内模板；版本比较用局部数值段比较器（dragonwell 先例），支持 3/4/5 段混排。
- **风险**：
  - 版本方案一年内已从三段切到五段（2026-10 起 `.0.3`），比较器与解析必须段数无关；
  - kona-v1.json 更新节奏不受我们控制（当前欠 2 轮），新装最新版大概率走 MD5 分支；
  - `.b2` 重建先例说明 tag↔资产映射不能写死模板；
  - fiber 期理论上可能成为某仓 latest（当前不是）——`/releases/latest` 消费处加 tag 防御性校验成本低；
  - GitHub API 未认证限额下 5 发/次 available 刷新属正常开销（项目已有 GitHub API 消费先例），available-cache TTL 已缓解。

## 未决问题

1. kona.qq.com 是否另有 SHA256 清单：本网络 DNS 不通无法验证（canonical 官网证据已足够，design 可注明" unreachable，未验证"）。
2. kona-v1.json 何时补上 2026-07/2026-10 两轮（决定新版本走 SHA256 还是 MD5 分支的比例）；无 SLA，只能按"命中即 SHA256"设计。
3. `.0.3` 第四五段的官方语义（Kona 自有 patch 号？）未见文档说明，只确认了与 JVM 版本串逐字相等这一事实。
4. `TencentKona-21-ga`（→21.0.0）是否入 available 表：纯产品决策，留 design 定。
