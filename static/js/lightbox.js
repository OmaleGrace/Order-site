(function () {
    var box = document.createElement('div');
    box.className = 'lightbox';
    box.setAttribute('role', 'dialog');
    box.setAttribute('aria-modal', 'true');
    box.innerHTML =
        '<button type="button" class="lightbox-close" aria-label="Close">&times;</button>' +
        '<img alt="">' +
        '<div class="lightbox-caption"></div>';
    document.body.appendChild(box);

    var bigImg = box.querySelector('img');
    var caption = box.querySelector('.lightbox-caption');

    function open(img) {
        bigImg.src = img.currentSrc || img.src;
        bigImg.alt = img.alt || '';
        caption.textContent = img.alt || '';
        box.classList.add('open');
        document.body.style.overflow = 'hidden';
    }

    function close() {
        box.classList.remove('open');
        document.body.style.overflow = '';
    }

    document.addEventListener('click', function (e) {
        var img = e.target.closest('img.zoomable');
        if (img) { open(img); return; }
        if (e.target === box || e.target.closest('.lightbox-close')) close();
    });

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') close();
        if (e.key === 'Enter' && e.target.matches && e.target.matches('img.zoomable')) open(e.target);
    });

    document.querySelectorAll('img.zoomable').forEach(function (img) {
        img.tabIndex = 0;
    });
})();