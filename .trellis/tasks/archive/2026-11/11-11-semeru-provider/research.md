# Research: IBM Semeru provider 接入前字段级事实调研

- Query: 为「新增 IBM Semeru provider」做接入前字段级事实调研（tag 格式 / 资产命名 / GA 过滤 / 旁路校验文件 / 历史保留 / API 预算 / LTS 判定）
- Scope: external（GitHub ibmruntimes 组织，全部经 `gh api` 实测）+ IBM 官网
- Date: 2026-11-12
- 工具约束遵守情况: 全程 `gh api`（已认证），未用裸 curl 打 api.github.com；仅 api.adoptium.net 用了 curl

## 结论速览

1. **tag/资产命名在 2026-04 CPU 起全面切换新格式**：旧 `jdk-21.0.5+11_openj9-0.48.0` → 新 `jdk-21.0.12.0`（四段点分，无 `+build`、无 `_openj9` 串）；2026-01-30 CPU 是最后一批旧格式 tag，2026-02 的 OpenJ9 refresh（tag `jdk-21.0.10+7.1_openj9-0.57.0`）资产名已先行切新格式（`21.0.10.1.zip`）。**两条线（新旧）格式确实不一致，且新格式是当前与未来的主线**。
2. 8 线旧格式特殊（`jdk8u402-b06_openj9-0.43.0`，jdk8u 风格），新格式归一为 `jdk-8.0.492.0`；历史还有无 `jdk` 前缀 / 连字符两枚 GA 变体 tag。
3. 非 GA 全部是 OpenJ9 milestone（`-m1`/`-m1a`/`-m2`/`-m2a`）与 `-rc1a` 后缀，**`.prerelease` 布尔可靠**；11 仓 2021 年混入 2 枚 `-certified` GA tag，靠资产前缀 `ibm-semeru-open-jdk_x64_windows_` 天然排除。
4. `.zip.sha256.txt` 是 **`<hash><两空格><文件名>`** 的 sha256sum 文本格式（非裸 hex）；**`.zip.json` 自带 `sha256` 字段**（与 sha256.txt 一致）+ 完整版本结构 → 只下 json 一次即可拿全校验与版本信息。
5. release 不删除：8 仓 62 条（30 GA）回溯到 2021-07-30，老 patch 的 Windows zip 至今在列；21 仓例外缺首发 GA（`jdk-21+35` 只有 `-m0` 里程碑 tag）。
6. open GA 线现共 **14 仓**（8,11,16,17,18,19,20,21,22,23,24,25 + 新增 26,27），每仓 `/releases/latest` 共 14 请求；GraphQL 可 1 请求拿全但**必须认证**（jvm 运行时无 token，不可用）；Adoptium API 不收录 semeru（实测 404）。
7. LTS = 8/11/17/21/25（IBM 官方下载页原文标注）；**26/27 均非 LTS**（LTS 节奏 21→25→29 每两年，27 是 2026-09 的短期版）；8 线可用性延至 2030-12。
8. **无 `aarch64_windows` 资产**（8/11/21/25 四例 latest 全量资产验证，Windows 仅 x64，8 线另有 x86-32）。

## 逐题事实

### Q1 tag 格式（三代演进）

