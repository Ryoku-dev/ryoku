import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "FolioSearch.js" as Finder

// Settings as a drawer that buds from the settings button and welds to the islands
Item {
    id: drawer

    required property PickerState state
    required property var folio
    property bool shown: false
    readonly property alias pageArea: pageHost
    readonly property bool searching: drawer.folio.query.length > 0
    readonly property bool slim: body.targetW < 900 * Theme.scale

    readonly property real s: Theme.scale
    anchors.fill: parent

    function glyphFor(key) {
        switch (key) {
        case "wallpaper": return "wallpaper"
        case "colours": return "palette"
        case "lighting": return "light"
        case "picker": return "view_carousel"
        case "system": return "memory"
        }
        return "settings"
    }
    function changedIn(section) {
        drawer.folio.rev
        if (!section) return 0
        var n = 0, cs = section.controls || []
        for (var i = 0; i < cs.length; i++) {
            var k = cs[i].key
            if (typeof k === "string" && k.length > 0 && k.indexOf(".N") < 0 && k.indexOf("{") < 0
                    && cs[i].kind !== "action" && cs[i].kind !== "static" && !Settings.isDefault(k))
                n++
        }
        return n
    }
    function changedInTab(tab) {
        var n = 0, secs = tab ? tab.sections || [] : []
        for (var i = 0; i < secs.length; i++) n += drawer.changedIn(secs[i])
        return n
    }

    property bool rotating: false
    function _refreshRotation() {
        Daemon.call("wall.rotation_status", {}, function (r, e) { if (r) drawer.rotating = r.active === true })
    }
    Connections {
        target: Daemon
        function onEvent(name, data) {
            if (name === "ryogami.wall.random_started" || name === "ryogami.wall.random_stopped") drawer._refreshRotation()
        }
    }
    onShownChanged: if (drawer.shown) drawer._refreshRotation()
    readonly property var quick: {
        drawer.folio.rev
        var follow = Settings.value("theme.policy") === "wallpaper"
        var muted = Settings.value("wallpaperMute") === true
        var optimise = Settings.value("performance.autoOptimizeImages") === true
        return [
            { id: "rotate", glyph: "autorenew", title: I18n.tr("Auto-rotate"),
              line: drawer.rotating ? I18n.tr("Running") : I18n.tr("Off"), on: drawer.rotating },
            { id: "colours", glyph: "palette", title: I18n.tr("Wallpaper colours"),
              line: follow ? I18n.tr("Each wallpaper sets the palette") : I18n.tr("Palette held"), on: follow },
            { id: "mute", glyph: muted ? "volume_off" : "volume_up", title: I18n.tr("Mute wallpapers"),
              line: muted ? I18n.tr("Videos and scenes are silent") : I18n.tr("Sound on"), on: muted },
            { id: "optimise", glyph: "auto_awesome", title: I18n.tr("Optimise new images"),
              line: optimise ? I18n.tr("New images shrink to WebP") : I18n.tr("Kept as they arrive"), on: optimise }
        ]
    }
    function toggleQuick(id) {
        if (id === "rotate") Daemon.call("settings.action", { id: "randomRotate" }, function () { drawer._refreshRotation() })
        else if (id === "colours") Settings.set("theme.policy", Settings.value("theme.policy") === "wallpaper" ? "off" : "wallpaper")
        else if (id === "mute") Settings.set("wallpaperMute", Settings.value("wallpaperMute") !== true)
        else if (id === "optimise") Settings.set("performance.autoOptimizeImages", Settings.value("performance.autoOptimizeImages") !== true)
    }

    function stepSection(d) {
        var tab = drawer.folio.activeTab
        var n = tab && tab.sections ? tab.sections.length : 0
        if (n === 0) return
        drawer.folio._selectSection(drawer.folio.activeTabIndex, (drawer.folio.activeSectionIndex + d + n) % n)
    }

    DrawerBody {
        id: body
        state: drawer.state
        key: "settings"
        anchorX: drawer.state.settingsAnchorX
        shown: drawer.shown
        // A section that previews on the cards slims the drawer to the right, so the scene stays in view
        maxWidth: drawer.folio.isDesign ? 620 : 1320

        Item {
            id: content
            anchors.fill: parent
            readonly property real pad: 28 * drawer.s

            Item {
                id: head
                x: content.pad
                y: 22 * drawer.s
                width: content.width - content.pad * 2
                height: 48 * drawer.s
                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2 * drawer.s
                    Text {
                        text: I18n.tr("Settings")
                        font.family: Lithic.font.sans
                        font.weight: Font.Bold
                        font.pixelSize: Theme.fs(24)
                        color: Lithic.fg
                        renderType: Text.NativeRendering
                    }
                    Row {
                        spacing: 6 * drawer.s
                        Text {
                            text: I18n.tr("Ryogami")
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(13)
                            color: Lithic.fg3
                            renderType: Text.NativeRendering
                        }
                        Glyph { name: "chevron_right"; size: 14; ink: Lithic.fg3; anchors.verticalCenter: parent.verticalCenter }
                        Text {
                            text: drawer.searching ? I18n.tr("Search") : (drawer.folio.activeTab ? drawer.folio.activeTab.title : "")
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(13)
                            color: Lithic.fg3
                            renderType: Text.NativeRendering
                        }
                        Glyph { visible: !drawer.searching; name: "chevron_right"; size: 14; ink: Lithic.fg3; anchors.verticalCenter: parent.verticalCenter }
                        Text {
                            visible: !drawer.searching
                            text: drawer.folio.activeSection ? drawer.folio.activeSection.title : ""
                            font.family: Lithic.font.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fs(13)
                            color: Lithic.fg2
                            renderType: Text.NativeRendering
                        }
                    }
                }
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 10 * drawer.s
                    Search {
                        id: search
                        width: drawer.slim ? 200 * drawer.s : 380 * drawer.s
                        anchors.verticalCenter: parent.verticalCenter
                        placeholderText: I18n.tr("Search every setting")
                        shortcutHint: "Ctrl F"
                        text: drawer.folio.query
                        onTextEdited: drawer.folio.query = text
                        Keys.onReturnPressed: drawer.openFirst()
                        Keys.onEnterPressed: drawer.openFirst()
                        Keys.onDownPressed: if (drawer.searching) results.forceActiveFocus()
                        Keys.onEscapePressed: (e) => {
                            if (text.length > 0) { drawer.folio.query = ""; text = "" }
                            else drawer.folio.closeRequested()
                            e.accepted = true
                        }
                    }
                    IslandButton {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "close"
                        tooltip: I18n.tr("Close")
                        onTriggered: drawer.folio.closeRequested()
                    }
                }
            }

            Row {
                id: quickRow
                visible: !drawer.slim
                height: visible ? implicitHeight : 0
                x: content.pad
                y: head.y + head.height + 20 * drawer.s
                spacing: 10 * drawer.s
                Repeater {
                    model: drawer.quick
                    QuickTile {
                        required property var modelData
                        width: (body.targetW - 56 * drawer.s - 30 * drawer.s) / 4
                        glyph: modelData.glyph
                        title: modelData.title
                        line: modelData.line
                        on: modelData.on
                        onTriggered: drawer.toggleQuick(modelData.id)
                    }
                }
            }

            Row {
                id: rail
                x: content.pad
                y: quickRow.y + quickRow.height + (drawer.slim ? 0 : 22 * drawer.s)
                spacing: 6 * drawer.s
                Repeater {
                    model: drawer.folio.tabs
                    IslandButton {
                        required property var modelData
                        required property int index
                        glyph: drawer.glyphFor(modelData.tabKey)
                        label: modelData.title
                        labelWhenOn: drawer.slim
                        on: !drawer.searching && drawer.folio.activeTabIndex === index
                        badge: drawer.slim && !on ? 0 : drawer.changedInTab(modelData)
                        tooltip: I18n.tr("Ctrl+%1").arg(index + 1)
                        onTriggered: { drawer.folio.query = ""; drawer.folio._selectTab(index) }
                    }
                }
            }
            Flow {
                id: slimSections
                visible: drawer.slim && !drawer.searching
                x: content.pad
                y: rail.y + rail.height + 10 * drawer.s
                width: content.width - content.pad * 2
                height: visible ? implicitHeight : 0
                spacing: 6 * drawer.s
                Repeater {
                    model: drawer.slim && drawer.folio.activeTab ? drawer.folio.activeTab.sections : []
                    FacetChip {
                        required property var modelData
                        required property int index
                        text: modelData.title
                        count: drawer.changedIn(modelData) > 0 ? String(drawer.changedIn(modelData)) : ""
                        on: index === drawer.folio.activeSectionIndex
                        onTriggered: drawer.folio._selectSection(drawer.folio.activeTabIndex, index)
                    }
                }
            }
            Rectangle {
                id: railRule
                x: content.pad
                y: (slimSections.visible ? slimSections.y + slimSections.height : rail.y + rail.height) + 14 * drawer.s
                width: content.width - content.pad * 2
                height: 1
                color: Lithic.vein
            }

            ListView {
                id: sections
                visible: !drawer.searching && !drawer.slim
                x: content.pad - 4 * drawer.s
                y: railRule.y + 18 * drawer.s
                width: 268 * drawer.s
                height: footer.y - y - 12 * drawer.s
                clip: true
                spacing: 4 * drawer.s
                boundsBehavior: Flickable.StopAtBounds
                model: drawer.folio.activeTab ? drawer.folio.activeTab.sections : []
                currentIndex: drawer.folio.activeSectionIndex
                highlightMoveDuration: Theme.standard
                highlightResizeDuration: Theme.standard
                highlight: CutRect {
                    width: sections.width
                    cut: LithicTokens.cut.small * drawer.s
                    fill: Lithic.container
                }
                delegate: Item {
                    id: secRow
                    required property var modelData
                    required property int index
                    readonly property bool open: index === drawer.folio.activeSectionIndex
                    readonly property int changed: drawer.changedIn(modelData)
                    width: sections.width
                    height: secCol.implicitHeight + 20 * drawer.s
                    CutRect {
                        anchors.fill: parent
                        cut: LithicTokens.cut.small * drawer.s
                        fill: Lithic.alpha(Lithic.fg, secArea.containsMouse && !secRow.open ? 0.06 : 0)
                    }
                    Column {
                        id: secCol
                        x: 14 * drawer.s
                        width: parent.width - 28 * drawer.s - (secRow.changed > 0 ? 26 * drawer.s : 0)
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 2 * drawer.s
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            text: secRow.modelData.title
                            font.family: Lithic.font.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fs(14)
                            color: secRow.open ? Lithic.containerInk : Lithic.fg
                            renderType: Text.NativeRendering
                        }
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            maximumLineCount: 1
                            text: secRow.modelData.subtitle || ""
                            visible: text.length > 0
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(12)
                            color: secRow.open ? Lithic.alpha(Lithic.containerInk, 0.75) : Lithic.fg3
                            renderType: Text.NativeRendering
                        }
                    }
                    CutRect {
                        visible: secRow.changed > 0
                        anchors.right: parent.right
                        anchors.rightMargin: 12 * drawer.s
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.max(20 * drawer.s, changedText.implicitWidth + 10 * drawer.s)
                        height: 20 * drawer.s
                        cut: 3 * drawer.s
                        fill: Lithic.alpha(Lithic.sel, secRow.open ? 0.3 : 0.16)
                        Text {
                            id: changedText
                            anchors.centerIn: parent
                            text: String(secRow.changed)
                            font.family: Lithic.font.mono
                            font.pixelSize: Theme.fs(10)
                            color: secRow.open ? Lithic.containerInk : Lithic.sel
                            renderType: Text.NativeRendering
                        }
                    }
                    MouseArea {
                        id: secArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: drawer.folio._selectSection(drawer.folio.activeTabIndex, secRow.index)
                    }
                }
            }

            CutRect {
                id: pagePlane
                visible: !drawer.searching
                x: drawer.slim ? content.pad : sections.x + sections.width + 16 * drawer.s
                y: sections.y
                width: content.width - x - content.pad
                height: sections.height
                cut: LithicTokens.cut.mid * drawer.s
                fill: Theme.planes.card
                strokeWidth: Qt.colorEqual(Theme.planes.cardStroke, "transparent") ? 0 : 1
                strokeColor: Theme.planes.cardStroke
                Item {
                    id: pageHost
                    anchors.fill: parent
                    clip: true
                }
            }

            ListView {
                id: results
                visible: drawer.searching
                x: content.pad
                y: railRule.y + 18 * drawer.s
                width: content.width - content.pad * 2
                height: footer.y - y - 12 * drawer.s
                clip: true
                spacing: 6 * drawer.s
                keyNavigationEnabled: true
                model: drawer.searching ? Finder.search(drawer.folio.query, drawer.folio.tabs, 24) : []
                highlightMoveDuration: Theme.fast
                highlight: CutRect {
                    width: results.width
                    cut: LithicTokens.cut.small * drawer.s
                    fill: Lithic.alpha(Lithic.fg, 0.08)
                    strokeWidth: results.activeFocus ? 1 : 0
                    strokeColor: Lithic.sel
                }
                Keys.onReturnPressed: drawer.openResult(results.currentIndex)
                Keys.onEnterPressed: drawer.openResult(results.currentIndex)
                Keys.onUpPressed: (e) => { if (results.currentIndex <= 0) search.forceActiveFocus(); else results.decrementCurrentIndex() }
                delegate: Item {
                    id: hit
                    required property var modelData
                    required property int index
                    width: results.width
                    height: 58 * drawer.s
                    Glyph {
                        x: 16 * drawer.s
                        anchors.verticalCenter: parent.verticalCenter
                        name: drawer.glyphFor(hit.modelData.tabKey)
                        size: 20
                        ink: Lithic.sel
                    }
                    Column {
                        x: 50 * drawer.s
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 3 * drawer.s
                        Text {
                            text: hit.modelData.title
                            font.family: Lithic.font.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fs(15)
                            color: Lithic.fg
                            renderType: Text.NativeRendering
                        }
                        Text {
                            text: hit.modelData.tabLabel + "  \u203a  " + hit.modelData.sectionTitle
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(12)
                            color: Lithic.fg3
                            renderType: Text.NativeRendering
                        }
                    }
                    Glyph {
                        anchors.right: parent.right
                        anchors.rightMargin: 16 * drawer.s
                        anchors.verticalCenter: parent.verticalCenter
                        name: "arrow_forward"
                        size: 18
                        ink: hitArea.containsMouse || results.currentIndex === hit.index ? Lithic.fg : Lithic.fg3
                    }
                    MouseArea {
                        id: hitArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onEntered: results.currentIndex = hit.index
                        onClicked: drawer.openResult(hit.index)
                    }
                }
                Text {
                    visible: results.count === 0
                    anchors.centerIn: parent
                    text: I18n.tr("Nothing matches \u201c%1\u201d. Try a shorter word.").arg(drawer.folio.query)
                    font.family: Lithic.font.sans
                    font.pixelSize: Theme.fs(14)
                    color: Lithic.fg3
                    renderType: Text.NativeRendering
                }
            }

            Row {
                id: footer
                x: content.pad
                y: content.height - height - 16 * drawer.s
                spacing: 18 * drawer.s
                Repeater {
                    model: [
                        { k: "Ctrl F", v: I18n.tr("search") },
                        { k: "Ctrl 1\u20135", v: I18n.tr("categories") },
                        { k: "Alt \u2191\u2193", v: I18n.tr("sections") },
                        { k: "Esc", v: I18n.tr("close") }
                    ]
                    Row {
                        required property var modelData
                        spacing: 6 * drawer.s
                        CutRect {
                            width: keyText.implicitWidth + 12 * drawer.s
                            height: 20 * drawer.s
                            cut: 3 * drawer.s
                            fill: Lithic.alpha(Lithic.fg, 0.07)
                            Text {
                                id: keyText
                                anchors.centerIn: parent
                                text: modelData.k
                                font.family: Lithic.font.mono
                                font.pixelSize: Theme.fs(9.5)
                                color: Lithic.fg2
                                renderType: Text.NativeRendering
                            }
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: modelData.v
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(12)
                            color: Lithic.fg3
                            renderType: Text.NativeRendering
                        }
                    }
                }
            }
        }
    }

    function focusSearch() { search.forceActiveFocus(); search.selectAll() }
    function openFirst() { if (results.count > 0) drawer.openResult(0) }
    function openResult(i) {
        var list = Finder.search(drawer.folio.query, drawer.folio.tabs, 24)
        if (i < 0 || i >= list.length) return
        var r = list[i]
        search.text = ""
        drawer.folio._openResult(r.tabIndex, r.sectionIndex, r.controlId)
    }
}
