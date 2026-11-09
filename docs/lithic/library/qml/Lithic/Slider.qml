import QtQuick
import QtQuick.Templates as T

T.Slider {
    id: root

    property string previewState: ""
    property string accessibleName: "Value"

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
        let direction = 0;
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Down)
            direction = -1;
        else if (event.key === Qt.Key_Right || event.key === Qt.Key_Up)
            direction = 1;

        if (direction !== 0) {
            const minimum = Math.min(from, to);
            const maximum = Math.max(from, to);
            value = Math.max(minimum, Math.min(maximum, value + direction * _keyboardStep));
            event.accepted = true;
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
            track: Lithic.stone3
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
