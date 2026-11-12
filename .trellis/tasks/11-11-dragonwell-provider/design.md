# Design: Alibaba Dragonwell provider

> 事实依据见 research.md；本文件只记决策与结构。

## 1. 模块边界

- 新包 `internal/provider/dragonwell/`，单文件 `dragonwell.go` + `dragonwell_test.go`。
- 嵌入 `provider.Base`，`Name()="dragonwell"`、`DisplayName()`，四入口 + `init()` 注册。
- `Configurable.Configure(arch, mirror)`：只消费 arch；MirrorURL 恒空（GitHub 直连）。

## 2. 数据源与端点

- 基址 `https://api.github.com/repos/dragonwell-project/`，repo = `dragonwell{major}`。
- **majors 写死 `{8, 11, 17, 21, 25}`，全部 LTS**（25 线已有 4 个 Standard GA，调研拍板纳入；新 LTS 线手动维护）。
- `Available()` 静态列表（零请求）。
- **四入口统一走 `GET /releases?per_page=100` + 本地过滤**（research Q7 建议，5 仓 ≤50 条单页全量）：
  - `ListVersions(major)`：过滤后全量，`CompareVersions` 降序
  - `LatestPatch(major)`：过滤后首条（**不用 `/releases/latest`**——风味不保真实证）
  - `Resolve`：过滤后列表内匹配
  - 三个方法共用一次拉取 + 进程内按 major 缓存（CLI 单进程单命令，简单 map 即可，无 TTL 需求）
- 限流错误文案同 semeru 约定。

## 3. 解析契约

- **GA/风味过滤**：tag 前缀 `dragonwell-standard-` **且** 后缀 `-ga`（`.prerelease` 字段不可用实证；extended/preview/裸前缀/`+0` 变体/古董全排除）。**额外要求**：release 内存在合格 Windows 资产（无资产的 release 不入表）。
- **ReleaseName = tag 去前缀、去 `_jdk-...-ga` 尾巴**（dragonwell 侧版本原样）：`21.0.12.0.12+8` / `25.0.4.0.4+7` / `11.0.32.28` / `8.30.29`。组内唯一（update 号单调递增）、`java -version` 输出一致。
- **资产挑选**：assets 内前缀 `Alibaba_Dragonwell_Standard_` + **后缀 `_x64_windows.zip` 锚定结尾**（排除 `-fix.zip`/`.sha256.txt`/`.zip.json`/`-sbom.json`/`.sig`）。**禁止由 tag 拼资产名**（11 线 `.9` build 段反例）。
- **Resolve 双分支**（version-parsing.md）：`IsPureMajor` → LatestPatch；否则 ReleaseName 精确匹配（无短形，名字本身无冗余尾巴）。
- **版本比较**：`app.CompareVersions` 直接复用（`25.0.4.0.4+7` → [25,0,4,0,4,7]；`8.30.29` → [8,30,29]）。新形态登记 `internal/app/version_test.go`。
- major 提取 `app.MajorOfVersion`。
- `browser_download_url` 原样使用（`%2B` 已编码，勿二次编解码）。

## 4. 校验和

- 同 release 资产 `<zip名>.sha256.txt`：**两列格式但第二列是内部构建路径**——`strings.Fields()[0]` 取第一列 hex，不按文件名匹配。`ChecksumAlgo` 默认 sha256。

## 5. 架构守卫

`arch == "aarch64"` 四入口统一报「Dragonwell 不提供 Windows ARM64 构建，请改用 temurin 或 microsoft」（corretto/graalvm 同款哨兵）。

## 6. 测试策略

- 表驱动：三线 tag → ReleaseName（21/25 含 `+build`、11 无 build、8 短形）、风味过滤（standard/extended/preview/裸前缀/`+0`/`_GA` 古董）、资产挑选（`-fix.zip` 排除、`.sha256.txt` 排除）、sha256.txt 内部路径列解析、aarch64 守卫、混排排序、11 线 extended 版本号更高时不被误选（前缀过滤在前）。
- `internal/app/version_test.go` 登记：`21.0.12.0.12+8`、`11.0.32.28`、`8.30.29`、`25.0.4.0.4+7`。

## 7. 权衡记录

- 列表拉取 vs latest 端点：列表（风味保真 + 一拉多用）；代价 1 请求/仓 vs latest 同价，无损失。
- 25 线纳入：事实零成本（路由一行），与 21 线同构。
- 拆分前裸前缀（2022-08 前 Standard 前身）不入表：避免版本归一复杂度；早期用户可用官方渠道，损失极小。
- `.zip.json` 旁路不接：同 semeru 决策。
