---
layout: landing
title: 首页
---

<!-- ================= Hero（左文右终端） ================= -->
<section class="hero">
  <div class="container hero-split">
    <div class="hero-text">
      <span class="hero-badge">开源 · MIT · Windows x64 / ARM64 原生</span>
      <h1>Windows 上的<br><span class="grad">Java 版本管理器</span></h1>
      <p class="hero-sub">
        类似 nvm-windows / jabba 的 JDK 版本管理工具。
        一键安装、秒级切换、自动配置 PATH 与 JAVA_HOME，全程免管理员权限。
      </p>

      <div class="install-cmd">
        <code id="hero-install-cmd">iwr -useb "https://raw.githubusercontent.com/BaixuanZhu/JVM/main/install.ps1" | iex</code>
        <button class="copy-btn copy-btn-primary" data-copy="#hero-install-cmd" aria-live="polite">复制安装命令</button>
      </div>

      <div class="hero-actions">
        <a class="btn btn-ghost" href="{{ site.download_setup }}">下载安装包</a>
        <a class="hero-docs-link" href="{{ '/docs/install/' | relative_url }}">查看安装指南 →</a>
      </div>
      <p class="hero-note">安装脚本自动识别 x64 / ARM64；ARM64 设备也可<a href="{{ site.download_setup_arm64 }}">下载 ARM64 安装包</a>。</p>
    </div>

    <!-- 无 JS / 减弱动效时显示静态内容；否则可播放或停止打字动画 -->
    <div class="terminal hero-term">
      <div class="terminal-bar">
        <span class="dot r" aria-hidden="true"></span><span class="dot y" aria-hidden="true"></span><span class="dot g" aria-hidden="true"></span>
        <span class="term-title">PowerShell</span>
        <button class="term-motion-toggle" id="term-motion-toggle" type="button" hidden>停止动画</button>
      </div>
      <pre id="demo-term"><span class="p">jvm install 21</span>

<span class="p">jvm use 21</span>

<span class="p">cd my-app</span>  <span class="c"># 项目里有 .jvmrc: corretto@17</span>

<span class="o">java -version</span>  <span class="c"># 自动切到 17, 现在就是它</span></pre>
    </div>
  </div>
</section>

<!-- ================= 快速开始 ================= -->
<section class="section quickstart">
  <div class="container">
    <h2 class="section-title fade-in">3 步开始</h2>
    <p class="section-sub fade-in">安装、切换、验证。每条命令都可以直接复制。</p>

    <div class="quickstart-grid">
      <div class="quickstart-step fade-in">
        <div class="step-heading"><span class="step-number" aria-hidden="true">1</span><h3>安装 JDK</h3></div>
        <p>默认安装 Temurin 21 的最新 GA 版本。</p>
        <div class="quickstart-command"><code id="quick-install">jvm install 21</code><button class="copy-btn" data-copy="#quick-install" aria-live="polite">复制</button></div>
      </div>
      <div class="quickstart-step fade-in">
        <div class="step-heading"><span class="step-number" aria-hidden="true">2</span><h3>切换版本</h3></div>
        <p>当前终端与新终端都会使用 JDK 21。</p>
        <div class="quickstart-command"><code id="quick-use">jvm use 21</code><button class="copy-btn" data-copy="#quick-use" aria-live="polite">复制</button></div>
      </div>
      <div class="quickstart-step fade-in">
        <div class="step-heading"><span class="step-number" aria-hidden="true">3</span><h3>验证结果</h3></div>
        <p>显示当前版本，并实际执行 java -version。</p>
        <div class="quickstart-command"><code id="quick-current">jvm current</code><button class="copy-btn" data-copy="#quick-current" aria-live="polite">复制</button></div>
      </div>
    </div>
    <p class="quickstart-note fade-in">首次使用请先完成 <a href="{{ '/docs/install/' | relative_url }}">jvm 安装与终端配置</a>；需要其他发行版时，使用 <code>jvm install corretto@21</code> 这样的前缀语法即可。</p>
  </div>
</section>

