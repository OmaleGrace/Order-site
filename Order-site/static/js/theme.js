// Applies the chosen theme before the page is drawn, so there is no flash of the wrong colours.
// Load it in <head> without "defer". The choice lives in a cookie named "theme"
// (light, dark or system) and in localStorage as a backup.
(function () {
  'use strict';
  var root = document.documentElement;
  var mq = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;

  function readCookie() {
    var m = document.cookie.match(/(?:^|;\s*)theme=(light|dark|system)/);
    return m ? m[1] : null;
  }
  function stored() {
    var c = readCookie();
    if (c) return c;
    try { var v = localStorage.getItem('theme'); if (v === 'light' || v === 'dark' || v === 'system') return v; } catch (_) {}
    return 'system';
  }
  function resolve(choice) { return choice === 'system' ? (mq && mq.matches ? 'dark' : 'light') : choice; }

  function apply(choice) {
    var theme = resolve(choice);
    root.setAttribute('data-theme', theme);
    root.setAttribute('data-theme-choice', choice);
    root.style.colorScheme = theme;
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', theme === 'dark' ? '#121815' : '#174a32');
  }
  function remember(choice) {
    try { localStorage.setItem('theme', choice); } catch (_) {}
    document.cookie = 'theme=' + choice + '; path=/; max-age=31536000; samesite=lax' + (location.protocol === 'https:' ? '; secure' : '');
  }

  apply(stored());
  if (mq) {
    var onChange = function () { if (stored() === 'system') apply('system'); };
    if (mq.addEventListener) mq.addEventListener('change', onChange); else if (mq.addListener) mq.addListener(onChange);
  }

  window.LaikiTheme = { set: function (choice) { remember(choice); apply(choice); }, get: stored };

  // Account page: keep this device in step with the saved setting, and save changes right away.
  document.addEventListener('DOMContentLoaded', function () {
    var panel = document.querySelector('[data-theme-panel]');
    if (!panel) return;
    var saved = panel.getAttribute('data-theme-saved');
    if (saved === 'light' || saved === 'dark' || saved === 'system') { remember(saved); apply(saved); }
    var form = panel.querySelector('form');
    form.classList.add('is-live');
    form.addEventListener('change', function (e) {
      if (!e.target || e.target.name !== 'theme') return;
      var choice = e.target.value;
      window.LaikiTheme.set(choice);
      var body = new URLSearchParams({ theme: choice });
      fetch('/account/theme', { method: 'POST', credentials: 'same-origin', headers: { 'Accept': 'application/json', 'Content-Type': 'application/x-www-form-urlencoded' }, body: body }).catch(function () {});
    });
  });
})();
