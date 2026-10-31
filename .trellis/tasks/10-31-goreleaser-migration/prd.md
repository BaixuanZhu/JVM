# GoReleaser 发版管线迁移（子任务 1/3）

> 顺序约束：本任务是三个子任务的基座，**必须最先完成**。winget-automation 与 scoop-distribution 依赖本任务产出的稳定资产产线。父任务：`10-31-release-automation`。

## Goal

用 GoReleaser 接管发版产线（编译双架构二进制、便携 zip、checksums.txt、GitHub Release 创建与 notes 注入），NSIS 安装器保留 `installer/jvm.nsi` 走 before hook + `release.extra_files`（混合模式）。存量消费者（`jvm upgrade` / `install.ps1`）零感知。

## 已确认事实（证据锚点）

- 资产名契约与三消费者、checksums 逐行匹配容错、NSIS 定制逻辑清单、单测 gate、CHANGELOG notes 抽取、版本注入语义：见父任务 `10-31-release-automation/prd.md` Background 节（单一事实源，此处不重复）。
- `installer/jvm.nsi:20,36,69` 的路径约定：以脚本所在目录（installer/）为基准引用 `..\dist\<arch>\jvm.exe`，`OutFile ..\dist\jvm-windows-<arch>-setup.exe`——与 GoReleaser 自己的 `dist/` 布局不同，需要桥接（拷贝或改 jvm.nsi 路径约定，design.md 决定）。
- Makefile 目标去留现状：`build`（开发，Bootstrap=off）/ `build-dist` / `installer`（=build-dist+makensis）/ `release`（=build-dist+Compress-Archive）/ `dist-all`；goreleaser 接管后 `build-dist`/`release`/`dist-all` 的发版职责被替代。

## Requirements

1. 新增 `.goreleaser.yaml`：
   - `builds`：GOOS=windows、GOARCH=[amd64, arm64]、ldflags `-s -w -X jvm/internal/app.Version={{.Version}}`（不注入 Bootstrap，等价 build-dist 语义）、`-trimpath`；
   - `archives`：产物名 `jvm-windows-{{ .Arch }}.zip`，zip 内单个 `jvm.exe`；
   - `checksums`：文件名必须是 `checksums.txt`（goreleaser 默认是 `<project>_checksums.txt`，会被 install.ps1/upgrade.go 找不到）；setup.exe 与 install.ps1 走 `extra_files`，默认不进 checksums，维持现状（只含两个 zip）；
   - `before.hooks`：`go vet ./...` + `go test ./...`（单测 gate 保留，红则中止）；
   - `release.header`：before hook 里用现有 awk 逻辑从 CHANGELOG.md 抽版本段落生成 header 文件（AGENTS 硬约束），抽不到时的回退行为需明确；
   - NSIS：before/构建环节继续调 `makensis`（choco 安装 NSIS + GITHUB_PATH 的既有处理搬迁），产出 setup.exe ×2 经 `release.extra_files` 上传；
2. 改写 `.github/workflows/release.yml` 为 goreleaser 编排（tag 触发、windows-latest、choco nsis、GoReleaser action 版本 pin）；
3. Makefile 清理：删除发版职责被替代的目标（`build-dist`/`release`/`dist-all`）或明确保留理由，`make build`（开发）与 `installer`（本地手打安装器）保留；`make help` 同步；
4. AGENTS.md「Setup commands / 发布」章节改写为新管线；CHANGELOG.md 记录。

## Acceptance Criteria

- [ ] `goreleaser release --snapshot --skip=publish` 本地产出：两个 zip（命名正确、内含单 jvm.exe）、setup.exe ×2、checksums.txt（恰好两行、sha256sum 文本格式）——与旧管线产物结构逐项对比一致；
- [ ] snapshot 产出的二进制运行 `jvm.exe version` 显示注入版本且启动自举行为与 build-dist 一致（Bootstrap 未关闭）；
- [ ] 单测 gate 生效：`before.hooks` 中 `go test` 失败时发版中止；
- [ ] 真实发版（配合父任务集成验收）：打 tag 后 Release 页面出现 5 类资产 + 从 CHANGELOG 抽取的 body；
- [ ] 旧版 `jvm upgrade` 指向新 release 自更新成功（资产名匹配 + checksums 校验通过）；
- [ ] `install.ps1` 从新 release 安装成功；
- [ ] AGENTS.md / CHANGELOG.md 已同步。

## Out of Scope

- winget / scoop 相关编排（子任务 2/3）；
- 改动 `internal/upgrade`、`install.ps1` 的逻辑（它们是兼容性验收的裁判，不能既改裁判又改运动员）；
- goreleaser 的其它打包格式（deb/rpm/msi/msix）；
- ci.yml 与集成测试触发方式。

## Open Questions

- 无阻塞项。混合模式（NSIS 保留 make）已由 jvm.nsi 定制逻辑证据决定；goreleaser dist 布局与 jvm.nsi 路径约定的桥接方式属 design.md 细节。
