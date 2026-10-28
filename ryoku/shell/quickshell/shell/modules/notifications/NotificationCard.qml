pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import "../../components"

// One notification card, shared by the history panel and the popup surface
// (contract 07 sec 2.3). A bordered surface tile with a header (app name, time,
// and an open/expand/close cluster), a bold summary, an optional body, and one
// primary button per action. The reference text-only look remains the default;
// the opt-in image setting adds a token-sized thumbnail row when an image,
// application icon, or fallback glyph resolves.
//
// Popups pass `compact: true`: the body clamps to two lines and the actions
// hide, so the toast stays a tidy glance; the expand chevron reveals the full
// body and its action buttons. The open button surfaces the freedesktop
// "default" action, which is otherwise unreachable from a toast. The history
// panel leaves `compact` false, so it is unchanged bar the open button appearing
// when an app sent a default action.
Rectangle {
    id: card

    required property var notif
    // The delegate outlives its model entry: a dismissed toast animates out while
    // the service has already dropped it, so every read goes through this instead
    // of dereferencing a null. It is a function, not a property: a property binds
    // once, and the object dying emits no change, so the cached reference stayed
    // dead and every read off it threw.
    function n() { return card.notif || ({}) }
    // Fired after an action runs; the panel closes the menu on it, the popup
    // ignores it (contract 07 sec 4.3).
    signal actionInvoked()

    // Popups start compact and expand on demand; the history panel is always
    // full (compact stays false).
    property bool compact: false
    property bool expanded: false
    property bool unifiedFrame: false

    // Per-monitor UI scale, threaded from the popup surface (default 1 = no-op,
    // so the history panel that composes this card unchanged stays identical).
    property real us: 1
    // Only explicit expand/collapse clicks ease the frame height. Image loads
    // and every other layout change snap to their measured height so the popup
    // surface never trails content that has already moved.
    readonly property real targetImplicitHeight: body.implicitHeight + Theme.paddingMd * 2 * card.us
    property real animatedImplicitHeight: 0
    property bool userHeightChange: false
    Component.onCompleted: card.animatedImplicitHeight = card.targetImplicitHeight
    function toggleExpanded() {
        card.userHeightChange = true;
        card.expanded = !card.expanded;
        heightAnimationStart.restart();
    }
    function animateUserHeightChange() {
        heightAnimation.stop();
        const targetHeight = card.targetImplicitHeight;
        heightAnimation.from = card.animatedImplicitHeight;
        heightAnimation.to = targetHeight;
        heightAnimation.start();
        card.userHeightChange = false;
    }
    onTargetImplicitHeightChanged: {
        if (card.userHeightChange) {
            heightAnimationStart.restart();
            return;
        }
        heightAnimation.stop();
        card.animatedImplicitHeight = card.targetImplicitHeight;
    }
    Timer {
        id: heightAnimationStart
        interval: 16
        onTriggered: card.animateUserHeightChange()
    }
    NumberAnimation {
        id: heightAnimation
        target: card
        property: "animatedImplicitHeight"
        duration: Motion.rowReveal
        easing.type: Motion.rowRevealCurve
    }

    // Only real actions get a button: the freedesktop "default" action (surfaced
    // as the open button, not a row button) and any action with no label are
    // dropped, so a bare default no longer draws an empty pill.
    readonly property var visibleActions: {
        const all = card.n().actions || [];
        const out = [];
        for (let i = 0; i < all.length; i++)
            if (all[i] && all[i].identifier !== "default" && (all[i].text || "").length > 0)
                out.push(all[i]);
        return out;
    }

    // The freedesktop default action ("click the notification to open"): surfaced
    // as the open button instead of a click target. null when the app sent none.
    readonly property var defaultAction: {
        const all = card.n().actions || [];
        for (let i = 0; i < all.length; i++)
            if (all[i] && all[i].identifier === "default")
                return all[i];
        return null;
    }

    // Compact clamps the body to two lines; the chevron shows only when expand
    // actually reveals something (a longer body or any action). `bodyOverflows`
    // is measured off a hidden unclamped copy so it stays true once expanded.
    readonly property int compactLines: 2
    readonly property bool showFull: !card.compact || card.expanded
    readonly property bool bodyOverflows: bodyMeasure.lineCount > card.compactLines
    readonly property bool expandable: card.compact && (card.visibleActions.length > 0 || card.bodyOverflows)

    readonly property bool imagesEnabled: Config.notifications?.show_images === true
    function imageSource(value, themed) {
        let raw = String(value || "");
        if (raw.length === 0)
            return "";
        if (raw.indexOf("image://icon/") === 0) {
            raw = decodeURIComponent(raw.substring(13));
            themed = true;
        }
        if (raw.charAt(0) === "/" || raw.indexOf("file:") === 0)
            return raw.indexOf("file:") === 0 ? raw : "file://" + raw;
        if (raw.indexOf(":") > 0)
            return raw;
        return themed ? Icons.path(raw, true) : raw;
    }
    readonly property string notificationImageSource: card.imagesEnabled
        ? card.imageSource(card.n().image, false) : ""
    readonly property string appIconSource: card.imagesEnabled
        ? card.imageSource(card.n().appIcon, true) : ""
    readonly property bool hasNotificationImage: card.notificationImageSource.length > 0

    // Countdown frame (popups only): a border that traces the card and drains
    // over the popup's lifespan, so a glance shows how long is left. The history
    // panel and persistent popups pass 0 and draw no frame.
    property int lifespanMs: 0
    readonly property bool countingDown: card.lifespanMs > 0 && !card.unifiedFrame
    property real remaining: 1
    NumberAnimation on remaining {
        running: card.countingDown
        from: 1
        to: 0
        duration: Math.max(1, card.lifespanMs)
        easing.type: Easing.Linear
    }

    radius: card.unifiedFrame ? 0 : Theme.radiusWidget * card.us
    border.width: card.unifiedFrame ? 0 : Theme.borderWidth * card.us
    border.color: Theme.outline
    color: card.unifiedFrame ? "transparent" : Theme.surface
    implicitHeight: card.animatedImplicitHeight

    // A hidden, unclamped copy of the body, used only to learn whether the real
    // body would overflow the compact clamp (so the chevron appears only when it
    // has something to show). Never painted.
    Text {
        id: bodyMeasure
        visible: false
        width: card.width - Theme.paddingMd * 2 * card.us
        text: card.n().body || ""
        font.family: Theme.fontPrimary
        font.pixelSize: Theme.fontSm * card.us
        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
    }

    // One header affordance: a hover-tinted square icon button. Open, expand and
    // close share it so the cluster reads as one control set.
    component HeaderButton: Rectangle {
        id: hb
        property string glyph: ""
        property color activeColor: Theme.onSurface
        property real iconRotation: 0
        signal clicked()

        width: (Theme.iconSm + Theme.paddingSm * 2) * card.us
        height: width
        radius: Theme.radiusWidget * card.us
        color: hbHov.hovered
            ? Qt.tint(Theme.surface, Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08))
            : "transparent"

        MaterialIcon {
            anchors.centerIn: parent
            text: hb.glyph
            font.pixelSize: Theme.iconSm * card.us
            color: hbHov.hovered ? hb.activeColor : Theme.onSurfaceVariant
            rotation: hb.iconRotation
            Behavior on rotation { NumberAnimation { duration: Motion.chevronRotate; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }
        }

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: hb.clicked()
            HoverHandler { id: hbHov }
        }
    }

    Column {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Theme.paddingMd * card.us
        spacing: Theme.paddingMd * card.us

        // Header: app name (fills), arrival time, and the open/expand/close cluster.
        Item {
            width: parent.width
            height: Math.max(appName.implicitHeight, timeLabel.implicitHeight, btnRow.height)

            Text {
                id: appName
                anchors.left: parent.left
                anchors.right: timeLabel.left
                anchors.rightMargin: Theme.paddingSm * card.us
                anchors.verticalCenter: parent.verticalCenter
                text: card.n().appName || ""
                color: Theme.onSurfaceVariant
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm * card.us
                font.weight: Font.Medium
                elide: Text.ElideRight
            }

            Text {
                id: timeLabel
                anchors.right: btnRow.left
                anchors.rightMargin: Theme.paddingSm * card.us
                anchors.verticalCenter: parent.verticalCenter
                text: Notifs.timeLabel(card.notif)
                color: Theme.onSurfaceVariant
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm * card.us
            }

            Row {
                id: btnRow
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                // Open: invoke the freedesktop default action; shown only when the
                // app sent one.
                HeaderButton {
                    glyph: "open_in_new"
                    visible: card.defaultAction !== null
                    onClicked: {
                        if (card.defaultAction)
                            card.defaultAction.invoke();
                        card.actionInvoked();
                    }
                }

                // Expand: reveal the full body and the action buttons; the chevron
                // turns over as it opens.
                HeaderButton {
                    glyph: "expand_more"
                    visible: card.expandable
                    iconRotation: card.expanded ? 180 : 0
                    onClicked: card.toggleExpanded()
                }

                HeaderButton {
                    glyph: "close"
                    onClicked: Notifs.dismiss(card.notif)
                }
            }
        }

        Rectangle {
            id: thumbnail
            readonly property bool imageReady: notificationImage.status === Image.Ready
            readonly property bool iconReady: appIconImage.status === Image.Ready
            readonly property bool imageUnavailable: !card.hasNotificationImage
                || notificationImage.status === Image.Error
            readonly property bool iconUnavailable: card.appIconSource.length === 0
                || appIconImage.status === Image.Error
            readonly property bool useImage: card.hasNotificationImage && thumbnail.imageReady
            readonly property bool useIcon: thumbnail.imageUnavailable && thumbnail.iconReady
            readonly property bool useGlyph: thumbnail.imageUnavailable && thumbnail.iconUnavailable

            visible: card.imagesEnabled && (thumbnail.useImage || thumbnail.useIcon || thumbnail.useGlyph)
            width: (Theme.iconLg + Theme.paddingLg) * card.us
            height: width
            radius: Theme.radiusWidget * card.us
            color: "transparent"
            clip: true

            Image {
                id: notificationImage
                anchors.fill: parent
                source: card.notificationImageSource
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                smooth: true
                visible: thumbnail.useImage
            }

            Image {
                id: appIconImage
                anchors.fill: parent
                anchors.margins: Theme.paddingSm * card.us
                source: card.appIconSource
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: true
                visible: thumbnail.useIcon
            }

            MaterialIcon {
                anchors.centerIn: parent
                visible: thumbnail.useGlyph
                text: card.n().urgency === 2 ? "priority_high" : "notifications"
                font.pixelSize: Theme.iconLg * card.us
                color: Theme.onSurfaceVariant
            }
        }

        // Summary: bold, wraps.
        Text {
            width: parent.width
            text: card.n().summary || ""
            color: Theme.onSurface
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontMd * card.us
            font.weight: Font.Bold
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        // Body: present when non-empty; clamped to two lines while a popup is
        // compact and collapsed, full once expanded (or in the history panel).
        Text {
            width: parent.width
            visible: (card.n().body || "").length > 0
            text: card.n().body || ""
            color: Theme.onSurfaceVariant
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontSm * card.us
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            maximumLineCount: card.showFull ? 9999 : card.compactLines
            elide: card.showFull ? Text.ElideNone : Text.ElideRight
        }

        // Actions: one primary button per action, full width (contract 07 sec
        // 2.3). Hidden while a popup is compact and collapsed; the expand
        // chevron brings them in with a short fade.
        Column {
            id: actionCol
            width: parent.width
            spacing: Theme.paddingSm * card.us
            visible: card.visibleActions.length > 0 && card.showFull
            opacity: card.showFull ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }

            Repeater {
                model: card.visibleActions

                delegate: Rectangle {
                    id: actionBtn
                    required property var modelData

                    // the column by id, not `parent`: a Repeater delegate has no
                    // parent while it is being created, so a notification that
                    // carries actions logged a TypeError for every button.
                    width: actionCol.width
                    height: actionLabel.implicitHeight + Theme.paddingSm * 2 * card.us
                    radius: Theme.radiusWidget * card.us
                    color: actionHov.hovered ? Theme.vermLit : Theme.primary

                    Behavior on color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }

                    Text {
                        id: actionLabel
                        anchors.centerIn: parent
                        width: parent.width - Theme.paddingMd * 2 * card.us
                        horizontalAlignment: Text.AlignHCenter
                        text: actionBtn.modelData.text
                        color: Theme.onPrimary
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * card.us
                        font.weight: Font.Bold
                        elide: Text.ElideRight
                    }

                    MouseArea {
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            actionBtn.modelData.invoke();
                            card.actionInvoked();
                        }
                        HoverHandler { id: actionHov }
                    }
                }
            }
        }
    }

    // The draining countdown, stroked over the card's own border: the accent
    // traces the whole rounded rect at full life and recedes clockwise from the
    // top as the lifespan runs out, uncovering the plain outline underneath.
    Canvas {
        id: timerFrame
        anchors.fill: parent
        visible: card.countingDown
        renderStrategy: Canvas.Cooperative
        onPaint: {
            const ctx = getContext("2d");
            ctx.reset();
            if (!card.countingDown)
                return;
            const sw = Theme.borderWidth * card.us + 1;
            const o = sw / 2;
            const w = width - sw;
            const h = height - sw;
            const rad = Math.max(0, Math.min(card.radius, w / 2, h / 2));
            const perim = 2 * (w - 2 * rad) + 2 * (h - 2 * rad) + 2 * Math.PI * rad;
            const drawn = Math.max(0, Math.min(perim, card.remaining * perim));
            ctx.beginPath();
            ctx.moveTo(o + w / 2, o);
            ctx.lineTo(o + w - rad, o);
            ctx.arcTo(o + w, o, o + w, o + rad, rad);
            ctx.lineTo(o + w, o + h - rad);
            ctx.arcTo(o + w, o + h, o + w - rad, o + h, rad);
            ctx.lineTo(o + rad, o + h);
            ctx.arcTo(o, o + h, o, o + h - rad, rad);
            ctx.lineTo(o, o + rad);
            ctx.arcTo(o, o, o + rad, o, rad);
            ctx.lineTo(o + w / 2, o);
            ctx.lineWidth = sw;
            ctx.strokeStyle = Theme.flameGlow;
            ctx.lineCap = "butt";
            ctx.setLineDash([drawn, perim + 1]);
            ctx.stroke();
        }
        Connections {
            target: card
            function onRemainingChanged() { timerFrame.requestPaint(); }
        }
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onVisibleChanged: requestPaint()
    }
}
