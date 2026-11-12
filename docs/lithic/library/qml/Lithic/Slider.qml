import QtQuick
import QtQuick.Templates as T

T.Slider {
    id: root

    property string previewState: ""
    property string accessibleName: Lithic.tr("Value")

    readonly property bool _hoverActive: enabled && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && (pressed || previewState === "pressed")
    readonly property bool _focusActive: enabled && (visualFocus || previewState === "focus")
    readonly property real _keyboardStep: stepSize > 0
                                                  ? stepSize
                                                  : Math.abs(to - from) * LithicTokens.size.sliderKeyboardStep

    from: 0
    to: 100
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.sliderWidth
    implicitHeight: LithicTokens.size.sliderHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.Slider
    Accessible.name: accessibleName
    Keys.onPressed: function(event) {
        const minimum = Math.min(from, to);
        const maximum = Math.max(from, to);
        let next = value;
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Down)
            next = value - _keyboardStep;
        else if (event.key === Qt.Key_Right || event.key === Qt.Key_Up)
            next = value + _keyboardStep;
        else if (event.key === Qt.Key_Home)
            next = minimum;
        else if (event.key === Qt.Key_End)
            next = maximum;
        else
            return;
        event.accepted = true;
        next = Math.max(minimum, Math.min(maximum, next));
        // this handler replaces the template's keys, so it reports the move itself
        if (next !== value) {
            value = next;
            moved();
        }
    }


    background: Item {
        x: root.leftPadding
        y: root.topPadding + root.availableHeight / 2 - LithicTokens.size.sliderTrack / 2
        width: root.availableWidth
        height: LithicTokens.size.sliderTrack


        Blade {
            anchors.fill: parent
            fraction: root.visualPosition
            fill: Lithic.fg2
        }
    }

    handle: CutRect {
        id: sliderHandle
        x: root.leftPadding + root.visualPosition * (root.availableWidth - width)
        y: root.topPadding + root.availableHeight / 2 - height / 2
           + (root._pressActive ? Lithic.pressSink : 0)
        width: LithicTokens.size.sliderHandleWidth
        height: LithicTokens.size.sliderHandleHeight
        fill: root._hoverActive ? Lithic.hoverFill(Lithic.sel) : Lithic.sel
        cut: Math.min(width / 2, height / 4)
        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: root._focusActive
            cut: sliderHandle.cut
        }


        ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
        ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
        ChiselColorBehavior on fill {
            duration: root._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
        }
    }
}