| 线 | repo 内最早 GA | 旧格式 GA 样例（实测 tag） | 新格式 GA 样例（2026-04 起） |
|---|---|---|---|
| 8 | `jdk8u302-b08_openj9-0.27.0`（2021-07-30） | `jdk8u402-b06_openj9-0.43.0`、`jdk8u432-b06_openj9-0.48.0` | `jdk-8.0.492.0`（2026-04-29）、`jdk-8.0.504.5`、`jdk-8.0.512.0` |
| 11 | `jdk-11.0.12+7_openj9-0.27.0`（2021-07-30） | `jdk-11.0.25+9_openj9-0.48.0`、`jdk-11.0.16.1+1_openj9-0.33.1`（四段旧变体） | `jdk-11.0.31.0`、`jdk-11.0.32.15`、`jdk-11.0.33.0` |
| 17 | `jdk-17.0.1+12_openj9-0.29.1`（2021-12-09） | `jdk-17.0.10+7_openj9-0.43.0`、`jdk-17.0.8.1+1_openj9-0.40.0` | `jdk-17.0.19.0`、`jdk-17.0.21.0` |
| 21 | `jdk-21.0.1+12_openj9-0.42.0`（2024-01-11） | `jdk-21.0.5+11_openj9-0.48.0`、`jdk-21.0.9+10_openj9-0.56.0` | `jdk-21.0.11.0`、`jdk-21.0.12.10`、`jdk-21.0.13.0` |
| 22 | `jdk-22.0.1+8_openj9-0.45.0` | `jdk-22.0.2+9_openj9-0.46.1`（仓内最新，已停更） | 无（EOL 停在旧格式） |
| 25 | `jdk-25+36_openj9-0.55.0`（2025-09-25，首发 GA 无 patch 段） | `jdk-25.0.1+8_openj9-0.56.0`、`jdk-25.0.2+10_openj9-0.57.0` | `jdk-25.0.3.0`、`jdk-25.0.4.15`、`jdk-25.0.5.0` |

三代模式归纳：

| 代 | 时间窗 | tag 模式 | 说明 |
|---|---|---|---|
| 旧 | repo 起点 ~ 2026-01-30 | `jdk-{ver}+{build}_openj9-{oj9ver}` | `{ver}` 三段为主，偶有四段（OpenJDK 补丁的补丁，如 11.0.16.1、17.0.8.1）；大版本首发 `{major}+{build}` 无 patch 段（jdk-25+36）；**8 线特例** `jdk8u{u}-b{build}_openj9-{oj9ver}` |
| 过渡 | 2026-01-30 ~ 2026-02 | CPU GA 仍旧 tag；次月 OpenJ9 refresh tag 变 `+{build}.{n}_openj9-…` | 实测：21 线 `jdk-21.0.10+7_openj9-0.57.0`（2026-01-30 GA）→ `jdk-21.0.10+7.1_openj9-0.57.0`（2026-02-26）；8 线 `jdk8u482-b08.1_openj9-0.57.0`；11/17/25 线同构 |
| 新 | 2026-04-29 CPU 起 | `jdk-{major}.{minor}.{security}.{refresh}` | 四段点分，无 `+`、无 openj9 串；第 4 段 = OpenJ9 refresh 号（同一 CPU 周期内递增：`21.0.12.0` → `21.0.12.10` → `21.0.12.15`；json `scmRef: v0.63.0-release` 佐证为 OpenJ9 构建） |

8 线历史杂项（均 GA=false 之外的 GA 条目，解析需容错）：`8u345-b01_openj9-0.33.0`（2022-08-08，无 jdk 前缀，与同日 `jdk8u345-b01_openj9-0.33.0` 重复）、`jdk8u352-b08-openj9-0.35.0`（2022-11-07，`_` 写成 `-`，与 `jdk8u352-b08_openj9-0.35.0` 同日重复）。

21 仓首发查证：`/tags` 共 40 个 tag，无 `jdk-21+35_openj9-0.42.0`（GA），只有 `jdk-21+35_openj9-0.42.0-m0`（prerelease 里程碑）→ 21 线 repo 内最早 GA 是 21.0.1。

### Q2 Windows x64 zip 资产命名（tag → 资产映射）

资产模板：`ibm-semeru-open-jdk_x64_windows_<版本串>.zip`，同目录伴随 `.zip.sha256.txt` / `.zip.json` / `.zip.sig`，另有 `.msi` 安装器（不接）。

