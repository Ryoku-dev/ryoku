pragma ComponentBehavior: Bound
import QtQuick
import Quickshell.Io
import QtQuick.Dialogs
import "Singletons"
import Ryoku.Ui.Singletons

// The right-click menu for a plugin desktop tile, built on the shared
// DesktopMenu chrome in the quick-settings sidebar idiom. Beyond Lock + Hide it
// renders the plugin's own settings inline straight from its declared schema
// (manifest.metadata.settings), so a desktop widget is tuned in place without
// opening Settings.
//
// Controls are mouse-only (wallpaper layer = no keyboard):
//   choice -> chips, toggle -> switch, slider -> slider,
//   image  -> thumbnail strip scanned from ~/Pictures + a file chooser.
// Text fields are left to the hub. Each change emits
// settingChanged(id, key, value); the host persists via ryoku-plugins-place
// and the shell retunes live.
Item {
    id: menu

    anchors.fill: parent

    property string scope: ""
    property bool locked: false
    property var manifest: ({})
    property var placement: ({})
    property var vals: ({})         // live settings copy, optimistically updated
    property var pics: []           // scanned ~/Pictures paths for the image picker

    readonly property var schema: (manifest && manifest.metadata && manifest.metadata.settings) || []
    readonly property bool hasImage: schema.some(function (f) { return f.type === "image"; })

    // host-chrome (placement) values shown as menu controls: size and opacity
    // live in the tile's desktopWidget block, not the plugin's settings schema,
    // so every tile can resize and fade regardless of what it declares.
    readonly property var dw: (menu.placement && menu.placement.desktopWidget) || ({})
    property real sizeLive: 0.85
    property real opacityLive: 1
    // colour affordance, gated on the plugin declaring it honours a host accent.
    // colorAuto (default true) = the palette accent; colorAuto false + empty
    // colour = the plugin's own palette; a hex = a pinned colour.
    readonly property bool colorsCap: !!(menu.manifest && menu.manifest.capabilities
        && menu.manifest.capabilities.colors === true)
    readonly property bool colorAutoOn: menu.vals.colorAuto !== false
    readonly property string colorHex: (typeof menu.vals.color === "string") ? menu.vals.color : ""
    readonly property string colorMode: menu.colorAutoOn ? "auto" : (menu.colorHex.length > 0 ? "custom" : "default")

    signal hideRequested(string id)
    signal lockToggled(string id)
    signal settingChanged(string id, string key, var value)
    signal sizeChanged(string id, real scale)
    signal opacityChanged(string id, real opacity)

    function openFor(id, locked, x, y, manifest, placement) {
        menu.scope = id;
        menu.locked = locked;
        shell.px = x;
        shell.py = y;
        menu.manifest = manifest || ({});
        menu.placement = placement || ({});
        menu.vals = JSON.parse(JSON.stringify((placement && placement.settings) || {}));
        const g = (placement && placement.desktopWidget) || ({});
        menu.sizeLive = (g.scale !== undefined) ? g.scale : 0.85;
        menu.opacityLive = (g.opacity !== undefined) ? g.opacity : 1;
        shell.open = true;
        if (menu.hasImage)
            picScan.running = true;
    }
    function close() { shell.open = false; }

    function val(field) {
        return (menu.vals && menu.vals[field.key] !== undefined) ? menu.vals[field.key] : field.default;
    }
    function set(key, value) {
        var n = JSON.parse(JSON.stringify(menu.vals || {}));
        n[key] = value;
        menu.vals = n;
        menu.settingChanged(menu.scope, key, value);
    }
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    // Auto clears the pin; Default drops it to the plugin's own palette; Custom
    // pins a colour, seeding the palette accent when none is set yet so the
    // picker opens on a real base (mirrors the built-in setColorMode).
    function setColorMode(m) {
        if (m === "auto") {
            menu.set("colorAuto", true);
            return;
        }
        if (m === "default") {
            menu.set("colorAuto", false);
            menu.set("color", "");
            return;
        }
        menu.set("colorAuto", false);
        if (menu.colorHex.length === 0)
            menu.set("color", menu.hexOf(Scheme.accent));
    }

    // scan ~/Pictures (one level deep) for picker thumbnails.
    Process {
        id: picScan
        command: ["bash", "-c",
            "find \"${XDG_PICTURES_DIR:-$HOME/Pictures}\" -maxdepth 2 -type f \\( -iname '*.jpg' -o -iname '*.png' -o -iname '*.jpeg' -o -iname '*.webp' -o -iname '*.gif' \\) 2>/dev/null | head -24"]
        stdout: StdioCollector {
            onStreamFinished: menu.pics = text.split("\n").filter(function (l) { return l.trim().length > 0; })
        }
    }

    DesktopMenu {
        id: shell
        title: (menu.manifest && menu.manifest.name) ? menu.manifest.name : menu.scope
        gloss: "部品"
        cardWidth: menu.schema.length > 0 ? 300 : 248

        MenuRow {
            label: I18n.tr("Lock")
            value: menu.locked ? "On" : "Off"
            on: menu.locked
            closeOnTrigger: false
            onTriggered: {
                menu.lockToggled(menu.scope);
                menu.locked = !menu.locked;
            }
        }
        MenuRow {
            label: I18n.tr("Hide")
            onTriggered: menu.hideRequested(menu.scope)
        }

        // ── size + opacity (host chrome: placement, not the plugin's schema) ──
        // The discoverable equivalents of the corner bracket / Ctrl+wheel and a
        // tile fade; every tile can resize and fade. Both commit on release.
        MenuSection { label: I18n.tr("Adjust"); gloss: "調整" }
        MenuSlider {
            id: sizeSlider
            label: I18n.tr("Size")
            from: 0.5
            to: 2.5
            step: 0.02
            value: menu.sizeLive
            valueText: Math.round(sizeSlider.value * 100) + "%"
            onMoved: (v) => menu.sizeLive = v
            onReleased: (v) => { menu.sizeLive = v; menu.sizeChanged(menu.scope, v); }
        }
        MenuSlider {
            id: opacitySlider
            label: I18n.tr("Opacity")
            from: 0.2
            to: 1.0
            step: 0.01
            value: menu.opacityLive
            valueText: Math.round(opacitySlider.value * 100) + "%"
            onMoved: (v) => menu.opacityLive = v
            onReleased: (v) => { menu.opacityLive = v; menu.opacityChanged(menu.scope, v); }
        }

        // ── colour (only when the plugin declares it honours a host accent) ──
        // Auto = the palette accent (matugen-derived), Default = the plugin's own
        // palette, Custom = a pinned colour. Mirrors the built-in WidgetMenu's
        // colour rows; the pin persists through settings (colorAuto + color).
        MenuSection { visible: menu.colorsCap; label: I18n.tr("Colour"); gloss: "彩色" }
        Column {
            visible: menu.colorsCap
            width: parent.width
            spacing: Theme.s1
            Row {
                id: colorModeRow
                width: parent.width
                spacing: Theme.s1
                readonly property real cw: (width - 2 * Theme.s1) / 3
                MenuChip {
                    width: colorModeRow.cw; height: Theme.ctlH
                    label: I18n.tr("Auto"); selected: menu.colorMode === "auto"
                    onClicked: menu.setColorMode("auto")
                }
                MenuChip {
                    width: colorModeRow.cw; height: Theme.ctlH
                    label: I18n.tr("Default"); selected: menu.colorMode === "default"
                    onClicked: menu.setColorMode("default")
                }
                MenuChip {
                    width: colorModeRow.cw; height: Theme.ctlH
                    label: I18n.tr("Custom"); selected: menu.colorMode === "custom"
                    onClicked: menu.setColorMode("custom")
                }
            }
            PluginColorPicker {
                width: parent.width
                visible: menu.colorMode === "custom"
                seed: menu.colorHex
                onCommitted: (hex) => menu.set("color", hex)
            }
        }

        // ── settings, rendered from the plugin's schema ────────────────
        Column {
            visible: menu.schema.length > 0
            width: parent.width
            spacing: Theme.s3

            MenuSection {}

            Repeater {
                model: menu.schema

                delegate: Column {
                    id: fieldWrap
                    required property var modelData
                    required property int index
                    width: parent.width
                    spacing: Theme.s2

                    readonly property var f: fieldWrap.modelData
                    readonly property string grp: fieldWrap.f.group || ""
                    readonly property bool startsGroup: fieldWrap.index === 0
                        || ((menu.schema[fieldWrap.index - 1].group || "") !== fieldWrap.grp)
                    // no keyboard on the wallpaper layer -- text fields live in the hub.
                    readonly property bool shown: fieldWrap.f.type !== "text"

                    visible: fieldWrap.shown

                    // group eyebrow when this field opens a named group.
                    MenuSection {
                        visible: fieldWrap.startsGroup && fieldWrap.grp.length > 0
                        label: fieldWrap.grp
                        gloss: "部品"
                    }

                    // choice -> chips
                    Column {
                        visible: fieldWrap.f.type === "choice"
                        width: parent.width
                        spacing: Theme.s2
                        Text {
                            text: fieldWrap.f.label || fieldWrap.f.key
                            color: Theme.inkSoft
                            font.family: Theme.font
                            font.pixelSize: Theme.fSmall
                            font.weight: Font.Medium
                        }
                        Flow {
                            width: parent.width
                            spacing: Theme.s2
                            Repeater {
                                model: fieldWrap.f.options || []
                                delegate: MenuChip {
                                    id: opt
                                    required property var modelData
                                    selected: String(menu.val(fieldWrap.f)) === String(opt.modelData.value)
                                    label: I18n.tr(opt.modelData.label)
                                    onClicked: menu.set(fieldWrap.f.key, opt.modelData.value)
                                }
                            }
                        }
                    }

                    // toggle -> switch
                    Item {
                        visible: fieldWrap.f.type === "toggle"
                        width: parent.width
                        height: Theme.ctlH
                        Text {
                            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                            text: fieldWrap.f.label || fieldWrap.f.key
                            color: Theme.inkSoft
                            font.family: Theme.font
                            font.pixelSize: Theme.fSmall
                            font.weight: Font.Medium
                        }
                        Rectangle {
                            id: sw
                            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                            readonly property bool on: menu.val(fieldWrap.f) === true || menu.val(fieldWrap.f) === "true"
                            width: Theme.s6 + Theme.s2
                            height: Theme.s5
                            radius: Theme.menuTileRadius
                            color: sw.on ? Theme.bone : Theme.tile
                            border.width: 1
                            border.color: sw.on ? Theme.bone : Theme.line
                            Behavior on color { ColorAnimation { duration: Theme.quick } }
                            Rectangle {
                                width: sw.height - Theme.s2
                                height: sw.height - Theme.s2
                                radius: width / 2
                                y: Theme.s1
                                x: sw.on ? parent.width - width - Theme.s1 : Theme.s1
                                color: sw.on ? Theme.inkOnBone : Theme.inkDim
                                Behavior on x { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
                            }
                            TapHandler { onTapped: menu.set(fieldWrap.f.key, !sw.on) }
                            HoverHandler { cursorShape: Qt.PointingHandCursor }
                        }
                    }

                    // slider -> track + knob, commits on release.
                    Item {
                        id: slRow
                        visible: fieldWrap.f.type === "slider"
                        width: parent.width
                        height: Theme.s6
                        readonly property real lo: fieldWrap.f.min !== undefined ? fieldWrap.f.min : 0
                        readonly property real hi: fieldWrap.f.max !== undefined ? fieldWrap.f.max : 1
                        readonly property int dec: fieldWrap.f.decimals !== undefined ? fieldWrap.f.decimals : 2
                        property real live: Number(menu.val(fieldWrap.f))
                        readonly property real frac: slRow.hi > slRow.lo ? Math.max(0, Math.min(1, (slRow.live - slRow.lo) / (slRow.hi - slRow.lo))) : 0
                        Text {
                            id: slLbl
                            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                            width: Theme.s5 * 3
                            elide: Text.ElideRight
                            text: fieldWrap.f.label || fieldWrap.f.key
                            color: Theme.inkSoft
                            font.family: Theme.font
                            font.pixelSize: Theme.fSmall
                            font.weight: Font.Medium
                        }
                        Text {
                            id: slVal
                            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                            width: Theme.s6 + Theme.s2
                            horizontalAlignment: Text.AlignRight
                            text: slRow.live.toFixed(slRow.dec)
                            color: Theme.inkDim
                            font.family: Theme.mono
                            font.pixelSize: Theme.fMicro
                        }
                        Item {
                            id: track
                            anchors { left: slLbl.right; leftMargin: Theme.s3; right: slVal.left; rightMargin: Theme.s3; verticalCenter: parent.verticalCenter }
                            height: Theme.s5
                            Rectangle {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width
                                height: Theme.s1
                                radius: Theme.menuTileRadius
                                color: Theme.tile
                                Rectangle {
                                    width: Math.round(parent.width * slRow.frac)
                                    height: parent.height
                                    radius: Theme.menuTileRadius
                                    color: Theme.ink
                                }
                            }
                            Rectangle {
                                width: Theme.s4
                                height: Theme.s4
                                radius: width / 2
                                anchors.verticalCenter: parent.verticalCenter
                                x: Math.round((track.width - width) * slRow.frac)
                                color: Theme.surface
                                border.width: 1
                                border.color: Theme.ink
                            }
                            MouseArea {
                                anchors.fill: parent
                                anchors.margins: -Theme.s2
                                preventStealing: true
                                cursorShape: Qt.PointingHandCursor
                                function at(mx) {
                                    var fr = Math.max(0, Math.min(1, mx / track.width));
                                    var v = slRow.lo + fr * (slRow.hi - slRow.lo);
                                    var st = fieldWrap.f.step || 0.01;
                                    return Math.round(v / st) * st;
                                }
                                onPositionChanged: (m) => { if (pressed) slRow.live = at(m.x); }
                                onPressed: (m) => slRow.live = at(m.x)
                                onReleased: menu.set(fieldWrap.f.key, slRow.dec === 0 ? Math.round(slRow.live) : slRow.live)
                            }
                        }
                    }

                    // image -> thumb strip (Default + Browse + ~/Pictures)
                    Column {
                        visible: fieldWrap.f.type === "image"
                        width: parent.width
                        spacing: Theme.s2
                        Text {
                            text: fieldWrap.f.label || fieldWrap.f.key
                            color: Theme.inkSoft
                            font.family: Theme.font
                            font.pixelSize: Theme.fSmall
                            font.weight: Font.Medium
                        }
                        Flickable {
                            width: parent.width
                            height: Theme.s7 + Theme.s1
                            contentWidth: strip.implicitWidth
                            clip: true
                            interactive: contentWidth > width
                            boundsBehavior: Flickable.StopAtBounds
                            Row {
                                id: strip
                                spacing: Theme.s2
                                // "Default" clears the path -> bundled sample.
                                Rectangle {
                                    width: Theme.s6 * 2
                                    height: Theme.s7
                                    radius: Theme.menuTileRadius
                                    color: Theme.tile
                                    border.width: String(menu.val(fieldWrap.f)).length === 0 ? 2 : 1
                                    border.color: String(menu.val(fieldWrap.f)).length === 0 ? Theme.bone : Theme.line
                                    Text {
                                        anchors.centerIn: parent
                                        text: I18n.tr("Default")
                                        color: Theme.inkDim
                                        font.family: Theme.font
                                        font.pixelSize: Theme.fMicro
                                        font.weight: Font.Medium
                                    }
                                    TapHandler { onTapped: menu.set(fieldWrap.f.key, "") }
                                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                                }
                                // "Browse" -> system file chooser (portal).
                                Rectangle {
                                    width: Theme.s6 * 2
                                    height: Theme.s7
                                    radius: Theme.menuTileRadius
                                    color: brHov.hovered ? Theme.tileHover : Theme.tile
                                    border.width: 1
                                    border.color: brHov.hovered ? Theme.line : Theme.line
                                    Behavior on color { ColorAnimation { duration: Theme.quick } }
                                    Text {
                                        anchors.centerIn: parent
                                        text: I18n.tr("+ Browse")
                                        color: brHov.hovered ? Theme.ink : Theme.inkDim
                                        font.family: Theme.font
                                        font.pixelSize: Theme.fMicro
                                        font.weight: Font.Medium
                                    }
                                    HoverHandler { id: brHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler { onTapped: imgFileDlg.open() }
                                }
                                Repeater {
                                    model: menu.pics
                                    delegate: Rectangle {
                                        id: thumb
                                        required property var modelData
                                        readonly property bool sel: String(menu.val(fieldWrap.f)) === ("file://" + thumb.modelData)
                                        width: Theme.s6 * 2
                                        height: Theme.s7
                                        radius: Theme.menuTileRadius
                                        color: Theme.tile
                                        border.width: thumb.sel ? 2 : 1
                                        border.color: thumb.sel ? Theme.bone : Theme.line
                                        clip: true
                                        Image {
                                            anchors.fill: parent
                                            anchors.margins: Theme.s1
                                            source: "file://" + thumb.modelData
                                            fillMode: Image.PreserveAspectCrop
                                            asynchronous: true
                                            cache: true
                                            sourceSize.width: 132
                                        }
                                        TapHandler { onTapped: menu.set(fieldWrap.f.key, "file://" + thumb.modelData) }
                                        HoverHandler { cursorShape: Qt.PointingHandCursor }
                                    }
                                }
                            }
                        }
                        FileDialog {
                            id: imgFileDlg
                            title: I18n.tr("Choose an image")
                            nameFilters: ["Images (*.png *.jpg *.jpeg *.webp *.gif *.bmp)", "All files (*)"]
                            onAccepted: menu.set(fieldWrap.f.key, "" + imgFileDlg.selectedFile)
                        }
                    }
                }
            }
        }
    }
}
