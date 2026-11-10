# 版本号解析收敛到 app 层 + update temurin@8 等修复 — 技术设计

## 背景与动机

`jvm update temurin@8` 报「无效的版本号」的根因不是缺一个 if，而是**版本号语义多点维护**：

- 「从版本串提取大版本」在 5 个 provider（各自的 `majorStr` 首点切段）、
  `temurin/ea.go`（`majorOfReleaseName`）、`junction`（`MajorOf`）共 7 处各写一遍；
  首点切段遇到无点形态（`jdk8u512-b09` / `8u504+1`）全部失效。
- 「是否纯大版本号输入」有 5 份实现、两种语义（全数字 vs 不含点），
  「不含点」是错误定义——liberica `8u504+1` 被 `isPureMajor` 误判成大版本号查询，
  会静默装成「8 系最新」而非用户指定的精确版本。

发行版越多、版本号形态越多（已实测：Temurin 8 `jdk8u512-b09`、Liberica 8 `8u504+1`、
Corretto 8 `8.452.09.1`、Zulu 8 `8.0.452+9`），这个结构性问题越容易复发。
本次借修 bug 之机把语义收敛到 `internal/app` 单一事实源。

## 设计

### 1. app 层新增两个规范助手（`internal/app/version.go` 新文件）

```go
// IsPureMajor 报告 s 是否为纯数字大版本号 ("8"/"21") —— 用户输入中唯一表示
// "取该大版本最新版"的形态。带 patch/build/更新号的串都不是。全部 provider
// 共用的路由判据，单一事实源。
func IsPureMajor(s string) bool

// MajorOfVersion 从版本串提取大版本号：先剥可选 "jdk-"/"jdk"/"JDK-"/"JDK"
// 前缀，再取开头连续数字。"21.0.5+11"→21、"8.452.09.1"→8、"8u504+1"→8、
// "jdk8u512-b09"→8、"8"→8；解析不出正整数返回 0（调用方自行决定报错文案）。
func MajorOfVersion(s string) int
```

与既有 `ParseMajorVersion` 的分工：后者是**严格校验**（update/uninstall `--all`
要求用户必须输大版本号），保留不动；前者两兄弟是**宽容提取/判据**，供路由用。

### 2. provider 路由语义统一：废除格式枚举

六个 provider 的 `Resolve` 统一收敛为二元路由，**不再枚举版本串格式**：

```
IsPureMajor(v)  →  大版本取最新路径
否则            →  ResolveReleaseName(v) 精确版本路径（各发行版自己归一化名字）
```

- `temurin.Resolve`：现 `ContainsAny(".")||HasPrefix("jdk-")`（+需补的 `jdk8u`）
  枚举条件整体删除，换成上述二元路由。**结构性收益**：未来新增任何怪形态
  （新发行版、新命名），只需其 `ResolveReleaseName` 认识该形态，路由零改动。
- `liberica`/`zulu`/`corretto`/`microsoft`/`graalvm` 的 `Resolve`：`majorStr`
  首点切段 + `ParseMajorVersion` 三行舞 → `app.MajorOfVersion(v)`（0 值报错
  「无效的版本号」）；各自的 `isPureMajor` → `app.IsPureMajor`。
  其中 zulu/corretto/microsoft/graalvm 行为不变（点分形态两语义等价），
  liberica 是修 bug（无点形态）。
- `temurin/ea.go`：`isPureNumber` → `app.IsPureMajor`；`majorOfReleaseName` → `app.MajorOfVersion`。

### 3. junction 去重（保持导出签名，内部委托）

`junction` 可安全依赖 `app`（app 是零内部依赖的叶子层，无循环）：

- `MajorOf(s)`：保留「SplitDistro 剥 distro 前缀」这层目录名职责，后半段
  （剥 jdk 前缀 + 取开头数字）委托 `app.MajorOfVersion`。
- `pureMajor(s)`：改为薄封装委托 `app.IsPureMajor`。

### 4. 修复点（收敛后自然落地）

- `temurin.ResolveReleaseName`：`jdk8u*` 形态透传（`-bN` 即 build 号，
  本身是完整 release_name；已实测 Adoptium release_name 端点对
  `jdk8u512-b09` 返回 200）。
- liberica 8 `8u504+1` 由 §2 的路由修复自然覆盖，无需单独补丁。

### 5. 无需改动（已核实）

- `app.CompareVersions`/`versionSegments`：按非数字切段，`8u504+1` 与
  `jdk8u512-b09` 天然归一为 `[8,512,9]` 比较，形态无关，单点实现 ✓。
- `junction.ResolveVersion`/`stripPrefix`：剥 `jdk` 前缀后两侧对称归一
  `8uNNN-bNN` 精确匹配 ✓（use/uninstall/.jvmrc/exec 的 8 系输入本来就没坏）。
- `MigrateLegacyDirs`/`legacyToNewName`：裸 `jdk8uNNN-bNN` 旧目录迁移 ✓。

## 兼容性

- 之前好使的输入全部行为不变（`8`、`21.0.5+11`、`temurin@21.0.5+11`、
  `jdk-21.0.12+8`、各发行版点分形态）。
- 新增好使：`[temurin@]jdk8uNNN-bNN`（temurin 精确安装 + update 闭环）、
  `[liberica@]8uNNN+N`（liberica 精确安装 + update 闭环）。
- 纯垃圾输入（如 `abc`）在 temurin 下的报错从「无效的版本号」变为
  「缺少 build 号」，语义仍然成立，可接受。

## 测试策略

- `app` 表驱动：`IsPureMajor`/`MajorOfVersion` 全形态 fixture
  （七种真实发行版版本串 + 垃圾输入），这是**防回归的形态清单单一事实源**。
- `temurin`：`ResolveReleaseName` 的 `jdk8u*` 透传表测。
- 其余 provider：迁移是等价变换，既有单测保持全绿即回归防护。
- 真实 API 实测（`JVM_HOME` 隔离，不污染本机）：`install temurin@8` →
  `outdated` → `update temurin@8` 全链路；`install liberica@8u504+1` 精确装。

## 明确不做

- 不改 `ParseMajorVersion` 严格校验语义（update/uninstall --all 仍只收大版本号）。
- 不支持 `install` 粘贴完整目录名（任何版本都不支持，与本次无关）。
- 不为 temurin-ea 补 8 系形态（Adoptium 无 8 系 EA release）。
- 不做进程占用枚举/暂存改名（见 prd R3 的出范围说明）。
