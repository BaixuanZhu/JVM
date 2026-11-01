# Scoop manifest 交付说明（jvm @ Extras）

本目录是任务 `10-31-scoop-distribution` 的本地交付物。**提 PR 时只提交 `jvm.json` 一个文件**（放入 fork 的 `bucket/jvm.json`）；`verify-regex.ps1` 与本 README 仅为本地验证与依据记录，不进 Extras。

## jvm.json 字段依据

| 字段 | 值 / 写法 | 依据 |
|---|---|---|
| `version` | `0.16.0` | 当前 latest release（2026-11-01） |
| `description` | 英文一句话 | Extras 惯例（参考 `bucket/oras.json`） |
| `homepage` | 仓库地址 | — |
| `license` | `MIT` | 已核对仓库根 `LICENSE`（MIT, Copyright BaixuanZhu），勿凭记忆写 |
| `architecture.64bit/arm64` | amd64 / arm64 便携 zip 直链 | PRD 拍板：消费便携 zip，不用 setup.exe |
| `hash` ×2 | 见下 | 本机独立下载两个 zip 后 `sha256sum` 实算，与 release `checksums.txt` 逐行一致（双空格 GNU 格式） |
| `bin` | `jvm.exe` | zip 内单根级 jvm.exe，scoop 自动 shim |
| `checkver` | `{"github": "https://github.com/BaixuanZhu/JVM"}` | 见下「checkver 依据」 |
| `autoupdate.hash` | `url: $baseurl/checksums.txt` + `regex: $sha256\s+$basename` | 见下「autoupdate hash 依据」 |

### 两个 hash（独立验证，非抄录）

- amd64：`e2b163f0304fa4dcdb6bb59ccdba98245fb11be43f3ca1d5bc8c9ae58857726b`
- arm64：`5062924ace22975c10e27cfa7ecccf5accd0cf7e708d2fe4d23274180d16ec5c`

验证链：本机 curl 下载 `checksums.txt` + 两个 zip → `sha256sum` 实算逐一相符；随后 `scoop install` 安装时 scoop 自身又校验通过（"Checking hash ... OK"）。

### autoupdate hash 依据（regex 写法，按任务要求查证）

- 本机 scoop v0.6.0（commit ce867cc）源码 `lib/autoupdate.ps1`：
  - `find_hash_in_textfile`（43-112 行）：`$sha256` 模板替换为 `([a-fA-F0-9]{64})`（49 行）；`$basename` 取自该架构下载 URL 的文件名（`get_hash_for_app` 221 行），替换时经 `[Regex]::Escape` 转义（`lib/core.ps1` `substitute` 1318 行），按捕获组 1 提取。
  - `get_hash_for_app` 225 行：`$baseurl` = 该架构 URL 去文件名去尾斜杠 → `$baseurl/checksums.txt` 即 `.../download/v$version/checksums.txt`。
  - 另有免 regex 的文件名兜底匹配（98 行），但显式 regex 更稳（不依赖兜底顺序）。
- Extras 现网先例：`bucket/oras.json` 用完全相同的 `"$sha256\\s+$basename"`；`bucket/didder.json`/`bucket/plakar.json` 用 `$baseurl/...checksums.txt`。
- 本地演练：`verify-regex.ps1` 复刻上述替换管线，对真实 `checksums.txt` 双架构均提取出正确 hash（scoop 无 autoupdate 演练开关，此为等价交叉验证）。

### checkver 依据

本机 scoop `bin/checkver.ps1` 186 行：github checkver 默认 regex `(?:v|V)?([\d.-]+)`，tag `v0.16.0` → `0.16.0`，`v` 前缀无需自定义 regex。已用真实 `checkver.ps1 -App jvm -Dir <本目录>` 跑通，输出 `jvm: 0.16.0`。

## 本地安装验证记录（2026-11-01）

1. `scoop install <本目录>/jvm.json`（免 bucket 直装）：成功，scoop 侧 hash 校验 OK，shim `G:\Scoop\shims\jvm.exe` 创建。
2. 运行 `G:\Scoop\apps\jvm\current\jvm.exe version` → `jvm 0.16.0`。发行风味自举触发：用户 PATH（HKCU\Environment）**仅追加一条** `G:\Scoop\apps\jvm\current`（末尾）；`$PROFILE` 与 `~/.bashrc` 哈希不变（机器上已有当前版本集成块，幂等无重写）。
   - 注意：本机 `~/.bashrc` 存在旧的 `jvm` wrapper 函数（指向 `D:\Program Files\jvm\jvm.exe`），交互 bash 里会遮蔽 scoop shim——机器特有现象，非 manifest 问题；干净机器上新终端直接解析 shim。
3. `scoop checkup`：仅一条既有的 Developer Mode 警告，与 jvm 无关。
4. `scoop info jvm`：Name/Version/Source/Binaries 正常。
5. `scoop uninstall jvm`：apps 目录、current 链接、双 shim 全部移除；**用户 PATH 残留死链 `G:\Scoop\apps\jvm\current`**（自举写入，scoop 不回收）——按任务约束未手工改注册表，交主控清理。scoop cache 中 3MB zip 保留（无害，重装免下载）。

## 提 Extras PR 人工步骤（后续，需用户批准）

1. Fork `ScoopInstaller/Extras`；
2. 在 fork 建分支（惯例 `jvm` 或 `add-jvm`），把本目录 `jvm.json` 原样放入 `bucket/jvm.json`（单文件，勿带其他文件）；
3. PR 标题：`jvm: Add version 0.16.0`；描述要点：工具简介 + 主页 + 便携 zip 双架构（amd64/arm64）+ MIT 许可 + checkver/autoupdate 已配置（Excavator 可自动跟进）+ hash 源 `checksums.txt`；
4. 等 Extras CI（schema/hash/au 校验）绿，回应 reviewer 意见（可能被问知名度/发版稳定性，属收录裁量）；
5. 合并后回本仓库做第 4 步文档同步（README/AGENTS/CHANGELOG，另行提交）。
