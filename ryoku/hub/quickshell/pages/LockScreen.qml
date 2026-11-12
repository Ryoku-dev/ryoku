import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../schema/lock-screen.js" as Schema

// the lock screen's design: the gallery of installed designs, the one in use, and a way to unlocking
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property bool landed: false
    // a bespoke row the search landed on, spotlit until the timer clears it
    property string spot: ""
    // the design gallery once its body is built, where gallery deep links land
    property Item galleryGrid: null
    // the page's own rows by schema id, where their deep links land
    property var landRows: ({})

    readonly property var store: Changes.stores["lockdesign"] || null
    readonly property var bind: Schema.designBind
    readonly property string lockLauncher: "ryoku-qylock-lock"

    readonly property var skins: root.store ? root.store.skins : []
    readonly property string activeSlug: {
        Changes.revision;
        return root.store ? root.store.inUse : "";
    }
    readonly property var activeSkin: {
        Changes.revision;
        return root.store && root.activeSlug !== "" ? root.store.skin(root.activeSlug) : null;
    }
    readonly property string activeName: root.activeSkin ? root.activeSkin.name : ""
    readonly property string pending: root.store ? root.store.pending : ""
    readonly property string error: Changes.error(root.bind)

    readonly property bool clock24: Settings.get("general.clock_format_24_h") === true
    property bool keyPrompts: false
    property bool fingerprintOn: true
    property bool keyOn: false

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true)
    })

    readonly property string stateLine: root.activeName !== "" ? I18n.tr("In use: %1").arg(root.activeName) : ""

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    Connections {
        target: root.store
        function onLoadingChanged() {
            if (!root.store.loading && root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        root.landed = true;
        // the gallery's height moves every section below it, so land once it is filled and placed
        if (page.layout.length === 0 || !root.store || root.store.loading) {
            root.pendingRow = rowId;
            return;
        }
        landLater.rowId = rowId;
        landLater.restart();
    }

    function land(rowId) {
        if (page.sectionOfRow(rowId)) {
            page.focusRow(rowId);
            return;
        }
        // the gallery is the design section's own control, so a link to either lands on it,
        // not on the store row below it
        if ((rowId === "design-gallery" || rowId === "design") && root.galleryGrid && root.galleryGrid.visible) {
            page.reveal(root.galleryGrid);
            root.galleryGrid.forceActiveFocus(Qt.OtherFocusReason);
            return;
        }
        // a bespoke row: bring it into view and, when it acts, give it the keys, so
        // Return on a link to Clock opens Clock rather than the section's first row
        var drawn = root.landRows[rowId];
        if (drawn && drawn.visible) {
            page.reveal(drawn);
            if (drawn.activatable)
                drawn.forceActiveFocus(Qt.OtherFocusReason);
            root.spot = rowId;
            spotClear.restart();
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a deep link may name a section rather than a row
        var at = root.sectionIndex(row ? row.section : rowId);
        if (at >= 0)
            page.chooseSection(at);
        if (row && row.id !== "design-gallery") {
            root.spot = row.id;
            spotClear.restart();
        }
    }

    Timer {
        id: landLater
        property string rowId: ""
        interval: 150
        onTriggered: root.land(landLater.rowId)
    }

    function use(slug) {
        if (!root.store || slug === "" || root.pending !== "")
            return;
        if (slug === root.activeSlug) {
            root.preview(slug);
            return;
        }
        Changes.set(root.bind, slug, "Lock screen design", "lock-screen",
                    I18n.tr("Lock screen is now %1").arg(root.store.nameOf(slug)));
    }

    function preview(slug) {
        if (slug !== "")
            Spawn.runApp([root.lockLauncher, slug], { label: "lockscreen-preview" });
    }

    function readUnlocking() {
        unlockRead.running = true;
    }

    onActiveSlugChanged: {
        if (root.activeSlug === "")
            return;
        promptsRead.command = ["sh", "-c", "grep -qs 'sddm.keyPrompts = true' \"${XDG_DATA_HOME:-$HOME/.local/share}/qylock/themes/$1/Main.qml\"", "sh", root.activeSlug];
        promptsRead.running = true;
    }

    Component.onCompleted: {
        if (root.store)
            root.store.reload();
        else
            Changes.ensureStore("lockdesign", Qt.resolvedUrl("../components/lock-screen/LockDesignStore.qml"));
        root.readUnlocking();
    }

    Timer {
        id: spotClear
        interval: LithicTokens.motion.highlight + 400
        onTriggered: root.spot = ""
    }

    // a switch shows Working once it has run past a second
    Timer {
        id: slowMark
        interval: 1000
        onTriggered: root.slow = true
    }
    property bool slow: false
    onPendingChanged: {
        root.slow = false;
        if (root.pending !== "")
            slowMark.restart();
        else
            slowMark.stop();
    }

    Process {
        id: promptsRead
        onExited: code => root.keyPrompts = code === 0
    }

    // the lock's own switches: the fingerprint preference and the security key's lock conversation
    Process {
        id: unlockRead
        command: ["sh", "-c", "cat \"$HOME/.config/qylock/fingerprint\" 2>/dev/null || echo on; [ -e \"${XDG_CONFIG_HOME:-$HOME/.config}/qylock/pam/ryoku-lock-key\" ] && echo key || echo nokey"]
        stdout: StdioCollector { id: unlockOut }
        onExited: {
            var lines = unlockOut.text.trim().split("\n");
            var fp = (lines[0] || "on").trim().toLowerCase();
            root.fingerprintOn = !(fp === "off" || fp === "0" || fp === "false");
            root.keyOn = (lines[lines.length - 1] || "").trim() === "key";
        }
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "lock-screen"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ design: galleryBody, "this-design": thisBody, unlocking: unlockBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: galleryBody

        Column {
            id: gallery
            spacing: LithicTokens.space.s3

            property string selectedSlug: ""
            // until the user picks a tile, the selection follows the design in use
            property bool picked: false
            property bool revealed: false
            property bool refreshing: false

            readonly property var tiles: root.skins.map(s => {
                var current = s.slug === root.activeSlug;
                var busy = s.slug === root.pending;
                return {
                    id: s.slug,
                    title: s.name,
                    status: busy && !current ? I18n.tr("Applying")
                        : current && busy ? I18n.tr("Waiting for your password")
                        : current ? I18n.tr("In use")
                        : (s.theme !== "" ? s.theme : I18n.tr("Installed")),
                    source: s.preview,
                    current: current,
                    busy: busy,
                    actionText: current ? I18n.tr("Preview") : I18n.tr("Use"),
                    actionKind: "line"
                };
            })
            readonly property var selected: root.store ? root.store.skin(gallery.selectedSlug) : null

            function indexOf(slug) {
                for (var i = 0; i < gallery.tiles.length; i++)
                    if (gallery.tiles[i].id === slug)
                        return i;
                return -1;
            }

            function sync() {
                gallery.refreshing = false;
                if (gallery.tiles.length === 0)
                    return;
                if (!gallery.picked && gallery.indexOf(root.activeSlug) >= 0)
                    gallery.selectedSlug = root.activeSlug;
                else if (gallery.indexOf(gallery.selectedSlug) < 0)
                    gallery.selectedSlug = gallery.tiles[0].id;
                grid.currentIndex = gallery.indexOf(gallery.selectedSlug);
                if (!gallery.revealed && gallery.selectedSlug === root.activeSlug) {
                    gallery.revealed = true;
                    revealLater.restart();
                }
            }

            // the column places the grid on its next polish, so measure after it
            Timer {
                id: revealLater
                interval: 120
                onTriggered: gallery.reveal()
            }

            // bring the design in use into view once per open, unless search landed elsewhere
            function reveal() {
                if (root.landed || grid.currentIndex < 0)
                    return;
                var row = Math.floor(grid.currentIndex / grid.columns);
                var top = grid.y + row * (grid.tileHeight + grid.gap);
                var y0 = gallery.mapToItem(page, 0, top).y;
                var y1 = y0 + grid.tileHeight;
                if (y1 > page.height)
                    page.scrollTo(y0 - (page.height - grid.tileHeight) / 2);
            }

            onTilesChanged: {
                gallery.refreshing = true;
                Qt.callLater(gallery.sync);
            }
            Component.onCompleted: gallery.sync()
            Connections {
                target: root
                function onActiveSlugChanged() { gallery.sync(); }
            }

            Row {
                visible: !!root.store && root.store.loading
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Reading your installed designs")
                    typeRole: "detail"
                }
            }

            Row {
                visible: !!root.store && root.store.loadFailed && root.skins.length === 0
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - listRetry.width - LithicTokens.space.s3)
                    text: I18n.tr("Could not read the installed designs. Retry, and if it keeps failing, run ryoku doctor.")
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: listRetry
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: root.store.reload()
                }
            }

            Empty {
                visible: !!root.store && !root.store.loading && !root.store.loadFailed && root.skins.length === 0
                width: parent.width
                glyph: "lock"
                title: I18n.tr("No designs installed")
                body: I18n.tr("Lock screen designs come from RyoStore. Install one there and it shows up here.")
                actions: [{ text: I18n.tr("Browse RyoStore"), kind: "line", glyph: "open_in_new" }]
                onActionTriggered: index => Spawn.runApp(["ryostore", "open", "lockscreens"], { label: "ryostore" })
            }

            DesignGallery {
                id: grid
                visible: gallery.tiles.length > 0
                width: parent.width
                accessibleName: I18n.tr("Lock screen designs")
                model: gallery.tiles
                Component.onCompleted: root.galleryGrid = grid
                onActivated: index => {
                    var tile = gallery.tiles[index];
                    if (tile)
                        root.use(tile.id);
                }
                onCurrentIndexChanged: {
                    if (gallery.refreshing || currentIndex < 0 || currentIndex >= gallery.tiles.length)
                        return;
                    if (grid.activeFocus)
                        gallery.picked = true;
                    gallery.selectedSlug = gallery.tiles[currentIndex].id;
                }

                delegate: DesignTile {
                    required property var modelData
                    required property int index

                    width: grid.tileWidth
                    height: grid.tileHeight
                    focusPolicy: Qt.NoFocus
                    title: modelData.title
                    status: modelData.status
                    source: modelData.source
                    current: modelData.current
                    busy: modelData.busy
                    enabled: root.pending === "" || modelData.busy
                    actionText: root.pending === "" ? modelData.actionText : ""
                    actionKind: modelData.actionKind
                    selected: index === grid.currentIndex
                    focusVisible: selected && grid.visualFocus
                    onClicked: {
                        gallery.picked = true;
                        grid.forceActiveFocus(Qt.MouseFocusReason);
                        grid.select(index);
                    }
                    onActionTriggered: root.use(modelData.id)
                }
            }

            // the selected design in words, with its two actions
            Item {
                visible: !!gallery.selected
                width: parent.width
                implicitHeight: Math.max(about.implicitHeight, selectedActions.implicitHeight)

                Label {
                    id: about
                    anchors.left: parent.left
                    anchors.right: selectedActions.left
                    anchors.rightMargin: LithicTokens.space.s4
                    anchors.verticalCenter: parent.verticalCenter
                    text: {
                        var s = gallery.selected;
                        if (!s)
                            return "";
                        // a summary that only repeats the name, maybe with "Theme" or the
                        // packaging's name around it ("forest Qylock Theme"), says nothing more
                        var words = t => t.toLowerCase().split(/[\s\-_.,:;()]+/)
                            .filter(w => w !== "" && w !== "theme" && w !== "qylock" && w !== "sddm");
                        var named = words(s.name);
                        var parts = [s.summary, s.blurb].filter(p => p && p.length > 0).map(p => p.replace(/[.\s]+$/, ""))
                            .filter(p => words(p).some(w => named.indexOf(w) < 0));
                        return parts.length > 0 ? s.name + ": " + parts.join(". ") + "." : s.name;
                    }
                    typeRole: "body"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                Row {
                    id: selectedActions
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: LithicTokens.space.s2

                    Plaque {
                        kind: "quiet"
                        small: true
                        glyph: "play_arrow"
                        text: I18n.tr("Preview")
                        accessibleName: I18n.tr("Preview %1").arg(gallery.selected ? gallery.selected.name : "")
                        onClicked: root.preview(gallery.selectedSlug)
                    }
                    Plaque {
                        visible: gallery.selectedSlug !== root.activeSlug
                        kind: "line"
                        small: true
                        text: I18n.tr("Use")
                        busy: root.pending === gallery.selectedSlug
                        enabled: root.pending === ""
                        accessibleName: I18n.tr("Use %1").arg(gallery.selected ? gallery.selected.name : "")
                        onClicked: root.use(gallery.selectedSlug)
                    }
                }
            }

            Label {
                width: parent.width
                text: I18n.tr("Using a design restyles the sign-in screen too, so your computer asks for your password. If you cancel, only the lock screen changes.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            // what the switch in flight is waiting for
            Row {
                visible: root.pending !== ""
                width: parent.width
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: root.slow && root.pending !== ""
                    visible: active
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - LithicTokens.space.s2 - LithicTokens.size.iconPlaqueIcon
                    text: !root.store ? ""
                        : root.store.greeterPending
                        ? I18n.tr("Your lock screen now uses %1. Enter your password in the admin prompt to restyle the sign-in screen too.").arg(root.store.nameOf(root.pending))
                        : I18n.tr("Switching to %1").arg(root.store.nameOf(root.pending))
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }

            Label {
                visible: root.pending === "" && root.error === "" && !!root.store && !!root.store.result && root.store.result.ok
                width: parent.width
                text: root.store && root.store.result ? I18n.tr("The lock screen and the sign-in screen now use %1.").arg(root.store.nameOf(root.store.result.slug)) : ""
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Row {
                visible: root.pending === "" && root.error !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - setRetry.width - LithicTokens.space.s3)
                    text: root.error
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: setRetry
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: Changes.retry(root.bind)
                }
            }
        }
    }

    Component {
        id: thisBody

        Slab {
            Column {
                width: parent.width

                Label {
                    visible: root.activeSkin === null
                    width: parent.width
                    topPadding: LithicTokens.space.s2
                    bottomPadding: LithicTokens.space.s2
                    text: I18n.tr("No design is in use yet. Choose one above.")
                    typeRole: "detail"
                    inkRole: "fg3"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                SettingRow {
                    visible: root.activeSkin !== null
                    width: parent.width
                    first: true
                    activatable: true
                    highlighted: root.spot === "preview"
                    id: previewRow
                    Component.onCompleted: root.landRows["preview"] = previewRow
                    title: I18n.tr("Preview")
                    detail: I18n.tr("Try %1 on the real lock screen, then unlock with your password to come back").arg(root.activeName)
                    onActivated: root.preview(root.activeSlug)

                    Plaque {
                        kind: "quiet"
                        small: true
                        glyph: "play_arrow"
                        text: I18n.tr("Preview")
                        accessibleName: I18n.tr("Preview %1").arg(root.activeName)
                        onClicked: root.preview(root.activeSlug)
                    }
                }

                SettingRow {
                    visible: root.activeSkin !== null
                    width: parent.width
                    highlighted: root.spot === "key-prompts"
                    id: keyRow
                    Component.onCompleted: root.landRows["key-prompts"] = keyRow
                    title: I18n.tr("Security key prompts")
                    detail: root.keyPrompts
                        ? I18n.tr("%1 can ask for your security key's PIN, so a key that needs one works here")
                            .arg(root.activeName)
                        : I18n.tr("%1 cannot ask for a PIN. A touch-only key works here; a key that needs a PIN works with designs that can ask, such as Orbital")
                            .arg(root.activeName)

                    Label {
                        text: root.keyPrompts ? I18n.tr("Shown") : I18n.tr("Not shown")
                        typeRole: "label"
                        inkRole: "fg2"
                    }
                }

                SettingRow {
                    visible: root.activeSkin !== null
                    width: parent.width
                    activatable: true
                    highlighted: root.spot === "clock"
                    id: clockRow
                    Component.onCompleted: root.landRows["clock"] = clockRow
                    title: I18n.tr("Clock")
                    detail: (root.clock24 ? I18n.tr("Shows 24-hour time") : I18n.tr("Shows 12-hour time with AM and PM"))
                        + ". " + I18n.tr("It follows your time format in Time & language")
                    onActivated: root.hub.navigate("time-language", "clock-24h")

                    Glyph {
                        name: "arrow_forward"
                        size: LithicTokens.size.plaqueIcon
                        ink: Lithic.fg2
                    }
                }
            }
        }
    }

    Component {
        id: unlockBody

        Slab {
            Column {
                width: parent.width

                SettingRow {
                    width: parent.width
                    first: true
                    activatable: true
                    highlighted: root.spot === "unlocking"
                    id: unlockRow
                    Component.onCompleted: root.landRows["unlocking"] = unlockRow
                    title: I18n.tr("Sign-in & security")
                    detail: {
                        var ways = [I18n.tr("your password")];
                        if (root.fingerprintOn)
                            ways.push(I18n.tr("a fingerprint"));
                        if (root.keyOn)
                            ways.push(I18n.tr("a security key"));
                        var list = ways.length === 1 ? ways[0]
                            : ways.slice(0, -1).join(", ") + " " + I18n.tr("or") + " " + ways[ways.length - 1];
                        return I18n.tr("The lock screen opens with %1. Choose what unlocks it, and where else each works.").arg(list);
                    }
                    onActivated: root.hub.navigate("sign-in", "")

                    Glyph {
                        name: "arrow_forward"
                        size: LithicTokens.size.plaqueIcon
                        ink: Lithic.fg2
                    }
                }
            }
        }
    }
}
