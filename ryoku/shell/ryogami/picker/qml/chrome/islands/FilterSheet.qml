import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "Facets.js" as Facets

// The filter sheet that grows out of the search island
Item {
    id: sheet

    required property PickerState state
    required property var bar
    readonly property LibraryView view: sheet.state ? sheet.state.view : null
    readonly property string collection: sheet.view ? sheet.view.collection : ""
    readonly property real s: Theme.scale
    readonly property real pad: 22 * s

    implicitHeight: body.implicitHeight + sheet.pad * 2

    function tr(t) { return I18n.tr(t) }

    component TypeCount: LibraryView {
        required property string kind
        collection: sheet.collection
        folder: sheet.view ? sheet.view.folder : "*"
        typeFilter: kind
    }
    property list<LibraryView> typeCounts: [
        TypeCount { kind: "" }, TypeCount { kind: "static" },
        TypeCount { kind: "video" }, TypeCount { kind: "we" }
    ]
    TypeCount { id: everything; kind: "" ; folder: "*" }

    property var folders: []
    function _rebuildFolders() { sheet.folders = sheet.view ? Library.folders(sheet.collection) : [] }
    Component.onCompleted: sheet._rebuildFolders()
    Connections {
        target: Library
        function onChanged(collection) { if (collection === sheet.collection) sheet._rebuildFolders() }
    }
    onCollectionChanged: sheet._rebuildFolders()

    Column {
        id: body
        x: sheet.pad
        y: sheet.pad
        width: sheet.width - sheet.pad * 2
        spacing: 18 * sheet.s

        Item {
            width: parent.width
            height: 30 * sheet.s
            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 14 * sheet.s
                Text {
                    text: I18n.tr("Filters")
                    font.family: Lithic.font.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fs(18)
                    color: Lithic.fg
                    renderType: Text.NativeRendering
                }
                Text {
                    anchors.baseline: parent.children[0].baseline
                    text: sheet.view ? I18n.tr("%1 of %2 match").arg(sheet.view.count).arg(everything.count) : ""
                    font.family: Lithic.font.sans
                    font.pixelSize: Theme.fs(13)
                    color: Lithic.fg3
                    renderType: Text.NativeRendering
                }
            }
            FacetChip {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: sheet.bar.filtersInPlay > 0
                glyph: "restart_alt"
                text: I18n.tr("Clear all")
                onTriggered: sheet.bar.clearFilters()
            }
        }

        Row {
            id: typeRow
            visible: sheet.collection === "wallpapers"
            spacing: 8 * sheet.s
            readonly property var shownTypes: {
                sheet.bar.settingsRev
                var all = Facets.types(sheet.tr), out = []
                for (var i = 0; i < all.length; ++i) {
                    if (!sheet.bar.show(all[i].key)) continue
                    var t = Object.assign({}, all[i]); t.at = i; out.push(t)
                }
                return out
            }
            Repeater {
                model: typeRow.shownTypes
                CutRect {
                    id: tile
                    required property var modelData
                    readonly property bool on: sheet.view && sheet.view.typeFilter === modelData.value
                    readonly property bool hovered: tileArea.containsMouse
                    width: (body.width - 8 * sheet.s * (typeRow.shownTypes.length - 1)) / Math.max(1, typeRow.shownTypes.length)
                    height: 68 * sheet.s
                    cut: (LithicTokens.cut.mid + (tileArea.pressed ? Lithic.pressCutGrow : 0)) * sheet.s
                    fill: tile.on ? Lithic.container : Lithic.alpha(Lithic.fg, tile.hovered ? 0.09 : 0.05)
                    Behavior on fill { ColorAnimation { duration: Theme.fast } }
                    Glyph { x: 14 * sheet.s; y: 13 * sheet.s; name: modelData.glyph; size: 20; ink: tile.on ? Lithic.containerInk : Lithic.fg2 }
                    Text {
                        x: 14 * sheet.s
                        y: 40 * sheet.s
                        text: modelData.label
                        font.family: Lithic.font.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fs(13)
                        color: tile.on ? Lithic.containerInk : Lithic.fg
                        renderType: Text.NativeRendering
                    }
                    Text {
                        anchors.right: parent.right
                        anchors.rightMargin: 14 * sheet.s
                        y: 10 * sheet.s
                        text: String(sheet.typeCounts[modelData.at].count).padStart(2, "0")
                        font.family: Lithic.font.mono
                        font.pixelSize: Theme.fs(20)
                        color: tile.on ? Lithic.sel : Lithic.fg3
                        renderType: Text.NativeRendering
                    }
                    MouseArea {
                        id: tileArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: if (sheet.view) sheet.view.typeFilter = modelData.value
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: 8 * sheet.s
            visible: sheet.bar.show("filterBar.show.folder") && sheet.folders.length > 0
            SheetLabel { text: I18n.tr("Folders") }
            Flow {
                width: parent.width
                spacing: 8 * sheet.s
                Repeater {
                    model: {
                        var base = [{ label: I18n.tr("All folders"), value: "*" }, { label: I18n.tr("Main"), value: "" }]
                        for (var i = 0; i < sheet.folders.length; ++i)
                            base.push({ label: sheet.folders[i], value: sheet.folders[i] })
                        return base
                    }
                    FolderTile {
                        required property var modelData
                        collection: sheet.collection
                        label: modelData.label
                        value: modelData.value
                        on: sheet.view && sheet.view.folder === modelData.value
                        width: (body.width - 16 * sheet.s) / 3
                        onTriggered: if (sheet.view) sheet.view.folder = modelData.value
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: 8 * sheet.s
            visible: sheet.bar.show("filterBar.show.colors")
            Row {
                spacing: 10 * sheet.s
                SheetLabel { text: I18n.tr("Colour") }
                Text {
                    text: sheet.view && sheet.view.hueFilter !== -1 ? Facets.hueName(sheet.view.hueFilter, sheet.tr) : ""
                    font.family: Lithic.font.sans
                    font.pixelSize: Theme.fs(12)
                    color: Lithic.fg2
                    renderType: Text.NativeRendering
                }
            }
            Row {
                spacing: 6 * sheet.s
                Repeater {
                    model: Facets.hueValues()
                    Item {
                        id: swatch
                        required property int modelData
                        readonly property bool on: sheet.view && sheet.view.hueFilter === modelData
                        readonly property bool hovered: swatchArea.containsMouse
                        width: 34 * sheet.s
                        height: 34 * sheet.s
                        CutRect {
                            anchors.fill: parent
                            cut: 7 * sheet.s
                            fill: "transparent"
                            strokeWidth: 2 * sheet.s
                            strokeColor: Lithic.sel
                            opacity: swatch.on ? 1 : 0
                            scale: swatch.on ? 1 : 0.7
                            Behavior on opacity { NumberAnimation { duration: Theme.fast } }
                            Behavior on scale { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack } }
                        }
                        CutRect {
                            anchors.centerIn: parent
                            width: 24 * sheet.s
                            height: width
                            cut: 5 * sheet.s
                            fill: Facets.hueColor(swatch.modelData)
                            scale: swatch.hovered ? 1.15 : 1
                            Behavior on scale { NumberAnimation { duration: Theme.fast; easing.type: Easing.OutBack } }
                        }
                        MouseArea {
                            id: swatchArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: if (sheet.view) sheet.view.hueFilter = swatch.on ? -1 : swatch.modelData
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: 8 * sheet.s
            SheetLabel { text: I18n.tr("Order") }
            Flow {
                width: parent.width
                spacing: 6 * sheet.s
                Repeater {
                    model: {
                        sheet.bar.settingsRev
                        var all = Facets.orders(sheet.tr), out = []
                        for (var i = 0; i < all.length; ++i) if (sheet.bar.show(all[i].key)) out.push(all[i])
                        return out
                    }
                    FacetChip {
                        required property var modelData
                        glyph: modelData.glyph
                        text: modelData.label
                        on: sheet.view && sheet.view.sort === modelData.value
                        onTriggered: if (sheet.view) sheet.view.sort = modelData.value
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: 8 * sheet.s
            SheetLabel { text: I18n.tr("More") }
            Flow {
                width: parent.width
                spacing: 6 * sheet.s
                FacetChip {
                    visible: sheet.bar.show("filterBar.show.favourites")
                    glyph: "favorite"
                    text: I18n.tr("Favourites only")
                    on: sheet.view && sheet.view.favouritesOnly
                    onTriggered: if (sheet.view) sheet.view.favouritesOnly = !sheet.view.favouritesOnly
                }
                FacetChip {
                    visible: sheet.bar.show("filterBar.show.orient")
                    glyph: "crop_landscape"
                    text: I18n.tr("Wide")
                    on: sheet.view && sheet.view.orientation === "wide"
                    onTriggered: if (sheet.view) sheet.view.orientation = sheet.view.orientation === "wide" ? "" : "wide"
                }
                FacetChip {
                    visible: sheet.bar.show("filterBar.show.orient")
                    glyph: "crop_portrait"
                    text: I18n.tr("Tall")
                    on: sheet.view && sheet.view.orientation === "tall"
                    onTriggered: if (sheet.view) sheet.view.orientation = sheet.view.orientation === "tall" ? "" : "tall"
                }
                Repeater {
                    model: sheet.bar.show("filterBar.show.resolution") ? sheet.bar.resPresets() : []
                    FacetChip {
                        required property var modelData
                        required property int index
                        glyph: "aspect_ratio"
                        text: modelData.label || I18n.tr("Size %1").arg(index + 1)
                        on: sheet.bar.resIndex === index
                        onTriggered: sheet.bar.pickResolution(sheet.bar.resIndex === index ? -1 : index)
                    }
                }
                FacetChip {
                    visible: sheet.bar.show("filterBar.show.random")
                    glyph: "shuffle"
                    text: sheet.bar.rotationActive ? I18n.tr("Rotating") : I18n.tr("Rotate at random")
                    on: sheet.bar.rotationActive
                    onTriggered: sheet.bar.toggleRotation()
                }
            }
        }
    }
}
