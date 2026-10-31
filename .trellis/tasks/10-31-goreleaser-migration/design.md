# Design: GoReleaser 发版管线迁移

> 研究依据：`../10-31-release-automation/research/release-pipeline-facts.md`（能力边界、生命周期顺序、资产契约矩阵）。

## 架构与边界

```
push tag v*
  └─ release.yml (windows-latest)
       ├─ checkout / setup-go 1.26
       ├─ choco install nsis + GITHUB_PATH（既有处理原样保留）
       └─ goreleaser-action（pin 版本）
            └─ goreleaser release --clean
                 ├─ before.hooks:
                 │    ├─ go vet ./...
                 │    ├─ go test ./...            ← 单测 gate（红则中止，不建 release）
                 │    ├─ bash scripts/gen-release-header.sh   ← CHANGELOG → dist/release-header.md
                 │    ├─ make installer VERSION={{.Version}} GOARCH=amd64
                 │    └─ make installer VERSION={{.Version}} GOARCH=arm64
                 ├─ builds:  windows amd64/arm64（ldflags 注入 Version，不注入 Bootstrap）
                 ├─ archives: jvm-windows-{{.Arch}}.zip（内含单 jvm.exe）
                 ├─ checksums: checksums.txt（默认只含 archives，即两个 zip —— 与现状一致）
                 └─ release: header=dist/release-header.md
                      extra_files: setup.exe ×2 + install.ps1
```

改动面：新增 `.goreleaser.yaml`、`scripts/gen-release-header.sh`；改写 `release.yml`；`Makefile` 删 `release`/`dist-all` 目标；`AGENTS.md` 发版章节、`CHANGELOG.md`。**不动** `internal/`、`install.ps1`、`installer/jvm.nsi` 的逻辑（它们是兼容性验收的裁判）。

## 关键决策

1. **混合模式（NSIS 留 Makefile）**——goreleaser NSIS 模板覆盖不了 jvm.nsi 的定制逻辑（研究文档「GoReleaser 能力边界」）。NSIS 与 goreleaser 产物完全解耦：`make installer` 自带 `build-dist` 依赖自行编译，不消费 goreleaser 产物。**编译次数与现状持平**（现状 4 次：显式 build-dist ×2 + installer 内嵌 ×2；新方案 goreleaser builds ×2 + installer 内嵌 ×2）。setup.exe 与 zip 内的 exe 来自不同次编译——与现状语义一致，版本号同源注入。
2. **NSIS hook 落点 = goreleaser `before.hooks`**——生命周期顺序保证 `--clean` 先清 dist，before hook 产出的 setup.exe 不会被清且早于 extra_files 收集；`{{ .Version }}` 模板变量在 hook 命令里可用（实现时验证，失败则回退为 workflow step 在 goreleaser 前手动跑 + 去掉 `--clean` 改用独立输出目录）。
3. **checksums 现状保持**——只含两个 zip（extra_files 默认不进 checksums）。snapshot 验证若发现 goreleaser 行为不符，再显式配置排除。
4. **Release notes**——`scripts/gen-release-header.sh` 永远产出文件：抽到段落写段落，抽不到产出空文件，body 由 goreleaser 默认 changelog 兜底（等价现状的 `generate_release_notes` 回退）。
5. **过渡策略：直接替换，不双轨**——回滚 = `git revert`（release.yml + .goreleaser.yaml 同一提交批次）+ 必要时删 release 重打 tag。首版 tag 即真实验证窗口，由父任务集成验收覆盖。
6. **Makefile 目标去留**——删 `release`、`dist-all`（职责被 goreleaser 接管）；保留 `build`（开发）、`build-dist`（installer 的内部依赖 + 本地发行风味编译）、`installer`（本地手打安装器）。删除前 grep `.github/` 与 `scripts/` 确认无引用。
7. **goreleaser-action 版本 pin**——action 大版本 pin（@v6），goreleaser CLI 版本在 workflow 里显式 `version:` 固定（防止 latest 漂移破坏产线）。

## goreleaser 配置草案（骨架，实现时以 `goreleaser check` + snapshot 产物为准）

```yaml
version: 2
project_name: jvm
before:
  hooks:
    - go vet ./...
    - go test ./...
    - bash scripts/gen-release-header.sh
    - make installer VERSION={{ .Version }} GOARCH=amd64
    - make installer VERSION={{ .Version }} GOARCH=arm64
builds:
  - id: jvm
    main: .
    goos: [windows]
    goarch: [amd64, arm64]
    binary: jvm
    flags: [-trimpath]
    ldflags: [-s, -w, -X jvm/internal/app.Version={{ .Version }}]
archives:
  - id: portable
    ids: [jvm]
    formats: [zip]
    name_template: "jvm-windows-{{ .Arch }}"
checksum:
  name_template: "checksums.txt"
release:
  header: dist/release-header.md
  extra_files:
    - glob: "dist/jvm-windows-amd64-setup.exe"
    - glob: "dist/jvm-windows-arm64-setup.exe"
    - glob: "install.ps1"
```

## 兼容性验收矩阵（snapshot 对比 + 真实发版双验）

| 项 | 断言 |
|---|---|
| zip 名 | `jvm-windows-amd64.zip` / `jvm-windows-arm64.zip`（goreleaser `.Arch` 原样输出） |
| zip 内容 | entry 名 basename 为 `jvm.exe`（是否包顶层目录两消费者均容错） |
| checksums.txt | 恰好两行、`<hash>  <filename>` 格式、文件名正确 |
| setup.exe | ×2 存在、`jvm.nsi` 原样产出（零改动） |
| exe 版本 | `jvm.exe version` 输出注入版本；Bootstrap 未关闭（发行语义） |
| Release body | CHANGELOG 对应版本段落 |
| 自更新 | 旧版 `jvm upgrade` 对新 release 全链路成功 |

## 风险与回滚

- **主要风险**：goreleaser 模板变量在 before hook 的可用性（决策 2 的验证点）；zip 打包结构细节。两者均由本地 snapshot 验证兜住后才改 release.yml。
- **回滚**：单批次提交 → `git revert`；已发的坏 release 可删除重打 tag（发版资产消费者按 `releases/latest` 直链拉取，重建后自动恢复）。
