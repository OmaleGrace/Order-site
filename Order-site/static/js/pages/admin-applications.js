(function () {
    document.querySelectorAll('.review-form').forEach(function (form) {
        var approve = form.querySelector('button[value="approve"]');
        var reject = form.querySelector('button[value="reject"]');
        var checks = form.querySelectorAll('.check-item input');
        var method = form.querySelector('select[name="review_method"]');
        var note = form.querySelector('textarea[name="admin_note"]');
        var shop = form.getAttribute('data-name');

        function refresh() {
            var allTicked = Array.prototype.every.call(checks, function (c) { return c.checked; });
            approve.disabled = !(allTicked && method.value !== '');
            reject.disabled = [...note.value.trim()].length < 5;
        }

        Array.prototype.forEach.call(checks, function (c) { c.addEventListener('change', refresh); });
        method.addEventListener('change', refresh);
        note.addEventListener('input', refresh);

        // The styled confirmation dialog (static/js/confirm-dialog.js) reads these attributes.
        approve.setAttribute('data-confirm', 'Approve ' + shop + '? This creates their shop.');
        approve.setAttribute('data-confirm-title', 'Approve this application?');
        approve.setAttribute('data-confirm-ok', 'Approve');
        reject.setAttribute('data-confirm', 'Reject the application from ' + shop + '? They will see your note.');
        reject.setAttribute('data-confirm-title', 'Reject this application?');
        reject.setAttribute('data-confirm-ok', 'Reject');
        reject.setAttribute('data-confirm-tone', 'danger');

        refresh();
    });
})();
