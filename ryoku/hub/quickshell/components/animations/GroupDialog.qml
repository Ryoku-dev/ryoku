import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

// the settings of one window-manager animation, as the provider lists them
Dialog {
    id: dlg

    property string heading: ""
    property var rows: []
    property var hub: null

    function met(r) {
        if (!r.when)
            return true;
        var v = Changes.value(r.when.bind);
        return r.when.is.some(x => JSON.stringify(x) === JSON.stringify(v));
    }

    function shows(r) {
        return dlg.met(r) && (!dlg.hub || dlg.hub.rowAvailable(r));
    }

    // fixed lists, so a control is never rebuilt under the pointer
    readonly property var plain: dlg.rows.filter(r => r.more !== true)
    readonly property var rare: dlg.rows.filter(r => r.more === true)

    title: I18n.tr(dlg.heading)
    dialogWidth: 640
    actions: [{ text: I18n.tr("Done"), kind: "quiet" }]

    Column {
        width: parent.width

        Repeater {
            model: dlg.plain
            delegate: GroupRow {
                required property var modelData
                required property int index
                width: parent ? parent.width : 0
                row: modelData
                first: index === 0
            }
        }

        Disclosure {
            visible: dlg.rare.some(r => dlg.shows(r))
            width: parent.width
            text: I18n.tr("More options")

            Repeater {
                model: dlg.rare
                delegate: GroupRow {
                    required property var modelData
                    required property int index
                    width: parent ? parent.width : 0
                    row: modelData
                    first: index === 0
                }
            }
        }
    }

    component GroupRow: Column {
        id: item
        property var row: ({})
        property bool first: false
        readonly property string error: Changes.error(item.row.bind)
        visible: dlg.shows(item.row)
        spacing: LithicTokens.space.s1

        SettingRow {
            width: parent.width
            first: item.first
            title: I18n.tr(item.row.label || "")
            detail: item.row.detail ? I18n.tr(item.row.detail) : ""
            changed: Changes.isChanged(item.row)
            activatable: ["toggle", "action"].indexOf(item.row.control) >= 0
            onActivated: control.activate()
            onResetRequested: Changes.set(item.row.bind, Changes.defaultOf(item.row), item.row.changeLabel || item.row.label, "animations")

            RowControl {
                id: control
                row: item.row
                pageId: "animations"
                hub: dlg.hub
                overlayParent: dlg.hub ? dlg.hub.overlayItem : null
            }
        }

        Row {
            visible: item.error.length > 0
            width: parent.width
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                text: item.error
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retry
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: Changes.retry(item.row.bind)
            }
        }
    }
}
