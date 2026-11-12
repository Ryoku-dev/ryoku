import QtQuick
import QtQuick.Templates as T

T.SpinBox {
    id: root

    property string unit: ""
    property string previewState: ""
    property string accessibleName: Lithic.tr("Value")

    readonly property bool _focusActive: enabled && (visualFocus || previewState === "focus")
    // an editable stepper sizes its field for the widest bound, so a typed value fits
    readonly property real _valueWidth: editable ? Math.max(valueLabel.implicitWidth, boundMetrics.advanceWidth)
                                                 : valueLabel.implicitWidth

    function _commit() {
        const typed = root.valueFromText(valueInput.text, root.locale)
        if (typeof typed === "number" && !isNaN(typed)) {
            const low = Math.min(from, to)
            const high = Math.max(from, to)
            const stepped = stepSize > 0 ? from + Math.round((typed - from) / stepSize) * stepSize : typed
            const next = Math.max(low, Math.min(high, stepped))
            if (next !== value) {
                value = next
                valueModified()
            }
        }
        _showValue()
    }

    function _showValue() {
        valueInput.text = Qt.binding(() => root.displayText)
    }

    // clearing the field's own focus hands it back to the stepper, and leaving the field commits
    function _leaveField() {
        valueInput.focus = false
        root.focusReason = Qt.TabFocusReason
    }

    TextMetrics {
        id: boundMetrics
        font: valueLabel.font
        text: {
            const low = root.textFromValue(root.from, root.locale)
            const high = root.textFromValue(root.to, root.locale)
            return low.length > high.length ? low : high
        }
    }

    from: 0
    to: 100
    stepSize: 1
    editable: false
    // a setting reads as a plain number, as on the web and in Gio: no separator in a port or a year
    textFromValue: (value, locale) => Number(value).toLocaleString(root._plainLocale, "f", 0)
    readonly property var _plainLocale: {
        const plain = Qt.locale(root.locale.name)
        plain.numberOptions = Locale.OmitGroupSeparator
        return plain
    }
    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.iconPlaqueSmall * 2
                   + LithicTokens.size.stepperGap * 2
                   + LithicTokens.space.s2 * 2
                   + root._valueWidth
                   + (unit.length > 0 ? LithicTokens.size.stepperUnitGap + unitLabel.implicitWidth : 0)
    implicitHeight: LithicTokens.size.iconPlaqueSmall
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.SpinBox
    Accessible.name: accessibleName
    // an editable stepper holds focus itself, so the arrows step and Ctrl+Z reaches whoever holds
    // the stepper; a typed digit or minus opens the field with that character
    Keys.onPressed: event => {
        if (!editable || valueInput.activeFocus || !/^[0-9-]$/.test(event.text)
                || (event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier)))
            return
        valueInput.forceActiveFocus()
        valueInput.text = event.text
        event.accepted = true
    }
    // SpinBox makes its content a Tab stop of its own; the stepper is one stop, and an editable one
    // passes its focus inside anyway
    Component.onCompleted: contentItem.activeFocusOnTab = false

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
        // an editable SpinBox passes its focus to this item, so here it carries the stepper's name
        Accessible.role: Accessible.SpinBox
        Accessible.name: root.accessibleName
        Accessible.ignored: !root.editable

        FieldSurface {
            x: LithicTokens.size.iconPlaqueSmall + LithicTokens.size.stepperGap
            width: Math.max(0, parent.width - x * 2)
            height: parent.height
            restCut: LithicTokens.cut.small
            focused: valueInput.activeFocus || (root.editable && root._focusActive)
        }

        Label {
            id: valueLabel
            anchors.centerIn: parent
            anchors.horizontalCenterOffset: root.unit.length > 0
                                            ? -(LithicTokens.size.stepperUnitGap + unitLabel.implicitWidth) / 2
                                            : 0
            visible: !root.editable
            text: root.displayText
            typeRole: "label"
            numeric: true
            inkRole: "fg"
        }

        TextInput {
            id: valueInput
            anchors.verticalCenter: valueLabel.verticalCenter
            anchors.horizontalCenter: valueLabel.horizontalCenter
            width: root._valueWidth
            visible: root.editable
            enabled: root.editable && root.enabled
            horizontalAlignment: TextInput.AlignHCenter
            font: valueLabel.font
            color: Lithic.fg
            selectionColor: Lithic.sel
            selectedTextColor: Lithic.onSel
            selectByMouse: true
            inputMethodHints: Qt.ImhFormattedNumbersOnly
            Accessible.name: root.accessibleName
            text: root.displayText
            onActiveFocusChanged: {
                if (activeFocus) {
                    selectAll()
                    return
                }
                root._commit()
                // the field stays the stepper's focus child after focus moves elsewhere; drop it, so coming back lands on the stepper
                Qt.callLater(() => { if (!valueInput.activeFocus) valueInput.focus = false })
            }
            // Return commits and goes on, as a TextField's does, so a form around the stepper hears it
            Keys.onReturnPressed: event => {
                root._leaveField()
                event.accepted = false
            }
            Keys.onEnterPressed: event => {
                root._leaveField()
                event.accepted = false
            }
            // Escape drops an unsaved edit; with nothing typed it goes on, so a dialog can close
            Keys.onEscapePressed: event => {
                const edited = text !== root.displayText
                root._showValue()
                root._leaveField()
                event.accepted = edited
            }
        }

        Label {
            id: unitLabel
            anchors.left: valueLabel.right
            anchors.leftMargin: root.unit.length > 0 ? LithicTokens.size.stepperUnitGap : 0
            anchors.verticalCenter: valueLabel.verticalCenter
            visible: root.unit.length > 0
            text: root.unit
            typeRole: "caption"
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
        faceFocused: !root.editable && root._focusActive && root.value >= root.to
    }

    up.indicator: StepperFace {
        x: root.width - width
        anchors.verticalCenter: parent.verticalCenter
        glyph: "add"
        faceEnabled: root.enabled && root.value < root.to
        faceHovered: faceEnabled && root.up.hovered
        facePressed: faceEnabled && root.up.pressed
        faceFocused: !root.editable && root._focusActive && faceEnabled
    }
}
