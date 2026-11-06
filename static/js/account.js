// Account pages: copy buttons, live photo preview, password match check.
(function () {
  'use strict';
  document.querySelectorAll('[data-copy]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var el = document.querySelector(btn.getAttribute('data-copy'));
      if (!el) return;
      var text = el.textContent.trim();
      var done = function () {
        var old = btn.textContent;
        btn.textContent = 'Copied';
        setTimeout(function () { btn.textContent = old; }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () {});
      } else {
        var r = document.createRange(); r.selectNodeContents(el);
        var s = window.getSelection(); s.removeAllRanges(); s.addRange(r);
        try { document.execCommand('copy'); done(); } catch (e) {}
        s.removeAllRanges();
      }
    });
  });

  var file = document.querySelector('input[type=file][data-preview]');
  if (file) {
    file.addEventListener('change', function () {
      var f = file.files && file.files[0];
      var hint = document.getElementById('ap-photo-hint');
      var max = parseFloat(file.getAttribute('data-max-mb') || '5') * 1024 * 1024;
      if (!f) return;
      if (f.size > max) {
        file.value = '';
        if (hint) { hint.textContent = 'That photo is over ' + (max / 1048576) + 'MB. Please choose a smaller one.'; hint.classList.add('ap-hint-err'); }
        return;
      }
      if (hint) { hint.textContent = f.name + ' selected. Save changes to apply it.'; hint.classList.remove('ap-hint-err'); }
      var target = document.querySelector(file.getAttribute('data-preview'));
      if (!target || !window.URL) return;
      var url = URL.createObjectURL(f);
      if (target.tagName === 'IMG') { target.src = url; return; }
      var img = document.createElement('img');
      img.src = url; img.alt = 'Selected profile picture'; img.id = target.id; img.className = 'ap-avatar ap-avatar-lg';
      target.replaceWith(img);
    });
  }

  var np = document.getElementById('new_password'), cp = document.getElementById('confirm_password');
  if (np && cp) {
    var check = function () { cp.setCustomValidity(cp.value && cp.value !== np.value ? 'Passwords do not match' : ''); };
    np.addEventListener('input', check); cp.addEventListener('input', check);
  }

  // Highlight the settings section being read
  var links = document.querySelectorAll('.ap-subnav a');
  if (links.length && 'IntersectionObserver' in window) {
    var map = {};
    links.forEach(function (a) { map[a.getAttribute('href').slice(1)] = a; });
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (e) {
        if (e.isIntersecting) {
          links.forEach(function (a) { a.classList.remove('is-active'); });
          if (map[e.target.id]) map[e.target.id].classList.add('is-active');
        }
      });
    }, { rootMargin: '-20% 0px -65% 0px' });
    Object.keys(map).forEach(function (id) { var s = document.getElementById(id); if (s) io.observe(s); });
  }
})();
