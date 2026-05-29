// AgentGuard landing page — 2026.
//   1. smart OS-detecting install widget
//   2. click-to-copy buttons (install widget + AI-setup prompt)
//   3. AI-agent setup prompt generator (client-side)
//   4. nav scroll state + scroll-spy
//   5. marquee seamless loop (clone tracks, set duration from data-speed)
//   6. live terminal feed (typed verdict lines)
//   7. scroll-reveal via IntersectionObserver
//   8. mouse-following glow + bento hover spotlight
(function () {
  "use strict";

  var prefersReduced = window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* ---------- generic clipboard helper ---------- */
  function copyText(text, onOk) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(onOk).catch(function () {});
    } else {
      var ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand("copy"); onOk(); } catch (_) {}
      document.body.removeChild(ta);
    }
  }
  function flashCopied(btn, doneLabel, restoreLabel) {
    btn.classList.add("copied");
    var span = btn.querySelector("span");
    var prev = span ? span.textContent : "";
    if (span) span.textContent = doneLabel;
    setTimeout(function () {
      btn.classList.remove("copied");
      if (span) span.textContent = restoreLabel || prev;
    }, 1600);
  }

  /* ---------- 1. smart install widget ---------- */
  var widget = document.getElementById("installWidget");
  if (widget) {
    var tabs  = widget.querySelectorAll(".iw-tab");
    var panes = widget.querySelectorAll(".iw-pane");
    var detectedLabel = document.getElementById("iwDetectedOs");
    var copyBtn = document.getElementById("iwCopyBtn");

    function detectPlatform() {
      var uaData = navigator.userAgentData;
      var platform = uaData && uaData.platform
        ? uaData.platform.toLowerCase()
        : (navigator.platform || "").toLowerCase();
      var ua = (navigator.userAgent || "").toLowerCase();

      if (platform.indexOf("mac") !== -1 || ua.indexOf("mac os") !== -1)
        return { key: "macos", label: "macOS" };
      if (platform.indexOf("win") !== -1 || ua.indexOf("windows") !== -1)
        return { key: "windows-ps", label: "Windows (PowerShell)" };
      if (platform.indexOf("linux") !== -1 || ua.indexOf("linux") !== -1 ||
          ua.indexOf("x11") !== -1 || ua.indexOf("cros") !== -1)
        return { key: "linux", label: "Linux" };
      return { key: "linux", label: "Linux / Unix" };
    }

    function activate(key) {
      tabs.forEach(function (t) { t.classList.toggle("active", t.dataset.platform === key); });
      panes.forEach(function (p) { p.classList.toggle("active", p.dataset.pane === key); });
    }

    var detected = detectPlatform();
    if (detectedLabel) detectedLabel.textContent = detected.label;
    tabs.forEach(function (t) {
      if (t.dataset.platform === detected.key) t.classList.add("recommended");
    });
    activate(detected.key);

    tabs.forEach(function (tab) {
      tab.addEventListener("click", function () { activate(tab.dataset.platform); });
    });

    if (copyBtn) {
      copyBtn.addEventListener("click", function () {
        var activePane = widget.querySelector(".iw-pane.active");
        if (!activePane) return;
        var codeEl = activePane.querySelector(".iw-cmd");
        if (!codeEl) return;
        copyText(codeEl.innerText.trim(), function () {
          flashCopied(copyBtn, "Copied!", "Copy");
        });
      });
    }
  }

  /* ---------- 2. click-to-copy on <pre class="copy"> ---------- */
  document.querySelectorAll("pre.copy").forEach(function (el) {
    el.addEventListener("click", function () {
      copyText(el.innerText.trim(), function () {
        el.classList.add("copied");
        setTimeout(function () { el.classList.remove("copied"); }, 1400);
      });
    });
  });

  /* ---------- 3. AI-agent setup prompt generator ---------- */
  var pg = document.getElementById("promptGen");
  if (pg) {
    var AGENTS = {
      claude:   { name: "Claude Code",
                  cfg: "~/.claude.json (or claude_desktop_config.json)",
                  skill: "~/.claude/skills/agentguard/SKILL.md",
                  restart: "Claude Code may need a one-time restart to load the rewritten config." },
      cursor:   { name: "Cursor",
                  cfg: "~/.cursor/mcp.json",
                  skill: null,
                  restart: "Restart Cursor once after init so it reloads its MCP config." },
      codex:    { name: "Codex CLI",
                  cfg: "~/.codex/config.toml (TOML — let init patch it, don't hand-edit)",
                  skill: null,
                  restart: "Start a new Codex session after init." },
      gemini:   { name: "Gemini CLI",
                  cfg: "~/.gemini/settings.json",
                  skill: null,
                  restart: "Start a new Gemini CLI session after init." },
      windsurf: { name: "Windsurf",
                  cfg: "~/.codeium/windsurf/mcp_config.json",
                  skill: null,
                  restart: "Windsurf reloads MCP config without a full restart." }
    };

    function buildPrompt(key) {
      var a = AGENTS[key];
      var lines = [
        "I just installed AgentGuard (https://agentguard.space) — a local, open-source",
        "security sidecar that inspects every MCP tool call you make and blocks prompt",
        "injection, tool poisoning, rug-pulls, and credential leaks. It's local-only;",
        "nothing leaves my machine.",
        "",
        "Please set it up for me — do the work, don't make me edit configs by hand:",
        "",
        "1. Confirm the binary works:  agentguard --version  and  agentguard doctor",
        "2. Run  agentguard init --non-interactive  to detect this agent and route its",
        "   MCP servers through AgentGuard. It backs up my config first",
        "   (expected here: " + a.cfg + ").",
        "3. Start the dashboard:  agentguard dashboard --no-browser  and give me the URL",
        "   (http://127.0.0.1:7878).",
        "4. Trigger one real tool call so I can see a row appear in the dashboard, then",
        "   tell me the verdict it got (allow / flag / transform / block) and what that",
        "   means in one plain sentence.",
        "5. If anything is wrong, run  agentguard doctor  and explain the ✗ lines simply.",
        "   Never disable AgentGuard, change a policy, or delete data without asking me.",
        "",
        a.restart
      ];
      if (a.skill) {
        lines.push("");
        lines.push("You have an AgentGuard Skill at " + a.skill + " — read it first and follow it.");
      }
      return lines.join("\n");
    }

    var pgAgents = pg.querySelectorAll(".pg-agent");
    var pgPrompt = document.getElementById("pgPrompt");
    var pgName   = document.getElementById("pgAgentName");
    var pgCopy   = document.getElementById("pgCopyBtn");

    function selectAgent(key) {
      pgAgents.forEach(function (b) { b.classList.toggle("active", b.dataset.agent === key); });
      if (pgName) pgName.textContent = AGENTS[key].name;
      if (pgPrompt) pgPrompt.textContent = buildPrompt(key);
    }
    pgAgents.forEach(function (b) {
      b.addEventListener("click", function () { selectAgent(b.dataset.agent); });
    });
    selectAgent("claude");

    if (pgCopy) {
      pgCopy.addEventListener("click", function () {
        copyText(pgPrompt ? pgPrompt.textContent : "", function () {
          flashCopied(pgCopy, "Copied!", "Copy prompt");
        });
      });
    }
  }

  /* ---------- 4. nav scroll state + scroll-spy ---------- */
  var nav = document.getElementById("nav");
  var links = Array.prototype.slice.call(document.querySelectorAll(".nav nav a[href^='#']"));
  var sections = links.map(function (a) {
    return { link: a, el: document.getElementById(a.getAttribute("href").slice(1)) };
  }).filter(function (x) { return x.el; });

  function onScroll() {
    if (nav) nav.classList.toggle("scrolled", window.scrollY > 24);
    var y = window.scrollY + 110, cur = null;
    for (var i = 0; i < sections.length; i++) {
      if (sections[i].el.offsetTop <= y) cur = sections[i];
    }
    links.forEach(function (l) { l.classList.remove("active"); });
    if (cur) cur.link.classList.add("active");
  }
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  /* ---------- 5. marquee seamless loop ---------- */
  document.querySelectorAll(".marquee").forEach(function (m) {
    var track = m.querySelector(".marquee-track");
    if (!track) return;
    // duplicate content so the -50% keyframe lands on an identical frame
    track.innerHTML += track.innerHTML;
    var speed = parseFloat(m.getAttribute("data-speed")) || 40;
    track.style.setProperty("--dur", speed + "s");
  });

  /* ---------- 6. live terminal feed ---------- */
  var termFeed = document.getElementById("termFeed");
  if (termFeed && !prefersReduced) {
    var EVENTS = [
      ["ALLOW", "github.read_issue",        "132ms"],
      ["BLOCK", "bash.rm_rf",               "policy.violation"],
      ["WARN",  "slack.post_message",       "sensitive.channel"],
      ["ALLOW", "filesystem.read",          "12ms"],
      ["BLOCK", "github.comment",           "indirect.prompt.injection"],
      ["ALLOW", "notion.search",            "88ms"],
      ["WARN",  "fetch.url",                "secret.in.args"],
      ["BLOCK", "tools/list",               "schema.rug.pull"],
      ["ALLOW", "memory.get",               "4ms"],
      ["BLOCK", "stripe.create_charge",     "spend.cap.exceeded"]
    ];
    var MAX = 6, idx = 0;

    function addLine() {
      var e = EVENTS[idx % EVENTS.length];
      idx++;
      var row = document.createElement("div");
      row.className = "term-line";
      var verb = document.createElement("span");
      verb.className = "verb " + e[0];
      verb.textContent = e[0];
      var tool = document.createElement("span");
      tool.className = "tool";
      tool.textContent = e[1];
      var meta = document.createElement("span");
      meta.className = "meta";
      meta.textContent = e[2];
      row.appendChild(verb); row.appendChild(tool); row.appendChild(meta);
      termFeed.appendChild(row);
      while (termFeed.children.length > MAX) termFeed.removeChild(termFeed.firstChild);
    }

    // only run when the terminal is on screen — saves cycles + battery
    var running = false, timer = null;
    function tick() {
      addLine();
      timer = setTimeout(tick, 900 + Math.random() * 700);
    }
    var termSection = document.getElementById("terminal");
    if ("IntersectionObserver" in window && termSection) {
      new IntersectionObserver(function (entries) {
        entries.forEach(function (en) {
          if (en.isIntersecting && !running) { running = true; tick(); }
          else if (!en.isIntersecting && running) { running = false; clearTimeout(timer); }
        });
      }, { threshold: 0.2 }).observe(termSection);
    } else {
      // seed a few static lines if no observer
      for (var k = 0; k < MAX; k++) addLine();
    }
  } else if (termFeed) {
    // reduced motion: just paint a static snapshot
    [["ALLOW","github.read_issue","132ms"],["BLOCK","bash.rm_rf","policy.violation"],
     ["WARN","slack.post_message","sensitive.channel"],["ALLOW","filesystem.read","12ms"],
     ["BLOCK","github.comment","indirect.prompt.injection"]].forEach(function (e) {
      var row = document.createElement("div");
      row.className = "term-line"; row.style.animation = "none"; row.style.opacity = "1"; row.style.transform = "none";
      row.innerHTML = '<span class="verb ' + e[0] + '">' + e[0] + '</span>' +
                      '<span class="tool">' + e[1] + '</span>' +
                      '<span class="meta">' + e[2] + '</span>';
      termFeed.appendChild(row);
    });
  }

  /* ---------- 7. scroll-reveal ---------- */
  var revealEls = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window && !prefersReduced) {
    var ro = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (en.isIntersecting) { en.target.classList.add("in"); ro.unobserve(en.target); }
      });
    }, { threshold: 0.12, rootMargin: "0px 0px -40px 0px" });
    revealEls.forEach(function (el) { ro.observe(el); });
  } else {
    revealEls.forEach(function (el) { el.classList.add("in"); });
  }

  /* ---------- 8. mouse glow + bento spotlight ---------- */
  var glow = document.getElementById("mouseGlow");
  if (glow && !prefersReduced && window.matchMedia("(pointer: fine)").matches) {
    var gx = 0, gy = 0, raf = null;
    window.addEventListener("mousemove", function (e) {
      gx = e.clientX; gy = e.clientY;
      if (!raf) raf = requestAnimationFrame(function () {
        glow.style.transform = "translate(" + (gx - 300) + "px," + (gy - 300) + "px)";
        raf = null;
      });
    }, { passive: true });
  } else if (glow) {
    glow.style.display = "none";
  }

  document.querySelectorAll(".bento-card").forEach(function (card) {
    card.addEventListener("mousemove", function (e) {
      var r = card.getBoundingClientRect();
      card.style.setProperty("--mx", (e.clientX - r.left) + "px");
      card.style.setProperty("--my", (e.clientY - r.top) + "px");
    });
  });
})();
