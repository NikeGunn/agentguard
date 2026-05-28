// Tiny landing-page enhancements:
//  1. click-to-copy on every <pre class="copy">
//  2. stage-step accordion behavior (none yet — kept as a hook)
//  3. nav-link "active" highlight on scroll
//  4. tab switcher for the per-agent setup blocks
(function () {
  "use strict";

  // copy-on-click
  document.querySelectorAll("pre.copy").forEach(function (el) {
    el.addEventListener("click", function () {
      var text = el.innerText.trim();
      var ok = function () {
        el.classList.add("copied");
        setTimeout(function () { el.classList.remove("copied"); }, 1400);
      };
      if (navigator.clipboard) {
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

  // scroll-spy
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

  // tabbed agent setup
  document.querySelectorAll("[data-tabs]").forEach(function (root) {
    var tabs = root.querySelectorAll(".tab");
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
