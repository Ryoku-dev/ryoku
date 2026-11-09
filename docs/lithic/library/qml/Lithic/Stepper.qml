import QtQuick
import QtQuick.Templates as T

T.SpinBox {
    id: root

    property string unit: ""
    property string previewState: ""
    property string accessibleName: "Value"

    readonly property bool _focusActive: enabled && (visualFocus || previewState === "focus")

    from: 0
    to: 100
    stepSize: 1
    editable: false
    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.iconPlaqueSmall * 2
                   + LithicTokens.size.stepperGap * 2
                   + valueLabel.implicitWidth
                   + (unit.length > 0 ? LithicTokens.size.stepperUnitGap + unitLabel.implicitWidth : 0)
    implicitHeight: LithicTokens.size.iconPlaqueSmall
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.SpinBox
    Accessible.name: accessibleName

    component StepperFace: Item {
        property string glyph: ""
        property bool faceEnabled: true
        property bool faceHovered: false
        property bool facePressed: false
        property bool faceFocused: false

        implicitWidth: LithicTokens.size.iconPlaqueSmall
        implicitHeight: LithicTokens.size.iconPlaqueSmall
        opacity: faceEnabled ? 1 : Lithic.disabledOpacity

        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: parent.faceFocused
            cut: LithicTokens.cut.small
        }

        CutRect {
            anchors.fill: parent
            fill: parent.faceHovered ? Lithic.hoverInk(Lithic.fg2) : "transparent"
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.small + (parent.facePressed ? Lithic.pressCutGrow : 0)

            ChiselColorBehavior on fill {
                duration: parent.faceHovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }
            ChiselNumberBehavior on cut { duration: LithicTokens.motion.press }
        }

        Glyph {
            anchors.centerIn: parent
            width: LithicTokens.size.iconPlaqueIconSmall
            height: width
            name: parent.glyph
            ink: Lithic.fg2
            size: LithicTokens.size.iconPlaqueIconSmall
        }
    }

    background: Item {}

    contentItem: Item {
        Label {
            id: valueLabel
            anchors.centerIn: parent
            anchors.horizontalCenterOffset: root.unit.length > 0
                                            ? -(LithicTokens.size.stepperUnitGap + unitLabel.implicitWidth) / 2
                                            : 0
            text: root.displayText
            typeRole: "mono"
            inkRole: "fg"
        }

        Label {
            id: unitLabel
            anchors.left: valueLabel.right
            anchors.leftMargin: root.unit.length > 0 ? LithicTokens.size.stepperUnitGap : 0
            anchors.verticalCenter: valueLabel.verticalCenter
            visible: root.unit.length > 0
            text: root.unit
            typeRole: "monoSm"
            inkRole: "fg3"
        }
    }

    down.indicator: StepperFace {
        x: 0
        anchors.verticalCenter: parent.verticalCenter
        glyph: "remove"
        faceEnabled: root.enabled && root.value > root.from
        faceHovered: faceEnabled && (root.down.hovered || root.previewState === "hover")
        facePressed: faceEnabled && (root.down.pressed || root.previewState === "pressed")
        faceFocused: root._focusActive && root.value >= root.to
    }

    up.indicator: StepperFace {
        x: root.width - width
        anchors.verticalCenter: parent.verticalCenter
        glyph: "add"
        faceEnabled: root.enabled && root.value < root.to
        faceHovered: faceEnabled && root.up.hovered
        facePressed: faceEnabled && root.up.pressed
        faceFocused: root._focusActive && faceEnabled
    }
}
