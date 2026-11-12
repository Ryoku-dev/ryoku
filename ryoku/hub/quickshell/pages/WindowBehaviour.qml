import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/window-behaviour"
import "../schema/window-behaviour.js" as Schema

// tiling, gaps, focus, moving, workspaces: the shared rows and the window manager's own
Item {
    id: root

    required property var hub

    readonly property var layoutRow: Schema.rows.find(r => r.id === "tiling-layout")
    readonly property bool layoutShown: root.hub.rowAvailable(root.layoutRow)
    readonly property string layout: {
        var v = Changes.value(root.layoutRow.bind);
        return v === "master" || v === "scrolling" ? v : "dwindle";
    }

    // some window managers keep finer focus modes; the one in use stays visible and choosable
    readonly property var focusValue: Changes.value({ store: "desktop", key: "input.followMouse" })
    readonly property var extraFocus: Schema.extraFocus.find(o => o.value === root.focusValue)

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.map(r => {
            if (r.id !== "focus-follows" || !root.extraFocus)
                return r;
            return Object.assign({}, r, { control: "picker", options: r.options.concat([root.extraFocus]) });
        })
    })

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "window-behaviour"
        schema: root.schema
        bodies: root.layoutShown ? { tiling: sketchBody } : {}
    }

    Component {
        id: sketchBody
        Row {
            spacing: LithicTokens.space.s4

            LayoutSketch {
                id: sketch
                width: Math.min(240, parent.width * 0.4)
                layout: root.layout
            }

            Column {
                width: parent.width - sketch.width - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: LithicTokens.space.s2

                Label {
                    width: parent.width
                    text: root.layout === "master" ? I18n.tr("Main and stack")
                        : root.layout === "scrolling" ? I18n.tr("Scrolling") : I18n.tr("Split")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    text: root.layout === "master"
                        ? I18n.tr("One large window holds the focus; the others stack beside it.")
                        : root.layout === "scrolling"
                        ? I18n.tr("Windows stand in one long row of columns, and the row slides to keep the focused one in view.")
                        : I18n.tr("Each new window splits the focused one in two, so windows get smaller as you open more.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
        }
    }
}
