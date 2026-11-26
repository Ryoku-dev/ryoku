import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "Facets.js" as Facets

// The middle island
Item {
    id: island

    required property PickerState state
    required property var bar
    readonly property LibraryView view: island.state ? island.state.view : null
    readonly property string collection: island.view ? island.view.collection : ""
    readonly property bool walls: island.collection === "wallpapers" || island.collection === "workshop"
    readonly property real tuneCenterX: trailing.x + tune.x + tune.width / 2

    readonly property string query: island.view && island.view.query ? String(island.view.query) : ""
    readonly property int tagCount: island.view && island.view.tags ? island.view.tags.length : 0
    readonly property bool described: !!(island.view && island.view.keyOrder && island.view.keyOrder.length > 0)
    readonly property bool searching: island.query !== "" || island.tagCount > 0 || island.described

    function tr(t) { return I18n.tr(t) }

    readonly property var tokens: {
        island.bar.settingsRev
        var v = island.view, out = []
        if (!v || !island.walls) return out
        if (v.typeFilter !== "")
            out.push({ id: "type", glyph: v.typeFilter === "video" ? "movie" : v.typeFilter === "we" ? "view_in_ar" : "image",
                       label: Facets.typeName(v.typeFilter, island.tr) })
        if (v.folder !== "*" && v.folder !== undefined)
            out.push({ id: "folder", glyph: "folder", label: v.folder === "" ? I18n.tr("Main") : v.folder })
        if (v.hueFilter !== -1)
            out.push({ id: "hue", dot: Facets.hueColor(v.hueFilter), label: Facets.hueName(v.hueFilter, island.tr) })
        if (v.favouritesOnly)
            out.push({ id: "fav", glyph: "favorite", label: I18n.tr("Favourites") })
        if (island.bar.shapeLabel !== "")
            out.push({ id: "shape", glyph: v.orientation === "tall" ? "crop_portrait" : "crop_landscape", label: island.bar.shapeLabel })
        if (island.bar.resIndex >= 0)
            out.push({ id: "res", glyph: "aspect_ratio", label: island.bar.resLabel() || I18n.tr("Size") })
        return out
    }
    function drop(id) {
        var v = island.view
        if (!v) return
        if (id === "type") v.typeFilter = ""
        else if (id === "folder") v.folder = "*"
        else if (id === "hue") v.hueFilter = -1
        else if (id === "fav") v.favouritesOnly = false
        else if (id === "shape") v.orientation = ""
        else if (id === "res") island.bar.clearResolution()
    }

    Glyph {
        id: lens
        x: 16 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        name: "search"
        size: 20
        ink: island.searching || search.containsMouse ? Lithic.fg : Lithic.fg2
        rotation: search.containsMouse ? -12 : 0
        Behavior on rotation { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack } }
    }

    Flickable {
        id: tokenStrip
        anchors.left: lens.right
        anchors.leftMargin: 10 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        width: Math.min(tokenRow.implicitWidth, Math.max(0, trailing.x - x - 120 * Theme.scale))
        height: 30 * Theme.scale
        contentWidth: tokenRow.implicitWidth
        clip: true
        interactive: contentWidth > width
        boundsBehavior: Flickable.StopAtBounds
        Row {
            id: tokenRow
            spacing: 5 * Theme.scale
            Repeater {
                model: island.tokens
                Token {
                    required property var modelData
                    glyph: modelData.glyph || ""
                    dot: modelData.dot || "transparent"
                    text: modelData.label
                    onDropped: island.drop(modelData.id)
                }
            }
        }
    }

    Text {
        id: prompt
        visible: island.bar.show("filterBar.show.tagcloud")
        anchors.left: tokenStrip.right
        anchors.leftMargin: (island.tokens.length > 0 ? 10 : 0) * Theme.scale
        anchors.right: trailing.left
        anchors.rightMargin: 10 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        elide: Text.ElideRight
        text: island.query !== "" ? island.query
            : island.tagCount > 0 ? I18n.tr("%1 tags chosen").arg(island.tagCount)
            : island.described ? I18n.tr("Described")
            : island.walls && island.view ? I18n.tr("Search %1, or describe one").arg(
                  island.collection === "workshop" ? I18n.tr("%1 scenes").arg(island.view.count) : I18n.tr("%1 walls").arg(island.view.count))
            : I18n.tr("Search")
        font.family: Lithic.font.sans
        font.pixelSize: Theme.fs(14)
        color: island.searching ? Lithic.fg : search.containsMouse ? Lithic.fg2 : Lithic.fg3
        renderType: Text.NativeRendering
        Behavior on color { ColorAnimation { duration: Theme.fast } }
    }
    MouseArea {
        id: search
        visible: prompt.visible
        anchors.left: lens.left
        anchors.right: trailing.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.leftMargin: tokenStrip.width + 30 * Theme.scale
        hoverEnabled: true
        cursorShape: Qt.IBeamCursor
        onClicked: if (island.state) { island.state.filtersOpen = false; island.state.searchOpen = !island.state.searchOpen }
    }

    Row {
        id: trailing
        anchors.right: parent.right
        anchors.rightMargin: 6 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4 * Theme.scale

        IslandButton {
            visible: island.searching
            glyph: "close"
            size: 34
            tooltip: I18n.tr("Clear the search")
            onTriggered: {
                if (!island.view) return
                island.view.query = ""
                island.view.tags = []
                island.view.keyOrder = []
            }
        }
        Rectangle {
            width: 1
            height: 26 * Theme.scale
            anchors.verticalCenter: parent.verticalCenter
            color: Lithic.vein
        }
        IslandButton {
            id: tune
            visible: island.walls
            glyph: "tune"
            badge: island.tokens.length
            on: island.state && island.state.filtersOpen
            tooltip: I18n.tr("Filters")
            onTriggered: if (island.state) { island.state.searchOpen = false; island.state.filtersOpen = !island.state.filtersOpen }
        }
        IslandButton {
            visible: island.collection === "themes"
            glyph: "brush"
            label: I18n.tr("Design a theme")
            soon: true
            tooltip: I18n.tr("The theme designer is coming soon")
            on: island.state && island.state.sheet === "themeDesigner"
            onTriggered: if (island.state) island.state.openSheet("themeDesigner", ({}))
        }
        IslandButton {
            visible: island.collection === "rices"
            glyph: "add_photo_alternate"
            label: I18n.tr("Save look")
            tooltip: I18n.tr("Save this desktop as a rice")
            on: island.state && island.state.riceShare === "save"
            onTriggered: if (island.state) island.state.openRiceShare("save", null)
        }
        IslandButton {
            visible: island.collection === "rices"
            glyph: "download"
            label: I18n.tr("Import")
            tooltip: I18n.tr("Import a rice someone shared")
            on: island.state && island.state.riceShare === "import"
            onTriggered: if (island.state) island.state.openRiceShare("import", null)
        }
    }
}
