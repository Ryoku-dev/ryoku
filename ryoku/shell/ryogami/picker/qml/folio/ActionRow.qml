import QtQuick
import Ryoku.Ui.Singletons
import "FolioActions.js" as Actions

// Heavy or destructive actions arm first and confirm on a second press; benign ones fire at once.
Item {
    id: row

    property var control: ({})
    property var state
    property var options
    property var host
    // Runtime parameters merged into the action (e.g. { index } for a list item).
    property var actionArgs: ({})
    property real reveal: 1

    implicitHeight: col.implicitHeight

    readonly property bool _multi: Array.isArray(row.control.action)
    readonly property string _labelText: row.control.label && row.control.label.length > 0
        ? row.control.label : ""

    function _run(action, extra) {
        var args = ({});
        var src = row.actionArgs || ({});
        for (var k in src) if (src.hasOwnProperty(k)) args[k] = src[k];
        if (extra) for (var j in extra) if (extra.hasOwnProperty(j)) args[j] = extra[j];
        Actions.run(action, args, Settings, row.state, row.host);
    }

    Column {
        id: col
        width: row.width
        spacing: 6 * Theme.scale

        Flow {
            width: parent.width
            visible: row._multi
            spacing: 7 * Theme.scale

            Repeater {
                model: row._multi ? row.control.options : []
                delegate: FolioAction {
                    required property var modelData
                    required property int index
                    enabled: row.enabled
                    label: modelData.label !== undefined ? modelData.label : String(modelData.value)
                    onTriggered: row._run(row.control.action[index], { band: modelData.value })
                }
            }
        }

        Loader {
            active: !row._multi
            sourceComponent: Actions.isDestructive(row.control.action) ? destructiveBtn : plainBtn
        }

        Text {
            width: parent.width
            visible: !!row.control.help && row.control.help.length > 0
            text: row.control.help ? row.control.help : ""
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.56 * row.reveal)
            lineHeight: 1.38
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }

    Component {
        id: plainBtn
        FolioAction {
            enabled: row.enabled
            soon: Actions.comingSoon(row.control.action)
            confirm: !Actions.isImmediate(row.control.action)
            glyph: row._labelText.length === 0 ? "\uf021" : ""
            label: row._labelText
            onTriggered: row._run(row.control.action, null)
        }
    }
    Component {
        id: destructiveBtn
        FolioDestructiveAction {
            enabled: row.enabled
            confirm: true
            glyph: row._labelText.length === 0 ? "\uf1f8" : ""
            label: row._labelText.length === 0 ? I18n.tr("Remove") : row._labelText
            onTriggered: row._run(row.control.action, null)
        }
    }
}