| 情形 | tag（实测） | 资产名（实测） | 规则 |
|---|---|---|---|
| 新格式 | `jdk-21.0.13.0` | `ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip` | 版本串 = tag 去 `jdk-` 前缀，原样 |
| 旧格式（三段） | `jdk-21.0.5+11_openj9-0.48.0` | `ibm-semeru-open-jdk_x64_windows_21.0.5_11_openj9-0.48.0.zip` | `+`→`_`，`.` 保留 |
| 旧格式（四段） | `jdk-11.0.16.1+1_openj9-0.33.1` | `ibm-semeru-open-jdk_x64_windows_11.0.16.1_1_openj9-0.33.1.zip` | 同上 |
| 大版本首发 | `jdk-25+36_openj9-0.55.0` | `ibm-semeru-open-jdk_x64_windows_25_36_openj9-0.55.0.zip` | `+`→`_` |
| 8 线旧格式 | `jdk8u402-b06_openj9-0.43.0` | `ibm-semeru-open-jdk_x64_windows_8u402b06_openj9-0.43.0.zip` | 去 `-`（`8u402-b06`→`8u402b06`） |
| 过渡 refresh | `jdk-21.0.10+7.1_openj9-0.57.0` | `ibm-semeru-open-jdk_x64_windows_21.0.10.1.zip` | `+7.1`→`.1` 且**丢弃 openj9 串**（资产先行切新格式）；8 线同构 `jdk8u482-b08.1…`→`8.0.482.1.zip`、11 线 `jdk-11.0.30+7.1…`→`11.0.30.1.zip` |

- 同一 release 还有 jre/debugimage/testimage（同模板不同中段）与 Linux/mac/aix 全家桶；rpm 是另一命名系（`ibm-semeru-open-{major}-jdk-…`）。**资产筛选条件建议：前缀 `ibm-semeru-open-jdk_x64_windows_` + 后缀 `.zip`**，可一并排除 msi/jre/debugimage/testimage/x86-32/certified。
- **aarch64_windows：不存在**。实测 8/11/21/25 四仓 latest 全量资产，Windows 平台仅 `x64_windows`（8 线另有 `x86-32_windows` 32 位）；`aarch64` 仅出现在 linux/mac。结论：Semeru 无 Windows ARM64 构建，需 corretto/graalvm 同款 arch 守卫。

### Q3 GA 过滤

实测各仓 prerelease 占比：8 仓 32/62、11 仓 36/68、17 仓 34/61、21 仓 21/38、25 仓 11/20、22 仓 2/5。

| 非 GA 种类 | tag 样例（实测） | `.prerelease` |
|---|---|---|
| OpenJ9 milestone m1 | `jdk-21.0.13.0-m1`、`jdk-8.0.512.0-m1` | true |
| OpenJ9 milestone m1a/m2/m2a | `jdk-25.0.5.0-m2`、`jdk-25.0.1+8_openj9-0.57.0-m1a`、`jdk-11.0.13+05_openj9-0.29.0-m2a` | true |
| RC | `8u342-b07_openj9-0.33.0-rc1a`（8 仓，2022） | true |
| certified（GA 标记但非 open 线） | `jdk-11.0.12.0-certified`、`jdk-11.0.12.1-certified`（11 仓 2021-09，资产前缀 `ibm-semeru-certified-`） | **false** |

过滤建议（双保险）：`prerelease == false` 为主 + tag 尾缀排除 `-m1|-m1a|-m2|-m2a|-rc`（防上游漏标）+ 资产名前缀精确匹配（天然排除 certified）。`/releases/latest` REST 端点自动跳过 prerelease，实测返回的都是 GA。

另：EA 在独立仓（`semeru11-ea-binaries`、`semeru17-ea-binaries` 等），GA 仓内无 EA 混入。

### Q4 `.zip.sha256.txt` 与 `.zip.json` 内容格式

`.sha256.txt` 原文（新格式，jdk-21.0.13.0）：

```
ebea8e701ca80769e0575e0c72db4c5131b4afde3a33fcac3fe46cc96996c68e  ibm-semeru-open-jdk_x64_windows_21.0.13.0.zip
```

`.sha256.txt` 原文（旧格式，jdk-21.0.5+11_openj9-0.48.0）：