<!-- ================= 核心能力 ================= -->
<section class="section section-alt">
  <div class="container">
    <h2 class="section-title fade-in">为 Windows Java 开发者而生</h2>
    <p class="section-sub fade-in">从下载到切换，每一步都替你做好。</p>

    <div class="feature-grid">
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">🌐</div>
        <h3>多发行版，一键安装</h3>
        <p><code>jvm install 21</code> 安装最新 GA 版本；Temurin / Corretto / Microsoft / Zulu / Liberica / GraalVM 随心选，下载自动校验。</p>
      </div>
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">🔄</div>
        <h3>秒级切换</h3>
        <p><code>jvm use 21</code> 基于 Windows junction 切换，当前终端即时生效，新终端也无需刷新环境变量。</p>
      </div>
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">📄</div>
        <h3>项目版本自动切换</h3>
        <p><code>jvm pin</code> 固定项目版本，cd 进入自动切换、离开自动恢复；兼容 <code>.java-version</code> / <code>.tool-versions</code> / <code>.sdkmanrc</code>。</p>
      </div>
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">⬆️</div>
        <h3>Patch 一键升级</h3>
        <p><code>jvm update 21</code> 装最新 patch、自动切换、清理旧版一条龙；<code>update --all</code> 批量升级全部落后组。</p>
      </div>
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">💾</div>
        <h3>缓存与离线</h3>
        <p>安装包缓存让重装免重下，<code>available</code> 结果缓存 10 分钟；内网环境还能直接安装本地 zip 包。</p>
      </div>
      <div class="feature-card fade-in">
        <div class="icon" aria-hidden="true">🤖</div>
        <h3>脚本与 CI 友好</h3>
        <p>只读命令支持 <code>--json</code>，<code>home</code> 单行打印 JAVA_HOME；<code>jvm exec 17 -- mvn test</code> 不动全局即可验证多版本。</p>
      </div>
    </div>
  </div>
</section>

<!-- ================= 发行版 ================= -->
<section class="section">
  <div class="container">
    <h2 class="section-title fade-in">七大发行版，一个命令</h2>
    <p class="section-sub fade-in">省略前缀默认 Temurin，精确到小版本也没问题。</p>

    <div class="distro-grid">
      <div class="distro-card fade-in">
        <h3>Temurin</h3>
        <p class="vendor">Eclipse Adoptium · 默认 · 清华镜像加速</p>
        <code>jvm install 21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>Corretto</h3>
        <p class="vendor">Amazon · 含 SHA256 校验清单</p>
        <code>jvm install corretto@21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>Microsoft</h3>
        <p class="vendor">Microsoft Build of OpenJDK · 仅 LTS</p>
        <code>jvm install microsoft@21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>Zulu</h3>
        <p class="vendor">Azul · 覆盖最全的 LTS / 最新版</p>
        <code>jvm install zulu@21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>Liberica</h3>
        <p class="vendor">BellSoft · 全架构 · 仅 LTS</p>
        <code>jvm install liberica@21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>GraalVM</h3>
        <p class="vendor">Oracle · CPU LTS 线（21/25）· 官方 CDN</p>
        <code>jvm install graalvm@21</code>
      </div>
      <div class="distro-card fade-in">
        <h3>Temurin EA</h3>
        <p class="vendor">Adoptium 早期访问版 · GA 前预览</p>
        <code>jvm install temurin-ea@28</code>
      </div>
    </div>

    <p class="distro-note fade-in">精确版本同样支持：<code>jvm install 21.0.12+8</code>；内网环境可装本地 zip：<code>jvm install temurin@21.0.5+11 D:\jdk.zip</code><br>
    ARM64 设备自动下载 ARM64 版 JDK（Temurin / Microsoft / Zulu / Liberica；Corretto 与 GraalVM 官方暂无 Windows ARM64 构建）。<br>
    <a href="{{ '/docs/distros/' | relative_url }}">查看完整发行版与 ARM64 支持矩阵 →</a></p>
  </div>
</section>

<!-- ================= CTA ================= -->
<section class="section cta">
  <div class="container">
    <h2 class="section-title fade-in">现在就试试</h2>
    <p class="section-sub fade-in">一行命令，一分钟装好，随时卸载。</p>
    <div class="hero-actions fade-in">
      <a class="btn btn-primary" href="{{ '/docs/install/' | relative_url }}">查看安装指南</a>
      <a class="btn btn-ghost" href="{{ site.github_repo }}" target="_blank" rel="noopener">在 GitHub 上 Star</a>
    </div>
  </div>
</section>
