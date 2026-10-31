# Implement: GoReleaser 发版管线迁移

## 执行清单（有序）

1. [ ] `scripts/gen-release-header.sh`：从 `release.yml:86-112` 搬移 awk 逻辑；输入 CHANGELOG.md、输出现版本段落到 `dist/release-header.md`；抽不到产出空文件（goreleaser changelog 兜底）。bash + ASCII 注释。
2. [ ] `.goreleaser.yaml`：按 design.md 骨架。先 `goreleaser check` 通过。
3. [ ] 本地 snapshot 验证（前置：`scoop install goreleaser`；本机 NSIS 已配？`makensis /VERSION` 不通则先装）：
   - [ ] `goreleaser release --snapshot --skip=publish --clean` 全程绿（含 before hooks：vet/test/gen-header/make installer ×2）；
   - [ ] 产物矩阵逐项核对（design.md「兼容性验收矩阵」）：zip 名 ×2、zip 内容、checksums.txt 两行格式、setup.exe ×2；
   - [ ] snapshot exe 跑 `version` 确认版本注入与自举语义（用 `JVM_HOME` 指向临时目录隔离，防污染开发机）；
   - [ ] before hook 的 `{{ .Version }}` 展开正确（make installer 日志里的版本号）。
4. [ ] 改写 `.github/workflows/release.yml`：checkout → setup-go → choco nsis + GITHUB_PATH（原样保留）→ goreleaser-action（pin 大版本 + goreleaser CLI 显式版本）。权限 `contents: write` 保持。
5. [ ] Makefile：删 `release`/`dist-all` 目标与 help 对应行，更新头部注释；删除前 `grep -rn "dist-all\|make release" .github/ scripts/ Makefile` 确认无引用。
6. [ ] `AGENTS.md`「Setup commands / 发布」段落改写为新管线（GoReleaser、goreleaser snapshot 验证方式、Makefile 目标变化、发版流程不变的部分：CHANGELOG 硬约束、tag 触发）。
7. [ ] `CHANGELOG.md` `[Unreleased]` 记录（变更类：发版管线迁移 GoReleaser，用户可见的安装方式无变化）。
8. [ ] `make fmt` / `make vet` / `make test` 全绿（本任务不改 Go 代码，理论上零影响）。
9. [ ] 提交（Conventional Commits，参考历史风格；建议 `build(release): 发版管线迁移 GoReleaser, NSIS 安装器保留 Makefile 混合产出`）。

## 验证命令汇总

```bash
goreleaser check
goreleaser release --snapshot --skip=publish --clean
ls dist/                          # 5 类产物名核对
unzip -l dist/jvm-windows-amd64.zip
cat dist/checksums.txt            # 恰好两行
./dist/amd64/jvm.exe version      # 版本号
make test
```

## 风险文件与回滚点

- `.github/workflows/release.yml`：唯一影响产线的文件，出问题 `git revert` 整批提交即可回旧管线。
- `.goreleaser.yaml` / `scripts/gen-release-header.sh`：新增文件，revert 即消除。
- `Makefile`：删目标前 grep 引用（清单第 5 步）；`make build` 开发链路不受影响。
- 真实发版窗口：合并 main 后的下一次 tag；届时执行父任务集成验收清单（`../10-31-release-automation/prd.md`），失败则 revert + 删 release + 重打 tag。

## start 前检查

- [ ] `implement.jsonl` / `check.jsonl` 已 curate（spec/guides 与父任务 research 文档）。
- [ ] 本机 goreleaser 可用（或接受仅在 CI 验证——不推荐，snapshot 是本任务的核心护栏）。
