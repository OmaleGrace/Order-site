document.querySelectorAll('.toggle-password').forEach(function (btn) {
    btn.addEventListener('click', function () {
        var input = document.getElementById(btn.dataset.target);
        var isHidden = input.type === 'password';
        input.type = isHidden ? 'text' : 'password';
        btn.textContent = isHidden ? 'Hide' : 'Show';
    });
});

(function () {
    var passEl = document.getElementById('password');
    var confirmEl = document.getElementById('confirm');
    var passMsg = document.getElementById('password-error');
    var confirmMsg = document.getElementById('confirm-error');
    var submitBtn = document.querySelector('form button[type="submit"]');

    function checkPassword(v) {
        if ([...v].length < 8) return 'Password must be at least 8 characters';
        if (!/\p{Lu}/u.test(v)) return 'Add a capital letter';
        if (!/\p{Ll}/u.test(v)) return 'Add a lowercase letter';
        if (!/\p{Nd}/u.test(v)) return 'Add a number';
        if (!/[^\p{L}\p{N}\s]/u.test(v)) return 'Add a special character, like ! @ # $';
        return '';
    }

    function checkConfirm() {
        return confirmEl.value === passEl.value ? '' : 'Passwords do not match';
    }

    function update() {
        var pe = checkPassword(passEl.value);
        passMsg.textContent = passEl.value === '' ? '' : pe;
        passEl.classList.toggle('invalid', passEl.value !== '' && pe !== '');

        var ce = confirmEl.value === '' ? '' : checkConfirm();
        confirmMsg.textContent = ce;
        confirmEl.classList.toggle('invalid', ce !== '');

        submitBtn.disabled = !(checkPassword(passEl.value) === '' && confirmEl.value !== '' && checkConfirm() === '');
    }

    passEl.addEventListener('input', update);
    confirmEl.addEventListener('input', update);
    submitBtn.disabled = true;
})();
