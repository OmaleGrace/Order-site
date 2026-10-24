(function () {
    if (location.pathname.indexOf('/vendor/billing') === 0) return;

    fetch('/vendor/billing/status', { credentials: 'same-origin', headers: { 'Accept': 'application/json' } })
        .then(function (r) {
            if (!r.ok) throw new Error('not a vendor');
            return r.json();
        })
        .then(function (d) {
            var text = '';
            var action = 'Open billing';

            if (d.state === 'setup') {
                text = 'Your shop is not open to customers yet. Pay your setup fee to open it.';
                action = 'Pay now';
            } else if (d.state === 'expired') {
                text = 'Your shop is closed because your paid period ended. Customers cannot see your dishes.';
                action = 'Renew now';
            } else if (d.state === 'soon') {
                text = d.days_left <= 1
                    ? 'Your shop closes within a day unless you renew.'
                    : 'Your shop closes in ' + d.days_left + ' days unless you renew.';
                action = 'Renew now';
            } else {
                return;
            }

            var bar = document.createElement('div');
            bar.className = 'vendor-banner' + (d.state === 'soon' ? ' soon' : '');
            bar.setAttribute('role', 'status');

            var span = document.createElement('span');
            span.textContent = text;

            var link = document.createElement('a');
            link.href = '/vendor/billing';
            link.textContent = action;

            bar.appendChild(span);
            bar.appendChild(link);

            var main = document.querySelector('main');
            if (main) main.insertBefore(bar, main.firstChild);
        })
        .catch(function () {});
})();