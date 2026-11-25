pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "look"
import "../../services/lib/markdown.js" as Md

// A pinned Quick Ask answer: Hyakki's seal on the desktop, dragged anywhere, its
// answer one tap away. Dragging it onto Remove unpins it.
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
    readonly property real bubbleSize: Math.round(56 * s)
    readonly property real edge: Math.round(13 * s)
    readonly property bool configuredScreen: Config.askBubble.screen.length > 0
        ? Config.askBubble.screen === String(screen ? screen.name : "")
        : Quickshell.screens.length > 0 && Quickshell.screens[0] === screen
    readonly property bool shown: enabled && configuredScreen && !suppressed
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
        bubble.x = right ? root.screenWidth - bubble.width - root.edge : root.edge;
        bubble.y = Math.max(root.edge, Math.min(root.screenHeight - bubble.height - root.edge, bubble.y));
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

            Shape {
                id: dismissTarget
                x: Math.round((parent.width - width) / 2)
                y: parent.height - height - Math.round(34 * root.s)
                width: Math.round(132 * root.s)
                height: Math.round(52 * root.s)
                s: root.s
                cutSize: Look.cutMenu
                fill: Look.popover
                strokeColor: Lithic.jasper
                strokeWidth: 1
                opacity: drag.active ? 1 : 0
                Behavior on opacity {
                    NumberAnimation { duration: Look.dur(Look.stateMs) }
                }
                Row {
                    anchors.centerIn: parent
                    spacing: Math.round(8 * root.s)
                    Symbol { anchors.verticalCenter: parent.verticalCenter; s: root.s; name: "delete"; size: 18; ink: Lithic.jasper }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Remove")
                        color: Lithic.fg
                        font.family: Look.sans
                        font.pixelSize: Math.round(13 * root.s)
                        renderType: Text.NativeRendering
                    }
                }
            }

            Item {
                id: bubble
                width: root.bubbleSize
                height: root.bubbleSize
                z: 3

                RectangularShadow {
                    anchors.fill: disc
                    offset.y: Math.round(4 * root.s)
                    blur: Math.round(16 * root.s)
                    radius: Look.cornerAt(LithicTokens.cut.large, disc.width, disc.height, root.s)
                    color: Look.shadow
                }
                Shape {
                    id: disc
                    anchors.fill: parent
                    s: root.s
                    cutSize: LithicTokens.cut.large
                    fill: Lithic.sel
                }
                AskOrb {
                    anchors.centerIn: parent
                    s: root.s
                    orbSize: root.bubbleSize - Math.round(13 * root.s)
                    active: root.shown && bubbleHover.hovered
                    preset: "avatar"
                    ink: Lithic.onSel
                }
                HoverHandler { id: bubbleHover; cursorShape: drag.active ? Qt.ClosedHandCursor : Qt.PointingHandCursor }
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

            Item {
                id: answerCard
                visible: root.shown && root.cardOpen
                x: bubble.x < win.width / 2
                    ? Math.min(win.width - width - root.edge, bubble.x + bubble.width + root.edge)
                    : Math.max(root.edge, bubble.x - width - root.edge)
                y: Math.max(root.edge, Math.min(win.height - height - root.edge, bubble.y - height / 2 + bubble.height / 2))
                width: Math.min(Math.round(420 * root.s), win.width - Math.round(68 * root.s))
                height: Math.min(answerColumn.implicitHeight + Math.round(42 * root.s), win.height * 0.46)
                z: 2

                RectangularShadow {
                    anchors.fill: parent
                    offset.y: Math.round(8 * root.s)
                    blur: Math.round(28 * root.s)
                    radius: Look.lithic ? 0 : Look.cornerAt(Look.cutFrame, answerCard.width, answerCard.height, root.s)
                    color: Look.shadow
                }
                Shape {
                    anchors.fill: parent
                    s: root.s
                    cutSize: Look.cutFrame
                    cutTopRight: Look.cutFrameQuiet * root.s
                    cutBottomLeft: Look.cutFrameQuiet * root.s
                    fill: Look.frame
                    strokeColor: Look.frameStroke
                    strokeWidth: Qt.colorEqual(Look.frameStroke, "transparent") ? 0 : 1
                }
                Flickable {
                    anchors.fill: parent
                    anchors.margins: Math.round(21 * root.s)
                    contentHeight: answerColumn.implicitHeight
                    boundsBehavior: Flickable.StopAtBounds
                    clip: true
                    Column {
                        id: answerColumn
                        width: parent.width
                        spacing: Math.round(8 * root.s)
                        Text {
                            width: parent.width
                            visible: text.length > 0
                            text: root.question
                            color: Lithic.fg
                            font.family: Look.sans
                            font.pixelSize: Math.round(14 * root.s)
                            font.weight: Font.Medium
                            wrapMode: Text.WordWrap
                            renderType: Text.NativeRendering
                        }
                        TextEdit {
                            width: parent.width
                            text: Md.render(root.answer.length > 0 ? root.answer : I18n.tr("No pinned answer"), Look.prose(root.s, 13, Look.frame))
                            textFormat: TextEdit.RichText
                            color: Lithic.fg2
                            selectionColor: Lithic.sel
                            selectedTextColor: Lithic.onSel
                            font.family: Look.sans
                            font.pixelSize: Math.round(13 * root.s)
                            wrapMode: TextEdit.Wrap
                            readOnly: true
                            selectByMouse: true
                            onLinkActivated: url => {
                                const safe = Md.safeUrl(url);
                                if (safe.length > 0)
                                    Spawn.runApp(["xdg-open", safe], { label: "browser" });
                            }
                        }
                    }
                }
            }
        }
    }
}
