(function () {
    'use strict';
    // On small screens the top nav is a swipeable row. Scroll it so the current page's
    // link is in view instead of hiding off to the right.
    var current = document.querySelector('header nav a[aria-current="page"]');
    if (!current) return;
    var nav = current.closest('nav');
    if (!nav || nav.scrollWidth <= nav.clientWidth) return;
    var offset = current.getBoundingClientRect().left - nav.getBoundingClientRect().left;
    nav.scrollLeft += offset - (nav.clientWidth - current.offsetWidth) / 2;
})();
