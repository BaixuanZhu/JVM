# jvm — Windows 上的 JDK 版本管理器

[English](README.md) | 简体中文

装多个 JDK，切换以毫秒计，全程不需要管理员权限。支持 Temurin、Corretto、Microsoft、Zulu、Liberica、GraalVM、Dragonwell、Kona、Semeru。

<!-- 待补：5 秒 GIF，`jvm use 21` 之后同一个终端里 `java -version` 立刻变化。
     这一段的说服力超过本页其余所有内容的总和。 -->

## 为什么还要再造一个

Windows 上没有 `sdkman`，现成的每条路都要付点代价：

| 方案 | 卡在哪 |
|---|---|
| 手动改 `JAVA_HOME` | 每次切换都要重开终端和 IDE |
| sdkman | 只跑 bash，进了 WSL 装的 JDK，Windows 侧 IDE 看不见 |
| jabba | 早已停止维护 |
| Chocolatey / Scoop | 装起来方便，切版本还是回到手动 |
| 用 `setx` 改 `PATH` | 超过 1024 字符的部分会被静默截断 |

jvm 只解决「切」这一半。`PATH` 和 `JAVA_HOME` 只写一次，指向一个 junction，切版本就是重写这个 junction 的指向——几毫秒，不需要提权，新开的终端自动就是新版本。

## 安装

```powershell
iwr -useb "https://raw.githubusercontent.com/BaixuanZhu/JVM/main/install.ps1" | iex
```

装完重开一次终端。要求 Windows 10 以上，不需要管理员权限，也不需要 Go 环境。

偏好安装包或便携版的话，在 [Releases](https://github.com/BaixuanZhu/JVM/releases) 里按架构下载。

## 用法

```bash
jvm install 21              # 装 Temurin JDK 21 最新版
jvm use 21                  # 切换，当前这个终端立即生效
jvm install corretto@21     # 换个发行版
```

项目根目录放一个 `.jvmrc` 就能固定版本。`jvm use` 不带参数时会从当前目录逐级向上查找，子目录同样命中。

它还认 `.java-version`、`.tool-versions`（asdf / mise）和 `.sdkmanrc`。团队里其他人不必改用 jvm，你自己用就够了。

完整命令参考、`--json` 输出与配置项：[docs/commands.zh-CN.md](docs/commands.zh-CN.md)

## 工作原理

```
~/.jvm/
  versions/
    temurin-21.0.12+8/
    corretto-21.0.12.8.1/
  current/          ← junction，指向当前选中的版本
  config.toml
```

`PATH` 指向 `~/.jvm/current/bin`，`JAVA_HOME` 指向 `~/.jvm/current`，两者都在安装时写一次，之后不再改动。切换版本就是重建那个 junction。

junction 是 NTFS 的 reparse point，创建时不校验 `SeCreateSymbolicLinkPrivilege`，所以不像符号链接那样需要管理员权限或开发者模式。jvm 直接调 `FSCTL_SET_REPARSE_POINT` 创建，不经过 `mklink`。代价是 junction 只能指向本地卷的目录——对 JDK 这个场景没有影响。

更难的是另一半。子进程改不了父 shell 的环境变量，所以单靠一个 `jvm.exe`，永远做不到「执行完就让当前终端的 `java -version` 变掉」。jvm 首次运行时往 `$PROFILE`（PowerShell 5.x 和 7+）与 `~/.bashrc` 里注入一个 wrapper 函数，函数先调真正的二进制，遇到 `use` 就在当前会话里刷新 `JAVA_HOME` 和 `PATH`。

## 支持的发行版

| 发行版 | 前缀 | Windows ARM64 |
|---|---|---|
| Eclipse Temurin | `temurin`（默认） | 部分 |
| Temurin 早期访问版 | `temurin-ea` | 部分 |
| Amazon Corretto | `corretto` | 无 |
| Microsoft Build of OpenJDK | `microsoft` | 有 |
| Azul Zulu | `zulu` | 有 |
| BellSoft Liberica | `liberica` | 有 |
| Oracle GraalVM | `graalvm` | 无 |
| Alibaba Dragonwell（Standard） | `dragonwell` | 无 |
| Tencent Kona | `kona` | 无 |
| IBM Semeru（OpenJ9） | `semeru` | 无 |

十家的下载 API 形态各不相同：indexmap JSON、两步元数据查询、只给 SHA1 的校验、需要 CDN 顺序探测才能拿到资产、GitHub releases 旁路校验。这些差异统一收敛在 `internal/provider/` 的 `Provider` 接口之后。

## 设计决策

| 问题 | 选择 | 原因 |
|---|---|---|
| 切换机制 | Windows junction | 免提权，新终端也生效 |
| junction 创建 | 原生 `FSCTL_SET_REPARSE_POINT` | 不调 `cmd.exe`，无注入面 |
| 当前终端生效 | 启动时注入 shell wrapper | 子进程改不了父进程环境 |
| `PATH` 注入 | `HKCU\Environment` + 广播 `WM_SETTINGCHANGE` | `setx` 会截断长 `PATH` |
| 下载源 | Temurin 走清华镜像，失败回退官方 CDN | 国内快，镜像滞后也不会错 |
| 发行版 | 九家加 Temurin EA | 全部提供纯 zip，不必解包 MSI |

## 构建

需要 Go 1.26+ 和 GNU Make。

```bash
make build       # → dist/<arch>/jvm.exe（开发构建，不写 PATH 和 profile）
make build-dist  # 发行构建，保留启动自举
make installer   # NSIS 安装包，需要 scoop install nsis
```

发版产物由 GoReleaser 生成。推一个 `v*` 标签会触发工作流。

## 许可

[MIT](LICENSE)
