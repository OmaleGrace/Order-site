(function () {
  var input = document.getElementById('topup-amount');
  document.querySelectorAll('.wl-chip').forEach(function (chip) {
    chip.addEventListener('click', function () { if (input) { input.value = chip.getAttribute('data-amount'); input.focus(); } });
  });
})();