```
81f683cef58420eeb54c5cba24141753a58261f8c07399f9b0fd501789ea177e  ibm-semeru-open-jdk_x64_windows_21.0.5_11_openj9-0.48.0.zip
```

→ **sha256sum 文本格式（hash + 两个空格 + 文件名），非裸 hex**（与 graalvm 的裸 hex `.sha256` 不同），解析取第一列即可。

`.zip.json` 关键字段（新格式 jdk-21.0.13.0 原文节选）：

```json
{
    "vendor": "IBM Corporation",
    "os": "windows",
    "arch": "x64",
    "variant": "openj9",
    "version": {
        "minor": 0, "patch": 0, "security": 13,
        "msi_product_version": "21.0.13.0",
        "major": 21, "version": "21.0.13.0",
        "semver": "21.0.13+0", "build": 0, "opt": null
    },
    "scmRef": "v0.63.0-release",
    "version_data": "jdk21u",
    "binary_type": "jdk",
    "sha256": "ebea8e701ca80769e0575e0c72db4c5131b4afde3a33fcac3fe46cc96996c68e",
    "full_version_output": "openjdk version \"21.0.13.0.3\" 2026-10-24 LTS\r\nIBM Semeru Runtime Open Edition 21.0.13.0 (build 21.0.13.0.3+1-LTS)\r\nEclipse OpenJ9 VM 21.0.13.0 (build 21.0.13.0.3+1-openj9-0.63.0, …)",
    "variant_version": { "major": "0", "minor": "63", "security": "0" }
}
```

旧格式差异（jdk-21.0.5+11 原文节选）：`"version": "21.0.5+11-LTS"`、`"semver": "21.0.5+11.0.LTS"`、`"opt": "LTS"`、`"variant_version": {"minor": "48"}`。

→ 结论：**`.zip.json` 的 `sha256` 字段与 sha256.txt 完全一致，且带结构化版本（major/version/build/opt）与 `binary_type`**：只下载一次 json 即可拿「下载 URL（由 tag+资产名自拼）+ 校验和 + 版本确认」。注意 json 还内嵌 `configure_arguments` 等大文本字段（整个文件约 30KB+，解析时只取需要的键）。新格式 `opt` 为 null（LTS 字样只在 full_version_output 文本里），旧格式 `opt` = "LTS"。

### Q5 历史保留策略（releases 分页实测，`--paginate` 全量）

| repo | release 总数 | 其中 GA | repo 内最早 GA | Windows zip 覆盖 |
|---|---|---|---|---|
| semeru8 | 62 | 30 | `jdk8u302-b08_openj9-0.27.0`（2021-07-30） | 29/30 有（`jdk8u302-b08_openj9-0.27.1` 为 Linux-only 部分重发，无 Windows 资产） |
| semeru11 | 68 | 32 | `jdk-11.0.12+7_openj9-0.27.0`（2021-07-30） | 29/32（2 枚 certified 无 open zip；0.27.1 同上 Linux-only） |
| semeru17 | 61 | 27 | `jdk-17.0.1+12_openj9-0.29.1`（2021-12-09） | 全部有 |
| semeru21 | 38 | 17 | `jdk-21.0.1+12_openj9-0.42.0`（2024-01-11） | 全部有；**首发 `jdk-21+35` GA 缺席**（仅 -m0 里程碑 tag） |
| semeru25 | 20 | 9 | `jdk-25+36_openj9-0.55.0`（2025-09-25 首发） | 全部有 |
| semeru22 | 5 | 3 | `jdk-22.0.1+8_openj9-0.45.0` | 全部有（EOL 停更） |

结论：**release 条目与旧 patch 的 zip 资产不删除**（2021-07 的 Windows zip 至今在资产列表中），ListVersions 完整性、Resolve 精确匹配旧版均可（上限即 repo 起点，早于 2021-07 的版本不存在）。例外两处：个别「部分重发」GA 只带 Linux 资产；21 线首发 GA 缺席。

### Q6 API 预算

