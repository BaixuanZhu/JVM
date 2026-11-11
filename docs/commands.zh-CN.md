# 命令参考

[English](commands.md) | 简体中文

返回 [README](../README.zh-CN.md)

## 版本管理

```bash
jvm install 21                  # 安装 Temurin JDK 21 最新版（默认发行版）
jvm install corretto@21         # Amazon Corretto 21
jvm install microsoft@21        # Microsoft Build of OpenJDK 21
jvm install zulu@21             # Azul Zulu 21
jvm install liberica@21         # BellSoft Liberica 21
jvm install graalvm@21          # Oracle GraalVM 21（CPU LTS 线）
jvm install temurin-ea@28       # Temurin 28 早期访问版（GA 前预览）
jvm install 21.0.12+8           # Temurin 精确版本，含 build 号
jvm install temurin@21.0.5+11 D:\jdk.zip   # 从本地 zip 安装，全程不走网络

jvm use 21                      # 切到 21（该大版本下的最新 patch）
jvm use corretto@21             # 切到 Corretto 21
jvm use                         # 无参数：读当前目录的 .jvmrc

jvm pin corretto@21             # 固定此目录用 Corretto 21（写入 .jvmrc）
jvm pin                         # 把当前生效的版本写入 .jvmrc

jvm uninstall 21                # 默认需确认，-y 跳过。
                                # 卸载正在使用的版本会自动回退到剩余最新
jvm uninstall temurin@21 --all -y   # 批量卸载该大版本组下的全部已装 patch

jvm exec 17 -- mvn test         # 用 JDK 17 执行命令，不动全局版本
jvm update 21 -y                # 升级到最新 patch：装新 → 切换 → 清理旧版
jvm update --all -y             # 批量升级全部落后版本组，一次确认，失败不阻断其余
```

## 查询

```bash
jvm list                    # 已安装版本，→ 标记当前
jvm available               # 可安装的大版本，标记 LTS，结果缓存 10 分钟
jvm available corretto      # Corretto 可安装的版本
jvm available -a            # 列出每个大版本的全部子版本
jvm available --major 21    # 只看 JDK 21 的全部子版本
jvm available -r            # 绕过缓存，直查上游
jvm current                 # 当前版本（会实际执行 java -version）
jvm home                    # 打印 JAVA_HOME，供脚本与 IDE 引用
jvm outdated                # 已装版本里哪些有新 patch
jvm cache                   # 下载缓存，附文件日期
jvm cache clean             # 清空下载缓存
jvm cache clean --older-than 30d   # 只清修改于 30 天前的文件
jvm doctor                  # 14 项诊断：PATH、junction、JAVA_HOME、集成、残留
jvm doctor --fix            # 诊断并修复可修项（-y 跳过逐条确认）
```

## 机器可读输出

`list`、`available`、`outdated`、`doctor` 支持 `--json`（短名 `-j`）。只改输出格式，退出码与缓存行为完全一致。错误仍走 stderr 并以非零退出，stdout 始终是纯 JSON 或空。

```bash
jvm list --json             # {"installed":[{distro,version,major,dir,current}...]}
jvm available --json        # 表格形态给 rows；-a / --major 给 groups，含 LTS 标记
jvm outdated --json         # {"groups":[{distro,major,local,latest,upgradable,failed}...]}
jvm doctor --json           # {"ok":bool,"checks":[{name,ok,detail,fix}...]}，与 --fix 互斥
```

```bash
jvm list --json | jq '.installed[].version'
jvm outdated --json | jq '.groups[] | select(.upgradable)'
jvm doctor --json | jq -e '.ok'
```

`doctor` 在检查失败时退出码仍为 0，脚本里请读 `ok` 字段，不要依赖退出码。

## 项目级版本

项目根目录放一个 `.jvmrc`。团队成员或未来的你在该目录（或任意子目录）运行不带参数的 `jvm use`，就会切到固定版本。

```bash
jvm pin corretto@21         # 在当前目录写入 .jvmrc

# 或手动创建，一行，语法与命令行一致：
#   21                    # Temurin 21 最新
#   corretto@21.0.12.8.1  # 发行版 + 完整版本
```

格式是 `[distro@]version`，`#` 开头为注释行。

`jvm use` 从当前目录逐级向上找最近的 `.jvmrc`，monorepo 也能命中。

