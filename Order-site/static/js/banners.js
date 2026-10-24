(function () {
    var track = document.getElementById('banner-track');
    var dotsBox = document.getElementById('banner-dots');
    if (!track || !dotsBox) return;

    var slides = track.children;
    var dots = [];
    var current = 0;
    var timer = null;

    for (var i = 0; i < slides.length; i++) {
        (function (index) {
            var dot = document.createElement('button');
            dot.type = 'button';
            dot.setAttribute('aria-label', 'Show banner ' + (index + 1));
            dot.addEventListener('click', function () { go(index); restart(); });
            dotsBox.appendChild(dot);
            dots.push(dot);
        })(i);
    }

    function setActive(index) {
        current = index;
        dots.forEach(function (d, i) { d.classList.toggle('active', i === index); });
    }

    function go(index) {
        track.scrollTo({ left: slides[index].offsetLeft - track.offsetLeft, behavior: 'smooth' });
        setActive(index);
    }

    track.addEventListener('scroll', function () {
        var w = slides[0].offsetWidth + 12;
        var index = Math.round(track.scrollLeft / w);
        if (index !== current && index >= 0 && index < slides.length) setActive(index);
    });

    function restart() {
        clearInterval(timer);
        timer = setInterval(function () { go((current + 1) % slides.length); }, 5000);
    }

    ['touchstart', 'mouseenter'].forEach(function (ev) {
        track.addEventListener(ev, function () { clearInterval(timer); });
    });
    ['touchend', 'mouseleave'].forEach(function (ev) {
        track.addEventListener(ev, restart);
    });

    setActive(0);
    restart();
})();