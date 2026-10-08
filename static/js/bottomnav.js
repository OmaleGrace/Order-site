(function () {
    var path = location.pathname;

    var tabs = [
        { href: '/', label: 'Home', icon: '🏠', active: path === '/' },
        { href: '/menu', label: 'Menu', icon: '🍽️', active: path.indexOf('/menu') === 0 },
        { href: '/cart', label: 'Cart', icon: '🛒', active: path.indexOf('/cart') === 0, badge: true },
        { href: '/orders', label: 'Orders', icon: '📦', active: path.indexOf('/orders') === 0 },
        { href: '/account', label: 'Account', icon: '👤', active: path.indexOf('/account') === 0 }
    ];

    var nav = document.createElement('nav');
    nav.className = 'bottom-nav';
    nav.setAttribute('aria-label', 'Main');
    nav.innerHTML = tabs.map(function (t) {
        return '<a href="' + t.href + '"' + (t.active ? ' class="active"' : '') + '>' +
            '<span class="bn-icon">' + t.icon +
            (t.badge ? '<span class="bn-badge" hidden>0</span>' : '') +
            '</span><span>' + t.label + '</span></a>';
    }).join('');

    document.body.appendChild(nav);
    document.body.classList.add('has-bottom-nav');

    fetch('/cart/count', { credentials: 'same-origin' })
        .then(function (r) { return r.json(); })
        .then(function (d) {
            var b = nav.querySelector('.bn-badge');
            if (b && d.count > 0) {
                b.textContent = d.count > 99 ? '99+' : d.count;
                b.hidden = false;
            }
        })
        .catch(function () {});
})();