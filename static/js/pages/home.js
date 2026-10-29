(function () {
    var tabs = document.querySelectorAll('.tab-btn');
    tabs.forEach(function (btn) {
        btn.addEventListener('click', function () {
            tabs.forEach(function (b) {
                var on = b === btn;
                b.classList.toggle('active', on);
                b.setAttribute('aria-selected', on ? 'true' : 'false');
                document.getElementById('tab-' + b.dataset.tab).hidden = !on;
            });
        });
    });
})();
