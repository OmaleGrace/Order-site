// "Suggest reply": asks the AI for a draft answer to a support ticket and puts it in the
// reply box. Nothing is saved or sent until the admin presses "Save reply".
(function () {
  document.querySelectorAll('.ai-reply-btn').forEach(function (button) {
    button.addEventListener('click', async function () {
      var id = button.getAttribute('data-ticket');
      var box = document.getElementById('reply-' + id);
      var status = button.parentElement.querySelector('.ai-reply-status');
      if (!box) return;
      if (box.value.trim() && !window.confirm('Replace the text in the reply box with an AI suggestion?')) return;
      button.disabled = true;
      status.textContent = 'Writing a suggestion…';
      try {
        var response = await fetch('/admin/support/ai-reply', {
          method: 'POST',
          headers: { 'Accept': 'application/json', 'Content-Type': 'application/x-www-form-urlencoded' },
          body: 'id=' + encodeURIComponent(id)
        });
        var data = await response.json().catch(function () { return {}; });
        if (!response.ok) { status.textContent = data.error || 'Could not write a suggestion right now.'; return; }
        box.value = data.draft || '';
        box.focus();
        status.textContent = 'Suggestion added. Check it, edit if needed, then press Save reply.';
      } catch (_) {
        status.textContent = 'Could not reach the AI service. Please try again.';
      } finally {
        button.disabled = false;
      }
    });
  });
})();
