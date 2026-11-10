# update temurin@8 修复 + outdated 提示联动 --all + CLI 输出去装饰性 emoji

## Goal

四个用户报告的问题一次收口：`jvm update temurin@8` 报「无效的版本号」的 bug、
`outdated` 与 `update --all` 的提示联动缺失、版本目录被运行中 java 进程占用时
uninstall/update 的提示改进、CLI 输出装饰性 emoji 清理。

## Requirements

### R1 修复 temurin 8 (jdk8uXXX-bYY) 版本名路由失败 (bug)

- 现象：`jvm update temurin@8` 在确认后走到安装步骤报
  `无效的版本号: jdk8u512-b09 (请输入大版本号, 例如 21、17、11)`。
- 根因：`internal/provider/temurin/temurin.go` 的 `Resolve` 按格式分流，
  `jdk8u512-b09` 既不含 `.` 也不是 `jdk-` 前缀，落入 `ParseMajorVersion` 分支报错；
  `ResolveReleaseName` 也会因不含 `+` 误报「缺少 build 号」。
  Temurin 8 的 release_name 就是 `jdk8u512-b09` 形态（无点、`jdk8u` 前缀、`-bN` build 后缀，
  已实测 Adoptium feature_releases/8/ga 首条与 release_name 端点均为此形态且返回 200）。
- 修复（temurin.go，两处）：
  - `Resolve`：路由条件增加 `strings.HasPrefix(v, "jdk8u")` → release_name 精确查询分支。
    用紧前缀而非 `jdk\d+u` 泛化：u 命名仅 JDK 8 一代（JEP 223 起废除），写死零误报。
  - `ResolveReleaseName`：`jdk8u*` 形态透传（`-bN` 即 build 号，本身是完整 release_name）。
- 受益面：`update temurin@8` 闭环 与 `install [temurin@]jdk8uXXX-bYY` 精确安装共用该路径，一并修好。
- 约束：不改动 `MajorOf`/`CompareVersions`/`ResolveVersion`（已正确处理 8 系形态：
  outdated 分组、目录精确匹配 `stripPrefix` 剥 `jdk` 后按 `8uNNN-bNN` 对称比较，均无 bug）。
- 明确不在范围：`8.0.512+9` 点分 semver 形态输入（API 无此 release_name，必 404，
  且无任何入口产出该串）；`install` 粘贴完整目录名（任何版本都不支持，非本 bug）；
  temurin-ea 的 8 系（Adoptium 无 8 系 EA release）。

### R1b 修复 liberica 8 (8uNNN+N) 同类路由 bug

- 实测 BellSoft API：JDK 8 (x64) 版本串为 `8u504+1` 形态（无点），与 temurin 同病：
  - `liberica.Resolve` 的 majorStr 取「首个 `.` 前整段」→ `ParseMajorVersion("8u504+1")` 报错
    （`update liberica@8` 第二步、`install liberica@8u504+1` 精确安装均触雷）；
  - `isPureMajor` 定义是「不含 `.`」→ 即使放过 majorStr，`8u504+1` 也会被误判成
    纯大版本号查询，静默装「8 系最新」而非用户指定的精确版本。
- 修复（liberica.go，两处）：
  - majorStr 提取改为「开头连续数字」（`8u504+1` → 8，`21.0.5+11` → 21）；
  - `isPureMajor` 收紧为「全数字」。
- 其余发行版核实结论（不在范围、无需改动）：corretto 8 `8.452.09.1` / zulu 8 `8.0.452+9`
  均带点走现有路由 ✓；microsoft/graalvm 无 8 系。

### R1c 版本号语义收敛到 app 层单一事实源 (结构性要求)

- 用户明确要求：版本号问题随发行版增多必然复发，必须封装好、避免多点维护相同代码。
- 现状：「从版本串提取大版本」7 处各写一遍（5 provider 的 majorStr 首点切段 +
  ea.majorOfReleaseName + junction.MajorOf）；「是否纯大版本号」5 份实现、两种语义
  （全数字 vs 不含点，后者是错误定义，liberica 雷即源于此）。
- 要求：
  - `internal/app` 新增规范助手 `IsPureMajor` / `MajorOfVersion`（分工与既有
    `ParseMajorVersion` 严格校验写进文档注释），全形态表测做防回归形态清单；
  - 全部 provider 的 `Resolve` 路由统一为二元判据（IsPureMajor → 大版本取最新，
    否则 → ResolveReleaseName 精确路径），废除格式枚举——新形态只需
    ResolveReleaseName 认识，路由零改动；
  - junction 的 pureMajor/MajorOf 内部委托 app（保留导出签名与目录名职责）；
  - 实施后仓内不再存在 provider 本地 majorStr 提取与 isPureMajor/isPureMajor 定义。
- 技术方案与迁移映射见 design.md。

### R2 outdated 可升级提示联动 update --all (提示文案)

- `update --all`/`-a` 自 v0.14.0 已存在，但 `outdated` 的提示文案只示范单组
  `jvm update <distro@大版本>`，用户无从得知批量入口。
- 修复：`printOutdated` 的提示区在可升级组 ≥ 2 时追加一行
  `全部升级: jvm update --all`；单组时保持现有单组提示不追加（避免噪音）。

