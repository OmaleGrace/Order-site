// Fills in the vendor's shareable shop link and wires up Copy and WhatsApp.
(function () {
  var input = document.getElementById('share-link');
  if (!input) return;
  var url = location.origin + input.getAttribute('data-path');
  input.value = url;
  var status = document.getElementById('share-status');
  var wa = document.getElementById('share-whatsapp');
  if (wa) wa.href = 'https://wa.me/?text=' + encodeURIComponent('Order from my shop on LaikiHub: ' + url);
  var copy = document.getElementById('share-copy');
  if (copy) copy.addEventListener('click', async function () {
    try {
      await navigator.clipboard.writeText(url);
      status.textContent = 'Copied!';
    } catch (_) {
      input.select();
      status.textContent = 'Press copy on your keyboard to copy the link.';
    }
  });
})();
