import QtQuick
import QtQuick.Templates as T

T.AbstractButton {
    id: root

    property string kind: "gold"
    property bool small: false
    property string glyph: ""
    property bool busy: false
    property string previewState: ""
    property string accessibleName: text

    readonly property real controlCut: small ? LithicTokens.cut.small : LithicTokens.cut.mid
    readonly property real sideInset: small ? LithicTokens.size.plaqueInsetSmall : LithicTokens.size.plaqueInset
    readonly property real glyphSize: small ? LithicTokens.size.plaqueIconSmall : LithicTokens.size.plaqueIcon
    readonly property real contentGap: small ? LithicTokens.size.plaqueGapSmall : LithicTokens.size.plaqueGap
    readonly property bool _hoverActive: enabled && !busy && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && !busy && (pressed || previewState === "pressed")
    readonly property bool _focusActive: enabled && (visualFocus || previewState === "focus")
    readonly property PlaqueRecipe _recipe: PlaqueRecipe { kind: root.kind }
    readonly property color ink: _recipe.ink

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    leftPadding: sideInset
    rightPadding: sideInset
    implicitWidth: implicitContentWidth + leftPadding + rightPadding
    implicitHeight: small ? LithicTokens.size.plaqueHeightSmall : LithicTokens.size.plaqueHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.Button
    Accessible.name: accessibleName.length > 0 ? accessibleName : text
    Accessible.description: busy ? Lithic.tr("Busy") : ""

    // Return and Enter press a focused plaque, as on the web and in Gio; a busy plaque swallows them
    Keys.onReturnPressed: if (!busy) click()
    Keys.onEnterPressed: if (!busy) click()
    Keys.onPressed: function(event) {
        if (busy && event.key === Qt.Key_Space)
            event.accepted = true;
    }

    background: Item {
        implicitWidth: root.implicitWidth
        implicitHeight: root.implicitHeight

        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: root._focusActive
            cut: root.controlCut
        }

        CutRect {
            anchors.fill: parent
            fill: root._recipe.fill(root._hoverActive)
            strokeColor: root._recipe.stroke
            strokeWidth: root._recipe.strokeWidth
            cut: root.controlCut + (root._pressActive ? Lithic.pressCutGrow : 0)

            ChiselColorBehavior on fill {
                duration: root._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }
            ChiselNumberBehavior on cut {
                duration: LithicTokens.motion.press
            }
        }
    }

    contentItem: Item {
        implicitWidth: contentRow.implicitWidth
        implicitHeight: contentRow.implicitHeight

        Row {
            id: contentRow
            anchors.centerIn: parent
            spacing: (working.visible || glyphItem.visible) && labelItem.visible ? root.contentGap : 0

            transform: Translate {
                y: root._pressActive ? Lithic.pressSink : 0
                ChiselNumberBehavior on y {
                    duration: LithicTokens.motion.press
                }
            }

            Working {
                id: working
                visible: root.busy
                active: root.busy
                size: root.glyphSize
            }

            Glyph {
                id: glyphItem
                width: visible ? root.glyphSize : 0
                height: root.glyphSize
                visible: !root.busy && root.glyph.length > 0
                name: root.glyph
                ink: root.ink
                size: root.glyphSize
            }

            Label {
                id: labelItem
                visible: root.text.length > 0
                text: root.text
                typeRole: "label"
                ink: root.ink
            }
        }
    }

    MouseArea {
        anchors.fill: parent
        z: 10
        enabled: root.enabled && root.busy
        acceptedButtons: Qt.AllButtons
    }
}
