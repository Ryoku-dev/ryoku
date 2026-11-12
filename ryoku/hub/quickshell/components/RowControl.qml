import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"

// the control on the right of a setting row, chosen by the row's `control`
Item {
    id: root

    required property var row
    property string pageId: ""
    property Item overlayParent: null
    readonly property var value: Changes.value(root.row.bind)
    readonly property real maxWidth: LithicTokens.size.settingsControlMax

    implicitWidth: loader.item ? loader.item.implicitWidth : 0
    implicitHeight: loader.item ? loader.item.implicitHeight : 0

    function commit(v) {
        Changes.set(root.row.bind, v, root.row.changeLabel || root.row.label, root.pageId);
    }

    // the toggle row is clickable as a whole
    function activate() {
        if (root.row.control === "toggle")
            root.commit(root.value !== true);
        else if (root.row.control === "handoff" || root.row.control === "action")
            Spawn.run(root.row.command);
    }

    function optionIndex() {
        var opts = root.row.options || [];
        for (var i = 0; i < opts.length; i++)
            if (JSON.stringify(opts[i].value) === JSON.stringify(root.value))
                return i;
        return -1;
    }

    function optionLabels() {
        return (root.row.options || []).map(o => I18n.tr(o.label));
    }

    function shown(v) {
        if (v === undefined || v === null)
            return "";
        if (root.row.pct)
            return Math.round(Number(v) * 100) + "%";
        var n = Number(v);
        var digits = root.row.step !== undefined && root.row.step < 0.01 ? 3 : 2;
        var text = Number.isInteger(n) ? String(n) : n.toFixed(digits).replace(/0+$/, "").replace(/\.$/, "");
        return root.row.unit ? text + " " + root.row.unit : text;
    }

    Loader {
        id: loader
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        sourceComponent: {
            switch (root.row.control) {
            case "toggle": return toggleControl;
            case "segmented": return segmentedControl;
            case "slider": return sliderControl;
            case "stepper": return stepperControl;
            case "picker": return pickerControl;
            case "text": return textControl;
            case "color": return colorControl;
            case "chord": return chordControl;
            case "action":
            case "handoff": return openControl;
            }
            return null;
        }
    }

    Component {
        id: toggleControl
        Toggle {
            checked: root.value === true
            Accessible.name: I18n.tr(root.row.label)
            onToggled: root.commit(checked)
        }
    }

    Component {
        id: segmentedControl
        Segmented {
            width: Math.min(implicitWidth, root.maxWidth)
            model: root.optionLabels()
            currentIndex: Math.max(0, root.optionIndex())
            accessibleName: I18n.tr(root.row.label)
            onActivated: index => root.commit(root.row.options[index].value)
        }
    }

    Component {
        id: pickerControl
        Picker {
            width: Math.min(Math.max(implicitWidth, 200), root.maxWidth)
            options: root.optionLabels()
            currentIndex: root.optionIndex()
            overlayParent: root.overlayParent
            Accessible.name: I18n.tr(root.row.label)
            onActivated: index => root.commit(root.row.options[index].value)
        }
    }

    Component {
        id: sliderControl
        Row {
            spacing: LithicTokens.space.s3
            Slider {
                id: slider
                anchors.verticalCenter: parent.verticalCenter
                width: 200
                from: root.row.min !== undefined ? root.row.min : 0
                to: root.row.max !== undefined ? root.row.max : 1
                stepSize: root.row.step !== undefined ? root.row.step : 0
                value: Number(root.value) || 0
                accessibleName: I18n.tr(root.row.label)
                onMoved: root.commit(Math.round(value * 1000) / 1000)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: 56
                horizontalAlignment: Text.AlignRight
                text: root.shown(slider.value)
                typeRole: "label"
                inkRole: "fg2"
                numeric: true
            }
        }
    }

    Component {
        id: stepperControl
        Stepper {
            from: root.row.min !== undefined ? root.row.min : 0
            to: root.row.max !== undefined ? root.row.max : 100
            stepSize: 1
            value: Math.round(Number(root.value) || 0)
            unit: root.row.unit ? root.row.unit : ""
            editable: true
            accessibleName: I18n.tr(root.row.label)
            onValueModified: root.commit(value)
        }
    }

    Component {
        id: textControl
        TextField {
            width: root.maxWidth
            text: root.value === undefined || root.value === null ? "" : String(root.value)
            placeholderText: root.row.placeholder ? root.row.placeholder : ""
            mono: root.row.mono === true
            Accessible.name: I18n.tr(root.row.label)
            onEditingFinished: {
                if (text !== String(root.value === undefined || root.value === null ? "" : root.value))
                    root.commit(text);
            }
        }
    }

    Component {
        id: colorControl
        ColorField {
            id: colorField
            width: 220
            overlayParent: root.overlayParent
            accessibleName: I18n.tr(root.row.label)
            swatches: [
                { label: I18n.tr("Accent"), color: Lithic.gold },
                { label: I18n.tr("Jade"), color: Lithic.jade },
                { label: I18n.tr("Lapis"), color: Lithic.lapis },
                { label: I18n.tr("Amber"), color: Lithic.amber },
                { label: I18n.tr("Ink"), color: Lithic.fg }
            ]
            onPicked: c => root.commit(colorField.formatHex(c))
            Binding {
                target: colorField
                property: "color"
                value: root.value ? String(root.value) : "#000000"
            }
        }
    }

    Component {
        id: chordControl
        ChordField {
            chord: root.value ? String(root.value) : ""
            accessibleName: I18n.tr(root.row.label)
            onRecorded: chord => root.commit(chord)
            onCleared: root.commit("")
        }
    }

    Component {
        id: openControl
        Plaque {
            kind: "quiet"
            small: true
            glyph: root.row.control === "handoff" ? "open_in_new" : ""
            text: root.row.actionText ? I18n.tr(root.row.actionText) : I18n.tr("Open")
            accessibleName: I18n.tr(root.row.label)
            onClicked: root.activate()
        }
    }
}
