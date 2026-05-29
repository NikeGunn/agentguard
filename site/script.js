// AgentGuard landing-page enhancements:
//  1. smart OS-detecting install widget (hero)
//  2. click-to-copy on <pre class="copy"> + the install widget
//  3. scroll-spy nav highlighting
//  4. tab switcher for per-agent setup blocks
(function () {
  "use strict";

  /* ---------- 1. smart install widget ---------- */
  var widget = document.getElementById("installWidget");
  if (widget) {
    var tabs  = widget.querySelectorAll(".iw-tab");
    var panes = widget.querySelectorAll(".iw-pane");
    var detectedLabel = document.getElementById("iwDetectedOs");
    var copyBtn = document.getElementById("iwCopyBtn");

    function detectPlatform() {
      // Prefer modern userAgentData when available (Chrome/Edge); fall
      // back to userAgent string parsing.
      var uaData = navigator.userAgentData;
      var platform = "";
      if (uaData && uaData.platform) {
        platform = uaData.platform.toLowerCase();
      } else {
        platform = (navigator.platform || "").toLowerCase();
      }
      var ua = (navigator.userAgent || "").toLowerCase();

      if (platform.indexOf("mac") !== -1 || ua.indexOf("mac os") !== -1) {
        return { key: "macos", label: "macOS" };
      }
      if (platform.indexOf("win") !== -1 || ua.indexOf("windows") !== -1) {
        // Windows users usually have PowerShell available everywhere;
        // recommend it. Git Bash is one tab away if they prefer.
        return { key: "windows-ps", label: "Windows (PowerShell)" };
      }
      if (platform.indexOf("linux") !== -1 || ua.indexOf("linux") !== -1 ||
          ua.indexOf("x11") !== -1 || ua.indexOf("cros") !== -1) {
        return { key: "linux", label: "Linux" };
      }
      // Unknown — default to the most general path.
      return { key: "linux", label: "Linux / Unix" };
    }

    function activate(key) {
      tabs.forEach(function (t) {
        t.classList.toggle("active", t.dataset.platform === key);
      });
      panes.forEach(function (p) {
        p.classList.toggle("active", p.dataset.pane === key);
      });
    }

    var detected = detectPlatform();
    if (detectedLabel) detectedLabel.textContent = detected.label;
    // Mark the detected tab with a sparkle pseudo-element.
    tabs.forEach(function (t) {
      if (t.dataset.platform === detected.key) t.classList.add("recommended");
    });
    activate(detected.key);

    // Tab clicks
    tabs.forEach(function (tab) {
      tab.addEventListener("click", function () {
        activate(tab.dataset.platform);
      });
    });

    // Copy button — copies the currently-active pane's command
    if (copyBtn) {
      copyBtn.addEventListener("click", function () {
        var activePane = widget.querySelector(".iw-pane.active");
        if (!activePane) return;
        var codeEl = activePane.querySelector(".iw-cmd");
        if (!codeEl) return;
        var text = codeEl.innerText.trim();
        var ok = function () {
          copyBtn.classList.add("copied");
          var span = copyBtn.querySelector("span");
          var prev = span ? span.textContent : "";
          if (span) span.textContent = "Copied!";
          setTimeout(function () {
            copyBtn.classList.remove("copied");
            if (span) span.textContent = prev || "Copy";
          }, 1600);
        };
        if (navigator.clipboard && window.isSecureContext) {
          navigator.clipboard.writeText(text).then(ok).catch(function () {});
        } else {
          var ta = document.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed"; ta.style.opacity = "0";
          document.body.appendChild(ta);
          ta.select();
          try { document.execCommand("copy"); ok(); } catch (_) {}
          document.body.removeChild(ta);
        }
      });
    }
  }

  /* ---------- 2. click-to-copy on every <pre class="copy"> ---------- */
  document.querySelectorAll("pre.copy").forEach(function (el) {
    el.addEventListener("click", function () {
      var text = el.innerText.trim();
      var ok = function () {
        el.classList.add("copied");
        setTimeout(function () { el.classList.remove("copied"); }, 1400);
      };
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(ok).catch(function () {});
      } else {
        var ta = document.createElement("textarea");
        ta.value = text;
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); ok(); } catch (_) {}
        document.body.removeChild(ta);
      }
    });
  });

  /* ---------- 3. scroll-spy ---------- */
  var links = Array.prototype.slice.call(document.querySelectorAll(".nav nav a[href^='#']"));
  var sections = links.map(function (a) {
    var id = a.getAttribute("href").slice(1);
    return { link: a, el: document.getElementById(id) };
  }).filter(function (x) { return x.el; });

  function spy() {
    var y = window.scrollY + 90;
    var cur = null;
    for (var i = 0; i < sections.length; i++) {
      if (sections[i].el.offsetTop <= y) cur = sections[i];
    }
    links.forEach(function (l) { l.classList.remove("active"); });
    if (cur) cur.link.classList.add("active");
  }
  window.addEventListener("scroll", spy, { passive: true });
  spy();

  /* ---------- 4. tab switcher for [data-tabs] blocks ---------- */
  document.querySelectorAll("[data-tabs]").forEach(function (root) {
    var tabs  = root.querySelectorAll(".tab");
    var panes = root.querySelectorAll(".tab-pane");
    tabs.forEach(function (tab, i) {
      tab.addEventListener("click", function () {
        tabs.forEach(function (t) { t.classList.remove("active"); });
        panes.forEach(function (p) { p.classList.remove("active"); });
        tab.classList.add("active");
        if (panes[i]) panes[i].classList.add("active");
      });
    });
  });

  /* ---------- 5. AI-agent setup prompt generator ---------- */
  // Pick an agent, get a tailored, copy-ready prompt the user pastes into that
  // agent's chat. The agent then sets AgentGuard up unattended. 100% client-side.
  var pg = document.getElementById("promptGen");
  if (pg) {
    // Per-agent specifics: human name, the config file the agent should expect
    // AgentGuard to patch, and whether a shipped Skill is available.
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
        var text = pgPrompt ? pgPrompt.textContent : "";
        var ok = function () {
          pgCopy.classList.add("copied");
          var span = pgCopy.querySelector("span");
          var prev = span ? span.textContent : "";
          if (span) span.textContent = "Copied!";
          setTimeout(function () {
            pgCopy.classList.remove("copied");
            if (span) span.textContent = prev || "Copy prompt";
          }, 1600);
        };
        if (navigator.clipboard && window.isSecureContext) {
          navigator.clipboard.writeText(text).then(ok).catch(function () {});
        } else {
          var ta = document.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed"; ta.style.opacity = "0";
          document.body.appendChild(ta);
          ta.select();
          try { document.execCommand("copy"); ok(); } catch (_) {}
          document.body.removeChild(ta);
        }
      });
    }
  }
})();
