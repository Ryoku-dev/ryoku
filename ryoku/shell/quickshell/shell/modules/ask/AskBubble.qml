pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import shell.services

Scope {
    id: root

    required property var screen
    property bool enabled: Config.askBubble.enabled
    property bool suppressed: false
    property string question: ""
    property string answer: ""
    property bool cardOpen: false

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real bubbleSize: 56 * s
    readonly property bool configuredScreen: Config.askBubble.screen.length > 0
        ? Config.askBubble.screen === String(screen ? screen.name : "")
        : Quickshell.screens.length > 0 && Quickshell.screens[0] === screen
    readonly property bool shown: enabled && configuredScreen && !suppressed
    readonly property bool motionAllowed: shown && !Tokens.reduceMotion && !Motion.reduce
    property bool persisting: false

    onSuppressedChanged: if (suppressed) cardOpen = false

    function restorePosition() {
        const maxX = Math.max(0, root.screenWidth - root.bubbleSize);
        const maxY = Math.max(0, root.screenHeight - root.bubbleSize);
        bubble.x = Math.round(maxX * Config.askBubble.x);
        bubble.y = Math.round(maxY * Config.askBubble.y);
    }

    function persistPosition() {
        const maxX = Math.max(1, root.screenWidth - root.bubbleSize);
        const maxY = Math.max(1, root.screenHeight - root.bubbleSize);
        root.persisting = true;
        Config.patchAskBubble("x", Math.max(0, Math.min(1, bubble.x / maxX)));
        Config.patchAskBubble("y", Math.max(0, Math.min(1, bubble.y / maxY)));
        Config.patchAskBubble("screen", String(screen ? screen.name : ""));
        root.persisting = false;
    }

    function finishDrag() {
        const centerX = bubble.x + bubble.width / 2;
        const centerY = bubble.y + bubble.height / 2;
        if (centerX >= dismissTarget.x && centerX <= dismissTarget.x + dismissTarget.width
                && centerY >= dismissTarget.y && centerY <= dismissTarget.y + dismissTarget.height) {
            cardOpen = false;
            Config.patchAskBubble("enabled", false);
            return;
        }
        const right = bubble.x + bubble.width / 2 >= root.screenWidth / 2;
        bubble.x = right ? root.screenWidth - bubble.width - Tokens.s3 * root.s : Tokens.s3 * root.s;
        bubble.y = Math.max(Tokens.s3 * root.s, Math.min(root.screenHeight - bubble.height - Tokens.s3 * root.s, bubble.y));
        persistPosition();
    }

    Component.onCompleted: restorePosition()
    onScreenChanged: Qt.callLater(restorePosition)

    Connections {
        target: Config
        function onAskBubbleChanged() {
            if (!drag.active && !root.persisting)
                root.restorePosition();
        }
    }

    PanelWindow {
        id: win
        screen: root.screen
        visible: root.shown
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-ask-bubble"
        WlrLayershell.keyboardFocus: root.cardOpen ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
        mask: Region {
            x: drag.active ? 0 : bubble.x
            y: drag.active ? 0 : bubble.y
            width: drag.active ? win.width : bubble.width
            height: drag.active ? win.height : bubble.height
            Region { item: root.cardOpen ? answerCard : null; intersection: Intersection.Combine }
        }

        FocusScope {
            anchors.fill: parent
            focus: root.cardOpen
            Keys.onEscapePressed: root.cardOpen = false

            Rectangle {
                id: dismissTarget
                x: (parent.width - width) / 2
                y: parent.height - height - Tokens.s5 * root.s
                width: 132 * root.s
                height: 54 * root.s
                radius: height / 2
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.lineStrong
                opacity: drag.active ? 1 : 0
                Row {
                    anchors.centerIn: parent
                    spacing: Tokens.s2 * root.s
                    Text { text: "delete"; color: Tokens.inkMuted; font.family: "Material Symbols Rounded"; font.pixelSize: Tokens.fBody * root.s }
                    Text { text: I18n.tr("Remove"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s }
                }
            }

            Item {
                id: bubble
                width: root.bubbleSize
                height: root.bubbleSize
                z: 3
                RectangularShadow { anchors.fill: disc; radius: disc.radius; blur: Tokens.s3 * root.s; spread: Tokens.border; color: Qt.alpha(Tokens.paper, 0.52) }
                Rectangle { id: disc; anchors.fill: parent; radius: width / 2; color: Tokens.bone; border.width: Tokens.border; border.color: Tokens.line }
                AskOrb { anchors.centerIn: parent; s: root.s; orbSize: root.bubbleSize - Tokens.s3 * root.s; active: root.shown; mode: "ask"; preset: "avatar"; ink: Tokens.inkOnBone }
                HoverHandler { cursorShape: drag.active ? Qt.ClosedHandCursor : Qt.PointingHandCursor }
                TapHandler { onTapped: root.cardOpen = !root.cardOpen }
                DragHandler {
                    id: drag
                    target: bubble
                    xAxis.minimum: 0
                    xAxis.maximum: Math.max(0, win.width - bubble.width)
                    yAxis.minimum: 0
                    yAxis.maximum: Math.max(0, win.height - bubble.height)
                    onActiveChanged: {
                        if (active) root.cardOpen = false;
                        else root.finishDrag();
                    }
                }
            }

            Rectangle {
                id: answerCard
                visible: root.shown && root.cardOpen
                x: bubble.x < win.width / 2
                    ? Math.min(win.width - width - Tokens.s3 * root.s, bubble.x + bubble.width + Tokens.s3 * root.s)
                    : Math.max(Tokens.s3 * root.s, bubble.x - width - Tokens.s3 * root.s)
                y: Math.max(Tokens.s3 * root.s, Math.min(win.height - height - Tokens.s3 * root.s, bubble.y - height / 2 + bubble.height / 2))
                width: Math.min(420 * root.s, win.width - Tokens.s5 * root.s * 2)
                implicitHeight: answerColumn.implicitHeight + Tokens.s4 * root.s * 2
                height: Math.min(implicitHeight, win.height * 0.46)
                radius: Tokens.radius * root.s
                color: Tokens.paper
                border.width: Tokens.border
                border.color: Tokens.lineStrong
                z: 2
                clip: true
                Flickable {
                    anchors.fill: parent
                    anchors.margins: Tokens.s4 * root.s
                    contentHeight: answerColumn.implicitHeight
                    boundsBehavior: Flickable.StopAtBounds
                    Column {
                        id: answerColumn
                        width: parent.width
                        spacing: Tokens.s2 * root.s
                        Text { width: parent.width; visible: text.length > 0; text: root.question; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s; font.weight: Font.DemiBold; wrapMode: Text.WordWrap }
                        TextEdit { width: parent.width; text: root.answer.length > 0 ? root.answer : I18n.tr("No pinned answer"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s; wrapMode: TextEdit.Wrap; readOnly: true; selectByMouse: true }
                    }
                }
            }
        }
    }
}
