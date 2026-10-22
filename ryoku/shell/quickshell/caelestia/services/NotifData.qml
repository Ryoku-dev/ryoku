pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Services.Notifications
import Caelestia
import Caelestia.Config
import caelestia.services
import caelestia.utils

QtObject {
    id: notif

    property bool popup
    property bool closed
    property var locks: new Set()

    property date time: new Date()
    property string timeStr: qsTr("now")

    readonly property Timer timeStrTimer: Timer {
        running: !notif.closed
        repeat: true
        interval: 5000
        onTriggered: notif.updateTimeStr()
    }

    property Notification notification
    property string id
    property string summary
    property string body
    property string appIcon
    // A Material Symbols name to fall back on when there is no icon theme entry to
    // resolve through appIcon, as is the case for shell-raised notifications.
    property string materialIcon
    property string appName
    property string image
    // Body with markup stripped, for single-line previews (eliding raw markup cuts tags in half)
    readonly property string bodyPlain: body.replace(/<[^>]*>/g, "").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, "\"").replace(/&#39;|&apos;/g, "'").replace(/&amp;/g, "&").trim()
    // Summary is always shown on a single line, so collapse any whitespace too
    readonly property string summaryPlain: summary.replace(/<[^>]*>/g, "").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, "\"").replace(/&#39;|&apos;/g, "'").replace(/&amp;/g, "&").replace(/\s+/g, " ").trim()
    property var hints
    property real expireTimeout: GlobalConfig.notifs.defaultExpireTimeout
    property int urgency: NotificationUrgency.Normal
    property bool resident
    property bool hasActionIcons
    // A notification the shell raised itself. Kept out of the on-disk history:
    // its actions hold live callbacks that would not survive a restart.
    property bool shellRaised
    property list<var> actions

    readonly property bool hasFullscreen: Kwin.hasFullscreen()

    readonly property Timer timer: Timer {
        running: true
        interval: notif.expireTimeout > 0 ? notif.expireTimeout : notif.hasFullscreen ? GlobalConfig.notifs.fullscreenExpireTimeout : GlobalConfig.notifs.defaultExpireTimeout
        onTriggered: {
            if (GlobalConfig.notifs.expire || notif.hasFullscreen)
                notif.popup = false;
        }
    }

    readonly property LazyLoader dummyImageLoader: LazyLoader {
        active: false

        // qmllint disable uncreatable-type
        PanelWindow {
            // qmllint enable uncreatable-type
            implicitWidth: TokenConfig.sizes.notifs.image
            implicitHeight: TokenConfig.sizes.notifs.image
            color: "transparent"
            mask: Region {}

            Image {
                function tryCache(): void {
                    if (status !== Image.Ready || width != TokenConfig.sizes.notifs.image || height != TokenConfig.sizes.notifs.image)
                        return;

                    const cacheKey = notif.appName + notif.summary + notif.id + notif.image;
                    let h1 = 0xdeadbeef, h2 = 0x41c6ce57, ch;
                    for (let i = 0; i < cacheKey.length; i++) {
                        ch = cacheKey.charCodeAt(i);
                        h1 = Math.imul(h1 ^ ch, 2654435761);
                        h2 = Math.imul(h2 ^ ch, 1597334677);
                    }
                    h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507);
                    h1 ^= Math.imul(h2 ^ (h2 >>> 13), 3266489909);
                    h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507);
                    h2 ^= Math.imul(h1 ^ (h1 >>> 13), 3266489909);
                    const hash = (h2 >>> 0).toString(16).padStart(8, 0) + (h1 >>> 0).toString(16).padStart(8, 0);

                    const cache = `${Paths.notifimagecache}/${hash}.png`;
                    CUtils.saveItem(this, Qt.resolvedUrl(cache), () => {
                        notif.image = cache;
                        notif.dummyImageLoader.active = false;
                    });
                }

                anchors.fill: parent
                source: Qt.resolvedUrl(notif.image)
                fillMode: Image.PreserveAspectCrop
                cache: false
                asynchronous: true
                opacity: 0

                onStatusChanged: tryCache()
                onWidthChanged: tryCache()
                onHeightChanged: tryCache()
            }
        }
    }

    readonly property Connections conn: Connections {
        function onClosed(): void {
            if (!notif.closed)
                notif.close();
        }

        function onSummaryChanged(): void {
            notif.summary = notif.notification.summary;
        }

        function onBodyChanged(): void {
            notif.body = notif.notification.body;
        }

        function onAppIconChanged(): void {
            notif.appIcon = notif.notification.appIcon;
        }

        function onAppNameChanged(): void {
            notif.appName = notif.notification.appName;
        }

        function onImageChanged(): void {
            notif.image = notif.notification.image;
            notif.maybeTriggerDummyImageLoader();
        }

        function onExpireTimeoutChanged(): void {
            notif.expireTimeout = notif.notification.expireTimeout;
        }

        function onUrgencyChanged(): void {
            notif.urgency = notif.notification.urgency;
        }

        function onResidentChanged(): void {
            notif.resident = notif.notification.resident;
        }

        function onHasActionIconsChanged(): void {
            notif.hasActionIcons = notif.notification.hasActionIcons;
        }

        function onActionsChanged(): void {
            // qmllint disable unresolved-type
            notif.actions = notif.notification.actions.map(a => ({
                        // qmllint enable unresolved-type
                        identifier: a.identifier,
                        text: a.text,
                        invoke: () => a.invoke()
                    }));
        }

        function onHintsChanged(): void {
            notif.hints = notif.notification.hints;
        }

        target: notif.notification
    }

    function updateTimeStr(): void {
        const diff = Date.now() - time.getTime();
        const m = Math.floor(diff / 60000);

        if (m < 1) {
            timeStr = qsTr("now");
            timeStrTimer.interval = 5000;
        } else {
            const h = Math.floor(m / 60);
            const d = Math.floor(h / 24);

            if (d > 0) {
                timeStr = `${d}d`;
                timeStrTimer.interval = 3600000;
            } else if (h > 0) {
                timeStr = `${h}h`;
                timeStrTimer.interval = 300000;
            } else {
                timeStr = `${m}m`;
                timeStrTimer.interval = m < 10 ? 30000 : 60000;
            }
        }
    }

    function maybeTriggerDummyImageLoader(): void {
        // Absolute image-path hints (e.g. Chromium) arrive as image://icon//abs/path, which renders as a missing icon
        if (image.startsWith("image://icon//"))
            image = "file://" + image.slice("image://icon/".length);
        if (image && !image.startsWith("image://icon/") && !image.startsWith(Paths.notifimagecache))
            dummyImageLoader.active = true;
    }

    function lock(item: Item): void {
        locks.add(item);
    }

    function unlock(item: Item): void {
        locks.delete(item);
        if (closed)
            close();
    }

    function close(): void {
        if (closed && !notification)
            return;
        closed = true;
        if (locks.size > 0)
            return;

        if (Notifs.list.includes(this))
            Notifs.list = Notifs.list.filter(n => n !== this);
        const notifObj = notification;
        notification = null;
        notifObj?.dismiss();
        destroy();
    }

    Component.onCompleted: {
        if (!notification)
            return;

        id = notification.id;
        summary = notification.summary;
        body = notification.body;
        appIcon = notification.appIcon;
        appName = notification.appName;
        image = notification.image;
        maybeTriggerDummyImageLoader();
        expireTimeout = notification.expireTimeout;
        hints = notification.hints;
        urgency = notification.urgency;
        resident = notification.resident;
        hasActionIcons = notification.hasActionIcons;
        // qmllint disable unresolved-type
        actions = notification.actions.map(a => ({
                    // qmllint enable unresolved-type
                    identifier: a.identifier,
                    text: a.text,
                    invoke: () => a.invoke()
                }));
    }
}
