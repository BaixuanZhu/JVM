# 实施清单

前置：design.md 已评审。按序执行，每步后跑对应验证；任一步红了先修再前进。

## A. 版本号收敛 + 8 系修复（R1/R1b/R1c）

- [ ] A1 新建 `internal/app/version.go`：`IsPureMajor` + `MajorOfVersion`，
      文档注释写明分工（vs `ParseMajorVersion`）与「单一事实源」约束
- [ ] A2 `app` 表测：全形态 fixture（`8`/`21` 纯大版本；`21.0.5+11`、`25.0.4.1+1`、
      `8.452.09.1`、`8.0.452+9`、`8u504+1`、`jdk8u512-b09`、`jdk8u504-b01`、
      `jdk-21.0.12+8`、`28+14-ea-beta`；垃圾：``/`abc`/`jdk`/`u45`）
- [ ] A3 `temurin`：`Resolve` 改二元路由（IsPureMajor → latest，否则 exact）；
      `ResolveReleaseName` 放行 `jdk8u*` 透传；表测补 `jdk8u*` 用例
- [ ] A4 `temurin/ea.go`：`isPureNumber`/`majorOfReleaseName` 委托 app 助手（删除本地实现）
- [ ] A5 `liberica`：`Resolve` 用 `app.MajorOfVersion`；`isPureMajor` 换 `app.IsPureMajor`（修 bug）
- [ ] A6 `zulu` / `corretto` / `microsoft` / `graalvm`：同款迁移（等价变换），
      删各自 `isPureMajor`/`majorStr` 本地实现
- [ ] A7 `junction`：`pureMajor` 委托 `app.IsPureMajor`；`MajorOf` 后半段委托
      `app.MajorOfVersion`（保留 SplitDistro 前半段），签名不动
- [ ] A8 验证：`make fmt && make vet && make test` 全绿；
      真实 API 实测（JVM_HOME 隔离）：`install temurin@8` → `outdated` →
      `update temurin@8`（-y，确认装上 `temurin-jdk8uNNN-bNN`）；
      `install liberica@8u504+1` 精确装

## B. outdated 提示联动 --all（R2）

- [ ] B1 `printOutdated`：可升级组 ≥2 时追加提示行（含 `jvm update --all`）；
      单组不追加
- [ ] B2 表测/既有输出测试同步（若 printOutdated 无纯函数测试则补最小断言）

## C. 删除失败提示改进（R3）

- [ ] C1 `cmd.go` uninstall 单版本：删除失败信息补「可能被运行中的 java 进程
      占用，停止相关进程后重试」+ 原始错误，保持非零退出
- [ ] C2 `cmd/update.go` 单组与 --all 的「未能删除」汇总提示改口径
      （不再暗示手动 uninstall 必能成功）
- [ ] C3 相关单测断言同步

## D. emoji 清理（R4）

- [ ] D1 全仓非测试 Go 源码去装饰性 emoji（清单见 PRD；保留 doctor ✓/✗
      及行内对其的引用），逐文件删 emoji + 其后空格，文案本体不动
- [ ] D2 `internal/jdk/jdk_test.go` 等断言含 emoji 的测试同步
- [ ] D3 复扫：python 脚本全仓扫描仅剩 ✓/✗ 白名单；`make fmt/vet/test` 全绿

## E. 收尾

- [ ] E1 CHANGELOG.md `[Unreleased]` 记录（修复×2、变更×2）
- [ ] E2 spec 沉淀：`.trellis/spec/backend/` 新增或补充「版本号解析规范」
      （新 provider 必须用 app 助手、禁止本地 majorStr/isPureMajor）
- [ ] E3 提交（Conventional Commits，可拆 2-3 笔：fix(providers) 版本号收敛+8系、
      feat(cmd) outdated 联动+删除提示、style 全仓 emoji）+ journal + 归档

回滚点：每节独立成 commit，任一节出问题 `git revert` 对应提交即可，节间无隐式依赖
（D 节纯文案、B/C 节纯提示，与 A 节代码正交）。
