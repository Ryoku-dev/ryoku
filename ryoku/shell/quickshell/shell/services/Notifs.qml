pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.Notifications
import Ryoku.Ui.Singletons

// Notification model, faithful to the reference wayle-notification service
// (contract 07 sec 4.3, sec 8; verified against monitoring.rs). Two flat,
// newest-first lists with NO app grouping:
//   history  the panel list: every tracked, non-transient notification, newest
//            first, self-expiring per remove_expired.
//   popups   the transient popup list: newest first (insert at 0), gated by DND,
//            dedup-then-front by id, each with its own display timer.
// Urgency is parsed by the server but has ZERO effect here: no styling, no
// ordering, no timeout weighting, exactly as upstream. DND is shell-owned (bound
// from Flags.dnd in shell.qml) and suppresses popups only; history still fills.
Singleton {
    id: root

    // Shell-owned Do Not Disturb; gates popups only.
    property bool dnd: false

    // popup_duration: the reference wayle builder default. Quickshell's server
    // carries no popup timer, so the shell owns it.
    readonly property int popupDuration: 5000


    // Arrival wall-clock per id, for newest-first ordering and expiry math.
    property var arrivalMs: ({})
    // Popup display deadlines per id (absent = persist, no timer).
    property var popupExpireAt: ({})

    // Raw server-tracked notifications (the rail's has-notifs dot reads this).
    readonly property var tracked: server.trackedNotifications.values

    // Panel list: flat, newest-first, transient excluded. A notification whose
    // remove_expired has elapsed has already left `tracked` (historyReaper calls
    // expire()); expire_timeout 0 is excluded here because it is removed from
    // history at once while its popup persists (contract 07 sec 8 asymmetry).
    readonly property var history: {
        var t = root.tracked;
        var out = [];
        for (var i = 0; i < t.length; i++) {
            var n = t[i];
            if (n.transient || n.expireTimeout === 0)
                continue;
            out.push(n);
        }
        out.sort(function(a, b) { return (root.arrivalMs[b.id] || 0) - (root.arrivalMs[a.id] || 0); });
        return out;
    }

    // Popup list: flat, newest-first. Managed imperatively so the display timers
    // can drive it.
    property var popups: []

    // Popup timer rule (contract 07 sec 8): expire_timeout -1 (server default)
    // -> popupDuration; 0 -> never (persist); n>0 -> min(popupDuration, n).
    function popupTtl(n) {
        var et = n.expireTimeout;
        if (et === 0) return -1;
        if (et < 0) return root.popupDuration;
        return Math.min(root.popupDuration, et);
    }

    // Two popups are the same alert when their app, summary and body match; a
    // resend (new id, same content) or two apps firing the same one would else
    // stack identical cards.
    function sameContent(a, b) {
        return (a.appName || "") === (b.appName || "")
            && (a.summary || "") === (b.summary || "")
            && (a.body || "") === (b.body || "");
    }

    // Add a popup: dedup-then-insert-front. A re-posted id, or a distinct id with
    // the same content, replaces the live card (moved to the top with a fresh
    // timer) rather than adding a twin; then arm its timer unless it is persistent.
    function addPopup(n) {
        var list = root.popups.filter(function(p) { return p.id !== n.id && !root.sameContent(p, n); });
        list.unshift(n);
        root.popups = list;
        var ttl = root.popupTtl(n);
        var e = Object.assign({}, root.popupExpireAt);
        if (ttl < 0)
            delete e[n.id];
        else
            e[n.id] = Date.now() + ttl;
        root.popupExpireAt = e;
        schedulePopupReap();
    }

    function removePopup(n) {
        root.popups = root.popups.filter(function(p) { return p !== n; });
        var e = Object.assign({}, root.popupExpireAt);
        delete e[n.id];
        root.popupExpireAt = e;
    }

    // Close button (contract 07 sec 4.3): DismissedByUser.
    function dismiss(n) {
        if (n && typeof n.dismiss === "function")
            n.dismiss();
    }

    // Clear all (contract 07 sec 4.3): dismiss every history item. Their popups
    // fall away with them via the closed hook; a persistent popup (expire_timeout
    // 0, not in history) is deliberately left, matching dismiss_all.
    function clearAll() {
        var h = root.history.slice();
        for (var i = 0; i < h.length; i++)
            if (typeof h[i].dismiss === "function")
                h[i].dismiss();
    }

    // Card time label (contract 07 sec 2.3): 24h "HH:MM"; 12h "hh:MM am/pm".
    function timeLabel(n) {
        if (!n)
            return "";
        var ts = root.arrivalMs[n.id];
        if (!ts)
            return "";
        return TimeFormat.format(new Date(ts));
    }

    // Popup reaper: remove popups past their deadline. A transient popup is
    // released entirely (expire) since it never entered history; a normal popup
    // just leaves the popup list and stays in history.
    Timer {
        id: popupReaper
        onTriggered: {
            var now = Date.now();
            var e = Object.assign({}, root.popupExpireAt);
            var live = [];
            var release = [];
            for (var i = 0; i < root.popups.length; i++) {
                var p = root.popups[i];
                var due = e[p.id];
                if (due !== undefined && due <= now) {
                    delete e[p.id];
                    if (p.transient && typeof p.expire === "function")
                        release.push(p);
                } else {
                    live.push(p);
                }
            }
            root.popupExpireAt = e;
            if (live.length !== root.popups.length)
                root.popups = live;
            for (var j = 0; j < release.length; j++)
                release[j].expire();
            root.schedulePopupReap();
        }
    }

    function schedulePopupReap() {
        var now = Date.now();
        var soonest = -1;
        for (var id in root.popupExpireAt) {
            var due = root.popupExpireAt[id];
            if (soonest < 0 || due < soonest)
                soonest = due;
        }
        popupReaper.stop();
        if (soonest >= 0) {
            popupReaper.interval = Math.max(1, soonest - now);
            popupReaper.start();
        }
    }

    // History reaper (remove_expired, contract 07 sec 8): a notification with
    // expire_timeout n>0 leaves history at arrival + n (full, uncapped); -1 and
    // 0 are handled elsewhere (-1 never, 0 excluded from history immediately).
    Timer {
        id: historyReaper
        onTriggered: {
            var now = Date.now();
            var t = root.tracked;
            for (var i = 0; i < t.length; i++) {
                var n = t[i];
                if (n.transient || n.expireTimeout <= 0)
                    continue;
                if ((root.arrivalMs[n.id] || 0) + n.expireTimeout <= now && typeof n.expire === "function")
                    n.expire();
            }
            root.scheduleHistoryReap();
        }
    }

    function scheduleHistoryReap() {
        var now = Date.now();
        var soonest = -1;
        var t = root.tracked;
        for (var i = 0; i < t.length; i++) {
            var n = t[i];
            if (n.transient || n.expireTimeout <= 0)
                continue;
            var due = (root.arrivalMs[n.id] || 0) + n.expireTimeout;
            if (soonest < 0 || due < soonest)
                soonest = due;
        }
        historyReaper.stop();
        if (soonest >= 0) {
            historyReaper.interval = Math.max(1, soonest - now);
            historyReaper.start();
        }
    }

    // On close (dismiss / expire / app request): drop the popup and forget the
    // bookkeeping; history recomputes from the shrunken tracked list.
    function hookClosed(n) {
        n.closed.connect(function(reason) {
            root.removePopup(n);
            var a = Object.assign({}, root.arrivalMs);
            delete a[n.id];
            root.arrivalMs = a;
            root.schedulePopupReap();
            root.scheduleHistoryReap();
        });
    }

    NotificationServer {
        id: server
        keepOnReload: true
        bodySupported: true
        actionsSupported: true
        imageSupported: true

        Component.onCompleted: {
            var l = trackedNotifications.values;
            var a = Object.assign({}, root.arrivalMs);
            for (var i = 0; i < l.length; i++) {
                if (!a[l[i].id])
                    a[l[i].id] = Date.now();
                root.hookClosed(l[i]);
            }
            root.arrivalMs = a;
            root.scheduleHistoryReap();
        }

        onNotification: function(n) {
            var a = Object.assign({}, root.arrivalMs);
            a[n.id] = Date.now();
            root.arrivalMs = a;
            n.tracked = true;
            root.hookClosed(n);
            if (!root.dnd)
                root.addPopup(n);
            root.scheduleHistoryReap();
        }
    }
}
