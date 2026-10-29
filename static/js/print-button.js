(function () {
    'use strict';
    // Buttons marked data-print open the browser's print dialog (which can also save as PDF).
    document.querySelectorAll('[data-print]').forEach(function (button) {
        button.addEventListener('click', function () { window.print(); });
    });
})();
