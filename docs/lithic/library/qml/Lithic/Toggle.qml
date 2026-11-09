import QtQuick
import QtQuick.Templates as T

T.Switch {
    id: root

    property string previewState: ""

    readonly property bool _hoverActive: enabled && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && (pressed || previewState === "pressed")
    readonly property bool _focusActive: enabled && (visualFocus || previewState === "focus")

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: contentItem.implicitWidth
    implicitHeight: Math.max(LithicTokens.size.toggleHeight, labelItem.implicitHeight)
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.CheckBox
    Accessible.name: text
    Accessible.checked: checked

    background: FocusRing {
        x: -outset
        y: -outset
        width: root.width + outset * 2
        height: root.height + outset * 2
        keyboardFocusVisible: root._focusActive
        cut: LithicTokens.cut.toggleTrack
    }

    contentItem: Item {
        implicitWidth: LithicTokens.size.toggleWidth
                       + (root.text.length > 0 ? LithicTokens.size.toggleLabelGap + labelItem.implicitWidth : 0)
        implicitHeight: Math.max(LithicTokens.size.toggleHeight, labelItem.implicitHeight)

        transform: Translate {
            y: root._pressActive ? Lithic.pressSink : 0
            ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
        }

        CutRect {
            id: track
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: LithicTokens.size.toggleWidth
            height: LithicTokens.size.toggleHeight
            fill: {
                const base = root.checked ? Lithic.sel : Lithic.stone4;
                return root._hoverActive ? Lithic.mix(base, Lithic.fg, LithicTokens.alpha.hoverPlane) : base;
            }
            strokeColor: Lithic.vein
            strokeWidth: root.checked ? 0 : LithicTokens.size.hairline
            cut: LithicTokens.cut.toggleTrack

            ChiselColorBehavior on fill { duration: LithicTokens.motion.field }

            CutRect {
                width: LithicTokens.size.toggleKnob
                height: width
                x: root.checked
                   ? parent.width - LithicTokens.size.toggleInset - width
                   : LithicTokens.size.toggleInset
                anchors.verticalCenter: parent.verticalCenter
                fill: root.checked ? Lithic.onSel : Lithic.fg2
                cut: LithicTokens.cut.toggleKnob

                ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
                ChiselColorBehavior on fill { duration: LithicTokens.motion.field }
            }
        }

        Label {
            id: labelItem
            anchors.left: track.right
            anchors.leftMargin: root.text.length > 0 ? LithicTokens.size.toggleLabelGap : 0
            anchors.verticalCenter: parent.verticalCenter
            visible: root.text.length > 0
            text: root.text
            typeRole: "label"
            inkRole: "fg"
        }
    }
}
