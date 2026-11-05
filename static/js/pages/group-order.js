// Group order request form: show the address box only for delivery, and count the notes.
(function () {
  var form = document.getElementById('group-form');
  if (!form) return;
  var radios = form.querySelectorAll('input[name="delivery_method"]');
  var addressBox = document.getElementById('go-address-box');
  var address = document.getElementById('address');
  var notes = document.getElementById('notes');
  var count = document.getElementById('notes-count');

  function method() {
    var picked = form.querySelector('input[name="delivery_method"]:checked');
    return picked ? picked.value : 'pickup';
  }
  function syncMethod() {
    var delivery = method() === 'delivery';
    addressBox.hidden = !delivery;
    address.required = delivery;
    if (!delivery) address.setCustomValidity('');
  }
  function syncCount() {
    if (!notes || !count) return;
    var n = Array.from(notes.value).length;
    count.textContent = n + ' / 1000';
  }
  radios.forEach(function (r) { r.addEventListener('change', syncMethod); });
  if (notes) notes.addEventListener('input', syncCount);
  syncMethod();
  syncCount();
})();
