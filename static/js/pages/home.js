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

// Let a mouse wheel move horizontal card rails too, while preserving normal page scrolling
// when the pointer is outside a rail or the rail has reached its end.
document.querySelectorAll('.shop-row, .cat-row, .h-row, .banner-track').forEach(function (rail) {
    rail.addEventListener('wheel', function (event) {
        if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
        var canMove = rail.scrollWidth > rail.clientWidth;
        var movingRight = event.deltaY > 0;
        var hasRoom = movingRight
            ? rail.scrollLeft + rail.clientWidth < rail.scrollWidth - 1
            : rail.scrollLeft > 0;
        if (canMove && hasRoom) {
            rail.scrollLeft += event.deltaY;
            event.preventDefault();
        }
    }, { passive: false });
});
