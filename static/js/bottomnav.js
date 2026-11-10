(function () {
    var path = location.pathname;

    var tabs = [
        { href: '/menu', label: 'Menu', icon: 'menu', active: path.indexOf('/menu') === 0 },
        { href: '/cart', label: 'Cart', icon: 'cart', active: path.indexOf('/cart') === 0, badge: true },
        { href: '/orders', label: 'Orders', icon: 'orders', active: path.indexOf('/orders') === 0 },
        { href: '/account', label: 'Account', icon: 'user', active: path.indexOf('/account') === 0 }
    ];

    var nav = document.createElement('nav');
    nav.className = 'bottom-nav';
    nav.setAttribute('aria-label', 'Main');
    nav.innerHTML = tabs.map(function (t) {
        return '<a href="' + t.href + '"' + (t.active ? ' class="active"' : '') + '>' +
            '<span class="bn-icon">' + (window.LaikiIcons ? window.LaikiIcons.svg(t.icon) : '') +
            (t.badge ? '<span class="bn-badge" hidden>0</span>' : '') +
            '</span><span>' + t.label + '</span></a>';
    }).join('');

    document.body.appendChild(nav);
    document.body.classList.add('has-bottom-nav');

    function loadCounts() {
        if (!window.__laikihubHeaderCountsPromise) {
            window.__laikihubHeaderCountsPromise = fetch('/header/counts', { credentials: 'same-origin' })
                .then(function (r) { if (!r.ok) throw new Error('header counts unavailable'); return r.json(); });
        }
        return window.__laikihubHeaderCountsPromise;
    }

    loadCounts().then(function (d) {
        var b = nav.querySelector('.bn-badge');
        if (b && Number(d.cart_count) > 0) {
            b.textContent = d.cart_count > 99 ? '99+' : d.cart_count;
            b.hidden = false;
        }
    }).catch(function () {});
})();
