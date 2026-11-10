/* LaikiHub UI kit: small, dependency-free helpers shared by every page.
   window.LaikiUI.toast("Saved", {type: "success"})   // type: info | success | error | warn
   window.LaikiUI.busy(button, true|false)             // spinner on a button while work happens
   Nothing here changes what a form sends or where it goes. */
(function () {
  'use strict';
  var region;

  function getRegion() {
    if (region && document.body.contains(region)) return region;
    region = document.createElement('div');
    region.className = 'ds-toasts';
    region.setAttribute('role', 'status');
    region.setAttribute('aria-live', 'polite');
    document.body.appendChild(region);
    return region;
  }

  function toast(message, opts) {
    opts = opts || {};
    var type = opts.type || 'info';
    var el = document.createElement('div');
    el.className = 'ds-toast';
    el.setAttribute('data-type', type);
    if (type === 'error') el.setAttribute('role', 'alert');
    var text = document.createElement('span');
    text.textContent = String(message == null ? '' : message);
    var close = document.createElement('button');
    close.type = 'button';
    close.setAttribute('aria-label', 'Dismiss');
    close.textContent = '\u00d7';
    el.appendChild(text);
    el.appendChild(close);
    getRegion().appendChild(el);

    var timer;
    function dismiss() {
      clearTimeout(timer);
      if (!el.parentNode) return;
      el.classList.add('is-leaving');
      setTimeout(function () { if (el.parentNode) el.parentNode.removeChild(el); }, 220);
    }
    close.addEventListener('click', dismiss);
    var ms = typeof opts.duration === 'number' ? opts.duration : (type === 'error' ? 6000 : 3500);
    if (ms > 0) timer = setTimeout(dismiss, ms);
    return dismiss;
  }

  function busy(btn, on) {
    if (!btn) return;
    if (on) { btn.setAttribute('aria-busy', 'true'); } else { btn.removeAttribute('aria-busy'); }
  }

  // Spinner on the button that submitted a normal (page-reloading) form. Forms handled by
  // script call preventDefault, so they are left alone.
  document.addEventListener('submit', function (e) {
    var form = e.target;
    var btn = e.submitter;
    if (!btn || !form || form.hasAttribute('data-no-busy')) return;
    setTimeout(function () {
      if (e.defaultPrevented || !form.checkValidity()) return;
      busy(btn, true);
    }, 0);
  });
  window.addEventListener('pageshow', function (e) {
    if (!e.persisted) return;
    var els = document.querySelectorAll('[aria-busy="true"]');
    for (var i = 0; i < els.length; i++) els[i].removeAttribute('aria-busy');
  });

  // Keyboard users can jump past the header to the page content.
  function addSkipLink() {
    var main = document.querySelector('main');
    if (!main || document.querySelector('.ds-skip')) return;
    if (!main.id) main.id = 'main-content';
    var a = document.createElement('a');
    a.className = 'ds-skip';
    a.href = '#' + main.id;
    a.textContent = 'Skip to content';
    document.body.insertBefore(a, document.body.firstChild);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', addSkipLink);
  else addSkipLink();


  // Branded launch screen: only when opened as an installed app, once per session, never on reloads.
  function launchScreen() {
    var standalone = (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) || window.navigator.standalone;
    if (!standalone) return;
    try { if (sessionStorage.getItem('ds_launched')) return; sessionStorage.setItem('ds_launched', '1'); } catch (_) { return; }
    var el = document.createElement('div');
    el.className = 'ds-launch';
    el.setAttribute('aria-hidden', 'true');
    el.innerHTML = '<div class="ds-launch-mark"><img class="no-ph" src="/static/img/laikihub-mark.svg" alt="" width="72" height="72"><span>' + (document.title.split('|').pop() || '').trim() + '</span></div>';
    document.body.appendChild(el);
    setTimeout(function () { el.classList.add('is-done'); setTimeout(function () { el.remove(); }, 400); }, 450);
  }
  // Photos fade their shimmer placeholder away once loaded.
  function watchImages() {
    var imgs = document.images;
    for (var k = 0; k < imgs.length; k++) {
      var im = imgs[k];
      if (im.complete) im.classList.add('is-loaded');
      else { im.addEventListener('load', function () { this.classList.add('is-loaded'); }, { once: true }); im.addEventListener('error', function () { this.classList.add('is-loaded'); }, { once: true }); }
    }
  }
  function boot() { launchScreen(); watchImages(); }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();

  window.LaikiUI = { toast: toast, busy: busy };
})();
