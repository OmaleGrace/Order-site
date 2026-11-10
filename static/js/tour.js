/* LaikiHub product tour: spotlights real elements on the page you are on.
   The tour for a page is picked by its address (/admin, /vendor, /rider, otherwise customer).
   A step is shown only if its element exists on the page, and the server only renders links a
   person may use, so the tour can never point at something they are not allowed to open.
   Finished tours are remembered on this device. Restart: any element with data-tour-start,
   the round help button, or LaikiTour.start(). */
(function () {
  'use strict';
  var path = location.pathname;
  var role = path.indexOf('/admin') === 0 ? 'admin' : path.indexOf('/vendor') === 0 ? 'vendor' : path.indexOf('/rider') === 0 ? 'rider' : 'customer';
  var nav = '.portal-nav a[href="%"], header a[href="%"], nav a[href="%"]';
  function link(h) { return nav.replace(/%/g, h); }
  var TOURS = {
    customer: [
      { sel: '.hero-search, .home-search, input[type=search]', t: 'Search', d: 'Type a dish or a shop name to find what you are craving.' },
      { sel: '.home-section, .shop-card, .dish-card', t: 'Browse shops and food', d: 'Scroll to see shops near you and popular dishes. Tap any card for details.' },
      { sel: '.bottom-nav a[href="/cart"], a[href="/cart"]', t: 'Your cart', d: 'Everything you add waits here. Check out when you are ready.' },
      { sel: '.bottom-nav a[href="/orders"], a[href="/orders"]', t: 'Track orders', d: 'See each order move from the kitchen to your door, and reorder in one tap.' },
      { sel: 'a[href="/wallet"]', t: 'Wallet', d: 'Your balance and payment history.' },
      { sel: '.bottom-nav a[href="/account"], a[href="/account"]', t: 'Account', d: 'Update your details, addresses and appearance.' }
    ],
    vendor: [
      { sel: link('/vendor/dashboard'), t: 'Dashboard', d: 'Today at a glance: sales, orders and what needs attention.' },
      { sel: link('/vendor/orders'), t: 'Orders', d: 'Accept, prepare and complete customer orders.' },
      { sel: link('/vendor/menu'), t: 'Menu', d: 'Add dishes, change prices and mark items sold out.' },
      { sel: link('/vendor/analytics'), t: 'Analytics', d: 'See what sells best and when.' },
      { sel: link('/vendor/statements'), t: 'Earnings', d: 'Statements and payouts for your shop.' },
      { sel: link('/vendor/coupons'), t: 'Coupons', d: 'Create discount codes for your customers.' },
      { sel: link('/vendor/delivery'), t: 'Delivery', d: 'Set the areas you deliver to and your prices.' },
      { sel: link('/vendor/ai'), t: 'AI writing', d: 'Draft dish descriptions and replies in seconds.' }
    ],
    rider: [
      { sel: '.hero, header', t: 'Your delivery', d: 'This page shows the order you are delivering.' },
      { sel: 'select', t: 'Delivery status', d: 'Pick the current step so the customer sees live updates.' },
      { sel: '.btn', t: 'Save the update', d: 'Tap to send the new status to the customer.' }
    ],
    admin: [
      { sel: link('/admin'), t: 'Overview', d: 'Platform totals and recent activity.' },
      { sel: link('/admin/orders'), t: 'Orders', d: 'Watch and manage every order.' },
      { sel: link('/admin/vendors'), t: 'Vendors', d: 'Review and manage shops.' },
      { sel: link('/admin/riders'), t: 'Riders', d: 'Assign and manage delivery riders.' },
      { sel: link('/admin/users'), t: 'Users', d: 'Find and manage accounts.' },
      { sel: link('/admin/settlements'), t: 'Settlements', d: 'Review payouts and revenue reports.' }
    ]
  };
  var KEY = 'laikihub_tour_done_' + role;
  var hole, card, steps, i;

  function done() { try { localStorage.setItem(KEY, '1'); } catch (_) {} }
  function visible(el) { var r = el.getBoundingClientRect(); return r.width > 0 && r.height > 0; }
  function find(sel) { var n = document.querySelectorAll(sel); for (var k = 0; k < n.length; k++) if (visible(n[k])) return n[k]; return null; }

  function end(finished) {
    if (finished) done();
    if (hole) hole.remove();
    if (card) card.remove();
    document.removeEventListener('keydown', onKey);
    window.removeEventListener('resize', place);
    hole = card = null;
  }
  function onKey(e) { if (e.key === 'Escape') end(false); else if (e.key === 'ArrowRight') go(1); else if (e.key === 'ArrowLeft') go(-1); }
  function place() {
    if (!hole || !steps[i]) return;
    var r = steps[i].el.getBoundingClientRect(), pad = 6;
    hole.style.cssText = 'top:' + (r.top - pad) + 'px;left:' + (r.left - pad) + 'px;width:' + (r.width + pad * 2) + 'px;height:' + (r.height + pad * 2) + 'px';
    if (window.innerWidth >= 760) {
      var below = r.bottom + 16 + 190 < window.innerHeight;
      card.style.top = below ? Math.min(r.bottom + 16, window.innerHeight - 200) + 'px' : Math.max(12, r.top - 196) + 'px';
      card.style.left = Math.max(12, Math.min(r.left, window.innerWidth - 392)) + 'px';
      card.style.bottom = 'auto';
    }
  }
  function render() {
    var s = steps[i];
    s.el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'smooth' });
    card.innerHTML = '<div class="ds-tour-bar" aria-hidden="true"><i style="width:' + ((i + 1) / steps.length * 100) + '%"></i></div>' +
      '<h3></h3><p></p><div class="ds-tour-actions"><button type="button" class="ds-tour-skip">Skip</button>' +
      (i > 0 ? '<button type="button" class="btn ds-btn ds-btn-ghost" data-d="-1">Back</button>' : '') +
      '<button type="button" class="btn ds-btn ds-btn-primary" data-d="1">' + (i === steps.length - 1 ? 'Done' : 'Next') + '</button></div>';
    card.querySelector('h3').textContent = s.t;
    card.querySelector('p').textContent = s.d + '  (' + (i + 1) + ' of ' + steps.length + ')';
    card.querySelector('.ds-tour-skip').onclick = function () { end(true); };
    var b = card.querySelectorAll('[data-d]');
    for (var k = 0; k < b.length; k++) b[k].onclick = function () { go(+this.getAttribute('data-d')); };
    setTimeout(place, 320);
    place();
    card.querySelector('[data-d="1"]').focus({ preventScroll: true });
  }
  function go(d) {
    if (i + d >= steps.length) return end(true);
    if (i + d < 0) return;
    i += d; render();
  }
  function start() {
    end(false);
    steps = [];
    (TOURS[role] || []).forEach(function (s) { var el = find(s.sel); if (el) { s.el = el; steps.push(s); } });
    if (!steps.length) { if (window.LaikiUI) window.LaikiUI.toast('There is no tour for this page.', { type: 'info' }); return; }
    i = 0;
    hole = document.createElement('div'); hole.className = 'ds-tour-hole';
    card = document.createElement('div'); card.className = 'ds-tour-card';
    card.setAttribute('role', 'dialog'); card.setAttribute('aria-live', 'polite'); card.setAttribute('aria-label', 'Product tour');
    document.body.appendChild(hole); document.body.appendChild(card);
    document.addEventListener('keydown', onKey);
    window.addEventListener('resize', place);
    render();
  }

  document.addEventListener('click', function (e) {
    var t = e.target.closest && e.target.closest('[data-tour-start]');
    if (t) { e.preventDefault(); start(); }
  });
  function addButton() {
    if (document.querySelector('.ds-tour-fab') || path.indexOf('/login') === 0 || path.indexOf('/signup') === 0) return;
    var b = document.createElement('button');
    b.type = 'button'; b.className = 'ds-tour-fab'; b.setAttribute('data-tour-start', '');
    b.setAttribute('aria-label', 'Take a tour of this page');
    b.innerHTML = window.LaikiIcons ? window.LaikiIcons.svg('help') : '?';
    document.body.appendChild(b);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', addButton); else addButton();
  window.LaikiTour = { start: start, role: role };
})();