- open GA 线仓库全集（2026-11 实测，org 搜索 `org:ibmruntimes binaries in:name`）：**14 个** —— `semeru{8,11,16,17,18,19,20,21,22,23,24,25,26,27}-binaries`。任务背景里的 `16..25` 清单已过时：**2026 年新增 semeru26（3 月 GA）、semeru27（9 月 GA）两仓**。
- 每仓一发 `/releases/latest`：**14 请求/次 Available**（按任务旧清单算是 12 发，但 26/27 已存在不该漏）。另有 certified/ea 仓与 `semeru-jextract-binaries`（无关，排除）。
- GraphQL 批量实测可行（一条查询 14 仓 latestRelease.tagName 全部返回），**但 GitHub GraphQL 必须认证**；jvm 运行时无 token，故实现层只能走未认证 REST（60 次/时/IP 硬限额）→ 14 发/次必须靠 available-cache 的 TTL 兜底，或收窄到活跃线（LTS 8/11/17/21/25 + 当期短期 26/27 = 7 发）。
- EOL 短线（16/18/19/20/22/23/24）latest 永不再变（实测 latest 停在各自末代：16.0.2+7、18.0.2+9、19.0.2+7、20.0.2+9、22.0.2+9、23.0.2+7、24.0.2+12），可用更长 TTL 或静态表。
- Adoptium API 不收录 semeru（实测）：`GET https://api.adoptium.net/v3/assets/latest/11/openj9?os=windows&architecture=x64&image_type=jdk` → **404**；`/v3/info/available_releases` 亦为 temurin 体系数据。无更省的官方聚合端点（IBM 下载页为 HTML+XHR 动态加载，公开 JSON 端点未发现）。

### Q7 LTS 判定（官方出处）

- **IBM 官方下载页**（developer.ibm.com/languages/java/semeru-runtimes/downloads/）版本选择器原文实测：「**Java 8 (LTS) Java 11 (LTS) Java 17 (LTS) Java 21 (LTS) Java 25 (LTS) Java 26 Java 27**」→ 8/11/17/21/25 = LTS，**26 与 27 均无 LTS 标注**。
- 注意纠偏：**27 不是 LTS**。JDK 21 起 LTS 节奏为每两年（21→25→**29**），26（2026-03）与 27（2026-09）都是短期版。佐证：Adoptium `/v3/info/available_releases` 实测 `available_lts_releases=[8,11,17,21,25]`、`most_recent_lts=25`。
- IBM Support 「IBM Semeru Runtimes support」页（ibm.com/support/pages/semeru-runtimes-support）按 25/21/17/11/8 组织 = 活跃 LTS 支持集；release notes 页覆盖 27/25/21/17/11/8（LTS + 当期短期线），侧证 16/18/19/20/22/23/24 已出支持窗。
- Semeru 8 可用性延长至 **2030-12**（IBM community 公告 "Semeru Runtime for Java 8 availability extension through December 2030"）。
- 任务假设「16/18/19/20/22/24 非 LTS」成立（IBM 下载页已下架这些版本，不在支持页）。

## 对 design 的建议

**ReleaseName 规范候选**（决定 `~/.jvm/versions/semeru-{ReleaseName}` 目录名与用户输入匹配）：

| 候选 | 新格式样例 | 旧格式样例 | 优 / 劣 |
|---|---|---|---|
| A. tag 去 `jdk-` 前缀 | `21.0.13.0` | `21.0.5+11_openj9-0.48.0`、`8u402-b06_openj9-0.43.0` | 与 GitHub tag 一一对应、release 页可直读；8 线旧格式风格杂（`+`/`-`/`jdk8u`），但 temurin 已有 `+` 目录先例 |
| B. 资产名版本串 | `21.0.13.0` | `21.0.5_11_openj9-0.48.0`、`8u402b06_openj9-0.43.0` | 与下载文件名同源，拼接 URL 无需反向转换 |
| C. 规范化四段 `major.minor.security.build` | `21.0.13.0` | `21.0.5.11`、`8.402.0.6`（需自造映射） | 用户最简；但丢 openj9 信息、新旧两代映射规则要长期维护、8 线 build 语义混杂，不推荐做主键 |

