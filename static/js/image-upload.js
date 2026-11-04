// Lets a vendor pick a photo from their phone or computer. The photo is shrunk in the
// browser first (phone cameras make 5 MB+ files), then uploaded with the form.
(function () {
  var MAX_SIDE = 1280;

  function show(input, file) {
    var preview = document.querySelector('[data-image-preview="' + input.id + '"]');
    if (!preview) return;
    if (preview.dataset.url) URL.revokeObjectURL(preview.dataset.url);
    var url = URL.createObjectURL(file);
    preview.dataset.url = url;
    preview.src = url;
    preview.hidden = false;
  }

  function say(input, text) {
    var status = document.querySelector('[data-image-status="' + input.id + '"]');
    if (status) status.textContent = text;
  }

  function shrink(file) {
    return new Promise(function (resolve, reject) {
      var img = new Image();
      var src = URL.createObjectURL(file);
      img.onload = function () {
        URL.revokeObjectURL(src);
        var scale = Math.min(1, MAX_SIDE / Math.max(img.width, img.height));
        var canvas = document.createElement('canvas');
        canvas.width = Math.round(img.width * scale);
        canvas.height = Math.round(img.height * scale);
        var ctx = canvas.getContext('2d');
        ctx.fillStyle = '#fff';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
        canvas.toBlob(function (blob) {
          if (!blob) { reject(new Error('resize failed')); return; }
          resolve(new File([blob], 'photo.jpg', { type: 'image/jpeg' }));
        }, 'image/jpeg', 0.85);
      };
      img.onerror = function () { URL.revokeObjectURL(src); reject(new Error('unreadable')); };
      img.src = src;
    });
  }

  document.querySelectorAll('input[type="file"][data-image-upload]').forEach(function (input) {
    input.addEventListener('change', async function () {
      var file = input.files && input.files[0];
      if (!file) { say(input, ''); return; }
      say(input, 'Preparing photo…');
      try {
        var small = await shrink(file);
        var transfer = new DataTransfer();
        transfer.items.add(small);
        input.files = transfer.files;
        show(input, small);
        say(input, 'Photo ready (' + Math.max(1, Math.round(small.size / 1024)) + ' KB). It will upload when you save.');
      } catch (_) {
        // Could not resize here (for example an unusual format). Let the server check the original.
        if (file.size > 3 * 1024 * 1024) {
          input.value = '';
          say(input, 'That photo is too big or in a format we can\'t read. Please choose a JPG or PNG.');
        } else {
          show(input, file);
          say(input, 'Photo selected.');
        }
      }
    });
    if (input.form) {
      input.form.addEventListener('reset', function () {
        var preview = document.querySelector('[data-image-preview="' + input.id + '"]');
        if (preview) { preview.hidden = true; preview.removeAttribute('src'); }
        say(input, '');
      });
    }
  });
})();
