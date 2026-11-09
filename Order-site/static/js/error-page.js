(function () {
    'use strict';
    // "Go back" only makes sense when there is somewhere to go back to.
    var back = document.querySelector('[data-go-back]');
    if (back && window.history.length > 1) {
        back.hidden = false;
        back.addEventListener('click', function () { window.history.back(); });
    }
    var reload = document.querySelector('[data-reload]');
    if (reload) {
        reload.addEventListener('click', function () { window.location.reload(); });
    }
})();
