import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "rules.js" as Rules

// one change a rule can make, drawn in the rule editor; undefined means not set
SettingRow {
    id: row

    required property var prop
    property var value: undefined
    // the options this window manager offers, for mode, tri and pick
    property var options: row.prop.options || []
    property Item overlayParent: null
    readonly property string problem: row.prop.kind === "text" ? Rules.problem(row.prop, row.value) : ""

    signal chosen(var v)

    function problemText(code) {
        switch (code) {
        case "size": return I18n.tr("Use width x height, like %1").arg(row.prop.example);
        case "point": return I18n.tr("Use two numbers, like %1").arg(row.prop.example);
        case "fraction": return I18n.tr("Use a number from 0.1 to 1");
        case "positive": return I18n.tr("Use a number above 0, like %1").arg(row.prop.example);
        }
        return "";
    }

    function shown(v) {
        if (row.prop.pct)
            return Math.round(Number(v) * 100) + "%";
        if (row.prop.unit === "px")
            return I18n.tr("%1 px").arg(Math.round(Number(v)));
        return String(Math.round(Number(v) * 100) / 100);
    }

    width: parent ? parent.width : 0
    plane: Lithic.field
    title: I18n.tr(row.prop.label)
    detail: {
        var p = row.prop;
        if (p.offOnly && row.options.length === 1 && row.options[0].value === "off")
            return I18n.tr(p.offOnly.detail);
        return p.detail ? I18n.tr(p.detail) : "";
    }
    activatable: row.prop.kind === "flag"
    onActivated: row.chosen(row.value === true ? undefined : true)

    Loader {
        sourceComponent: {
            switch (row.prop.kind) {
            case "flag": return flagControl;
            case "mode":
            case "tri": return segmentedControl;
            case "pick": return pickerControl;
            case "text": return textControl;
            case "amount": return amountControl;
            }
            return null;
        }
    }

    Component {
        id: flagControl
        Toggle {
            checked: row.value === true
            Accessible.name: row.title
            onToggled: row.chosen(checked ? true : undefined)
        }
    }

    Component {
        id: segmentedControl
        Segmented {
            readonly property var values: [undefined].concat(row.options.map(o => o.value))
            width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
            model: [I18n.tr("Default")].concat(row.options.map(o => I18n.tr(o.label)))
            currentIndex: Math.max(0, values.indexOf(row.value))
            accessibleName: row.title
            onActivated: index => row.chosen(values[index])
        }
    }

    Component {
        id: pickerControl
        Picker {
            readonly property var values: [undefined].concat(row.options.map(o => o.value))
            width: Math.min(Math.max(implicitWidth, 220), LithicTokens.size.settingsControlMax)
            options: [I18n.tr("Off")].concat(row.options.map(o => I18n.tr(o.label)))
            currentIndex: Math.max(0, values.indexOf(row.value))
            overlayParent: row.overlayParent
            Accessible.name: row.title
            onActivated: index => row.chosen(values[index])
        }
    }

    Component {
        id: textControl
        TextField {
            id: field
            function load() {
                field.text = row.value === undefined || row.value === null ? "" : String(row.value);
            }
            width: 200
            mono: true
            placeholderText: I18n.tr("Not set")
            errorText: row.problemText(row.problem)
            Accessible.name: row.title
            Component.onCompleted: field.load()
            onTextEdited: row.chosen(text.trim() === "" ? undefined : text)
            // the editor fills its values in after the rows are made
            Connections {
                target: row
                function onValueChanged() {
                    if (!field.activeFocus)
                        field.load();
                }
            }
        }
    }

    Component {
        id: amountControl
        Row {
            spacing: LithicTokens.space.s3
            readonly property bool on: row.value !== undefined && row.value !== null

            Slider {
                id: slider
                anchors.verticalCenter: parent.verticalCenter
                visible: parent.on
                width: 150
                from: row.prop.min
                to: row.prop.max
                stepSize: row.prop.step || 0
                value: parent.on ? Number(row.value) : row.prop.start
                accessibleName: row.title
                onMoved: row.chosen(Math.round(value * 1000) / 1000)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                visible: parent.on
                width: 52
                horizontalAlignment: Text.AlignRight
                text: row.shown(slider.value)
                typeRole: "label"
                inkRole: "fg2"
                numeric: true
            }
            Toggle {
                anchors.verticalCenter: parent.verticalCenter
                checked: parent.on
                Accessible.name: I18n.tr("Set %1").arg(row.title)
                onToggled: row.chosen(checked ? row.prop.start : undefined)
            }
        }
    }
}
