import QtQuick
import Ryoku.Ui.Singletons
import "FolioActions.js" as Actions

Item {
    id: row

    property var control: ({})
    property var state
    property var options
    property real reveal: 1
    // Supplied by the page so a snapshot captures exactly what this mode exposes.
    property var paramKeys: []

    readonly property string _mode: String(Settings.value("components.wallpaperSelector.displayMode") || "slices")
    readonly property var _presets: {
        return Actions.asList(Settings.value("components.wallpaperSelector.presets"));
    }

    implicitHeight: col.implicitHeight

    function _write(arr) { Settings.set("components.wallpaperSelector.presets", arr); }

    function _save() {
        var snap = ({});
        for (var i = 0; i < row.paramKeys.length; i++) {
            var k = row.paramKeys[i];
            snap[k] = Settings.value(k);
        }
        var arr = row._presets.slice();
        arr.push({ name: I18n.tr("Style") + " " + (arr.length + 1), mode: row._mode, values: snap });
        row._write(arr);
    }
    function _apply(i) {
        var p = row._presets[i];
        if (!p || !p.values)
            return;
        for (var k in p.values)
            if (p.values.hasOwnProperty(k))
                Settings.set(k, p.values[k]);
    }
    function _remove(i) {
        var arr = row._presets.slice();
        arr.splice(i, 1);
        row._write(arr);
    }
    function _rename(i, name) {
        var arr = row._presets.slice();
        if (i < 0 || i >= arr.length)
            return;
        var p = ({});
        for (var k in arr[i]) if (arr[i].hasOwnProperty(k)) p[k] = arr[i][k];
        p.name = name;
        arr[i] = p;
        row._write(arr);
    }

    Column {
        id: col
        width: row.width
        spacing: 8 * Theme.scale

        FolioAction {
            enabled: row.enabled && row.paramKeys.length > 0
            glyph: "\uf0c7"
            label: I18n.tr("Save current style")
            onTriggered: row._save()
        }

        Text {
            width: parent.width
            visible: row._presets.length === 0
            text: I18n.tr("No saved styles yet. Save the current layout to reuse it later.")
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.5 * row.reveal)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Repeater {
            model: row._presets

            delegate: Rectangle {
                id: card
                required property var modelData
                required property int index
                width: col.width
                height: cardRow.implicitHeight + 14 * Theme.scale
                color: Theme.withAlpha(Theme.surfaceContainer, 0.55)
                border.width: 1
                border.color: Theme.withAlpha(Theme.outline, 0.32)
                visible: !card.modelData.mode || card.modelData.mode === row._mode

                Row {
                    id: cardRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: 10 * Theme.scale
                    anchors.rightMargin: 10 * Theme.scale
                    spacing: 8 * Theme.scale

                    TextField {
                        anchors.verticalCenter: parent.verticalCenter
                        width: card.width * 0.42
                        variant: "field"
                        enabled: row.enabled
                        text: card.modelData.name ? card.modelData.name : ""
                        placeholder: I18n.tr("Style name")
                        onCommitted: (t) => row._rename(card.index, t)
                    }
                    FolioAction {
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: row.enabled
                        glyph: "\uf00c"
                        label: I18n.tr("Apply")
                        onTriggered: row._apply(card.index)
                    }
                    FolioDestructiveAction {
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: row.enabled
                        confirm: true
                        label: I18n.tr("Delete")
                        onTriggered: row._remove(card.index)
                    }
                }
            }
        }
    }
}
