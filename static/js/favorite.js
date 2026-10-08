(function () {
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('.fav-btn');
        if (!btn) return;
        e.preventDefault();
        if (btn.disabled) return;
        btn.disabled = true;

        fetch('/favorites/toggle', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'id=' + encodeURIComponent(btn.dataset.id)
        })
            .then(function (r) {
                if (r.status === 401) { location.href = '/login'; throw new Error('login'); }
                if (!r.ok) throw new Error('failed');
                return r.json();
            })
            .then(function (d) {
                btn.classList.toggle('on', d.favorited);
                btn.setAttribute('aria-pressed', d.favorited ? 'true' : 'false');

                if (!d.favorited && btn.dataset.remove) {
                    var card = btn.closest('.food-card');
                    if (card) card.remove();
                    if (!document.querySelector('.food-card')) location.reload();
                }
            })
            .catch(function () {})
            .then(function () { btn.disabled = false; });
    });
})();