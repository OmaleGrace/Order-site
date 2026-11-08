(() => {
    // 1. Live orders: check quietly for changes and reload only when one happened. The old page
    //    reloaded every 15 seconds, which threw the reader back to the top mid-scroll and closed
    //    any open "Order items" panel.
    const signature = (root) => Array.from(root.querySelectorAll('.order-card'))
        .map((c) => (c.dataset.orderId || '') + ':' + (c.dataset.status || '')).join('|');
    if (document.querySelector('.order-live')) {
        const before = signature(document);
        const check = async () => {
            if (document.visibilityState !== 'visible') return;
            try {
                const r = await fetch(window.location.href, {cache: 'no-store', headers: {'Accept': 'text/html'}});
                if (!r.ok) return;
                const doc = new DOMParser().parseFromString(await r.text(), 'text/html');
                if (signature(doc) !== before) window.location.reload();
            } catch (_) { /* offline: try again next time */ }
        };
        setInterval(check, 15000);
    }

    // 2. The rider map is an embedded map that grabs finger swipes, so a swipe that started over it
    //    moved the map instead of scrolling the page. It now ignores touches until it is tapped.
    document.querySelectorAll('.rider-map-wrap').forEach((wrap) => {
        const lock = () => wrap.classList.remove('is-active');
        wrap.querySelector('.map-unlock').addEventListener('click', () => wrap.classList.add('is-active'));
        document.addEventListener('touchstart', (e) => { if (!wrap.contains(e.target)) lock(); }, {passive: true});
        document.addEventListener('scroll', lock, {passive: true});
    });
})();
