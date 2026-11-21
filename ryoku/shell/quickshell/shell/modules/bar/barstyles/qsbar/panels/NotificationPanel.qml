import QtQuick
import "../modules"
import Quickshell
import Quickshell.Wayland
import Quickshell.Io
import Quickshell.Services.Notifications
import Ryoku.Ui.Singletons

PanelWindow {
    id: notifPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-notifications"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    // ── native quickshell notification server ──────────────────────────────
    // Quickshell becomes its own org.freedesktop.Notifications D-Bus provider
    NotificationServer {
        id: notifServer
        keepOnReload: true       // survive quickshell hot-reloads
        bodySupported: true
        // This legacy QS Bar store keeps text only; the resident server owns
        // opt-in image notifications.
        imageSupported: false
        actionsSupported: true

        onNotification: function(notification) {
            notification.tracked = true   // keep it alive / don't auto-discard

            var key = "n:" + notification.id + ":" + (++notifPanel.seq)

            // keep a transient (non-persisted) live reference so dismiss/invoke
            // can still reach the real Notification object while it's alive
            var lm = notifPanel.liveMap
            lm[key] = notification
            notifPanel.liveMap = lm

            var entry = {
                key: key,
                appName: notification.appName || "",
                summary: notification.summary || "",
                body: notification.body || "",
                firstSeen: notifPanel.seq
            }

            var out = [entry].concat(notifPanel.recent)
            if (out.length > 50) out = out.slice(0, 50)
            notifPanel.recent = out            // reassign → bindings fire
            notifPanel.pruneDismissed()

            // when the notification closes (app withdrew it, expired, or we
            // dismissed it) drop the live ref; the history entry stays put
            notification.closed.connect(function(reason) {
                var lm2 = notifPanel.liveMap
                delete lm2[key]
                notifPanel.liveMap = lm2
            })

            notifPanel.saveCache()
        }
    }

    property var recent: []             // [{key,appName,summary,body,firstSeen}]
    property var dismissed: ({})         // key -> true (persisted)
    property var liveMap: ({})           // key -> live Notification (transient, not persisted)
    property int seq: 0                  // monotonic first-seen counter (ordering)
    property bool cacheLoaded: false
    property string lastSaved: ""

    // pending = not dismissed → drives both the list and the badge
    readonly property var pending: {
        var out = []
        for (var i = 0; i < recent.length; i++)
            if (!dismissed[recent[i].key]) out.push(recent[i])
        return out
    }
    readonly property int unreadCount: pending.length
    // scrollable list height cap, clamped to the monitor
    readonly property int listCap: Math.max(120, Math.min(420, notifPanel.height - 220))

    Binding { target: root; property: "notifCount"; value: notifPanel.unreadCount }

    // ── persistent cache (quickshell is the sole writer; write only on change) ──
    readonly property string cachePath: Quickshell.env("HOME") + "/.cache/qs-rise-notifications.json"
    FileView {
        id: cacheFile
        path: notifPanel.cachePath
        onLoaded: {
            try {
                var j = JSON.parse(cacheFile.text())
                notifPanel.seq       = j.seq || 0
                notifPanel.recent    = Array.isArray(j.recent) ? j.recent : []
                notifPanel.dismissed = (j.dismissed && typeof j.dismissed === "object") ? j.dismissed : ({})
                notifPanel.lastSaved = cacheFile.text()
            } catch (e) {
                notifPanel.recent = []; notifPanel.dismissed = ({})
            }
            notifPanel.cacheLoaded = true
        }
        onLoadFailed: {                  // first run: no cache yet
            notifPanel.cacheLoaded = true
        }
    }
    // force the initial load (don't rely on implicit auto-load)
    Component.onCompleted: cacheFile.reload()

    function saveCache() {
        if (!notifPanel.cacheLoaded) return
        var state = JSON.stringify({
            seq: notifPanel.seq,
            recent: notifPanel.recent,
            dismissed: notifPanel.dismissed
        })
        if (state === notifPanel.lastSaved) return   // no real change → no write
        notifPanel.lastSaved = state
        cacheFile.setText(state)
    }

    // prune dismissed keys that fell off the (max 50) retained history
    function pruneDismissed() {
        var present = {}
        for (var i = 0; i < notifPanel.recent.length; i++) present[notifPanel.recent[i].key] = true
        var nd = {}, changed = false
        for (var k in notifPanel.dismissed) {
            if (present[k]) nd[k] = true; else changed = true
        }
        if (changed) notifPanel.dismissed = nd
    }

    // ── actions ──
    function dismissOne(entry) {
        var nd = {}
        for (var k in notifPanel.dismissed) nd[k] = true
        nd[entry.key] = true
        notifPanel.dismissed = nd                // reassign → bindings update

        var live = notifPanel.liveMap[entry.key]
        if (live) {
            live.tracked = false                 // == live.dismiss()
            var lm = notifPanel.liveMap
            delete lm[entry.key]
            notifPanel.liveMap = lm
        }
        notifPanel.saveCache()
    }

    function dismissAll() {
        var nd = {}
        for (var k in notifPanel.dismissed) nd[k] = true
        for (var i = 0; i < notifPanel.recent.length; i++) nd[notifPanel.recent[i].key] = true
        notifPanel.dismissed = nd

        for (var k2 in notifPanel.liveMap) notifPanel.liveMap[k2].tracked = false
        notifPanel.liveMap = ({})

        notifPanel.recent = []                   // clear own history; re-arriving
                                                    // notifications with the same key
                                                    // stay filtered out via `dismissed`
        notifPanel.saveCache()
    }

    property real reveal: root.notifVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: root.notifVisible ? 160 : 120
            easing.type: root.notifVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.notifVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea {
        anchors.fill: parent
        onClicked: root.notifVisible = false
    }

    Rectangle {
        id: card
        width: 320
        height: col.implicitHeight + 24
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: notifPanel.root
            ownerActive: notifPanel.root.notifVisible
            targetX: notifPanel.root.notifCaretBarX
            reveal: notifPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.notifBarX, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - notifPanel.reveal)
            : (barBottom + gap) - 2 * (1 - notifPanel.reveal)
        opacity: notifPanel.reveal
        focus: root.notifVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) {
                root.notifVisible = false
                event.accepted = true
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            // ── header ──
            Item {
                width: parent.width
                height: 24
                UiText {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: notifPanel.unreadCount > 0 ? I18n.tr("Notifications · %1").arg(notifPanel.unreadCount) : I18n.tr("Notifications")
                    color: root.ink
                    font.family: root.mono
                    font.pixelSize: 13
                    font.letterSpacing: 2
                    font.weight: Font.Medium
                }
                UiText {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: "✕"
                    color: closeMa.containsMouse ? root.seal : root.sumi
                    font.pixelSize: 12
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.notifVisible = false
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── notification list (scrollable; each individually dismissable) ──
            Flickable {
                width: parent.width
                height: Math.min(listCol.implicitHeight, notifPanel.listCap)
                contentHeight: listCol.implicitHeight
                clip: true
                interactive: listCol.implicitHeight > notifPanel.listCap
                boundsBehavior: Flickable.StopAtBounds
                flickableDirection: Flickable.VerticalFlick

                Column {
                    id: listCol
                    width: parent.width
                    spacing: 6

                    Repeater {
                        model: notifPanel.visible ? notifPanel.pending : []

                        delegate: Rectangle {
                            required property var modelData
                            // A click on an actionless notification expands
                            // its body instead of dismissing it, so a long
                            // alert can actually be read.
                            property bool expanded: false
                            width: listCol.width
                            height: entryCol.implicitHeight + 16
                            radius: root.panelButtonRadius
                            color: entryMa.containsMouse ? root.fillHover : root.fillIdle
                            border.color: entryMa.containsMouse ? root.seal : root.sep
                            border.width: 1
                            Behavior on color { ColorAnimation { duration: 120 } }

                            Column {
                                id: entryCol
                                anchors { left: parent.left; right: parent.right; top: parent.top }
                                anchors.margins: 8
                                anchors.topMargin: 8
                                anchors.rightMargin: 26
                                spacing: 3

                                UiText {
                                    text: modelData.appName || "App"
                                    color: root.sumiHi
                                    font.family: root.mono
                                    font.pixelSize: 10
                                    font.letterSpacing: 0.5
                                    width: parent.width
                                    elide: Text.ElideRight
                                }
                                UiText {
                                    text: modelData.summary || ""
                                    color: root.ink
                                    font.family: root.mono
                                    font.pixelSize: 11
                                    width: parent.width
                                    elide: Text.ElideRight
                                    visible: text !== ""
                                }
                                UiText {
                                    text: modelData.body || ""
                                    color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.6)
                                    font.family: root.mono
                                    font.pixelSize: 10
                                    width: parent.width
                                    wrapMode: Text.WordWrap
                                    maximumLineCount: expanded ? Infinity : 2
                                    elide: expanded ? Text.ElideNone : Text.ElideRight
                                    visible: text !== ""
                                }
                            }

                            MouseArea {
                                id: entryMa
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    var live = notifPanel.liveMap[modelData.key]
                                    if (live && live.actions && live.actions.length > 0) {
                                        live.actions[0].invoke()
                                        root.notifVisible = false
                                    } else {
                                        // No action to run: expand the body
                                        // so it can be read in full. A
                                        // notification the user was still
                                        // reading should not be dismissed
                                        // by the act of trying to read it.
                                        expanded = !expanded
                                    }
                                }
                            }

                            Rectangle {
                                anchors.top: parent.top; anchors.right: parent.right
                                anchors.topMargin: 4; anchors.rightMargin: 4
                                width: 18; height: 18; radius: 9
                                color: "transparent"
                                UiText {
                                    anchors.centerIn: parent
                                    text: "✕"
                                    color: xMa.containsMouse ? root.seal : Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.45)
                                    font.pixelSize: 10
                                }
                                MouseArea {
                                    id: xMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: notifPanel.dismissOne(modelData)
                                }
                            }
                        }
                    }

                    UiText {
                        visible: notifPanel.pending.length === 0
                        width: listCol.width
                        horizontalAlignment: Text.AlignHCenter
                        text: I18n.tr("No notifications")
                        color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.3)
                        font.family: root.mono
                        font.pixelSize: 11
                    }
                }
            }

            // ── clear all ──
            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                visible: notifPanel.pending.length > 0
                readonly property bool hovered: clearMa.containsMouse
                color: hovered ? root.fillHover : root.fillIdle
                border.color: hovered ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.centerIn: parent
                    text: I18n.tr("Clear all")
                    color: clearMa.containsMouse ? root.seal : root.sumi
                    font.family: root.mono; font.pixelSize: 11
                }
                MouseArea {
                    id: clearMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: notifPanel.dismissAll()
                }
            }
        }
    }
}
