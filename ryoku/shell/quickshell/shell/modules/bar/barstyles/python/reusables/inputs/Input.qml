import QtQuick
import QtQuick.Window
import QtQuick.Layouts
import QtQuick.Controls
import Quickshell.Io
import "../"
import "../../"

FocusScope {
    id: root
    implicitWidth: 180
    implicitHeight: 32

    readonly property var currentWindow: Window.window

    property color baseColor: "#313244"
    property color accentColor: "#89b4fa"
    property color textColor: "#cdd6f4"
    property color subTextColor: "#a6adc8"
    property color borderColor: "#45475a"
    property color errorColor: "#f38ba8"
    property color busyColor: "#fab387"

    property real cornerRadius: 12
    property real horizontalPadding: 8
    property real verticalPadding: 4

    property string iconFont: "Iosevka Nerd Font"
    property string fontFamily: ThemeBackend.fontFamily
    property int fontPixelSize: 12

    property string text: ""
    property string placeholderText: ""
    property int maximumLength: -1
    property var validator: null

    property bool masked: false
    property bool revealTyping: true
    property int revealDuration: 300

    property int horizontalAlignment: TextInput.AlignLeft
    property int charSlotWidth: -1
    property int charSpacing: 1
    property alias symbolSpacing: root.charSpacing
    readonly property real charSlotStep: (root.charSlotWidth > 0 ? root.charSlotWidth : globalCharMetrics.width) + root.charSpacing
    property real scrollOffset: 0

    property var maskPainter: function(ctx, size, color) {
        ctx.beginPath();
        ctx.arc(size / 2, size / 2, size * 0.18, 0, Math.PI * 2);
        ctx.fillStyle = color.toString();
        ctx.fill();
    }

    property string leadingIcon: ""
    property string trailingIcon: ""
    property bool showClearButton: false

    property bool enabled: true
    property bool hasError: false
    property bool isBusy: false
    readonly property bool hasFocus: innerInput.activeFocus
    property bool action_highlight: false
    property bool isHoveredOrHighlighted: mainHover.hovered || root.action_highlight
    property bool isWidgetVisible: true
    property bool showCaret: true

    property bool unfocusOnOutsideClick: true
    property alias loseFocusOnOutsideClick: root.unfocusOnOutsideClick

    readonly property color activeSignalColor: root.hasError ? root.errorColor : (root.isBusy ? root.busyColor : root.accentColor)
    property color caretColor: root.activeSignalColor

    property string keySound: "reusables/input/type.wav"
    property string errorSound: "reusables/inputfield/error.wav"
    property string clickSound: "reusables/button/click.wav"

    property real focusPop: 1.0

    signal textEdited(string newText)
    signal accepted(string finalText)
    signal cleared()
    signal clicked()
    signal triggered()

    Item {
        parent: root.currentWindow ? root.currentWindow.contentItem : null
        width: parent ? parent.width : 0
        height: parent ? parent.height : 0
        visible: root.unfocusOnOutsideClick && root.hasFocus
        enabled: root.unfocusOnOutsideClick

        PointHandler {
            enabled: root.unfocusOnOutsideClick
            acceptedButtons: Qt.AllButtons
            grabPermissions: PointerHandler.TakeOverForbidden
            target: null
            onActiveChanged: {
                if (root.unfocusOnOutsideClick && active && point) {
                    let pt = point.scenePosition || point.position;
                    if (pt) {
                        let pos = root.mapFromItem(null, pt.x, pt.y);
                        if (pos.x < 0 || pos.x > root.width || pos.y < 0 || pos.y > root.height) {
                            innerInput.focus = false;
                            root.focus = false;
                        }
                    }
                }
            }
        }
    }

    function copyToClipboard(str) {
        if (!str || str.length === 0) return;
        copyProc.running = false;
        copyProc.command = ["wl-copy", "--", str];
        copyProc.running = true;
    }

    function clear() {
        if (innerInput.text !== "" && typeof Sounds !== "undefined") {
            Sounds.playSfx(root.keySound);
        }
        innerInput.text = "";
        root.text = "";
        charModel.clear();
        root.scrollOffset = 0;
        root.cleared();
    }

    function forceInputFocus() {
        innerInput.forceActiveFocus();
    }

    function triggerShake() {
        shakeAnim.restart();
        if (typeof Sounds !== "undefined") {
            Sounds.playSfx(root.errorSound);
        }
    }

    function computeRowX(rowWidth, fieldWidth) {
        switch (root.horizontalAlignment) {
            case TextInput.AlignHCenter:
            case Qt.AlignHCenter:
                return (fieldWidth - rowWidth) / 2;
            case TextInput.AlignRight:
            case Qt.AlignRight:
                return fieldWidth - rowWidth;
            default:
                return 0;
        }
    }

    function updateScroll() {
        let totalW = charRow.contentWidth;
        let visibleW = fieldArea.width;

        if (visibleW <= 0) {
            return;
        }

        if (totalW <= visibleW) {
            switch (root.horizontalAlignment) {
                case TextInput.AlignHCenter:
                case Qt.AlignHCenter:
                    root.scrollOffset = (visibleW - totalW) / 2;
                    break;
                case TextInput.AlignRight:
                case Qt.AlignRight:
                    root.scrollOffset = visibleW - totalW;
                    break;
                default:
                    root.scrollOffset = 0;
                    break;
            }
            return;
        }

        let curX = innerInput.cursorPosition * root.charSlotStep;
        let minOffset = visibleW - totalW;
        let maxOffset = 0;
        let margin = 4;
        let curScreenX = curX + root.scrollOffset;

        if (curScreenX < margin) {
            root.scrollOffset = Math.min(maxOffset, Math.max(minOffset, margin - curX));
        } else if (curScreenX > visibleW - margin - 2) {
            root.scrollOffset = Math.min(maxOffset, Math.max(minOffset, visibleW - margin - 2 - curX));
        } else {
            root.scrollOffset = Math.min(maxOffset, Math.max(minOffset, root.scrollOffset));
        }
    }

    onHorizontalAlignmentChanged: updateScroll()

    onTextChanged: {
        if (innerInput.text !== root.text) {
            innerInput.text = root.text;
            syncModel();
        }
    }

    onHasFocusChanged: {
        if (hasFocus) {
            focusPopAnim.restart();
        }
    }

    function syncModel() {
        let str = innerInput.text;
        let oldCount = charModel.count;
        let newCount = str.length;

        let prefix = 0;
        while (prefix < oldCount && prefix < newCount && charModel.get(prefix).char === str[prefix]) {
            prefix++;
        }

        let suffix = 0;
        while (suffix < (oldCount - prefix) && suffix < (newCount - prefix) && charModel.get(oldCount - 1 - suffix).char === str[newCount - 1 - suffix]) {
            suffix++;
        }

        let deleteCount = oldCount - prefix - suffix;
        if (deleteCount > 0) {
            charModel.remove(prefix, deleteCount);
        }

        let insertStr = str.slice(prefix, newCount - suffix);
        for (let i = 0; i < insertStr.length; i++) {
            charModel.insert(prefix + i, { char: insertStr[i], stillPeeking: true });
        }

        if (root.masked && root.revealTyping) {
            globalRevealTimer.restart();
        }
        updateScroll();
    }

    Process {
        id: copyProc
    }

    SequentialAnimation {
        id: shakeAnim
        NumberAnimation { target: shakeT; property: "x"; from: 0; to: -6; duration: 50 }
        NumberAnimation { target: shakeT; property: "x"; from: -6; to: 6; duration: 50 }
        NumberAnimation { target: shakeT; property: "x"; from: 6; to: -4; duration: 50 }
        NumberAnimation { target: shakeT; property: "x"; from: -4; to: 4; duration: 50 }
        NumberAnimation { target: shakeT; property: "x"; from: 4; to: 0; duration: 50 }
    }

    SequentialAnimation {
        id: focusPopAnim
        NumberAnimation { target: root; property: "focusPop"; to: 1.03; duration: 110; easing.type: Easing.OutQuad }
        NumberAnimation { target: root; property: "focusPop"; to: 1.0; duration: 380; easing.type: Easing.OutQuint }
    }

    TextMetrics {
        id: globalCharMetrics
        font.family: root.fontFamily
        font.pixelSize: root.fontPixelSize
        text: "0"
    }

    ListModel {
        id: charModel
    }

    Timer {
        id: globalRevealTimer
        interval: root.revealDuration
        onTriggered: {
            for (let i = 0; i < charModel.count; i++) {
                charModel.setProperty(i, "stillPeeking", false);
            }
        }
    }

    HoverHandler {
        id: mainHover
        enabled: root.enabled
        cursorShape: Qt.IBeamCursor
    }

    TapHandler {
        onTapped: {
            root.forceInputFocus();
            root.clicked();
        }
    }

    Rectangle {
        id: bgShape
        anchors.fill: parent
        radius: root.cornerRadius
        clip: true
        opacity: root.enabled ? 1.0 : 0.5
        color: root.isHoveredOrHighlighted ? Qt.darker(root.baseColor, 1.14) : root.baseColor
        Behavior on color { ColorAnimation { duration: 180 } }

        scale: (root.hasError ? 1.04 : (root.isBusy ? 0.98 : 1.0)) * root.focusPop
        Behavior on scale { NumberAnimation { duration: 250; easing.type: Easing.OutQuint } }
        transform: Translate { id: shakeT; x: 0 }
    }

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: root.horizontalPadding
        anchors.rightMargin: root.horizontalPadding
        spacing: 8

        Text {
            visible: root.leadingIcon !== ""
            text: root.leadingIcon
            font.family: root.iconFont
            font.pixelSize: root.fontPixelSize
            color: (root.hasFocus || root.action_highlight) ? root.activeSignalColor : root.subTextColor
            Behavior on color { ColorAnimation { duration: 180 } }
            Layout.alignment: Qt.AlignVCenter
        }

        Item {
            id: fieldArea
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            onWidthChanged: root.updateScroll()

            Text {
                id: placeholderLabel
                text: root.placeholderText
                font.family: root.fontFamily
                font.pixelSize: root.fontPixelSize
                color: root.subTextColor
                opacity: (innerInput.text.length === 0 && charModel.count === 0) ? 0.45 : 0.0
                x: root.computeRowX(implicitWidth, fieldArea.width)
                anchors.verticalCenter: parent.verticalCenter
                Behavior on opacity { NumberAnimation { duration: 180 } }
            }

            Rectangle {
                id: selectionHighlight
                readonly property int selMin: Math.min(innerInput.selectionStart, innerInput.selectionEnd)
                readonly property int selMax: Math.max(innerInput.selectionStart, innerInput.selectionEnd)
                readonly property bool hasSelection: selMax > selMin

                anchors.verticalCenter: parent.verticalCenter
                height: Math.min(parent.height - 4, root.fontPixelSize * 1.7)
                radius: 4
                color: root.activeSignalColor
                opacity: hasSelection ? 0.28 : 0.0

                x: root.scrollOffset + (selMin * root.charSlotStep) - 2
                width: hasSelection ? ((selMax - selMin) * root.charSlotStep - root.charSpacing + 4) : 0

                Behavior on opacity { NumberAnimation { duration: 110; easing.type: Easing.OutCubic } }
                Behavior on x { NumberAnimation { duration: 60; easing.type: Easing.OutQuad } }
                Behavior on width { NumberAnimation { duration: 60; easing.type: Easing.OutQuad } }
            }

            ListView {
                id: charRow
                height: parent.height
                anchors.verticalCenter: parent.verticalCenter
                orientation: ListView.Horizontal
                interactive: false
                boundsBehavior: Flickable.StopAtBounds
                spacing: root.charSpacing
                width: contentWidth
                x: root.scrollOffset
                model: charModel

                onContentWidthChanged: root.updateScroll()

                property int entranceDuration: 420
                property real entranceOvershoot: 3.2
                property int exitDuration: 160

                Behavior on x { NumberAnimation { duration: 200; easing.type: Easing.OutQuad } }

                add: Transition {
                    ParallelAnimation {
                        NumberAnimation { property: "scale"; from: 0.3; to: 1.0; duration: charRow.entranceDuration; easing.type: Easing.OutBack; easing.overshoot: charRow.entranceOvershoot }
                        NumberAnimation { property: "y"; from: 10; to: 0; duration: charRow.entranceDuration; easing.type: Easing.OutBack; easing.overshoot: charRow.entranceOvershoot * 0.8 }
                        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: 140; easing.type: Easing.OutCubic }
                    }
                }

                remove: Transition {
                    ParallelAnimation {
                        NumberAnimation { property: "scale"; to: 0.3; duration: charRow.exitDuration; easing.type: Easing.InBack }
                        NumberAnimation { property: "y"; to: -8; duration: charRow.exitDuration; easing.type: Easing.InCubic }
                        NumberAnimation { property: "opacity"; to: 0; duration: charRow.exitDuration * 0.9; easing.type: Easing.InCubic }
                    }
                }

                displaced: Transition {
                    NumberAnimation { properties: "x,y"; duration: 260; easing.type: Easing.OutBack; easing.overshoot: 1.6 }
                }

                delegate: Item {
                    id: slot
                    property bool stillPeeking: model.stillPeeking !== undefined ? model.stillPeeking : true
                    width: root.charSlotWidth > 0 ? root.charSlotWidth : globalCharMetrics.width
                    height: charRow.height
                    transformOrigin: Item.Center
                    property bool revealed: !root.masked || (root.revealTyping && stillPeeking)

                    Text {
                        anchors.centerIn: parent
                        visible: slot.revealed
                        text: model.char
                        color: root.textColor
                        font.family: root.fontFamily
                        font.pixelSize: root.fontPixelSize
                    }

                    Canvas {
                        id: maskCanvas
                        anchors.centerIn: parent
                        visible: !slot.revealed
                        width: root.fontPixelSize
                        height: root.fontPixelSize
                        onPaint: {
                            var ctx = getContext("2d");
                            ctx.reset();
                            root.maskPainter(ctx, width, root.textColor);
                        }
                        Connections {
                            target: root
                            function onTextColorChanged() { maskCanvas.requestPaint(); }
                        }
                    }
                }
            }

            Rectangle {
                id: caretRect
                width: 2
                height: root.fontPixelSize * 1.2
                color: root.caretColor
                visible: root.showCaret && (root.hasFocus || root.action_highlight)
                anchors.verticalCenter: parent.verticalCenter
                x: root.scrollOffset + (innerInput.cursorPosition * root.charSlotStep)

                Behavior on x { NumberAnimation { duration: 200; easing.type: Easing.OutQuad } }

                SequentialAnimation on opacity {
                    running: root.showCaret && (root.hasFocus || root.action_highlight) && root.isWidgetVisible
                    loops: Animation.Infinite
                    NumberAnimation { to: 0; duration: 100; easing.type: Easing.InQuad }
                    PauseAnimation { duration: 400 }
                    NumberAnimation { to: 1; duration: 100; easing.type: Easing.OutQuad }
                    PauseAnimation { duration: 400 }
                }
            }

            TextInput {
                id: innerInput
                anchors.fill: parent
                focus: true
                opacity: 0
                color: "transparent"
                selectionColor: "transparent"
                selectedTextColor: "transparent"
                selectByMouse: true
                mouseSelectionMode: TextInput.SelectCharacters
                horizontalAlignment: root.horizontalAlignment
                font.family: root.fontFamily
                font.pixelSize: root.fontPixelSize
                enabled: root.enabled && !root.isBusy
                maximumLength: root.maximumLength > 0 ? root.maximumLength : 32767
                validator: root.validator

                onCursorPositionChanged: root.updateScroll()
                onSelectionStartChanged: root.updateScroll()
                onSelectionEndChanged: root.updateScroll()

                Keys.onPressed: function(event) {
                    if (event.matches(StandardKey.Copy) || (event.modifiers & Qt.ControlModifier && event.key === Qt.Key_C)) {
                        if (innerInput.selectedText.length > 0) {
                            root.copyToClipboard(innerInput.selectedText);
                            event.accepted = true;
                        }
                    }
                }

                onTextEdited: {
                    root.text = text;
                    syncModel();
                    if (typeof Sounds !== "undefined") {
                        Sounds.playSfx(root.keySound);
                    }
                    root.textEdited(text);
                }

                onAccepted: {
                    root.accepted(text);
                    root.triggered();
                }
            }
        }

        Item {
            id: trailingWrapper
            visible: root.trailingIcon !== "" || (root.showClearButton && innerInput.text.length > 0)
            Layout.preferredWidth: trailingTxt.implicitWidth + 16
            Layout.fillHeight: true

            scale: trailingMa.pressed ? 0.85 : (trailingMa.containsMouse ? 1.05 : 1.0)
            Behavior on scale { NumberAnimation { duration: 150; easing.type: Easing.OutQuint } }

            Rectangle {
                anchors.centerIn: parent
                width: parent.width
                height: width
                radius: root.cornerRadius - 2
                color: root.textColor
                opacity: trailingMa.pressed ? 0.12 : (trailingMa.containsMouse ? 0.06 : 0.0)
                Behavior on opacity { NumberAnimation { duration: 150 } }
            }

            Text {
                id: trailingTxt
                anchors.centerIn: parent
                text: root.showClearButton && innerInput.text.length > 0 ? "󰅖" : root.trailingIcon
                font.family: root.iconFont
                font.pixelSize: root.fontPixelSize
                color: trailingMa.containsMouse ? root.textColor : root.subTextColor
                Behavior on color { ColorAnimation { duration: 150 } }
            }

            MouseArea {
                id: trailingMa
                anchors.fill: parent
                hoverEnabled: root.enabled
                cursorShape: Qt.PointingHandCursor
                visible: root.showClearButton && innerInput.text.length > 0
                onClicked: {
                    if (typeof Sounds !== "undefined") {
                        Sounds.playSfx(root.clickSound);
                    }
                    root.clear();
                    root.forceInputFocus();
                    root.clicked();
                }
            }
        }
    }
}