### R3 版本目录被运行中 java 进程占用时, uninstall/update 的提示改进 (bug)

- 场景 (用户实报): 某版本目录正被运行中的 java 进程使用 (DLL 已加载),
  `update --all` 删除旧版失败 (宽容路径, 符合设计), 但其善后提示
  「稍后可 jvm uninstall 手动清理」有误导 —— 进程不停, 手动 uninstall 一样失败;
  手动 `jvm uninstall <版本>` 单版本路径硬失败, 裸错误无提示 (cmd.go 单版本
  路径 vs 同文件 --all 路径与 update 路径均有「(可能被进程占用)」提示, 单版本
  反而体验最差)。
- 修复 (纯文案, 不动控制流):
  - `cmd.go` uninstall 单版本删除失败信息: 补「可能被运行中的 java 进程
    (Gradle daemon / IDE / 应用) 占用, 停止相关进程后重试」+ 原始错误, 保持非零退出。
  - `cmd/update.go` 单组与 --all 的「N 个旧版本未能删除」汇总提示: 改为指向
    「可能被运行中的 java 进程占用, 停止相关进程后重试 jvm uninstall 手动清理」。
- 不做 (本轮明确出范围): 枚举/点名占用进程 (需 Toolhelp32 快照等一整块
  Windows API, 轻量任务性价比低); 暂存改名延迟删除 (加载中的 DLL 对 rename
  同样加锁, 方案不成立); 只读属性 chmod 重试 (无证据表明是本场景成因)。
- 附带说明 (无需代码): 旧无前缀目录 (如 17.0.20+8) 因同一把锁导致启动迁移
  `MigrateLegacyDirs` 的 rename 失败而保持旧名, 进程停止后随下次任意 jvm
  命令自动改名, 不需要用户干预。

### R4 CLI 输出去装饰性 emoji (轻量)

- 范围：全部 Go 源码中用户可见输出（stdout/stderr）里的装饰性 emoji 全部移除，
  涉及 🔍 ✅ ⚠️ 📦 🗑️ 🔄 📌 ⚡ 💡 🔐 ⬇️ 📂 🔗 ↩️ ⏭️ 🏥 🔧 ℹ️ 等，
  分布在 `main.go`、`cmd/*.go`、`internal/{jdk,junction,config,doctor,shell,updatecheck,upgrade}`
  与 6 个 provider 的 `Configure` 警告，共 122 处左右（含注释中引用 doctor ✗ 标记的行）。
- 保留：doctor 的 `✓` / `✗` 逐项功能标记（非装饰），含 `--fix` 输出与注释中对 ✗ 的引用。
- 文案本体不动：只删 emoji 与其后跟随的空格，中文提示语保持原语义；
  行内引用 ✗ 的说明文字（如「见上方 ✗ 项」）保留 ✗ 字符本身。
- 约束：`--json` 输出路径不受影响（本就无 emoji）；shell 集成脚本内容必须保持纯 ASCII
  （emoji 只出现在 console 输出，不进脚本）；同步修正断言输出含 emoji 的测试。

## Acceptance Criteria

- [ ] `jvm update temurin@8`（本地装有 jdk8u 旧版、远端有新版时）走完
      查询 → 计划 → 安装 → 切换/清理 全流程，不再报「无效的版本号」。
- [ ] `Resolve` 表驱动单测覆盖 `jdk8u512-b09` / `jdk8u504-b01` 形态路由到
      release_name 分支；`ResolveReleaseName` 覆盖 `jdk8u*` 透传。
- [ ] liberica `8u504+1` 精确安装不再报「无效的版本号」且不被误判为大版本号查询
      （majorStr 提取 + isPureMajor 收紧的表驱动单测）。
- [ ] `app.IsPureMajor`/`app.MajorOfVersion` 存在且带全形态表测；仓内 provider
      不再有本地 majorStr 提取与 isPureMajor/isPureNumber 定义（junction 委托 app）。
- [ ] 真实 API 实测（JVM_HOME 隔离，不污染本机）：dev 构建 `install temurin@8` →
      `outdated` → `update temurin@8` 全链路跑通；`install liberica@8u504+1` 精确装成功。
- [ ] `printOutdated` 可升级组 ≥ 2 时输出包含 `jvm update --all` 提示行，单组时不出现。
- [ ] uninstall 单版本删除失败时错误信息含「运行中的 java 进程」占用提示且非零退出；
      update 单组/--all 的「未能删除」汇总提示不再误导用户「手动清理必能成功」。
- [ ] 全仓非测试 Go 源码中不再出现装饰性 emoji（doctor 的 ✓/✗ 及其注释引用除外）；
      `make fmt` / `make vet` / `make test` 全绿。
- [ ] CHANGELOG.md 在 [Unreleased] 下按分类记录（修复 ×2：temurin8 路由、删除失败提示；
      变更 ×2：outdated 提示联动、emoji 清理）。

## Notes

- 用户环境（Administrator 机器）装有 temurin 8/17/21/25，修复后建议用户自测
  `jvm outdated` → `jvm update temurin@8`；集成脚本已有 outdated/update 用例，
  8 系形态如需覆盖可在单测层完成（不动集成脚本范围）。
- emoji 清理只动文案常量，不改任何控制流；`↩️`（U+21A9+FE0F）等组合字符注意删干净。
