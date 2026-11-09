document.querySelectorAll('.pay-form').forEach(function (f) {
    f.addEventListener('submit', function () {
        var b = f.querySelector('button');
        b.disabled = true;
        b.textContent = 'Taking you to Paystack...';
    });
});
window.addEventListener('pageshow', function () {
    document.querySelectorAll('.pay-form button').forEach(function (b) { b.disabled = false; });
});
