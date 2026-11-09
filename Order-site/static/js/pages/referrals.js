(function () {
  var input = document.getElementById('ref-link');
  var copy = document.getElementById('ref-copy');
  var status = document.getElementById('ref-status');
  if (!input || !copy) return;
  copy.addEventListener('click', async function () {
    try { await navigator.clipboard.writeText(input.value); status.textContent = ' Copied!'; }
    catch (_) { input.select(); status.textContent = ' Select the link and copy it.'; }
  });
})();
