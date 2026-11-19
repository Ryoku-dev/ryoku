pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../look"

// The bar's one input. At the root it leads with a search mark; inside a view it
// leads with that view's token, which is also the way back. The trailing slot is
// the view's own (Quick Ask's seal, for one).
Item {
    id: root

    required property real s
    property alias text: input.text
    property string token: ""
    property string placeholder: ""
    property Component accessory: null
    readonly property bool pushed: root.token.length > 0
    // Material's search pill, Adwaita's entry and Fluent's text box are planes
    // of their own; Lithic writes straight on the slip
    readonly property bool welled: !Look.lithic
    readonly property real wellInset: Math.round((Look.adwaita ? 10 : 8) * root.s)
    readonly property real side: Math.round((root.welled ? 14 : 18) * root.s)

    signal keyPressed(var event)
    signal edited(string text)
    signal backRequested()

    implicitHeight: Math.round((Look.material ? 60 : Look.adwaita ? 58 : Look.fluent ? 54 : 52) * root.s)

    function focusField() {
        input.forceActiveFocus();
    }

    function clear() {
        input.text = "";
    }

    // Fluent's top layer, rounded with the frame
    Shape {
        anchors.fill: parent
        visible: Look.fluent
        s: root.s
        cutSize: Look.cutFrame
        cutBottomLeft: 0
        cutBottomRight: 0
        fill: Look.fieldBand
    }
    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        visible: Look.fluent
        color: Look.rule
    }

    Shape {
        id: well
        x: root.wellInset
        y: root.wellInset
        width: parent.width - root.wellInset * 2
        height: parent.height - root.wellInset * (Look.adwaita ? 1.4 : 2)
        visible: root.welled
        s: root.s
        cutSize: Look.cutField
        fill: Look.well
        strokeColor: Look.wellStroke
        strokeWidth: Qt.colorEqual(Look.wellStroke, "transparent") ? 0 : 1
        clip: Look.fluent

        // Fluent underlines the box it is typing in with the accent
        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: Math.round(2 * root.s)
            anchors.rightMargin: Math.round(2 * root.s)
            height: Math.max(2, Math.round(2 * root.s))
            visible: Look.fluent
            color: Lithic.sel
        }
    }

    Item {
        id: line
        x: root.welled ? well.x : 0
        y: root.welled ? well.y : 0
        width: root.welled ? well.width : parent.width
        height: root.welled ? well.height : parent.height

        Symbol {
            id: mark
            x: root.side
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round(20 * root.s)
            s: root.s
            name: "search"
            size: 20
            ink: Lithic.fg3
            opacity: 1 - tokenPlate.shown
            visible: opacity > 0.01
        }

        Shape {
            id: tokenPlate

            property real shown: root.pushed ? 1 : 0
            readonly property real full: Math.ceil(tokenRow.implicitWidth + 16 * root.s)

            x: root.side - Math.round(4 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, tokenPlate.full * tokenPlate.shown)
            height: Math.round(28 * root.s)
            visible: tokenPlate.shown > 0.01
            clip: true
            s: root.s
            cutSize: Look.cutRow
            fill: tokenTap.pressed ? Look.press
                : Look.material ? Lithic.container
                : tokenHover.hovered ? Look.plate
                : Look.lithic || Look.fluent ? "transparent" : Look.hover
            strokeColor: Look.lithic ? Lithic.vein : Look.fluent ? Look.capStroke : "transparent"
            strokeWidth: Look.lithic || Look.fluent ? 1 : 0

            Behavior on shown {
                NumberAnimation { duration: Look.dur(Look.viewMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
            }

            Row {
                id: tokenRow
                x: Math.round(5 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(2 * root.s)

                Symbol {
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    name: "chevron_left"
                    size: 18
                    ink: Look.material ? Lithic.containerInk : Lithic.fg2
                }
                Text {
                    id: tokenLabel
                    anchors.verticalCenter: parent.verticalCenter
                    color: Look.material ? Lithic.containerInk : Lithic.fg
                    font.family: Look.monoMicro ? Look.mono : Look.sans
                    font.pixelSize: Math.round((Look.monoMicro ? 10 : 13) * root.s)
                    font.weight: Font.Medium
                    font.capitalization: Look.monoMicro ? Font.AllUppercase : Font.MixedCase
                    font.letterSpacing: Look.tracking * root.s
                    renderType: Text.NativeRendering
                    // the token keeps its word while it folds away
                    Connections {
                        target: root
                        function onTokenChanged() {
                            if (root.token.length > 0)
                                tokenLabel.text = root.token;
                        }
                    }
                    Component.onCompleted: tokenLabel.text = root.token
                }
            }

            HoverHandler { id: tokenHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: tokenTap; onTapped: root.backRequested() }
        }

        TextInput {
            id: input

            readonly property real lead: root.side + Math.max(mark.width, tokenPlate.width) + Math.round(12 * root.s)

            x: input.lead
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, parent.width - input.lead - root.side - (slot.item ? slot.width + Math.round(10 * root.s) : 0))
            focus: true
            clip: true
            color: Lithic.fg
            selectionColor: Lithic.sel
            selectedTextColor: Lithic.onSel
            selectByMouse: true
            font.family: Look.sans
            font.pixelSize: Math.round(16 * root.s)
            renderType: Text.NativeRendering
            cursorDelegate: Rectangle {
                width: Math.max(1, Math.round(1.5 * root.s))
                color: Lithic.sel
                visible: input.cursorVisible
            }
            // a pasted block reads as one line: every line break becomes a space
            function flatten() {
                const breaks = /[ \t]*(?:\r\n|[\r\n\u2028\u2029])+[ \t]*/g;
                if (input.text.search(breaks) < 0)
                    return;
                const head = input.text.slice(0, input.cursorPosition).replace(breaks, " ");
                const tail = input.text.slice(input.cursorPosition).replace(breaks, " ");
                input.text = head + tail;
                input.cursorPosition = head.length;
            }

            Keys.onPressed: event => root.keyPressed(event)
            onTextEdited: {
                input.flatten();
                root.edited(input.text);
            }

            Text {
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                visible: input.text.length === 0 && input.preeditText.length === 0
                text: root.placeholder
                color: Lithic.fg3
                font: input.font
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
        }

        Loader {
            id: slot
            anchors.right: parent.right
            anchors.rightMargin: root.side
            anchors.verticalCenter: parent.verticalCenter
            sourceComponent: root.accessory
        }
    }
}
