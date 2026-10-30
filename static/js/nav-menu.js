/* Site-wide navigation: a hamburger drawer on phones/tablets and flyout menus on desktop.
   It enhances the nav each page already has (so links, active state and badges keep working),
   and adds a top bar on pages that have none (login, sign-up, order confirmation, errors). */
(function () {
    'use strict';
    if (window.__nmLoaded) return;
    window.__nmLoaded = true;

    var MOBILE = window.matchMedia('(max-width: 900px)');
    var HOME_MOBILE = window.matchMedia('(max-width: 820px)');
    var HOVER = window.matchMedia('(hover: hover) and (pointer: fine)');
    var LOGO = '/static/img/enie-mark.svg';

    var ICONS = {
        '/': '🏠', '/menu': '🍽️', '/vendors': '🏪', '/favorites': '❤️', '/orders': '📦', '/messages': '💬', '/account': '👤',
        '/loyalty': '✨', '/cart': '🛒', '/help': '🆘', '/login': '🔑', '/signup': '📝', '/about': 'ℹ️', '/contact': '✉️',
        '/become-a-vendor': '🤝', '/notifications': '🔔', '/logout': '🚪',
        '/admin': '📊', '/admin/orders': '📦', '/admin/applications': '📝', '/admin/vendors': '🏪', '/admin/menu': '🍽️',
        '/admin/riders': '🛵', '/admin/support': '🆘', '/admin/users': '👥', '/admin/earnings': '💰', '/admin/settlements': '🏦',
        '/vendor/dashboard': '📊', '/vendor/orders': '📦', '/vendor/menu': '🍽️', '/vendor/delivery': '🛵', '/vendor/coupons': '🏷️',
        '/vendor/analytics': '📈', '/vendor/statements': '🧾', '/vendor/ai': '✍️', '/vendor/profile': '🏪', '/vendor/billing': '💳',
        '/terms': '📄', '/privacy': '🔒', '/guides': '📚'
    };

    // Which links collapse into a flyout, per kind of menu. Links not listed stay in the bar.
    var GROUPS = {
        customer: [
            { label: 'My account', icon: '👤', hrefs: ['/account', '/orders', '/favorites', '/loyalty', '/logout'] }
        ],
        vendor: [
            { label: 'Grow', icon: '📈', hrefs: ['/vendor/coupons', '/vendor/analytics', '/vendor/ai'] },
            { label: 'Shop', icon: '🏪', hrefs: ['/vendor/delivery', '/vendor/profile'] },
            { label: 'Money', icon: '💰', hrefs: ['/vendor/statements', '/vendor/billing'] },
            { label: 'More', icon: '⋯', hrefs: ['/help', '/', '/logout'] }
        ],
        admin: [
            { label: 'Marketplace', icon: '🏪', hrefs: ['/admin/applications', '/admin/vendors', '/admin/menu', '/admin/riders'] },
            { label: 'People', icon: '👥', hrefs: ['/admin/users', '/admin/support'] },
            { label: 'Money', icon: '💰', hrefs: ['/admin/earnings', '/admin/settlements'] },
            { label: 'More', icon: '⋯', hrefs: ['/', '/logout'] }
        ],
        public: [
            { label: 'More', icon: '⋯', hrefs: ['/about', '/contact', '/help', '/become-a-vendor'] }
        ]
    };
    var SUBTITLE = { customer: 'Browse & order', vendor: 'Vendor tools', admin: 'Admin console', public: 'Explore' };

    // Pages with no nav of their own get a generated one (set <body data-nm="public|customer">).
    var GENERATED = {
        public: [['/menu', 'Menu'], ['/vendors', 'Shops'], ['/become-a-vendor', 'Sell with us'], ['/about', 'About'], ['/contact', 'Contact'],
                 ['/help', 'Help & Support'], ['/login', 'Log in'], ['/signup', 'Sign up']],
        customer: [['/menu', 'Menu'], ['/vendors', 'Shops'], ['/orders', 'My orders'], ['/cart', 'Cart'], ['/account', 'Account'],
                   ['/help', 'Help & Support'], ['/logout', 'Log out']]
    };

    function norm(href) {
        try { var u = new URL(href, location.href); return u.origin === location.origin ? (u.pathname.replace(/\/+$/, '') || '/') : null; }
        catch (e) { return null; }
    }
    function esc(s) { return String(s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
    function labelOf(a) {
        var c = a.cloneNode(true);
        [].forEach.call(c.querySelectorAll('[hidden],.notification-unread-badge,.messages-unread-badge,.nm-caret,.cart-count,.cart-badge'), function (n) { n.remove(); });
        var t = (c.textContent || '').replace(/\s+/g, ' ').trim().replace(/^[^\p{L}\p{N}]+/u, '').replace(/^[\p{Extended_Pictographic}\uFE0F\u200d\s]+/u, '');
        return t || (a.getAttribute('aria-label') || a.title || '').trim();
    }
    function badgeOf(a) {
        var b = a.querySelector('.notification-unread-badge:not([hidden]),.messages-unread-badge:not([hidden]),.cart-count:not([hidden]),.cart-badge:not([hidden])');
        var n = b ? b.textContent.trim() : '';
        return n && n !== '0' ? n : '';
    }
    function kindOf(nav) {
        var hrefs = [].map.call(nav.querySelectorAll('a[href]'), function (a) { return norm(a.getAttribute('href')); });
        var has = function (p) { return hrefs.some(function (h) { return h === p; }); };
        if (hrefs.some(function (h) { return h && h.indexOf('/admin') === 0; })) return 'admin';
        if (hrefs.some(function (h) { return h && h.indexOf('/vendor/') === 0; })) return 'vendor';
        if (has('/orders') && has('/account')) return 'customer';
        return 'public';
    }
    function markCurrent(nav) {
        if (nav.querySelector('[aria-current="page"]')) return;
        var here = location.pathname.replace(/\/+$/, '') || '/';
        [].some.call(nav.querySelectorAll('a[href]'), function (a) {
            if (norm(a.getAttribute('href')) === here && here !== '/' && !a.target) { a.setAttribute('aria-current', 'page'); return true; }
            return false;
        });
    }

    /* ---------- Flyouts ---------- */
    var openGroup = null;
    function closeGroup(g, restoreFocus) {
        if (!g) return;
        g.classList.remove('is-open');
        var t = g.querySelector('.nm-trigger');
        if (t) { t.setAttribute('aria-expanded', 'false'); if (restoreFocus) t.focus(); }
        if (openGroup === g) openGroup = null;
    }
    function openIt(g) {
        if (openGroup && openGroup !== g) closeGroup(openGroup);
        g.classList.add('is-open');
        g.querySelector('.nm-trigger').setAttribute('aria-expanded', 'true');
        var fly = g.querySelector('.nm-flyout');
        fly.classList.remove('nm-flip');
        if (fly.getBoundingClientRect().right > window.innerWidth - 8) fly.classList.add('nm-flip');
        openGroup = g;
    }
    function buildGroup(def, links) {
        var g = document.createElement('div'); g.className = 'nm-group';
        var t = document.createElement('a');
        t.href = '#'; t.className = 'nm-trigger'; t.setAttribute('role', 'button');
        t.setAttribute('aria-haspopup', 'true'); t.setAttribute('aria-expanded', 'false');
        t.innerHTML = esc(def.label) + '<span class="nm-caret" aria-hidden="true"></span>';
        var fly = document.createElement('div'); fly.className = 'nm-flyout';
        var panel = document.createElement('div'); panel.className = 'nm-panel'; panel.setAttribute('role', 'menu');
        links.forEach(function (a) { a.setAttribute('role', 'menuitem'); panel.appendChild(a); });
        fly.appendChild(panel); g.appendChild(t); g.appendChild(fly);
        if (links.some(function (a) { return a.getAttribute('aria-current') === 'page'; })) t.classList.add('nm-has-current');

        t.addEventListener('click', function (e) {
            e.preventDefault();
            // a mouse user hovered it open: the click should pin it open, not shut it again
            if (g.classList.contains('is-open') && g.dataset.hover === '1') { g.dataset.hover = ''; return; }
            g.classList.contains('is-open') ? closeGroup(g) : openIt(g);
        });
        t.addEventListener('keydown', function (e) {
            if (e.key === ' ') { e.preventDefault(); g.classList.contains('is-open') ? closeGroup(g) : openIt(g); }
            if (e.key === 'ArrowDown') { e.preventDefault(); openIt(g); var f = panel.querySelector('a'); if (f) f.focus(); }
        });
        panel.addEventListener('keydown', function (e) {
            var items = [].slice.call(panel.querySelectorAll('a')), i = items.indexOf(document.activeElement);
            if (e.key === 'Escape') { e.preventDefault(); closeGroup(g, true); }
            else if (e.key === 'ArrowDown') { e.preventDefault(); items[(i + 1) % items.length].focus(); }
            else if (e.key === 'ArrowUp') { e.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
            else if (e.key === 'Home') { e.preventDefault(); items[0].focus(); }
            else if (e.key === 'End') { e.preventDefault(); items[items.length - 1].focus(); }
        });
        var timer;
        g.addEventListener('mouseenter', function () { if (!HOVER.matches || MOBILE.matches) return; clearTimeout(timer); if (!g.classList.contains('is-open')) { openIt(g); g.dataset.hover = '1'; } });
        g.addEventListener('mouseleave', function () { if (!HOVER.matches) return; clearTimeout(timer); timer = setTimeout(function () { g.dataset.hover = ''; closeGroup(g); }, 160); });
        g.addEventListener('focusout', function (e) { if (!g.contains(e.relatedTarget)) closeGroup(g); });
        return g;
    }
    function groupNav(nav, kind) {
        var defs = GROUPS[kind] || [];
        defs.forEach(function (def) {
            var members = [].filter.call(nav.children, function (el) {
                return el.tagName === 'A' && def.hrefs.indexOf(norm(el.getAttribute('href'))) !== -1;
            });
            if (members.length < 2) return;
            var ph = document.createComment('nm');
            nav.insertBefore(ph, members[0]);
            var g = buildGroup(def, members.slice());
            g.dataset.icon = def.icon;
            nav.replaceChild(g, ph);
        });
    }

    /* ---------- Drawer ---------- */
    var overlay, drawer, burger, lastFocus, ctx;

    function collect(nav) {
        var out = { items: [], foot: [] };
        function add(a) {
            var p = norm(a.getAttribute('href')), label = labelOf(a);
            if (!label) return null;
            return { href: a.getAttribute('href'), path: p, label: label, badge: badgeOf(a), target: a.target, rel: a.rel,
                     current: a.getAttribute('aria-current') === 'page' };
        }
        [].forEach.call(nav.children, function (el) {
            if (el.classList.contains('nm-group')) {
                var links = [].map.call(el.querySelectorAll('.nm-panel a'), add).filter(Boolean);
                links = links.filter(function (l) { if (l.path === '/logout') { out.foot.push(l); return false; } return true; });
                var lab = el.querySelector('.nm-trigger').textContent.trim();
                if (links.length) out.items.push({ group: lab, icon: el.dataset.icon || '', links: links });
            } else if (el.tagName === 'A') {
                var it = add(el); if (!it) return;
                if (it.path === '/login' || it.path === '/signup' || it.path === '/logout') out.foot.push(it); else out.items.push(it);
            }
        });
        return out;
    }
    function linkHtml(it, withIcon) {
        var ico = withIcon ? '<span class="nm-ico" aria-hidden="true">' + (ICONS[it.path] || '•') + '</span>' : '';
        return '<a href="' + esc(it.href) + '"' + (it.target ? ' target="' + esc(it.target) + '"' : '') + (it.rel ? ' rel="' + esc(it.rel) + '"' : '') +
            (it.current ? ' aria-current="page"' : '') + '>' + ico + '<span>' + esc(it.label) + '</span>' +
            (it.badge ? '<span class="nm-badge">' + esc(it.badge) + '</span>' : '') + '</a>';
    }
    function render() {
        var data = collect(ctx.nav);
        if (ctx.extra) [].forEach.call(ctx.extra.querySelectorAll('a'), function (a) {
            var p = norm(a.getAttribute('href')), l = labelOf(a);
            if (l) data.foot.push({ href: a.getAttribute('href'), path: p, label: l, badge: '', target: '', rel: '', current: false });
        });
        var html = '';
        data.items.forEach(function (it) {
            if (it.group) {
                var open = it.links.some(function (l) { return l.current; });
                html += '<details class="nm-acc"' + (open ? ' open' : '') + '><summary><span class="nm-ico" aria-hidden="true">' + esc(it.icon) + '</span><span>' + esc(it.group) +
                    '</span><span class="nm-caret" aria-hidden="true"></span></summary><div class="nm-acc-body">' + it.links.map(function (l) { return linkHtml(l, true); }).join('') + '</div></details>';
            } else html += linkHtml(it, true);
        });
        drawer.querySelector('.nm-drawer-nav').innerHTML = html;
        var foot = '';
        data.foot.forEach(function (it) {
            var cls = it.path === '/signup' ? 'nm-primary' : it.path === '/logout' ? 'nm-quiet' : '';
            foot += linkHtml(it, false).replace('<a ', '<a ' + (cls ? 'class="' + cls + '" ' : ''));
        });
        var f = drawer.querySelector('.nm-drawer-foot');
        f.innerHTML = foot; f.style.display = foot ? '' : 'none';
    }
    function buildOverlay() {
        overlay = document.createElement('div'); overlay.className = 'nm-overlay';
        overlay.innerHTML = '<aside class="nm-drawer" id="nm-drawer" role="dialog" aria-modal="true" aria-label="Site menu">' +
            '<div class="nm-drawer-head"><a class="nm-drawer-brand" href="' + esc(ctx.home) + '"><img src="' + LOGO + '" alt=""><span>' + esc(ctx.brand) + '</span></a>' +
            '<button type="button" class="nm-close" aria-label="Close menu">×</button></div>' +
            '<div class="nm-drawer-sub">' + esc(SUBTITLE[ctx.kind] || 'Menu') + '</div>' +
            '<nav class="nm-drawer-nav" aria-label="Site menu"></nav><div class="nm-drawer-foot"></div></aside>';
        document.body.appendChild(overlay);
        drawer = overlay.querySelector('.nm-drawer');
        overlay.addEventListener('click', function (e) { if (e.target === overlay) closeDrawer(); });
        overlay.querySelector('.nm-close').addEventListener('click', function () { closeDrawer(true); });
        drawer.addEventListener('click', function (e) { var a = e.target.closest && e.target.closest('a[href]'); if (a && a.target !== '_blank') closeDrawer(); });
        drawer.addEventListener('keydown', function (e) {
            if (e.key !== 'Tab') return;
            var f = [].filter.call(drawer.querySelectorAll('a[href],button,summary'), function (n) { return n.offsetParent !== null; });
            if (!f.length) return;
            var first = f[0], last = f[f.length - 1];
            if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
            else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
        });
        var sx = null;
        drawer.addEventListener('touchstart', function (e) { sx = e.touches[0].clientX; }, { passive: true });
        drawer.addEventListener('touchend', function (e) { if (sx !== null && sx - e.changedTouches[0].clientX > 70) closeDrawer(true); sx = null; }, { passive: true });
    }
    function openDrawer() {
        if (!overlay) buildOverlay();
        render();
        lastFocus = document.activeElement;
        overlay.classList.add('is-open'); document.body.classList.add('nm-lock');
        burger.setAttribute('aria-expanded', 'true'); burger.setAttribute('aria-label', 'Close menu');
        var cur = drawer.querySelector('a[aria-current="page"]') || drawer.querySelector('.nm-close');
        setTimeout(function () { (drawer.querySelector('.nm-close')).focus(); }, 30);
        void cur;
    }
    function closeDrawer(restore) {
        if (!overlay || !overlay.classList.contains('is-open')) return;
        overlay.classList.remove('is-open'); document.body.classList.remove('nm-lock');
        burger.setAttribute('aria-expanded', 'false'); burger.setAttribute('aria-label', 'Open menu');
        if (restore !== false && lastFocus && lastFocus.focus) lastFocus.focus();
    }

    /* ---------- Setup ---------- */
    function makeBurger() {
        var b = document.createElement('button');
        b.type = 'button'; b.className = 'nm-burger';
        b.setAttribute('aria-label', 'Open menu'); b.setAttribute('aria-expanded', 'false'); b.setAttribute('aria-controls', 'nm-drawer'); b.setAttribute('aria-haspopup', 'dialog');
        b.innerHTML = '<span class="nm-bars" aria-hidden="true"><i></i><i></i><i></i></span><span class="nm-burger-label">Menu</span>';
        b.addEventListener('click', function () { overlay && overlay.classList.contains('is-open') ? closeDrawer(true) : openDrawer(); });
        return b;
    }
    function brandInfo(scope, kind) {
        var el = (scope && scope.querySelector('.brand-name,.vendor-brand a,.admin-brand a,.logo,a.brand,.brand a,.home-sidebar-brand a')) ||
                 document.querySelector('header .brand-name,header a.brand,header .logo,header .vendor-brand a,.home-sidebar-brand a');
        var name = kind === 'vendor' ? 'Vendor dashboard' : kind === 'admin' ? 'Admin' : 'Menu', href = kind === 'vendor' ? '/vendor/dashboard' : kind === 'admin' ? '/admin' : '/';
        if (el) {
            var c = el.cloneNode(true);
            [].forEach.call(c.querySelectorAll('small,img,svg'), function (n) { n.remove(); });
            var t = (c.textContent || '').replace(/\s+/g, ' ').trim().replace(/^[^\p{L}\p{N}]+/u, '');
            if (t) name = t;
            var h = el.tagName === 'A' ? el : el.closest('a');
            if (h && h.getAttribute('href')) href = h.getAttribute('href');
        } else if (document.querySelector('meta[property="og:site_name"]')) name = document.querySelector('meta[property="og:site_name"]').content;
        return { name: name, href: href };
    }

    function init() {
        var body = document.body;
        var home = document.querySelector('.home-sidebar');
        var nav = (home && home.querySelector('.home-sidebar-nav')) || document.querySelector('header nav') ||
                  document.querySelector('body > nav, body > .nav, body > .portal-nav');
        var host, kind, extra = null, bar = null;

        if (!nav) {
            var mode = body.getAttribute('data-nm');
            if (!mode || !GENERATED[mode]) return;
            kind = mode;
            bar = document.createElement('header'); bar.className = 'nm-topbar';
            var b0 = brandInfo(null, kind);
            bar.innerHTML = '<a class="nm-topbar-brand" href="/"><img src="' + LOGO + '" alt=""><span>' + esc(b0.name === 'Menu' ? (document.title.split(/[|·–-]/).pop() || 'Home').trim() : b0.name) + '</span></a>';
            nav = document.createElement('nav'); nav.className = 'nm-topnav'; nav.setAttribute('aria-label', 'Main navigation');
            GENERATED[mode].forEach(function (p) {
                var a = document.createElement('a'); a.href = p[0]; a.textContent = p[1];
                if (p[0] === '/signup') a.className = 'nm-cta';
                nav.appendChild(a);
            });
            bar.appendChild(nav); body.insertBefore(bar, body.firstChild);
            host = bar;
        } else {
            kind = home ? 'public' : kindOf(nav);
            host = home ? home : nav.parentElement;
            if (!home && host === body) {            // nav sits directly in <body>: add a phone-only top bar
                bar = document.createElement('header'); bar.className = 'nm-topbar nm-mobile-only';
                var b1 = brandInfo(null, kind);
                bar.innerHTML = '<a class="nm-topbar-brand" href="' + esc(b1.href) + '"><img src="' + LOGO + '" alt=""><span>' + esc(b1.name) + '</span></a>';
                body.insertBefore(bar, nav); host = bar;
            }
            if (home) { extra = home.querySelector('.home-sidebar-account'); body.classList.add('nm-home'); }
        }

        markCurrent(nav);
        if (!home) groupNav(nav, kind);

        nav.classList.add('nm-src');
        if (extra) extra.classList.add('nm-src');
        var b = brandInfo(host, kind);
        ctx = { nav: nav, extra: extra, kind: kind, brand: b.name, home: b.href };

        burger = makeBurger();
        if (home) host.appendChild(burger);
        else if (bar) { bar.appendChild(burger); }
        else host.insertBefore(burger, nav);
        // generated top bar: desktop keeps its inline nav, so it must not be hidden by .nm-src
        if (nav.classList.contains('nm-topnav')) nav.classList.remove('nm-src');

        body.classList.add('nm-ready');

        document.addEventListener('click', function (e) { if (openGroup && !openGroup.contains(e.target)) closeGroup(openGroup); });
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape') {
                if (overlay && overlay.classList.contains('is-open')) { closeDrawer(true); }
                else if (openGroup) closeGroup(openGroup, true);
            }
        });
        function onResize() { var m = home ? HOME_MOBILE.matches : MOBILE.matches; if (!m) closeDrawer(false); if (m && openGroup) closeGroup(openGroup); }
        (MOBILE.addEventListener ? MOBILE.addEventListener.bind(MOBILE, 'change') : MOBILE.addListener.bind(MOBILE))(onResize);
        window.addEventListener('pageshow', function () { closeDrawer(false); document.body.classList.remove('nm-lock'); });
    }

    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
})();
