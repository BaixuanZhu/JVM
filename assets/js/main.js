// jvm 官网交互：复制按钮、文档导航、移动端导航、滚动入场与终端演示
(function () {
  "use strict";

  var reduceMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // ---- 复制按钮 ----
  function copyText(text, btn) {
    var original = btn.getAttribute("data-original-label");
    if (!original) {
      original = btn.textContent;
      btn.setAttribute("data-original-label", original);
    }

    var done = function () {
      if (btn._copyResetTimer) clearTimeout(btn._copyResetTimer);
      btn.textContent = "已复制";
      btn.classList.add("copied");
      btn._copyResetTimer = setTimeout(function () {
        btn.textContent = original;
        btn.classList.remove("copied");
      }, 1600);
    };

    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () { fallbackCopy(text, done); });
    } else {
      fallbackCopy(text, done);
    }
  }

  function fallbackCopy(text, done) {
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try { if (document.execCommand("copy")) done(); } catch (e) { /* 浏览器拒绝时保留原按钮状态 */ }
    document.body.removeChild(ta);
  }

  // 首页安装命令与快速开始命令
  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    btn.setAttribute("aria-live", "polite");
    btn.addEventListener("click", function () {
      var target = document.querySelector(btn.getAttribute("data-copy"));
      if (target) copyText(target.textContent.trim(), btn);
    });
  });

  // 文档页代码块：包装成「顶栏（语言标签 + 复制按钮）+ 代码区」结构
  document.querySelectorAll(".docs-content pre").forEach(function (pre) {
    // 从 kramdown/rouge 的 language-* 包装类里取语言名
    var lang = "";
    var wrapper = pre.closest("[class*='language-']");
    if (wrapper) {
      var m = wrapper.className.match(/language-([a-zA-Z0-9+#-]+)/);
      if (m) lang = m[1];
    }
    // 无语言的纯文本块统一显示 TEXT
    if (!lang || lang === "plaintext" || lang === "text") lang = "text";

    var block = document.createElement("div");
    block.className = "code-block";
    pre.parentNode.insertBefore(block, pre);

    var header = document.createElement("div");
    header.className = "code-header";

    var label = document.createElement("span");
    label.className = "code-lang";
    label.textContent = lang || "code";

    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.textContent = "复制";
    btn.setAttribute("aria-live", "polite");
    btn.addEventListener("click", function () {
      var code = pre.querySelector("code");
      copyText((code || pre).textContent, btn);
    });

    header.appendChild(label);
    header.appendChild(btn);
    block.appendChild(header);
    block.appendChild(pre);
  });

  // ---- 文档页：本页目录、标题锚点与宽表格 ----
  var docsContent = document.querySelector(".docs-content");
  if (docsContent) {
    var headings = docsContent.querySelectorAll("h2");
    if (headings.length >= 2) {
      var toc = document.createElement("details");
      toc.className = "page-toc";
      toc.open = !(window.matchMedia && window.matchMedia("(max-width: 720px)").matches);

      var summary = document.createElement("summary");
      summary.textContent = "本页目录";
      toc.appendChild(summary);

      var tocList = document.createElement("ul");
      headings.forEach(function (heading, index) {
        if (!heading.id) heading.id = "section-" + (index + 1);
        var title = heading.textContent;
        heading.setAttribute("aria-label", title);

        var item = document.createElement("li");
        var link = document.createElement("a");
        link.href = "#" + heading.id;
        link.textContent = title;
        item.appendChild(link);
        tocList.appendChild(item);

        var anchor = document.createElement("a");
        anchor.className = "heading-anchor";
        anchor.href = "#" + heading.id;
        anchor.textContent = "#";
        anchor.setAttribute("aria-label", "链接到本节：" + title);
        heading.appendChild(anchor);
      });
      toc.appendChild(tocList);

      var pageTitle = docsContent.querySelector("h1");
      if (pageTitle) docsContent.insertBefore(toc, pageTitle.nextSibling);
      else docsContent.insertBefore(toc, docsContent.firstChild);
    }

    document.querySelectorAll(".docs-content table").forEach(function (table) {
      var wrap = document.createElement("div");
      wrap.className = "table-wrap";
      wrap.tabIndex = 0;
      wrap.setAttribute("role", "region");
      wrap.setAttribute("aria-label", "可横向滚动的表格");
      table.parentNode.insertBefore(wrap, table);
      wrap.appendChild(table);
    });
  }

  // 手机端默认收起文档导航；桌面端始终展开
  var docsNav = document.querySelector(".docs-sidebar");
  if (docsNav && window.matchMedia) {
    var docsMedia = window.matchMedia("(max-width: 860px)");
    var docsMobileInitialized = false;
    var syncDocsNav = function () {
      if (docsMedia.matches) {
        if (!docsMobileInitialized) {
          docsNav.open = false;
          docsMobileInitialized = true;
        }
      } else {
        docsNav.open = true;
        docsMobileInitialized = false;
      }
    };
    syncDocsNav();
    if (docsMedia.addEventListener) docsMedia.addEventListener("change", syncDocsNav);
    else if (docsMedia.addListener) docsMedia.addListener(syncDocsNav);
  }

  // ---- 移动端主导航 ----
  var toggle = document.querySelector(".nav-toggle");
  var nav = document.querySelector(".site-nav");
  if (toggle && nav) {
    var setNavOpen = function (open) {
      nav.classList.toggle("open", open);
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      toggle.setAttribute("aria-label", open ? "关闭导航" : "打开导航");
    };
    toggle.addEventListener("click", function () {
      setNavOpen(!nav.classList.contains("open"));
    });
    nav.querySelectorAll("a").forEach(function (link) {
      link.addEventListener("click", function () { setNavOpen(false); });
    });
    document.addEventListener("keydown", function (event) {
      if (event.key === "Escape" && nav.classList.contains("open")) {
        setNavOpen(false);
        toggle.focus();
      }
    });
    document.addEventListener("click", function (event) {
      var header = toggle.closest(".site-header");
      if (nav.classList.contains("open") && header && !header.contains(event.target)) {
        setNavOpen(false);
      }
    });
  }

  // ---- 滚动入场动画：无 JS / 减弱动效时默认可见 ----
  var items = document.querySelectorAll(".fade-in");
  if (!reduceMotion && "IntersectionObserver" in window && items.length) {
    document.body.classList.add("motion-ready");
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add("visible");
          io.unobserve(entry.target);
        }
      });
    }, { threshold: 0.12 });
    items.forEach(function (el) { io.observe(el); });
  } else {
    items.forEach(function (el) { el.classList.add("visible"); });
  }

  // ---- 首页演示终端：可停止的打字动画；减弱动效时保留静态内容 ----
  var demo = document.getElementById("demo-term");
  var motionToggle = document.getElementById("term-motion-toggle");
  if (demo && motionToggle && !reduceMotion) {
    // [样式类, 文本]，cmd 为打字输入的命令，out 为逐行浮现的输出
    var script = [
      { type: "cmd", text: "jvm install 21" },
      { type: "out", lines: [
        ["dim", "正在解析 temurin@21 …"],
        ["dim", "下载 OpenJDK 21.0.12+8  [████████████] 100%"],
        ["o",   "✓ SHA256 校验通过"],
        ["o",   "✓ 安装完成：temurin-21.0.12+8"]
      ]},
      { type: "cmd", text: "jvm use 21" },
      { type: "out", lines: [
        ["o",   "✓ 已切换到 temurin-21.0.12+8"],
        ["dim", "当前终端同步生效，无需重开"]
      ]},
      { type: "cmd", text: "cd my-app" },
      { type: "out", lines: [
        ["dim", "发现 .jvmrc: corretto@17"],
        ["o",   "✓ 自动切换到 corretto-17.0.16.8.1"]
      ]},
      { type: "cmd", text: "java -version" },
      { type: "out", lines: [
        ["o",   'openjdk version "17.0.16" 2026-07-15 LTS'],
        ["dim", "OpenJDK Runtime Environment Corretto-17.0.16.8.1"]
      ]}
    ];

    var staticDemo = demo.innerHTML;
    var step = 0;
    var timer = null;
    var stopped = false;

    function clearActiveTimer() {
      if (timer !== null) {
        clearTimeout(timer);
        clearInterval(timer);
        timer = null;
      }
    }

    function schedule(callback, delay) {
      timer = setTimeout(function () {
        timer = null;
        if (!stopped) callback();
      }, delay);
    }

    function newline() { demo.appendChild(document.createTextNode("\n")); }

    function typeCommand(text, done) {
      if (stopped) return;
      if (demo.textContent.length > 0) newline(); // 命令块之间空一行
      var prompt = document.createElement("span");
      prompt.className = "dim";
      prompt.textContent = "$ ";
      var typed = document.createElement("span");
      typed.className = "p";
      var cursor = document.createElement("span");
      cursor.className = "cursor";
      demo.appendChild(prompt);
      demo.appendChild(typed);
      demo.appendChild(cursor);

      var i = 0;
      timer = setInterval(function () {
        if (stopped) { clearActiveTimer(); return; }
        typed.textContent = text.slice(0, ++i);
        if (i >= text.length) {
          clearActiveTimer();
          cursor.remove();
          newline();
          schedule(done, 420);
        }
      }, 55);
    }

    function showLines(lines, idx, done) {
      if (stopped) return;
      if (idx >= lines.length) { schedule(done, 500); return; }
      var span = document.createElement("span");
      span.className = lines[idx][0];
      span.textContent = lines[idx][1];
      demo.appendChild(span);
      newline();
      schedule(function () { showLines(lines, idx + 1, done); }, 360);
    }

    function runStep() {
      if (stopped) return;
      if (step >= script.length) {
        // 一轮结束，停留片刻后清屏重来
        schedule(function () {
          demo.innerHTML = "";
          step = 0;
          runStep();
        }, 5200);
        return;
      }
      var s = script[step++];
      if (s.type === "cmd") typeCommand(s.text, runStep);
      else showLines(s.lines, 0, runStep);
    }

    function startDemo() {
      clearActiveTimer();
      stopped = false;
      step = 0;
      demo.innerHTML = "";
      motionToggle.textContent = "停止动画";
      motionToggle.setAttribute("aria-label", "停止终端演示动画");
      runStep();
    }

    function stopDemo() {
      stopped = true;
      clearActiveTimer();
      demo.innerHTML = staticDemo;
      motionToggle.textContent = "播放动画";
      motionToggle.setAttribute("aria-label", "播放终端演示动画");
    }

    motionToggle.addEventListener("click", function () {
      if (stopped) startDemo();
      else stopDemo();
    });
    motionToggle.hidden = false;
    startDemo();
  }
})();
