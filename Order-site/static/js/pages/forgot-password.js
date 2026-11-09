(function () {
    var emailEl = document.getElementById('email');
    var msg = document.getElementById('email-error');
    var submitBtn = document.querySelector('form button[type="submit"]');

    function ok(v) {
        return /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(v.trim());
    }

    function show() {
        var bad = emailEl.value !== '' && !ok(emailEl.value);
        msg.textContent = bad ? 'Enter a valid email address' : '';
        emailEl.classList.toggle('invalid', bad);
    }

    function refresh() {
        submitBtn.disabled = !ok(emailEl.value);
    }

    emailEl.addEventListener('input', function () { show(); refresh(); });
    emailEl.addEventListener('blur', show);
    emailEl.addEventListener('change', refresh);
    refresh();
    setTimeout(refresh, 300);
})();
