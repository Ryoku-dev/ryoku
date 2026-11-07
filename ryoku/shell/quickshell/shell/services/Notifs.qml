pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.Notifications
import Ryoku.Ui.Singletons
import shell.modules.phone as Phone

// Notification model, faithful to the reference wayle-notification service
// (contract 07 sec 4.3, sec 8; verified against monitoring.rs). Two flat,
// newest-first lists with NO app grouping:
//   history  the panel list: every tracked, non-transient notification, newest
//            first, kept until dismissed or closed by its sender.
//   popups   the transient popup list: newest first (insert at 0), gated by DND,
//            dedup-then-front by id, each with its own display timer.
// Critical notifications persist until dismissed. Other urgency levels only pick
// the arrival cue; DND is shell-owned (bound from Flags.dnd in shell.qml) and
// suppresses popups and their cue while history still fills.
Singleton {
    id: root

    // Shell-owned Do Not Disturb; gates popups only.
    property bool dnd: false

    // popup_duration: the reference wayle builder default unless the active
    // subshell or bar style draws the banners with its own duration.
    readonly property int popupDuration: Shells.notificationMs > 0
        ? Shells.notificationMs : 5000


    // Arrival wall-clock per id, for newest-first ordering and expiry math.
    property var arrivalMs: ({})
    // Popup display deadlines per id (absent = persist, no timer).
    property var popupExpireAt: ({})

    // Raw server-tracked notifications (the rail's has-notifs dot reads this).
    readonly property var tracked: server.trackedNotifications.values

    // All screen-local bars acknowledge the same in-memory notification cursor.
    property double seenMs: 0
    readonly property double newestArrivalMs: {
        const notifications = root.tracked || [];
        let newest = 0;
        for (let index = 0; index < notifications.length; ++index)
            newest = Math.max(newest, Number(root.arrivalMs[notifications[index].id] || 0));
        return newest;
    }
    readonly property bool hasUnread: !root.dnd && root.newestArrivalMs > root.seenMs

    function markSeen(throughMs) {
        const value = throughMs === undefined
            ? root.newestArrivalMs : Number(throughMs || 0);
        if (value > root.seenMs)
            root.seenMs = value;
    }

    onDndChanged: {
        if (dnd)
            root.markSeen();
    }

    // Panel list: flat, newest-first, transient excluded. A sender's
    // expire_timeout only times its popup; the notification stays here until the
    // user dismisses it or the sender closes it, so the panel is a real history.
    readonly property var history: {
        var t = root.tracked;
        var out = [];
        for (var i = 0; i < t.length; i++) {
            var n = t[i];
            if (n.transient)
                continue;
            out.push(n);
        }
        out.sort(function(a, b) { return (root.arrivalMs[b.id] || 0) - (root.arrivalMs[a.id] || 0); });
        return out;
    }

    // Popup list: flat, newest-first. Managed imperatively so the display timers
    // can drive it.
    property var popups: []

    // Popup timer rule (contract 07 sec 8): critical or expire_timeout 0 persists;
    // -1 uses popupDuration; n>0 uses min(popupDuration, n).
    function popupTtl(n) {
        if (n.urgency === NotificationUrgency.Critical) return -1;
        var et = n.expireTimeout;
        if (et === 0) return -1;
        if (et < 0) return root.popupDuration;
        return Math.min(root.popupDuration, et);
    }

    function cancelPopupTimeout(id) {
        if (root.popupExpireAt[id] === undefined)
            return;
        var e = Object.assign({}, root.popupExpireAt);
        delete e[id];
        root.popupExpireAt = e;
        root.schedulePopupReap();
    }

    // Two popups are the same alert when their app, summary and body match; a
    // resend (new id, same content) or two apps firing the same one would else
    // stack identical cards.
    function sameContent(a, b) {
        return (a.appName || "") === (b.appName || "")
            && (a.summary || "") === (b.summary || "")
            && (a.body || "") === (b.body || "");
    }
    function isKdeConnectNotification(n) {
        if (!(Config.phone?.enabled ?? false))
            return false
        const name = String(n?.appName || "").toLowerCase()
        if (name === "kdeconnect" || name === "kde connect"
                || name === "org.kde.kdeconnect")
            return true
        return Phone.KdeConnectService.devices.some(device =>
            device.name && String(device.name).toLowerCase() === name)
    }

    // Add a popup: dedup-then-insert-front. A re-posted id, or a distinct id with
    // the same content, replaces the live card (moved to the top with a fresh
    // timer) rather than adding a twin; then arm its timer unless it is persistent.
    // The arrival cue. The daemon owns the clip, the user's choice of file, the
    // volume and the on/off switches (`sounds` in shell.json); a sender can still
    // ask for silence with the spec's suppress-sound hint. A bar style that
    // plays its own sound for its banners takes the cue over.
    function chime(n) {
        if (Config.styleOwnsNotificationSound)
            return;
        const hints = n.hints || {};
        if (hints["suppress-sound"] === true)
            return;
        const critical = n.urgency === NotificationUrgency.Critical;
        Quickshell.execDetached(["ryoku-shell", "sound", critical ? "notification-critical" : "notification"]);
    }

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
    // fall away with them via the closed hook.
    function clearAll() {
        root.markSeen();
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

    // Oldest history entries are released past this many, so a chatty sender
    // cannot grow the tracked list for the whole session.
    readonly property int historyLimit: 100

    function trimHistory() {
        var h = root.history;
        for (var i = h.length - 1; i >= root.historyLimit; i--)
            if (typeof h[i].expire === "function")
                h[i].expire();
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
        });
    }

    NotificationServer {
        id: server
        keepOnReload: true
        bodySupported: true
        actionsSupported: true
        imageSupported: Config.notifications?.show_images === true

        Component.onCompleted: {
            var l = trackedNotifications.values;
            var a = Object.assign({}, root.arrivalMs);
            for (var i = 0; i < l.length; i++) {
                if (!a[l[i].id])
                    a[l[i].id] = Date.now();
                root.hookClosed(l[i]);
            }
            root.arrivalMs = a;
            root.trimHistory();
        }

        onNotification: function(n) {
            const mirrored = root.isKdeConnectNotification(n)
            if (mirrored && Phone.KdeConnectService.considerTransferNotification(n)) {
                n.tracked = false
                return
            }
            if (mirrored && (!(Config.phone?.desktopNotifications ?? true)
                    || Phone.KdeConnectService.isIgnoredNotification(n))) {
                n.tracked = false
                return
            }
            var a = Object.assign({}, root.arrivalMs);
            a[n.id] = Date.now();
            root.arrivalMs = a;
            if (root.dnd)
                root.markSeen(a[n.id]);
            n.tracked = true;
            root.hookClosed(n);
            if (!root.dnd) {
                root.addPopup(n);
                root.chime(n);
            }
            root.trimHistory();
        }
    }
}
