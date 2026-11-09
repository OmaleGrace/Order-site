(function () {
    'use strict';
    // Styled replacement for the browser's confirm() popup.
    //   <form method="post" data-confirm="Delete this dish?" data-confirm-ok="Delete" data-confirm-tone="danger">
    // Works on a form, or on a submit button inside a form (the button's attributes win).
    if (typeof HTMLDialogElement === 'undefined') {
        // Very old browsers: fall back to the native popup.
        document.addEventListener('submit', function (e) {
            var src = e.submitter && e.submitter.hasAttribute('data-confirm') ? e.submitter : e.target;
            var q = src.getAttribute && src.getAttribute('data-confirm');
            if (q && !window.confirm(q)) e.preventDefault();
        }, true);
        return;
    }

    var dialog = null, okBtn, cancelBtn, textEl, titleEl, pending = null;

    function build() {
        dialog = document.createElement('dialog');
        dialog.className = 'confirm-dialog';
        dialog.setAttribute('aria-labelledby', 'confirm-dialog-title');
        dialog.innerHTML =
            '<form method="dialog" class="confirm-dialog-box">' +
            '<h2 id="confirm-dialog-title" class="confirm-dialog-title">Are you sure?</h2>' +
            '<p class="confirm-dialog-text"></p>' +
            '<div class="confirm-dialog-actions">' +
            '<button type="button" class="confirm-dialog-cancel" value="cancel">Cancel</button>' +
            '<button type="button" class="confirm-dialog-ok" value="ok">Confirm</button>' +
            '</div></form>';
        document.body.appendChild(dialog);
        titleEl = dialog.querySelector('.confirm-dialog-title');
        textEl = dialog.querySelector('.confirm-dialog-text');
        okBtn = dialog.querySelector('.confirm-dialog-ok');
        cancelBtn = dialog.querySelector('.confirm-dialog-cancel');

        cancelBtn.addEventListener('click', function () { dialog.close('cancel'); });
        okBtn.addEventListener('click', function () { dialog.close('ok'); });
        // Clicking the dark backdrop cancels.
        dialog.addEventListener('click', function (e) { if (e.target === dialog) dialog.close('cancel'); });
        dialog.addEventListener('close', function () {
            var p = pending; pending = null;
            if (!p) return;
            if (dialog.returnValue === 'ok') {
                p.form.setAttribute('data-confirmed', '1');
                if (p.form.requestSubmit) p.form.requestSubmit(p.submitter || undefined);
                else p.form.submit();
            } else if (p.opener && p.opener.focus) {
                p.opener.focus();
            }
        });
    }

    document.addEventListener('submit', function (e) {
        var form = e.target;
        if (!(form instanceof HTMLFormElement)) return;
        if (form.getAttribute('data-confirmed') === '1') {
            form.removeAttribute('data-confirmed');
            return;
        }
        var src = e.submitter && e.submitter.hasAttribute('data-confirm') ? e.submitter : form;
        var question = src.getAttribute('data-confirm');
        if (!question) return;

        e.preventDefault();
        if (!dialog) build();
        pending = { form: form, submitter: e.submitter, opener: document.activeElement };
        titleEl.textContent = src.getAttribute('data-confirm-title') || 'Please confirm';
        textEl.textContent = question;
        okBtn.textContent = src.getAttribute('data-confirm-ok') || 'Confirm';
        okBtn.classList.toggle('danger', src.getAttribute('data-confirm-tone') === 'danger');
        dialog.showModal();
        cancelBtn.focus(); // the safe choice is focused first
    }, true);
})();
