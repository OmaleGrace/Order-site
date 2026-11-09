/* First-launch guides: one for customers, one for vendors, one for riders.

   Which guide opens depends on the page: /vendor/... pages get the vendor guide, /rider/... pages
   get the rider guide, everything else gets the customer guide. Each guide shows itself once, the
   first time it is useful (customers and vendors: first visit from the installed app; riders: first
   time they open their delivery link). It also opens right after the phone reports the app was
   installed, whenever a button with data-app-guide-open is tapped (add ="vendor" or ="rider" to pick
   a guide), and on any page when the address ends in ?tour=1 (handy for previewing).
   Nothing is sent anywhere; "already seen" is remembered in this phone's own storage, per guide. */
(function () {
  'use strict';

  // Each guide remembers separately that it was seen. Change a key's number to show that guide again.
  var CUSTOMER_STEPS = [
    {
      art: '👋', tone: '#e7f1ea',
      title: 'Welcome to LaikiHub',
      lead: 'Order from local food shops and get it delivered or pick it up. Here is a 30-second tour.',
      points: [
        'Shops are checked before they sell here',
        'Pay securely, track your order, earn rewards'
      ]
    },
    {
      art: '🍽️', tone: '#fdeedd',
      title: 'Find something good',
      lead: 'Everything starts from the Menu and Shops tabs.',
      points: [
        'Menu shows dishes from every shop. Shops lets you browse one kitchen at a time',
        'Use search and filters to narrow down what you are craving',
        'Tap the heart to save a favourite for next time'
      ]
    },
    {
      art: '🛒', tone: '#e7f1ea',
      title: 'Order your way',
      lead: 'Add what you like to your cart, then check out in a minute.',
      points: [
        'Choose pickup or delivery at checkout',
        'Not hungry yet? Schedule your order for later',
        'Add a coupon code or spend loyalty points',
        'Pay securely through Paystack'
      ]
    },
    {
      art: '📍', tone: '#fdeedd',
      title: 'Follow your order',
      lead: 'You always know where your food is.',
      points: [
        'Live status updates as the shop prepares your order',
        'For deliveries, watch your rider on a live map',
        'Reorder a past meal in one tap, then leave a review'
      ]
    },
    {
      art: '🎁', tone: '#e7f1ea',
      title: 'Save as you go',
      lead: 'The more you order, the more you get back.',
      points: [
        'Earn loyalty points on every order',
        'Top up your wallet or redeem a gift card',
        'Invite friends with your referral link for bonus points',
        'Ordering with friends? Start a group order'
      ]
    },
    {
      art: '🧭', tone: '#fdeedd',
      title: 'Getting around',
      lead: 'The bar at the bottom of the screen is always there:',
      tabs: [['🍽️', 'Menu'], ['🛒', 'Cart'], ['📦', 'Orders'], ['👤', 'Account']],
      points: [
        'The menu button at the top right opens everything else: messages, wallet, favourites and more',
        'Stuck? Help & support has answers and a way to reach the team'
      ]
    }
  ];

  var VENDOR_STEPS = [
    {
      art: '🏪', tone: '#e7f1ea',
      title: 'Welcome to your shop',
      lead: 'This dashboard is where you run your shop from your phone. Here is a quick tour.',
      points: [
        'See recent orders, your shop status and your balance at a glance',
        'The menu button at the top right opens every tool'
      ]
    },
    {
      art: '✅', tone: '#fdeedd',
      title: 'Get your shop ready',
      lead: 'Customers can only order while your shop is active and paid up.',
      points: [
        'The Account readiness card shows what is still left to do',
        'New shops pay a one-time setup fee to open. There is no monthly fee',
        'Add and verify your payout bank account in Profile so you can be paid'
      ]
    },
    {
      art: '🍲', tone: '#e7f1ea',
      title: 'Build your menu',
      lead: 'Open My menu to add and manage your dishes.',
      points: [
        'Add dishes with prices, or duplicate one to save time',
        'Mark a dish unavailable when you run out and switch it back later',
        'Use Customizations for choices like sizes and extras'
      ]
    },
    {
      art: '🧾', tone: '#fdeedd',
      title: 'Handle orders',
      lead: 'Paid orders land in Orders.',
      points: [
        'Move each order along as you work: Preparing, Ready, Completed',
        'For deliveries, assign a rider. They get a private link to update the customer and share their live location',
        'Reply to customers in Messages'
      ]
    },
    {
      art: '🛵', tone: '#e7f1ea',
      title: 'Delivery and growth',
      lead: 'Set your terms and bring customers back.',
      points: [
        'Delivery sets your delivery fee, minimum order and free-delivery amount',
        'Coupons let you create discount codes',
        'Billing has a 30-day Pro plan with lower commission, and shop promotion'
      ]
    },
    {
      art: '💰', tone: '#fdeedd',
      title: 'Track your money',
      lead: 'Your balance is what you are owed after commission and payouts.',
      points: [
        'Billing shows your balance and the money sent to you',
        'Statements list your sales and can be exported',
        'Analytics shows how your shop is doing',
        'The AI assistant drafts menu descriptions, promotions and replies for you'
      ]
    }
  ];

  var RIDER_STEPS = [
    {
      art: '🛵', tone: '#e7f1ea',
      title: 'Your delivery task',
      lead: 'This private page is for one delivery. Use it to keep the customer updated.',
      points: [
        'No account needed. Just keep this page open while you deliver',
        'Keep the link to yourself, it is only for you'
      ]
    },
    {
      art: '🛍️', tone: '#fdeedd',
      title: 'Pick up the order',
      lead: 'Collect the order from the shop, then say so on this page.',
      points: [
        'Choose Picked up from vendor in the status box',
        'Tap Update delivery status'
      ]
    },
    {
      art: '📶', tone: '#e7f1ea',
      title: 'Update as you go',
      lead: 'Move the status forward at each stage and the customer is told.',
      points: [
        'Picked up, then On the way, then Arrived at customer',
        'Choose Delivered once the food is in their hands'
      ]
    },
    {
      art: '📍', tone: '#fdeedd',
      title: 'Share your location',
      lead: 'Let the customer follow you on a live map.',
      points: [
        'Tap Start sharing location and allow it when your phone asks',
        'Keep this page open while you ride',
        'Tap Stop sharing when you are done. Your location is sent only while sharing is on'
      ]
    },
    {
      art: '✅', tone: '#e7f1ea',
      title: 'Good to know',
      lead: 'A few things that help it all go smoothly.',
      points: [
        'If you see a secure connection warning, ask the shop for the secure (HTTPS) link, or location will not work',
        'Stuck? Contact the shop or admin who sent you the link'
      ]
    }
  ];

  var TOURS = {
    customer: { key: 'laikihub.guide.v1', steps: CUSTOMER_STEPS, start: "Let's go", end: 'Start ordering', endHref: '/menu',
                needsInstall: true,
                auto: function (p) { return { '/': 1, '/menu': 1, '/vendors': 1, '/account': 1, '/favorites': 1, '/orders': 1 }[p] === 1; } },
    vendor:   { key: 'laikihub.guide.vendor.v1', steps: VENDOR_STEPS, start: 'Show me around', end: 'Got it', endHref: null,
                needsInstall: true,
                auto: function (p) { return p === '/vendor/dashboard' || p === '/vendor/orders'; } },
    rider:    { key: 'laikihub.guide.rider.v1', steps: RIDER_STEPS, start: 'Show me', end: 'Got it', endHref: null,
                needsInstall: false, // riders open a link from a message, they do not install anything
                auto: function (p) { return /^\/rider\/[^/]+$/.test(p); } }
  };

  function currentPath() { return location.pathname.replace(/\/+$/, '') || '/'; }
  function tourForPath(p) {
    if (/^\/vendor(\/|$)/.test(p)) return 'vendor';
    if (/^\/rider\//.test(p)) return 'rider';
    return 'customer';
  }

  var overlay = null, slide = null, idx = 0, opener = null, tour = null, STEPS = [];

  function seen(t) { try { return localStorage.getItem(t.key) === '1'; } catch (e) { return false; } }
  function markSeen(t) { try { localStorage.setItem(t.key, '1'); } catch (e) { /* storage blocked: fine */ } }

  function isInstalledApp() {
    var mm = window.matchMedia ? function (q) { return window.matchMedia(q).matches; } : function () { return false; };
    return mm('(display-mode: standalone)') || mm('(display-mode: fullscreen)') ||
      window.navigator.standalone === true || /[?&]source=pwa\b/.test(location.search);
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function build() {
    overlay = el('div', 'ag-overlay');
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-modal', 'true');
    overlay.setAttribute('aria-labelledby', 'ag-title');
    overlay.setAttribute('aria-describedby', 'ag-lead');

    var sheet = el('div', 'ag-sheet');
    sheet.tabIndex = -1; // focus lands here first, so no ring shows for touch users
    var skip = el('button', 'ag-skip', 'Skip'); skip.type = 'button';
    slide = el('div', 'ag-slide'); slide.setAttribute('aria-live', 'polite');
    var dots = el('div', 'ag-dots'); dots.setAttribute('aria-hidden', 'true');
    for (var i = 0; i < STEPS.length; i++) dots.appendChild(el('i'));
    var actions = el('div', 'ag-actions');
    var back = el('button', 'ag-btn ag-back', 'Back'); back.type = 'button';
    var next = el('button', 'ag-btn ag-next', 'Next'); next.type = 'button';
    actions.appendChild(back); actions.appendChild(next);
    sheet.appendChild(skip); sheet.appendChild(slide); sheet.appendChild(dots); sheet.appendChild(actions);
    overlay.appendChild(sheet);

    skip.addEventListener('click', function () { close(false); });
    back.addEventListener('click', function () { go(idx - 1, 'back'); });
    next.addEventListener('click', function () {
      if (idx === STEPS.length - 1) close(true); else go(idx + 1, 'next');
    });

    // Swipe left / right to move between steps
    var x0 = null, y0 = null;
    overlay.addEventListener('touchstart', function (e) { x0 = e.touches[0].clientX; y0 = e.touches[0].clientY; }, { passive: true });
    overlay.addEventListener('touchend', function (e) {
      if (x0 === null) return;
      var dx = e.changedTouches[0].clientX - x0, dy = e.changedTouches[0].clientY - y0;
      x0 = null;
      if (Math.abs(dx) > 55 && Math.abs(dx) > Math.abs(dy) * 1.6) {
        if (dx < 0 && idx < STEPS.length - 1) go(idx + 1, 'next');
        else if (dx > 0 && idx > 0) go(idx - 1, 'back');
      }
    }, { passive: true });

    overlay._parts = { skip: skip, back: back, next: next, dots: dots, sheet: sheet };
  }

  function render(dir) {
    var s = STEPS[idx], p = overlay._parts;
    slide.textContent = '';
    slide.setAttribute('data-dir', dir || 'next');
    // restart the little slide-in animation
    slide.style.animation = 'none'; void slide.offsetWidth; slide.style.animation = '';

    var art = el('div', 'ag-art', s.art); art.setAttribute('aria-hidden', 'true'); art.style.setProperty('--ag-tone', s.tone);
    var title = el('h2', 'ag-title', s.title); title.id = 'ag-title';
    var lead = el('p', 'ag-lead', s.lead); lead.id = 'ag-lead';
    slide.appendChild(art);
    slide.appendChild(el('p', 'ag-count', 'Step ' + (idx + 1) + ' of ' + STEPS.length));
    slide.appendChild(title); slide.appendChild(lead);
    if (s.tabs) {
      var tabs = el('div', 'ag-tabs'); tabs.setAttribute('aria-hidden', 'true');
      s.tabs.forEach(function (t) { var c = el('span'); c.appendChild(el('b', null, t[0])); c.appendChild(document.createTextNode(t[1])); tabs.appendChild(c); });
      slide.appendChild(tabs);
    }
    var ul = el('ul', 'ag-points');
    s.points.forEach(function (t) { ul.appendChild(el('li', null, t)); });
    slide.appendChild(ul);

    var dots = p.dots.children;
    for (var i = 0; i < dots.length; i++) dots[i].className = i === idx ? 'on' : '';
    p.back.style.display = idx === 0 ? 'none' : '';
    var last = idx === STEPS.length - 1;
    p.next.textContent = idx === 0 ? tour.start : last ? tour.end : 'Next';
    p.skip.style.visibility = last ? 'hidden' : '';
  }

  function go(n, dir) {
    if (n < 0 || n >= STEPS.length) return;
    idx = n; render(dir);
    overlay._parts.sheet.scrollTop = 0;
  }

  function focusables() {
    return Array.prototype.filter.call(overlay.querySelectorAll('button'), function (b) {
      return b.style.display !== 'none' && b.style.visibility !== 'hidden';
    });
  }

  function onKey(e) {
    if (!overlay) return;
    if (e.key === 'Escape') { e.preventDefault(); close(false); }
    else if (e.key === 'ArrowRight' && idx < STEPS.length - 1) { go(idx + 1, 'next'); }
    else if (e.key === 'ArrowLeft' && idx > 0) { go(idx - 1, 'back'); }
    else if (e.key === 'Tab') {
      var f = focusables(); if (!f.length) return;
      var first = f[0], lastEl = f[f.length - 1];
      if (e.shiftKey && (document.activeElement === first || document.activeElement === overlay._parts.sheet)) { e.preventDefault(); lastEl.focus(); }
      else if (!e.shiftKey && document.activeElement === lastEl) { e.preventDefault(); first.focus(); }
      else if (!overlay.contains(document.activeElement)) { e.preventDefault(); first.focus(); }
    }
  }

  function open(name, from) {
    if (overlay) return;
    tour = TOURS[name] || TOURS.customer; STEPS = tour.steps;
    opener = from || document.activeElement;
    idx = 0;
    build(); render('next');
    document.body.appendChild(overlay);
    document.documentElement.classList.add('ag-lock');
    document.addEventListener('keydown', onKey);
    overlay._parts.sheet.focus({ preventScroll: true });
  }

  function close(finished) {
    if (!overlay) return;
    markSeen(tour); // skipped or finished, either way do not pop up again on its own
    document.removeEventListener('keydown', onKey);
    overlay.parentNode.removeChild(overlay);
    overlay = null; slide = null;
    document.documentElement.classList.remove('ag-lock');
    if (finished && tour.endHref && currentPath() !== tour.endHref) { location.href = tour.endHref; return; }
    if (opener && opener.focus) { try { opener.focus({ preventScroll: true }); } catch (e) { /* element gone */ } }
  }

  function start() {
    var path = currentPath(), name = tourForPath(path), t = TOURS[name];
    if (/[?&]tour=1\b/.test(location.search)) {
      // Preview link: open on any page, then clean the address so a refresh does not reopen it
      try { history.replaceState(null, '', location.pathname + location.search.replace(/([?&])tour=1&?/, '$1').replace(/[?&]$/, '') + location.hash); } catch (e) { /* ignore */ }
      setTimeout(function () { open(name); }, 300);
    } else if (!seen(t) && (!t.needsInstall || isInstalledApp()) && t.auto(path)) {
      setTimeout(function () { if (!document.querySelector('.nm-overlay.is-open')) open(name); }, 900);
    }
  }

  // Buttons anywhere on the site: <button type="button" data-app-guide-open>
  document.addEventListener('click', function (e) {
    var t = e.target.closest ? e.target.closest('[data-app-guide-open]') : null;
    if (t) { e.preventDefault(); var want = t.getAttribute('data-app-guide-open'); open(TOURS[want] ? want : tourForPath(currentPath()), t); }
  });
  // The phone says the app has just been installed
  window.addEventListener('appinstalled', function () { setTimeout(function () { open(tourForPath(currentPath())); }, 500); });

  window.LaikiGuide = { open: function (name) { open(TOURS[name] ? name : tourForPath(currentPath())); }, close: function () { close(false); } };

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start); else start();
}());
