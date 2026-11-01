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
