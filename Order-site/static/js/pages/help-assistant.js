// Help page assistant: quick answers for customers, with a pointer to the support form.
(function () {
  var form = document.getElementById('assistant-form');
  if (!form) return;
  var input = document.getElementById('assistant-question');
  var out = document.getElementById('assistant-answer');
  var button = form.querySelector('button[type="submit"]');
  form.addEventListener('submit', async function (e) {
    e.preventDefault();
    var q = input.value.trim();
    if (q.length < 3) { out.textContent = 'Please type your question first.'; return; }
    button.disabled = true;
    out.textContent = 'Thinking…';
    try {
      var response = await fetch('/help/ask', {
        method: 'POST',
        headers: { 'Accept': 'application/json', 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'question=' + encodeURIComponent(q)
      });
      var data = await response.json().catch(function () { return {}; });
      out.textContent = response.ok ? (data.answer || '') : (data.error || 'Could not get an answer right now. You can send us a support request below.');
    } catch (_) {
      out.textContent = 'Could not reach the assistant. You can send us a support request below.';
    } finally {
      button.disabled = false;
    }
  });
})();
