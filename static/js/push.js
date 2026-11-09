// Phone notifications for the installed app.
//
//  - Once the site is installed (opened from the home screen) and the person is logged in, a small
//    card offers to turn notifications on. The browser's permission question only appears after
//    they tap "Turn on", which is also what iPhones require.
//  - After they allow it, a real notification is sent straight away so they can see it works.
//  - Account > "Phone notifications" uses the same code to turn them on/off and send a test.
(function () {
  'use strict';
  var supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;
  var standalone = (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) || window.navigator.standalone === true;
  var DISMISS_KEY = 'laikihub-push-dismissed';

  function store(k, v) { try { if (v === undefined) return localStorage.getItem(k); localStorage.setItem(k, v); } catch (_) { return null; } }
  function keyBytes(b64) {
    var pad = '='.repeat((4 - (b64.length % 4)) % 4);
    var raw = atob((b64 + pad).replace(/-/g, '+').replace(/_/g, '/'));
    return Uint8Array.from(raw, function (c) { return c.charCodeAt(0); });
  }
  function sameKey(sub, bytes) {
    var cur = sub.options && sub.options.applicationServerKey;
    if (!cur) return true;
    var a = new Uint8Array(cur);
    if (a.length !== bytes.length) return false;
    for (var i = 0; i < a.length; i++) if (a[i] !== bytes[i]) return false;
    return true;
  }
  function json(url, opts) {
    opts = opts || {};
    opts.credentials = 'same-origin';
    opts.headers = Object.assign({ 'Accept': 'application/json' }, opts.headers || {});
    return fetch(url, opts).then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { d.__status = r.status; return d; }); });
  }
  function post(url, body) {
    return json(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
  }

  var api = { supported: supported, standalone: standalone };

  api.state = async function () {
    if (!supported) return { state: 'unsupported' };
    var st = await json('/push/status', { cache: 'no-store' });
    if (st.__status === 401) return { state: 'loggedout' };
    if (!st.enabled) return { state: 'off-server' };
    if (Notification.permission === 'denied') return { state: 'blocked', devices: st.devices };
    var reg = await navigator.serviceWorker.ready;
    var sub = await reg.pushManager.getSubscription();
    if (sub && Notification.permission === 'granted') return { state: 'on', devices: st.devices, key: st.key, sub: sub };
    return { state: 'off', devices: st.devices, key: st.key };
  };

  api.enable = async function () {
    var st = await json('/push/status', { cache: 'no-store' });
    if (st.__status === 401) throw new Error('Please log in first.');
    if (!st.enabled) throw new Error('Notifications are not set up on this site yet.');
    var permission = await Notification.requestPermission();
    if (permission !== 'granted') throw new Error(permission === 'denied' ? 'Notifications are blocked for this app. Turn them on in your phone settings.' : 'Notifications were not allowed.');
    var reg = await navigator.serviceWorker.ready;
    var bytes = keyBytes(st.key);
    var sub = await reg.pushManager.getSubscription();
    if (sub && !sameKey(sub, bytes)) { await sub.unsubscribe(); sub = null; }
    if (!sub) sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: bytes });
    var saved = await post('/push/subscribe', sub.toJSON());
    if (saved.__status !== 200) throw new Error(saved.error || 'Could not save your notification setting.');
    return true;
  };

  api.disable = async function () {
    var reg = await navigator.serviceWorker.ready;
    var sub = await reg.pushManager.getSubscription();
    if (sub) {
      await post('/push/unsubscribe', { endpoint: sub.endpoint });
      await sub.unsubscribe();
    }
    return true;
  };

  api.test = async function () {
    var r = await post('/push/test');
    if (r.__status !== 200) throw new Error(r.error || 'Could not send a test.');
    return r;
  };

  window.LaikiPush = api;
  if (!supported) return;

  // ---- keep an existing subscription fresh (new install, changed key, replaced by the phone) ----
  async function repair() {
    var s = await api.state();
    if (s.state === 'off' && Notification.permission === 'granted') { try { await api.enable(); } catch (_) {} }
    else if (s.state === 'on') { try { await post('/push/subscribe', s.sub.toJSON()); } catch (_) {} }
    return s;
  }

  // ---- the prompt card ----
  function toast(text) {
    var t = document.createElement('div');
    t.className = 'push-toast'; t.setAttribute('role', 'status'); t.textContent = text;
    document.body.appendChild(t);
    setTimeout(function () { t.remove(); }, 4500);
  }
  function showCard() {
    if (document.querySelector('.push-card')) return;
    var card = document.createElement('section');
    card.className = 'push-card'; card.setAttribute('role', 'dialog'); card.setAttribute('aria-label', 'Turn on notifications');
    card.innerHTML = '<div class="push-ico" aria-hidden="true">&#128276;</div><div class="push-text"><strong>Get order updates on your phone</strong><span>We will tell you when your order is confirmed, on its way or ready, and when a shop replies.</span></div><div class="push-actions"><button type="button" class="push-on">Turn on</button><button type="button" class="push-later">Not now</button></div>';
    document.body.appendChild(card);
    card.querySelector('.push-later').addEventListener('click', function () { store(DISMISS_KEY, String(Date.now())); card.remove(); });
    card.querySelector('.push-on').addEventListener('click', async function (ev) {
      var btn = ev.currentTarget; btn.disabled = true; btn.textContent = 'Turning on…';
      try {
        await api.enable();
        card.remove();
        try { await api.test(); } catch (_) {}
        toast('Notifications are on. A test notification is on its way.');
      } catch (err) {
        btn.disabled = false; btn.textContent = 'Turn on';
        card.querySelector('.push-text span').textContent = err.message || 'Something went wrong. Please try again.';
      }
    });
  }
  async function maybePrompt() {
    if (!standalone) return;                              // only inside the installed app
    var s = await repair();
    if (s.state !== 'off' || Notification.permission !== 'default') return;
    var last = Number(store(DISMISS_KEY) || 0);
    if (last && Date.now() - last < 3 * 24 * 3600 * 1000) return;   // asked recently
    setTimeout(showCard, 2500);
  }

  window.addEventListener('load', function () { maybePrompt().catch(function () {}); });
  window.addEventListener('appinstalled', function () {
    // The install event happens in the browser, not the app; the card shows on the first app launch.
    store(DISMISS_KEY, '0');
  });

  // ---- Account > Phone notifications panel ----
  var panel = document.querySelector('[data-push-panel]');
  if (panel) {
    var status = panel.querySelector('[data-push-status]');
    var on = panel.querySelector('[data-push-on]');
    var off = panel.querySelector('[data-push-off]');
    var test = panel.querySelector('[data-push-test]');
    var msg = panel.querySelector('[data-push-msg]');
    var say = function (t) { if (msg) msg.textContent = t || ''; };
    var render = async function () {
      var s = await api.state();
      var text = {
        unsupported: 'This browser cannot receive notifications. On iPhone, add the site to your Home Screen first (Share, then Add to Home Screen) and open it from there.',
        loggedout: 'Log in to manage notifications.',
        'off-server': 'Notifications are not set up on this site yet.',
        blocked: 'Notifications are blocked for this site. Allow them in your phone or browser settings, then come back.',
        on: 'On for this device.',
        off: 'Off for this device.'
      }[s.state];
      status.textContent = text + (s.devices ? ' (' + s.devices + ' device' + (s.devices === 1 ? '' : 's') + ' total)' : '');
      on.hidden = s.state !== 'off';
      off.hidden = s.state !== 'on';
      test.hidden = s.state !== 'on';
    };
    on.addEventListener('click', async function () { on.disabled = true; say(''); try { await api.enable(); await api.test(); say('Notifications are on. A test is on its way.'); } catch (e) { say(e.message); } on.disabled = false; render(); });
    off.addEventListener('click', async function () { say(''); try { await api.disable(); say('Notifications are off for this device.'); } catch (e) { say(e.message); } render(); });
    test.addEventListener('click', async function () {
      test.disabled = true; say('Sending…');
      try {
        var r = await api.test();
        say(r.sent > 0 ? 'Sent to ' + r.sent + ' device' + (r.sent === 1 ? '' : 's') + '. It should appear in a few seconds.' : (r.devices === 0 ? 'No device is subscribed yet.' : 'The phone did not accept it. Turn notifications off and on again.'));
      } catch (e) { say(e.message); }
      test.disabled = false; render();
    });
    render();
  }
})();
