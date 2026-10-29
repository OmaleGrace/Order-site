(function () {
    'use strict';
    // Stops double-clicks from submitting a form twice (two orders, two payments...).
    // After the first submit the button is disabled and shows "Please wait…".
    // Opt out of a form with data-allow-resubmit.
    function lock(form, submitter) {
        var buttons = form.querySelectorAll('button[type="submit"], button:not([type]), input[type="submit"]');
        buttons.forEach(function (b) {
            if (b.disabled) return;
            b.setAttribute('data-guarded', '1');
            // Disable on the next tick: a disabled submitter would not be sent with the form.
            window.setTimeout(function () {
                b.disabled = true;
                b.setAttribute('aria-busy', 'true');
            }, 0);
            if (b === submitter && b.tagName === 'BUTTON') {
                b.setAttribute('data-original-label', b.innerHTML);
                b.textContent = 'Please wait…';
            }
        });
        form.setAttribute('data-submitting', '1');
    }

    function unlock(form) {
        form.removeAttribute('data-submitting');
        form.querySelectorAll('[data-guarded]').forEach(function (b) {
            b.disabled = false;
            b.removeAttribute('aria-busy');
            b.removeAttribute('data-guarded');
            var label = b.getAttribute('data-original-label');
            if (label !== null) {
                b.innerHTML = label;
                b.removeAttribute('data-original-label');
            }
        });
    }

    document.addEventListener('submit', function (e) {
        var form = e.target;
        if (!(form instanceof HTMLFormElement) || form.hasAttribute('data-allow-resubmit')) return;
        if (e.defaultPrevented) return; // validation failed or another script took over
        if (form.getAttribute('data-submitting') === '1') {
            e.preventDefault();
            return;
        }
        // Forms that open a new tab or download a file never leave this page, so don't lock them.
        if (form.target && form.target !== '_self') return;
        lock(form, e.submitter);
        // Safety net: if the request never navigates away (network drop), let the person retry.
        window.setTimeout(function () { unlock(form); }, 20000);
    });

    // Coming back with the browser's Back button should not leave buttons stuck.
    window.addEventListener('pageshow', function () {
        document.querySelectorAll('form[data-submitting]').forEach(unlock);
    });
})();