建议 A 或 B 作 ReleaseName（信息无损、可逆），C 仅作解析/比较的中间层。

**解析要点**：
- tag 前缀两种（`jdk-` / `jdk8u`），8 线旧格式把 `jdk8u402-b06` 归一为 `8u402b06`；过渡期 `+7.1`→`.1` 且丢 openj9 串是唯一「tag ≠ 资产名直译」的特例，其余均满足「`+`→`_`、去 `jdk-`」。
- 新格式第 4 段是 OpenJ9 refresh（同 CPU 周期内 0→10→15 递增），版本比较需四段数值比较（现成 `CompareVersions` 若按点分数值比较可直接兼容，需确认 `+` 分隔旧格式排序不混排）。
- major 提取：新格式取第 1 段；旧格式 `jdk-21.0.5+11…` 取 `jdk-` 后第 1 段；8 线 `jdk8u402…` 需特判（`8u402` → major 8）。
- 用户输入建议同时接受：四段全串（`21.0.12.15`）、旧全串（`21.0.5+11_openj9-0.48.0`）、以及省略 openj9 串的短形（`21.0.5+11`）——省略形在同 major 内唯一。

**风险**：
- 无 Windows ARM64 → `arch=aarch64` 需 corretto/graalvm 同款统一守卫与报错文案。
- 8 线三代 tag 杂乱（含连字符/无前缀 GA 变体）→ 归一化函数必须表驱动测试。
- 21 线 repo 缺首发 GA → `available` 列表 21 组最早是 21.0.1（非 `21+35`）。
- 未认证 REST 60 次/时：Available 全量 14 发偏重，依赖 available-cache TTL（10 分钟条目级）+ EOL 线长 TTL/静态化缓解；无清华镜像可分流（GitHub release 资产无镜像，temurin-ea 先例直连）。
- sha256.txt 双空格格式与 graalvm 裸 hex 不同，若复用同一解析函数需兼容两种。

**校验和获取**：推荐下载 `.zip.json` 单次获取（`sha256` 字段 + `version` 结构自校验，可顺带核对 `binary_type=="jdk"`、`arch=="x64"`、`os=="windows"`）；`.zip.sha256.txt` 作备选（解析第一列）。

## 未决问题

- IBM 下载页的 XHR 数据端点是否可作为稳定聚合 API（本次从页面文本未探明；若存在可把 Available 降到 1 请求，值得实现期再抓一次浏览器请求确认）。
- 新格式第 4 段是否恒从 0 起步（观察样本 0/10/15，未见过非 0 起步的 GA 初版之外的情形；对实现无硬依赖）。
- 未认证 REST 60 次/时在共享出口 IP（公司 NAT）下的实际可用性，无实测数据。
- `.zip.sig`（GPG 签名?）未调研；若后续要加签名校验再补。
- semeru-jextract-binaries 与本任务无关，未深入。

## 数据来源

- `gh api repos/ibmruntimes/semeru{8,11,17,21,22,25,26,27}-binaries/releases...`（--paginate 全量 + latest + by-tag）
- `gh api repos/ibmruntimes/semeru21-binaries/tags`（首发 GA 查证）
- `gh api graphql`（14 仓 latestRelease 批量）
- `curl https://api.adoptium.net/v3/...`（收录性验证）
- [IBM Semeru Runtimes 下载页](https://developer.ibm.com/languages/java/semeru-runtimes/downloads/)（LTS 标注原文）
- [IBM Semeru Runtimes support](https://www.ibm.com/support/pages/semeru-runtimes-support)、[Semeru Runtimes release notes](https://www.ibm.com/support/pages/semeru-runtimes-release-notes)（支持版本集佐证）
- [Semeru Runtime for Java 8 availability extension through December 2030](https://community.ibm.com/community/user/discussion/ibm-semeru-runtime-for-java-8-availability-extension-through-december-2030)（8 线延期）
