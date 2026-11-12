import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// one window-manager animation: how long it takes, its curve and its style;
// every change applies at once with undo
Dialog {
    id: dlg

    property var row: null
    // [{ value, label }]
    property var curves: []
    property var styles: []
    property Item overlayParent: null

    readonly property var value: dlg.row ? Changes.value(dlg.row.bind) : undefined
    readonly property var fallback: dlg.row ? Changes.defaultOf(dlg.row) : undefined

    function set(field, v) {
        if (!dlg.value)
            return;
        var next = Object.assign({}, dlg.value);
        next[field] = v;
        Changes.set(dlg.row.bind, next, dlg.row.label, "animations");
    }

    function changed(field) {
        return !!dlg.value && !!dlg.fallback && JSON.stringify(dlg.value[field]) !== JSON.stringify(dlg.fallback[field]);
    }

    function indexIn(list, v) {
        for (var i = 0; i < list.length; i++)
            if (list[i].value === v)
                return i;
        return -1;
    }

    title: dlg.row ? I18n.tr(dlg.row.label) : ""
    dialogWidth: 600
    actions: [{ text: I18n.tr("Done"), kind: "quiet" }]

    Column {
        width: parent.width

        SettingRow {
            width: parent.width
            first: true
            title: I18n.tr("Turn on")
            detail: I18n.tr("Off makes this one animation instant")
            activatable: true
            changed: dlg.changed("enabled")
            onActivated: dlg.set("enabled", !(dlg.value && dlg.value.enabled))
            onResetRequested: dlg.set("enabled", dlg.fallback.enabled)
            Toggle {
                checked: !!dlg.value && dlg.value.enabled === true
                Accessible.name: I18n.tr("Turn on")
                onToggled: dlg.set("enabled", checked)
            }
        }

        SettingRow {
            width: parent.width
            title: I18n.tr("Length")
            detail: I18n.tr("How long it takes; longer is slower")
            changed: dlg.changed("speed")
            onResetRequested: dlg.set("speed", dlg.fallback.speed)
            Row {
                spacing: LithicTokens.space.s3
                Slider {
                    id: length
                    anchors.verticalCenter: parent.verticalCenter
                    width: 180
                    from: 0.1
                    to: 10
                    stepSize: 0.1
                    value: dlg.value ? Number(dlg.value.speed) || 1 : 1
                    accessibleName: I18n.tr("Length")
                    onMoved: dlg.set("speed", Math.round(value * 10) / 10)
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 64
                    horizontalAlignment: Text.AlignRight
                    text: I18n.tr("%1 ms").arg(Math.round(length.value * 100))
                    typeRole: "label"
                    inkRole: "fg2"
                    numeric: true
                }
            }
        }

        SettingRow {
            width: parent.width
            visible: dlg.curves.length > 0
            title: I18n.tr("Curve")
            detail: I18n.tr("The easing it follows; shape curves under More options")
            changed: dlg.changed("bezier")
            onResetRequested: dlg.set("bezier", dlg.fallback.bezier)
            Picker {
                width: 220
                options: dlg.curves.map(c => c.label)
                currentIndex: dlg.value ? dlg.indexIn(dlg.curves, dlg.value.bezier) : -1
                overlayParent: dlg.overlayParent
                Accessible.name: I18n.tr("Curve")
                onActivated: index => dlg.set("bezier", dlg.curves[index].value)
            }
        }

        SettingRow {
            width: parent.width
            visible: dlg.styles.length > 0
            title: I18n.tr("Style")
            detail: I18n.tr("The movement it makes")
            changed: dlg.changed("style")
            onResetRequested: dlg.set("style", dlg.fallback.style)
            Picker {
                width: 220
                options: dlg.styles.map(s => I18n.tr(s.label))
                currentIndex: dlg.value ? dlg.indexIn(dlg.styles, dlg.value.style) : -1
                overlayParent: dlg.overlayParent
                Accessible.name: I18n.tr("Style")
                onActivated: index => dlg.set("style", dlg.styles[index].value)
            }
        }
    }
}
