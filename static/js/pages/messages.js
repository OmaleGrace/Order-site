(() => {
  const list = document.getElementById('message-list');
  if (!list) return;
  let lastID = Number(list.dataset.lastId || 0);
  const threadID = list.dataset.threadId;
  list.scrollTop = list.scrollHeight;
  const TICKS = '<svg viewBox="0 0 18 11" width="18" height="11" aria-hidden="true"><path class="t1" d="M1 5.8l3.2 3.2 6.6-7.2"/><path class="t2" d="M8.4 9L15 1.8"/></svg>';
  const RANK = {sent: 0, delivered: 1, read: 2};

  // ---- ticks -------------------------------------------------------------
  const setStatus = (ticks, status) => {
    if (!ticks || !(status in RANK)) return;
    if (RANK[status] <= RANK[ticks.dataset.status]) return; // ticks only ever move forward
    ticks.dataset.status = status;
    ticks.setAttribute('aria-label', status);
  };
  const applyReceipts = (deliveredUpTo, readUpTo) => {
    list.querySelectorAll('.bubble.mine').forEach((bubble) => {
      const id = Number(bubble.dataset.id);
      const ticks = bubble.querySelector('.ticks');
      if (id <= readUpTo) setStatus(ticks, 'read');
      else if (id <= deliveredUpTo) setStatus(ticks, 'delivered');
    });
  };

  // ---- who sent it: a shop links to its page, a customer opens their picture ----
  const senderLabel = (m) => {
    const label = document.createElement('span'); label.className = 'sender';
    if (m.mine) { label.textContent = 'You'; return label; }
    if (list.dataset.otherKind === 'shop') {
      if (!list.dataset.shopUrl) { label.textContent = m.sender; return label; }
      const a = document.createElement('a');
      a.className = 'person-link'; a.href = list.dataset.shopUrl; a.title = 'View shop'; a.textContent = m.sender;
      label.append(a); return label;
    }
    const b = document.createElement('button');
    b.type = 'button'; b.className = 'person-link'; b.title = 'View profile picture'; b.textContent = m.sender;
    b.setAttribute('data-profile-photo', '');
    label.append(b); return label;
  };
  const initialAvatar = (name) => {
    const ch = (Array.from((name || '').trim())[0] || '?').toUpperCase().replace(/[&<>"']/g, '?');
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400"><rect width="400" height="400" fill="#e1f1e5"/>' +
      '<text x="200" y="200" font-family="system-ui,sans-serif" font-size="180" font-weight="700" fill="#14532d" text-anchor="middle" dominant-baseline="central">' + ch + '</text></svg>';
    return 'data:image/svg+xml,' + encodeURIComponent(svg);
  };
  // Reuses the site's existing picture viewer (lightbox.js), which opens any img.zoomable.
  const photo = document.createElement('img');
  photo.className = 'zoomable'; photo.hidden = true; document.body.appendChild(photo);
  document.addEventListener('click', (e) => {
    const trigger = e.target.closest ? e.target.closest('[data-profile-photo]') : null;
    if (!trigger) return;
    photo.alt = list.dataset.otherName || 'Customer';
    photo.src = list.dataset.otherPicture || initialAvatar(list.dataset.otherName);
    photo.click();
  });

  const addMessage = (m) => {
    const article = document.createElement('article');
    article.className = 'bubble' + (m.mine ? ' mine' : '');
    article.dataset.id = m.id;
    const sender = senderLabel(m);
    const body = document.createElement('div'); body.className = 'body'; body.textContent = m.body;
    const meta = document.createElement('div'); meta.className = 'meta';
    const time = document.createElement('time'); time.textContent = m.created_at;
    meta.append(time);
    if (m.mine) {
      const ticks = document.createElement('span');
      ticks.className = 'ticks'; ticks.setAttribute('role', 'img');
      ticks.dataset.status = m.status || 'sent'; ticks.setAttribute('aria-label', ticks.dataset.status);
      ticks.innerHTML = TICKS; // constant markup defined above, no user content
      meta.append(ticks);
    }
    article.append(sender, body, meta); list.appendChild(article);
  };

  // ---- copy a message: click with a mouse, long-press on touch -----------
  const toast = document.createElement('div');
  toast.className = 'copy-toast'; toast.setAttribute('role', 'status');
  document.body.appendChild(toast);
  let toastTimer;
  const showToast = (text) => {
    toast.textContent = text; toast.classList.add('show');
    clearTimeout(toastTimer); toastTimer = setTimeout(() => toast.classList.remove('show'), 1600);
  };
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext) { await navigator.clipboard.writeText(text); return true; }
    } catch (_) { /* fall through to the legacy method */ }
    const ta = document.createElement('textarea');
    ta.value = text; ta.setAttribute('readonly', ''); ta.style.cssText = 'position:fixed;top:0;left:-9999px;opacity:0';
    document.body.appendChild(ta); ta.select(); ta.setSelectionRange(0, text.length);
    let ok = false;
    try { ok = document.execCommand('copy'); } catch (_) {}
    ta.remove();
    return ok;
  }
  async function copyBubble(bubble) {
    const body = bubble.querySelector('.body');
    const ok = await copyText(body ? body.textContent : '');
    if (window.getSelection) window.getSelection().removeAllRanges();
    showToast(ok ? 'Message copied' : 'Could not copy this message');
    if (ok) { bubble.classList.add('copied'); setTimeout(() => bubble.classList.remove('copied'), 600); }
  }
  const bubbleOf = (e) => (e.target.closest ? e.target.closest('.bubble') : null);
  let lastPointer = 'mouse', pressTimer = null, pressStart = null;
  const cancelPress = () => { clearTimeout(pressTimer); pressTimer = null; };
  list.addEventListener('pointerdown', (e) => {
    lastPointer = e.pointerType || 'mouse';
    const bubble = bubbleOf(e);
    if (!bubble || lastPointer === 'mouse' || e.target.closest('.person-link')) return;
    pressStart = {x: e.clientX, y: e.clientY};
    cancelPress();
    pressTimer = setTimeout(() => { pressTimer = null; copyBubble(bubble); }, 500);
  });
  list.addEventListener('pointermove', (e) => {
    if (pressTimer && pressStart && Math.hypot(e.clientX - pressStart.x, e.clientY - pressStart.y) > 10) cancelPress();
  });
  ['pointerup', 'pointercancel', 'pointerleave', 'scroll'].forEach((t) => list.addEventListener(t, cancelPress));
  // Stop the phone's own long-press menu from fighting with our copy.
  list.addEventListener('contextmenu', (e) => { if (bubbleOf(e) && lastPointer !== 'mouse' && !e.target.closest('.person-link')) e.preventDefault(); });
  list.addEventListener('click', (e) => {
    const bubble = bubbleOf(e);
    if (!bubble || lastPointer !== 'mouse' || e.target.closest('.person-link')) return; // touch: long-press only; names open the profile
    if (String(window.getSelection())) return;       // the person is selecting text, don't interfere
    copyBubble(bubble);
  });

  // ---- polling -----------------------------------------------------------
  let polling = false;
  async function poll() {
    if (polling) return;
    polling = true;
    try {
      const visible = document.visibilityState === 'visible' ? 1 : 0;
      const response = await fetch(`/messages/${threadID}/updates?after=${lastID}&visible=${visible}`, {headers: {'Accept':'application/json'}, cache:'no-store'});
      if (!response.ok) return;
      const data = await response.json();
      const updates = data.messages || [];
      if (updates.length) {
        updates.forEach(m => { if (m.id > lastID) lastID = m.id; addMessage(m); });
        list.scrollTop = list.scrollHeight;
      }
      applyReceipts(data.delivered_up_to || 0, data.read_up_to || 0);
    } catch (_) { /* Keep the chat usable; retry on the next interval. */ }
    finally { polling = false; }
  }
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') poll(); });
  window.setInterval(poll, 5000);
  const draftButton = document.getElementById('ai-draft-button');
  if (draftButton) {
    draftButton.addEventListener('click', async () => {
      const status = document.getElementById('ai-draft-status');
      const textarea = document.getElementById('body');
      draftButton.disabled = true;
      status.textContent = 'Drafting a reply…';
      try {
        const response = await fetch(`/messages/${threadID}/ai-draft`, {method:'POST', headers:{'Accept':'application/json'}});
        if (!response.ok) {
          const contentType = response.headers.get('content-type') || '';
          if (contentType.includes('application/json')) {
            const errorData = await response.json();
            status.textContent = errorData.error || 'Could not draft a reply right now. Please try again in a moment.';
          } else {
            const text = await response.text();
            status.textContent = text || 'Could not draft a reply. Check AI configuration and try again.';
          }
          if (response.status === 401) {
            status.textContent += ' Open the login page and sign in again.';
          }
          return;
        }
        const result = await response.json();
        textarea.value = result.draft || '';
        textarea.focus();
        status.textContent = 'AI draft added to the message box. Review it carefully, then send when ready.';
      } catch (_) { status.textContent = 'Could not reach the AI service. Please try again.'; }
      finally { draftButton.disabled = false; }
    });
  }
})();
