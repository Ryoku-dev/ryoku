pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K

// The search line: a mode-identity glyph, the query in large type with its
// ghost completion, and the mode tabs. Lithic draws no field, only a rule under
// the line. Material draws a pill, Adwaita a rounded entry, Fluent an outlined
// field whose lower edge thickens while focused.
FocusScope {
    id: root

    property var engine: null

    readonly property bool inputFocused: input.activeFocus
    readonly property bool caretAtEnd: input.cursorPosition === input.text.length
        && input.selectionStart === input.selectionEnd
    readonly property string glyphName: {
        const glyph = root.engine ? root.engine.glyph : "seal";
        if (glyph !== "seal")
            return glyph;
        return K.Look.lithic ? "seal" : "search";
    }

    signal keyPressed(var event)
    signal overrideRequested(var event)

    function focusInput() {
        input.forceActiveFocus();
    }

    implicitHeight: 56

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: K.Style.border
        visible: K.Look.lithic
        color: K.Style.line
    }

    K.Shape {
        id: field
        anchors.left: parent.left
        anchors.right: tabs.left
        anchors.rightMargin: K.Style.s3
        anchors.verticalCenter: parent.verticalCenter
        height: K.Look.material ? 48 : 40
        visible: !K.Look.lithic
        cutSize: K.Look.material ? K.Look.cutXl : K.Look.cutSm
        fill: K.Look.fluent ? K.Style.tint5 : K.Style.tint10
        strokeWidth: K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Look.fluent ? K.Style.lineSoft : "transparent"

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: K.Look.corner(K.Look.cutSm, field.width, field.height)
            anchors.rightMargin: anchors.leftMargin
            visible: K.Look.fluent
            height: input.activeFocus ? 2 : K.Style.border
            color: input.activeFocus ? K.Style.ink : K.Style.lineSoft
        }
    }

    Item {
        id: glyphSlot
        anchors.left: parent.left
        anchors.leftMargin: K.Look.lithic ? 0 : K.Style.s3
        anchors.verticalCenter: parent.verticalCenter
        width: 26
        height: 26

        Repeater {
            model: ["seal", "search", "auto_awesome", "bolt", "calculate", "public"]

            delegate: Item {
                id: mark

                required property string modelData
                readonly property bool active: mark.modelData === root.glyphName

                anchors.fill: parent
                opacity: mark.active ? 1 : 0
                visible: opacity > 0

                Behavior on opacity {
                    enabled: !K.Style.still
                    NumberAnimation { duration: K.Style.ms(120); easing.type: Easing.OutCubic }
                }

                Text {
                    anchors.centerIn: parent
                    visible: mark.modelData === "seal"
                    text: "力"
                    color: K.Style.ink
                    font.family: K.Style.jp
                    font.pixelSize: 22
                }

                K.Glyph {
                    anchors.centerIn: parent
                    visible: mark.modelData !== "seal"
                    name: mark.modelData
                    size: 22
                    tone: "ink"
                }
            }
        }
    }

    Item {
        id: entry
        anchors.left: glyphSlot.right
        anchors.leftMargin: K.Style.s3
        anchors.right: field.right
        anchors.rightMargin: K.Look.lithic ? 0 : K.Style.s3
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        clip: true

        Text {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            visible: input.text.length === 0 && input.preeditText.length === 0
            text: I18n.tr("Search apps and more")
            color: K.Style.inkMuted
            font.family: K.Style.font
            font.pixelSize: K.Style.headline
            font.weight: Font.Normal
            elide: Text.ElideRight
        }

        Text {
            x: input.x + input.cursorRectangle.x + input.cursorRectangle.width
            anchors.verticalCenter: parent.verticalCenter
            visible: root.caretAtEnd && text.length > 0 && input.preeditText.length === 0
            text: root.engine ? root.engine.ghost : ""
            color: K.Style.inkFaint
            font: input.font
        }

        TextInput {
            id: input
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            focus: true
            color: K.Style.ink
            selectionColor: K.Style.bone
            selectedTextColor: K.Style.inkOnBone
            selectByMouse: true
            font.family: K.Style.font
            font.pixelSize: K.Style.headline
            font.weight: Font.Medium
            verticalAlignment: TextInput.AlignVCenter

            onTextChanged: {
                if (root.engine && root.engine.query !== input.text)
                    root.engine.query = input.text;
            }

            Keys.onPressed: event => root.keyPressed(event)
            Keys.onShortcutOverride: event => root.overrideRequested(event)
        }
    }

    Connections {
        target: root.engine
        function onQueryChanged() {
            if (input.text !== root.engine.query) {
                input.text = root.engine.query;
                input.cursorPosition = input.text.length;
            }
        }
    }

    ModeTabs {
        id: tabs
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        modes: root.engine ? root.engine.modes : []
        current: root.engine ? root.engine.mode : "all"
        onPicked: mode => {
            if (root.engine)
                root.engine.setMode(mode);
            root.focusInput();
        }
    }

    Component.onCompleted: {
        if (root.engine)
            input.text = root.engine.query;
    }
}
