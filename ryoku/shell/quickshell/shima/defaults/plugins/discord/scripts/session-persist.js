// Discord Session Persistence for iNiR WebApps
// ─────────────────────────────────────────────────
// Injected at DocumentReady/MainWorld via WebEngineScript.
//
// Problem: Chromium userscripts (even MainWorld/worldId=0) don't get
// window.localStorage bound — the Storage constructor exists but the
// instance is undefined. Discord's own page JS CAN use it.
//
// Solution: Create a same-origin about:blank iframe. Its contentWindow
// shares the same Storage backend, and its localStorage IS accessible
// from our script context. We use that as a proxy for all operations.
//
// Backup flow: localStorage → cookies (SQLite-persisted by WebEngine)
// Restore flow: cookies → localStorage (on restart, before Discord reads)

(function() {
    'use strict';

    if (!location.hostname.includes('discord')) return;

    var COOKIE_PREFIX = '__inir_';
    var MAX_AGE = 365 * 24 * 60 * 60;
    var KEYS = ['token', 'tokens', 'user_id_cache', 'MultiAccountStore'];

    function setCookie(name, value) {
        document.cookie = COOKIE_PREFIX + name + '=' + encodeURIComponent(value)
            + ';path=/;max-age=' + MAX_AGE + ';SameSite=Lax';
    }

    function getCookie(name) {
        var fullName = COOKIE_PREFIX + name + '=';
        var parts = document.cookie.split(';');
        for (var i = 0; i < parts.length; i++) {
            var c = parts[i].trim();
            if (c.indexOf(fullName) === 0)
                return decodeURIComponent(c.substring(fullName.length));
        }
        return null;
    }

    function isValid(v) {
        return v != null && v !== '' && v !== '{}' && v !== '""'
            && v !== 'null' && v !== 'undefined';
    }

    // ── Obtain localStorage via iframe proxy ────────────────────────
    // Chromium denies window.localStorage to userscripts, but a
    // same-origin about:blank iframe's contentWindow.localStorage works
    // and shares the same storage backend.

    var _ls = null;

    function getLocalStorage() {
        if (_ls) return _ls;

        // Try direct access first (might work in future Chromium versions)
        if (typeof localStorage !== 'undefined' && localStorage !== null) {
            _ls = localStorage;
            return _ls;
        }

        // Try Window.prototype getter
        try {
            var desc = Object.getOwnPropertyDescriptor(Window.prototype, 'localStorage');
            if (desc && desc.get) {
                var ls = desc.get.call(window);
                if (ls) { _ls = ls; return _ls; }
            }
        } catch(e) {}

        // Iframe proxy — the reliable fallback
        try {
            var iframe = document.createElement('iframe');
            iframe.style.display = 'none';
            iframe.src = 'about:blank';
            document.documentElement.appendChild(iframe);
            if (iframe.contentWindow && iframe.contentWindow.localStorage) {
                _ls = iframe.contentWindow.localStorage;
                return _ls;
            }
            iframe.remove();
        } catch(e) {}

        return null;
    }

    // ── Wait for DOM to be ready enough for iframe ──────────────────
    var attempts = 0;
    function waitForStorage() {
        var ls = getLocalStorage();
        if (ls) {
            onReady(ls);
            return;
        }
        if (++attempts > 100) return; // give up after 10s
        setTimeout(waitForStorage, 100);
    }

    function onReady(ls) {
        setCookie('status', 'active');

        // ── RESTORE from cookies → localStorage ─────────────────
        var currentToken = ls.getItem('token');
        var cookieToken = getCookie('token');

        if (!isValid(currentToken) && isValid(cookieToken)) {
            for (var i = 0; i < KEYS.length; i++) {
                var val = getCookie(KEYS[i]);
                if (isValid(val)) ls.setItem(KEYS[i], val);
            }
            // Reload so Discord picks up the restored token
            window.location.replace('https://discord.com/app');
            return;
        }

        // ── BACKUP localStorage → cookies ───────────────────────
        function backupAll() {
            for (var i = 0; i < KEYS.length; i++) {
                var val = ls.getItem(KEYS[i]);
                if (isValid(val)) setCookie(KEYS[i], val);
            }
        }

        // ── HOOK Storage.prototype.setItem ──────────────────────
        try {
            var _origSetItem = Storage.prototype.setItem;
            Storage.prototype.setItem = function(key, value) {
                if (KEYS.indexOf(key) !== -1 && isValid(value)) {
                    setCookie(key, value);
                }
                return _origSetItem.call(this, key, value);
            };
        } catch(e) {}

        // ── Periodic backup ─────────────────────────────────────
        var fastCount = 0;
        var fast = setInterval(function() {
            backupAll();
            if (++fastCount >= 30) clearInterval(fast);
        }, 2000);
        setInterval(backupAll, 30000);

        document.addEventListener('visibilitychange', function() {
            if (document.hidden) backupAll();
        });
        window.addEventListener('beforeunload', backupAll);
        window.addEventListener('pagehide', backupAll);

        backupAll();
    }

    if (document.documentElement) {
        waitForStorage();
    } else {
        document.addEventListener('DOMContentLoaded', waitForStorage);
    }
})();