`jvm pin` 只写文件，要立即生效再敲一次 `jvm use`。指定的版本必须已安装，`use` 会提示安装命令，不会静默下载。

## 兼容 sdkman / asdf / mise

团队里有人用别的版本管理器，提交到仓库的是各自的格式。jvm 直接读：

| 文件 | 来源工具 | 示例内容 | 读作 |
|---|---|---|---|
| `.java-version` | 历史习惯 | `21.0.2` | `21.0.2` |
| `.tool-versions` | asdf / mise | `java temurin-21.0.2+11` | `temurin@21.0.2+11` |
| `.sdkmanrc` | sdkman | `java=21.0.2-tem` | `temurin@21.0.2` |

同一目录多个文件并存时按 `.jvmrc` > `.java-version` > `.tool-versions` > `.sdkmanrc` 取用。不含 Java 信息的文件（例如只有 `nodejs` 行的 `.tool-versions`）自动跳过。

sdkman 的标识自动映射：`-tem` → temurin、`-amzn` → corretto、`-ms` → microsoft、`-zul` → zulu、`-librca` → liberica、`-graal` → graalvm。

这些生态的版本号普遍不带 build 号，精确匹配失败时降级为该大版本组内已安装的最新版。`.jvmrc` 与命令行参数不受影响，仍是严格匹配。

`jvm pin` 只写 `.jvmrc`，不会去改别人的格式文件。

## cd 自动切换

装了新版集成脚本后（升级 jvm 时自动重写 profile 获得），进入含 `.jvmrc` 或上述兼容文件的目录会自动切到该版本，离开则恢复你手动选定的版本。

```bash
jvm use 21          # 手动选定 21
cd D:\proj\legacy   # 该目录 .jvmrc 内容为 8 → 自动切到 8
cd ~                # 离开项目 → 恢复 21
```

- 目录与 rc 文件都没变时零开销（双层缓存，不会每次按回车都拉起 jvm）
- 固定版本未安装时只提示一行 `jvm install` 建议，不刷屏
- 手动 `jvm use` 永远优先，显式切换后即成为新的恢复基线
- 关闭方式：`~/.jvm/config.toml` 里设 `autoswitch = false`，或临时 `JVM_AUTOSWITCH=0`

## Shell 集成

jvm 首次运行时会把 wrapper 写进 `$PROFILE`（PowerShell 5.x 与 7+）和 `~/.bashrc`，因此 `jvm use` 能在**当前**会话里刷新 `JAVA_HOME` 和 `PATH`。装完重开一次终端，之后无需任何配置。

注入是幂等的，每次启动静默检查，缺了才补。不想用就删掉 profile 里带 `jvm shell init` 标记的那一块。

```bash
jvm init powershell             # 打印集成脚本
jvm init powershell --install   # 写入 $PROFILE
jvm init bash --install         # 写入 ~/.bashrc
```

CMD 不支持：`cmd.exe` 没有等价于 `$PROFILE` 的机制，doskey 宏体验也不好。CMD 用户在切换后新开窗口即可。

## 配置

可选，位于 `~/.jvm/config.toml`：

```toml
# 下载镜像源，默认清华 TUNA，仅 Temurin 走镜像
mirror = "https://mirrors.tuna.tsinghua.edu.cn/Adoptium"

# 目标架构，默认跟随 jvm 二进制自身架构，也接受 amd64 / arm64
arch = "aarch64"

# cd 进含 .jvmrc 的目录自动切换，默认开启
autoswitch = true

# 把 JDK 存储挪到其他盘。控制面（config.toml、current/、注册表项）留在 ~/.jvm。
# 已装版本需手动搬过去，jvm doctor 会提示。
# install_dir = "D:\\jdks"
```

环境变量优先级高于配置文件，适合一次性切换：

```powershell
$env:JVM_MIRROR = "https://your.mirror/Adoptium"
$env:JVM_ARCH = "aarch64"
$env:JVM_AUTOSWITCH = "0"
$env:JVM_INSTALL_DIR = "D:\jdks"
jvm install 21
```

## 自更新

```bash
jvm upgrade     # 检查 GitHub Releases 并更新 jvm 自身
jvm version
jvm help
```

`jvm upgrade` 读取 `internal/upgrade/upgrade.go` 里的 `githubRepo`（当前为 `BaixuanZhu/JVM`），按当前二进制架构拉取对应的便携版 zip。推 `v*` 标签会触发发版工作流。
