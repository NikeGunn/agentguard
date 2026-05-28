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
})();
