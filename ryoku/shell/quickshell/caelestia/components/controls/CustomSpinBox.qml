pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.services

RowLayout {
    id: root

    property real value
    property real max: Infinity
    property real min: -Infinity
    property real step: 1

    property bool isEditing: false
    property string displayText: root.value.toString()

    signal valueModified(value: real)

    spacing: Tokens.spacing.small

    implicitWidth: 156

    function _round(v) {
        const decimals = root.step < 1 ? Math.max(1, Math.ceil(-Math.log10(root.step))) : 0;
        return Math.round(v * Math.pow(10, decimals)) / Math.pow(10, decimals);
    }

    onValueChanged: {
        if (!root.isEditing) {
            root.displayText = root.value.toString();
        }
    }

    StyledTextField {
        id: textField

        Layout.preferredWidth: 62
        horizontalAlignment: TextInput.AlignHCenter

        inputMethodHints: Qt.ImhFormattedNumbersOnly
        text: root.isEditing ? text : root.displayText
        validator: DoubleValidator {
            bottom: root.min
            top: root.max
            decimals: root.step < 1 ? Math.max(1, Math.ceil(-Math.log10(root.step))) : 0
        }
        onActiveFocusChanged: {
            if (activeFocus) {
                root.isEditing = true;
            } else {
                root.isEditing = false;
                root.displayText = root.value.toString();
            }
        }
        onAccepted: {
            const numValue = parseFloat(text);
            if (!isNaN(numValue)) {
                const clampedValue = root._round(Math.max(root.min, Math.min(root.max, numValue)));
                root.displayText = clampedValue.toString();
                root.valueModified(clampedValue);
            } else {
                text = root.displayText;
            }
            root.isEditing = false;
        }
        onEditingFinished: {
            if (text !== root.displayText) {
                const numValue = parseFloat(text);
                if (!isNaN(numValue)) {
                    const clampedValue = root._round(Math.max(root.min, Math.min(root.max, numValue)));
                    root.displayText = clampedValue.toString();
                    root.valueModified(clampedValue);
                } else {
                    text = root.displayText;
                }
            }
            root.isEditing = false;
        }

        padding: Tokens.padding.extraSmall
        leftPadding: Tokens.padding.medium
        rightPadding: Tokens.padding.medium

        background: StyledRect {
            radius: Tokens.rounding.medium
            color: Colours.tPalette.m3surfaceContainerHigh
        }
    }

    StyledSlider {
        id: slider

        Layout.fillWidth: true

        from: isFinite(root.min) ? root.min : -Number.MAX_SAFE_INTEGER
        to: isFinite(root.max) ? root.max : Number.MAX_SAFE_INTEGER
        stepSize: root.step
        value: root.value

        onInteraction: v => {
            const raw = root.min + v * (root.max - root.min);
            const rounded = root._round(raw);
            if (root.value !== rounded) {
                root.displayText = rounded.toString();
                root.valueModified(rounded);
            }
        }
    }
}
