(function () {
    'use strict';

    function escapeHtml(value) {
        return String(value).replace(/[&<>'"]/g, function (ch) {
            return {'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[ch];
        });
    }

    function buildBell() {
        var nav = document.querySelector('header nav');
        if (!nav || nav.querySelector('.notification-nav-item')) return null;

        var item = document.createElement('a');
        item.href = '/notifications';
        item.className = 'notification-nav-item';
        item.setAttribute('aria-label', 'Notifications');
        item.title = 'Notifications';
        item.innerHTML = '<span class="notification-bell-icon" aria-hidden="true">🔔</span>' +
            '<span class="notification-nav-label">Notifications</span>' +
            '<span class="notification-unread-badge" hidden>0</span>';

        nav.insertBefore(item, nav.firstChild);
        return item;
    }

    function buildMessagesLink() {
        var nav = document.querySelector('header nav');
        if (!nav || nav.querySelector('.messages-nav-item')) return null;
        var item = document.createElement('a');
        item.href = '/messages';
        item.className = 'messages-nav-item';
        item.setAttribute('aria-label', 'Messages');
        item.title = 'Messages';
        item.innerHTML = '<span class="messages-nav-icon" aria-hidden="true">💬</span>' +
            '<span class="messages-nav-label">Messages</span>' +
            '<span class="messages-unread-badge" hidden>0</span>';
        nav.insertBefore(item, nav.firstChild ? nav.firstChild.nextSibling : null);
        return item;
    }

    function updateMessageBadge(item, count) {
        if (!item) return;
        var badge = item.querySelector('.messages-unread-badge');
        if (!badge) return;
        if (count > 0) {
            badge.textContent = count > 99 ? '99+' : String(count);
            badge.hidden = false;
            item.setAttribute('aria-label', 'Messages, ' + count + ' unread');
        } else {
            badge.hidden = true;
            item.setAttribute('aria-label', 'Messages');
        }
    }

    function updateBadge(item, count) {
        if (!item) return;
        var badge = item.querySelector('.notification-unread-badge');
        if (!badge) return;
        if (count > 0) {
            badge.textContent = count > 99 ? '99+' : String(count);
            badge.hidden = false;
            item.setAttribute('aria-label', 'Notifications, ' + count + ' unread');
        } else {
            badge.hidden = true;
            item.setAttribute('aria-label', 'Notifications');
        }
    }

    function loadCounts() {
        if (!window.__enieHeaderCountsPromise) {
            window.__enieHeaderCountsPromise = fetch('/header/counts', {
                credentials: 'same-origin',
                headers: {'Accept': 'application/json'}
            }).then(function (response) {
                if (!response.ok) throw new Error('header counts unavailable');
                return response.json();
            });
        }
        return window.__enieHeaderCountsPromise;
    }

    function loadCount() {
        var item = buildBell();
        var messages = buildMessagesLink();
        if (!item && !messages) return;

        loadCounts()
            .then(function (data) {
                updateBadge(item, Number(data.notification_count) || 0);
                updateMessageBadge(messages, Number(data.message_count) || 0);
            })
            .catch(function () {
                if (item && item.parentNode) item.parentNode.removeChild(item);
            });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', loadCount);
    } else {
        loadCount();
    }
})();
