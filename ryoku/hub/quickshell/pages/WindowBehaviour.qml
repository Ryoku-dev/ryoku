import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/window-behaviour"
import "../components/previews"
import "../schema/window-behaviour.js" as Schema

// tiling, gaps, focus, moving, workspaces: the shared rows and the window manager's own
Item {
    id: root

    required property var hub

    readonly property var layoutRow: Schema.rows.find(r => r.id === "tiling-layout")
    readonly property bool layoutShown: root.hub.rowAvailable(root.layoutRow)

    // a window manager that tiles each workspace by itself lets every display have its own layout
    readonly property bool perOutput: Settings.supports("perWorkspaceLayout")
    readonly property var tiling: Changes.stores.outputtiling || null
    readonly property var displays: root.tiling ? root.tiling.outputs : []
    readonly property bool scoped: !!root.tiling && root.tiling.scoped
    // with the link off, a display other than the main one shows the layout it has live, and the shared one until it has any
    readonly property var layoutBind: root.scoped
        ? { store: "outputtiling", key: "layout:" + root.tiling.active, fallback: Changes.value(root.layoutRow.bind) }
        : root.layoutRow.bind
    // Hyprdrive's Mosaic is a fourth layout while Hyprdrive is on
    readonly property bool mosaic: Settings.supports("hyprdrive")
        && Changes.value({ store: "desktop", key: "wm.hyprland.plugins.hyprdrive.enabled" }) === true
    readonly property string layout: {
        var v = Changes.value(root.layoutBind);
        return v === "master" || v === "scrolling" || (v === "mosaic" && root.mosaic) ? v : "dwindle";
    }

    function ensureTiling() {
        if (root.perOutput)
            Changes.ensureStore("outputtiling", Qt.resolvedUrl("../components/window-behaviour/OutputTilingStore.qml"));
    }
    onPerOutputChanged: root.ensureTiling()
    Component.onCompleted: {
        var had = !!Changes.stores.outputtiling;
        root.ensureTiling();
        if (had)
            Changes.stores.outputtiling.load();
    }

    Binding {
        target: root.tiling
        property: "pageVisible"
        value: root.visible
        when: root.tiling !== null
    }

    Component.onDestruction: {
        if (root.tiling)
            root.tiling.pageVisible = false;
    }

    function revisited() {
        if (!root.tiling)
            return;
        if (root.tiling.frameDirty) {
            root.tiling.frameDirty = false;
            Changes.touch();
        }
        if (!Changes.unsettled && root.tiling.writesInFlight === 0)
            root.tiling.load();
    }

    // the link and the display choice only mean something with two displays, or while the link is off
    function rowKept(row) {
        if (row.id === "layout-display")
            return root.displays.length >= 2;
        if (row.id === "link-displays")
            return root.displays.length >= 2 || (!!root.tiling && root.tiling.known && !root.tiling.linked);
        return true;
    }

    // some window managers keep finer focus modes; the one in use stays visible and choosable
    readonly property var focusValue: Changes.value({ store: "desktop", key: "input.followMouse" })
    readonly property var extraFocus: Schema.extraFocus.find(o => o.value === root.focusValue)

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => root.rowKept(r)).map(r => {
            if (r.id === "tiling-layout") {
                var layoutRow = root.mosaic ? Object.assign({}, r, { options: r.options.concat([Schema.mosaicOption]) }) : r;
                if (!root.scoped)
                    return layoutRow;
                return Object.assign({}, layoutRow, {
                    bind: root.layoutBind,
                    detail: I18n.tr("How new windows share the screen on %1").arg(root.tiling.active),
                    changeLabel: I18n.tr("Tiling layout on %1").arg(root.tiling.active)
                });
            }
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
        headerPreview: Component { TilingPreview {} }
        schema: root.schema
        controls: ({ "layout-display": displayChooser })
        bodies: root.layoutShown ? { tiling: sketchBody } : {}
    }

    Component {
        id: displayChooser
        Segmented {
            property var row
            width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
            model: root.displays
            currentIndex: Math.max(0, root.displays.indexOf(root.tiling ? root.tiling.active : ""))
            accessibleName: I18n.tr("Display")
            onActivated: index => root.tiling.select(root.displays[index])
        }
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
                        : root.layout === "scrolling" ? I18n.tr("Scrolling")
                        : root.layout === "mosaic" ? I18n.tr("Mosaic") : I18n.tr("Split")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    text: root.layout === "master"
                        ? I18n.tr("One large window holds the focus; the others stack beside it.")
                        : root.layout === "scrolling"
                        ? I18n.tr("Windows stand in one long row of columns, and the row slides to keep the focused one in view.")
                        : root.layout === "mosaic"
                        ? I18n.tr("Windows keep their own size and gather in one tight cluster, and tile only once they no longer fit. From Hyprdrive.")
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
