# Journal - zz (Part 1)

> AI development session journal
> Started: 2026-10-31

---

## 2026-11-01: GoReleaser 发版迁移实现 (10-31-goreleaser-migration)

- 父任务 release-automation 子任务 1/3：GoReleaser 混合模式接管发版产线，NSIS 留 Makefile。
- 实现两处相对 design 的偏差（实测驱动）：① `dist: release-dist` 独立输出目录（goreleaser 要求
  before hooks 后其 dist 为空，jvm.nsi 硬编码 dist/ 禁改）；② release.header 文件加载是 Pro 专属，
  body 改 CI 末步 `gh release edit --notes-file`。
- snapshot 矩阵全绿；trellis-check 无 CRITICAL。契约沉淀至 .trellis/spec/backend/release-pipeline.md。
- 坑：snapshot 自举验证裸跑一次会写用户 PATH（D:\code\jvm\dist\amd64），已用 internal/env
  .RemoveFromUserPath 一次性程序清理。后续跑发行风味二进制必须带 JVM_NO_BOOTSTRAP=1。
- 留给真实发版窗口：extra_files 上传、gh release edit、旧版 upgrade / install.ps1 全链路（父任务集成验收）。

## 2026-11-01 (续): CI 演练验证

- release.yml 加 workflow_dispatch 演练模式（snapshot + skip=publish，不建 Release）。
- CI run 41254509869 全绿：五条 before hooks 全跑（VERSION=0.15.0-SNAPSHOT-0a67c23 展开
  正确）、双架构、zip×2 + checksums、notes 覆盖步按条件跳过、gh release list 无新增。
- 剩余未验证项收窄为 publish 阶段专属行为：extra_files 上传、gh release edit 真实 tag、
  旧版 upgrade / install.ps1 消费链（父任务集成验收）。

## 2026-11-01 (续2): v0.16.0 首次真实发版 + 集成验收

- goreleaser-action v6 → v7（演练 run 41255176801 绿，Node20 弃用警告消失）。
- CHANGELOG 落段 → tag v0.16.0 → 新管线首次真实发版 run 41255649049 全绿
  （extra_files 上传 + gh release edit 两个 publish 阶段首验通过）。
- 验收通过：6 资产齐 + body 来自 CHANGELOG 段 + checksums 两行格式 + install.ps1
  隔离目录安装（SHA256 校验链路，注册表 InstallDir 用后已还原）。
- 唯一未闭环：旧版 jvm upgrade 自更新——本机 IP 匿名 API 配额被持续耗干（403），
  已挂 35 分钟后一次性重试（automation-5851c30f）。教训：jvm 的 updatecheck 与
  upgrade 都走匿名 API，开发机跑命令多了会把验收窗口挤掉；可考虑 CI runner 验。
- 2a/2b 已补 implement.md + jsonl 上下文，manifest 锚定 v0.16.0。

## 2026-11-01 (续3): 2a/2b 本地实现完成

- 2a winget：多文件 manifest（singleton 装不下双架构，schema maxItems=1）三件套落
  artifacts/winget-pkgs/，winget validate 绿，setup.exe sha256 本地实算（不在 checksums.txt）。
- 2b scoop：artifacts/jvm.json，hash regex $sha256\s+$basename 三重依据（scoop 源码/Extras
  先例/实测脚本），免 bucket 直装 0.16.0 + 卸载实测干净；自举 PATH 死链已清理。
- 均为外部仓库动作待用户批准：winget-pkgs New Package PR、Extras PR、WINGET_TOKEN 创建。
- jvm upgrade 验收移交用户本人。

## 2026-11-01 (续4): 两个外部提交的结局

- winget-pkgs New Package PR 已提: #467355（fork 新建 + 稀疏克隆 + 多文件 manifest，
  中文注释已剥离；CLA 待用户签、CI 安装验证待跑）。
- **Extras 挂起**：Package Request 有 required 知名度门槛（≥100 stars/50 forks），
  本仓库 4 stars 差一个数量级，如实提交必被拒。用户拍板挂起等知名度，不自建 bucket。
  2b 本地产物保留在 artifacts/，重启时从提 PR 步骤开始。
- 教训记入：向第三方仓库提交前先查 PR **和 issue** 模板的 required 门槛，研究阶段
  只看了 PR 面。

- jvm upgrade 自更新验收通过（用户执行 + 主控复核 jvm 0.16.0、无 .bak 残留）。
  子任务 1 全部验收项闭环；父任务集成验收 4/6，余两项锁在外部事件
  （winget PR #467355 合并 + 下次发版、scoop 线挂起）。

## 2026-11-10 fix: 版本号收敛 + outdated 联动 + emoji 清理 (11-10-update-jdk8-fix-and-emoji-cleanup)

用户报三问题: (1) `update temurin@8` 确认后报「无效的版本号: jdk8u512-b09」;
(2) outdated 与 update --all 无联动提示; (3) 装饰性 emoji 冗余。过程中追加第
四项: 版本被运行中 java 进程占用时 uninstall 裸报错、update 善后建议误导。

**关键决策——收敛而非补丁**: 用户明确要求封装好、避免多点维护。盘点出
「从版本串提大版本」12 处、「是否纯大版本号」5 份两种语义的重复实现;
liberica 8 (`8u504+1`, 实测 BellSoft API) 同病且更阴 (isPureMajor「不含点」
把精确版本误判成取最新)。方案: `internal/app/version.go` 三助手
(IsPureMajor / MajorOfVersion / ParseMajorOfVersion) 单一事实源, 全 provider
Resolve 统一二元路由废除格式枚举, junction 内部委托。spec 沉淀见
`.trellis/spec/backend/version-parsing.md` (版本串形态注册表 = app 表测
fixture)。注意 ✓(U+2713)/✗(U+2717)/✅(U+2705) 同处 Dingbats 区段, emoji
清理必须按显式字符集处理。

诊断备忘: `17.0.20+8` 无前缀目录是**永久容忍**态 (MigrateLegacyDirs 把纯
semver 视为已是新规范, 不改 distro 前缀名); 「进程停止后自动改名」说法错误,
裸 jdk-*/jdk8u* 目录才会迁移。Access is denied on extnet.dll = 运行中 JVM
加载的 DLL 不可删 (rename 同样被锁, 暂存改名方案不成立)。

验证: app 表测全形态 fixture; JVM_HOME 隔离实测 install temurin@jdk8u504-b01
→ outdated → update temurin@8 闭环 + liberica@8u504+1 精确装; make
fmt/vet/test 全绿; emoji 复扫仅剩 doctor ✓/✗。提交三笔:
0db701a(fix provider) / ed29464(fix cmd) / 0e54f9b(style)。

遗留: 用户机器 (Administrator) 需 `jvm upgrade` 后自测 temurin@8 升级;
被占用的 17.0.20+8 停进程后 `jvm uninstall temurin-17.0.20+8` 即可。
