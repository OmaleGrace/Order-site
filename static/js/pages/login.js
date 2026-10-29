document.querySelectorAll('.toggle-password').forEach(function (btn) {
    btn.addEventListener('click', function () {
        var input = document.getElementById(btn.dataset.target);
        var isHidden = input.type === 'password';
        input.type = isHidden ? 'text' : 'password';
        btn.querySelector('.eye-open').style.display = isHidden ? 'none' : 'block';
        btn.querySelector('.eye-closed').style.display = isHidden ? 'block' : 'none';
        btn.setAttribute('aria-label', isHidden ? 'Hide password' : 'Show password');
    });
});

(function () {
    var emailEl = document.getElementById('email');
    var passEl = document.getElementById('password');
    var emailMsg = document.getElementById('email-error');
    var submitBtn = document.querySelector('form button[type="submit"]');
    var capsLock = document.getElementById('caps-lock');
    passEl.addEventListener('keydown', function(e){
        if (typeof e.getModifierState === 'function') capsLock.classList.toggle('visible', e.getModifierState('CapsLock'));
    });
    passEl.addEventListener('keyup', function(e){
        if (typeof e.getModifierState === 'function') capsLock.classList.toggle('visible', e.getModifierState('CapsLock'));
    });

    function emailOk(v) {
        return /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(v.trim());
    }

    function showEmail() {
        var bad = emailEl.value !== '' && !emailOk(emailEl.value);
        emailMsg.textContent = bad ? 'Enter a valid email address' : '';
        emailEl.classList.toggle('invalid', bad);
    }

    function refresh() {
        submitBtn.disabled = !(emailOk(emailEl.value) && passEl.value.length > 0);
    }

    emailEl.addEventListener('input', function () { showEmail(); refresh(); });
    emailEl.addEventListener('blur', showEmail);
    passEl.addEventListener('input', refresh);
    emailEl.addEventListener('change', refresh);
    passEl.addEventListener('change', refresh);

    // Browsers can autofill saved logins without firing input events
    refresh();
    setTimeout(refresh, 300);
})();
