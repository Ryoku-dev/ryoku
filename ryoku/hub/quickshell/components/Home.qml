import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "about"
import "../registry/registry.js" as Registry
import "../registry/summaries.js" as Summaries

// Home: the profile plate. The reader's figure and name on the ground, what needs
// them, and this computer's readings; its look is theirs to change (profile.json)
Item {
    id: root

    required property var hub

    readonly property bool compact: root.hub.sizeClass === "compact"
    // from here the figure stands beside the column; narrower, behind its head
    readonly property bool roomy: root.width >= LithicTokens.size.boardColumnsAt
    readonly property real side: root.roomy ? LithicTokens.space.s6 : LithicTokens.space.s4
    // text never lies on the marble: the column keeps to its side and the statue stands
    // clear of it
    readonly property real columnWidth: !root.roomy ? root.width - 2 * root.side
        : Math.min(LithicTokens.size.dialogWidth + LithicTokens.space.s5, Math.round(root.width * 0.46) - root.side)
    // the stock figure is the site's: the Caracalla Farnese, a photograph (CC BY 2.5). Its
    // museum label stands upright in a gutter at the plate's outer edge, beside the figure
    readonly property bool stockFigure: (root.f("hero", ({})).kind || "default") === "default"
    readonly property bool labelled: root.stockFigure && root.roomy && flick.height >= creditLabel.width + LithicTokens.space.s6
    readonly property real gutter: root.labelled ? LithicTokens.space.s4 + creditLabel.height + LithicTokens.space.s3 : LithicTokens.space.s2
    // a narrow plate stands the figure beside the name, under the top bar, where the plate
    // is wide enough to leave the name its room; the words keep clear of it there too
    readonly property real besideWidth: 176
    readonly property bool beside: !root.roomy && root.width >= 2 * root.side + 280 + root.besideWidth
    readonly property real besideRoom: root.beside ? root.besideWidth + LithicTokens.space.s4 : 0

    readonly property bool updateKnown: Updates.lastChecked !== null
    readonly property var failures: Changes.failures()
    readonly property bool serviceDown: root.hub.serviceDown
    readonly property bool needsYou: Updates.available || Attention.items.length > 0 || root.failures.length > 0
    readonly property string continuePage: {
        var id = Registry.resolve(root.hub.lastSection);
        return id !== "" && root.hub.pageAvailable(id) && !Registry.page(id).handoff ? id : "";
    }

    // the profile's design, shared with the system card
    function f(path, def) { return ProfileStore.get(path, def); }
    function blockOn(id) { return root.f("blocks." + id, true) === true; }
    readonly property bool heroLeft: root.f("heroSide", "right") === "left"

    function title(s) {
        s = String(s || "");
        return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
    }

    // an Attention string, translated, with %1, %2 from its args
    function filled(text, args) {
        var out = text ? I18n.tr(text) : "";
        (args || []).forEach((a, i) => out = out.replace("%" + (i + 1), a));
        return out;
    }

    function focusRow(rowId) {}

    // the look's editor, made the first time it is asked for
    function customise() {
        Changes.ensureStore("card", Qt.resolvedUrl("about/CardStore.qml"));
        editorLoader.active = true;
        editorLoader.item.opener = customisePlaque;
        editorLoader.item.open();
    }

    Loader {
        id: editorLoader
        active: false
        sourceComponent: Component {
            CardEditor {
                parent: root.hub.overlayItem
                pageId: ""
                home: true
            }
        }
    }

    // the clock in the profile line; the text changes once a minute
    property var now: new Date()
    Timer {
        interval: 5000
        repeat: true
        running: root.shown
        onTriggered: root.now = new Date()
    }

    // this computer is read only while Home is on screen: every 3 s focused, 15 s when not
    readonly property bool shown: root.visible && root.Window.window !== null && root.Window.window.visible
    readonly property bool focused: root.Window.window !== null && root.Window.window.active
    readonly property bool readingsOn: root.blockOn("telemetry")
    Binding {
        target: LiveStats
        property: "active"
        value: true
        when: root.shown && root.readingsOn
    }
    Binding {
        target: LiveStats
        property: "interval"
        value: root.focused ? 3000 : 15000
        when: root.shown
    }

    readonly property int seriesLength: 40
    property var series: ({ core: [], gpu: [], mem: [], net: [] })
    Connections {
        target: LiveStats
        enabled: root.shown
        function onTickChanged() {
            var add = (list, v) => list.concat([v]).slice(-root.seriesLength);
            root.series = {
                core: add(root.series.core, LiveStats.cpu),
                gpu: add(root.series.gpu, LiveStats.gpuUtil),
                mem: add(root.series.mem, LiveStats.ram),
                net: add(root.series.net, LiveStats.netDown)
            };
            root.countUp();
        }
    }

    function rate(b) {
        if (b < 1024)
            return b + " B/s";
        if (b < 1048576)
            return (b / 1024).toFixed(b < 10240 ? 1 : 0) + " KB/s";
        return (b / 1048576).toFixed(1) + " MB/s";
    }

    readonly property var readingNames: ({
        core: I18n.tr("Processor"), gpu: I18n.tr("Graphics"), mem: I18n.tr("Memory"), net: I18n.tr("Network")
    })
    readonly property var readingKeys: {
        var chosen = root.f("vitals", ["core", "gpu", "mem", "net"]);
        return ["core", "gpu", "mem", "net"].filter(k => chosen.indexOf(k) >= 0 && (k !== "gpu" || LiveStats.gpuOk));
    }
    // the readings count up from nothing once, as the first of them arrives
    property real counted: 0
    property bool countedOnce: false
    function countUp() {
        if (root.countedOnce || LiveStats.tick === 0)
            return;
        root.countedOnce = true;
        count.start();
    }
    onShownChanged: root.countUp()
    NumberAnimation {
        id: count
        target: root
        property: "counted"
        from: 0
        to: 1
        duration: Lithic.duration(LithicTokens.motion.countUp)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }
    function readingValue(k) {
        if (LiveStats.tick === 0)
            return "";
        var part = v => Math.round(v * root.counted);
        if (k === "core")
            return part(LiveStats.cpu) + "%";
        if (k === "gpu")
            return part(LiveStats.gpuUtil) + "%";
        if (k === "mem")
            return part(LiveStats.ram) + "%";
        return root.rate(part(LiveStats.netDown));
    }

    // the plate's entrance, ryoku.dev's reveal: the column's groups rise in turn, the carved
    // word stamps in, the readings count up. Each plays once as Home opens and nothing moves
    // after; reduced motion plays none of it
    Component {
        id: riser
        Reveal {}
    }
    Component.onCompleted: {
        root.countUp();
        if (Lithic.reduceMotion)
            return;
        var order = 0;
        for (var i = 0; i < column.children.length; i++) {
            if (column.children[i].visible)
                riser.createObject(root, { item: column.children[i], order: order++ }).play();
        }
        stamp.start();
    }
    ParallelAnimation {
        id: stamp
        NumberAnimation {
            target: carved
            property: "opacity"
            from: 0
            to: 1
            duration: LithicTokens.motion.placardIn
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Lithic.curve
        }
        NumberAnimation {
            target: carved
            property: "scale"
            from: 1.06
            to: 1
            duration: LithicTokens.motion.placardIn
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Lithic.curve
        }
    }
    function readingProof(k) {
        if (LiveStats.tick === 0)
            return I18n.tr("Reading");
        if (k === "core")
            return LiveStats.cpuTemp > 0 ? I18n.tr("%1 °C").replace("%1", LiveStats.cpuTemp) : I18n.tr("Load %1").replace("%1", LiveStats.load.toFixed(2));
        if (k === "gpu")
            return I18n.tr("%1 °C").replace("%1", LiveStats.gpuTemp);
        if (k === "mem")
            return root.memory.length > 0 ? I18n.tr("of %1").replace("%1", root.memory) : "";
        return I18n.tr("%1 up").replace("%1", root.rate(LiveStats.netUp));
    }

    readonly property string memory: SysInfo.memoryKb > 0 ? I18n.tr("%1 GB").replace("%1", Math.round(SysInfo.memoryKb / 1048576)) : ""
    readonly property string upFor: {
        var s = SysInfo.upSeconds;
        if (s <= 0)
            return "";
        var d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60);
        if (d > 0)
            return I18n.tr("%1 d %2 h").replace("%1", d).replace("%2", h);
        if (h > 0)
            return I18n.tr("%1 h %2 min").replace("%1", h).replace("%2", m);
        return I18n.tr("%1 min").replace("%1", m);
    }
    property string network: ""
    Process {
        running: true
        command: ["nmcli", "-t", "-f", "NAME,TYPE", "connection", "show", "--active"]
        stdout: StdioCollector {
            onStreamFinished: {
                var lines = this.text.split("\n").filter(l => l.length > 0 && l.indexOf(":loopback") < 0);
                root.network = lines.length ? lines[0].split(":")[0] : "";
            }
        }
    }

    readonly property string release: {
        var name = Updates.currentName || Updates.currentVersion;
        var distro = SysInfo.sysDistro && SysInfo.sysDistro !== "Linux" ? SysInfo.sysDistro : "";
        if (name && distro)
            return I18n.tr("%1 on %2").replace("%1", name).replace("%2", distro);
        return name || distro;
    }
    // what stands still about this computer, as a datasheet; About has the table
    readonly property var datasheet: {
        var out = [];
        var key = s => I18n.tr(s).toLowerCase();
        if (root.release)
            out.push({ key: key("Ryoku"), value: root.release });
        if (Settings.provider)
            out.push({ key: key("Window manager"), value: root.title(Settings.provider) });
        if (SysInfo.sysGpu && SysInfo.sysGpu !== "-")
            out.push({ key: key("Graphics"), value: SysInfo.sysGpu });
        if (root.upFor)
            out.push({ key: key("Up for"), value: root.upFor });
        out.push({ key: key("Network"), value: root.network ? root.network : I18n.tr("Not connected") });
        if (SysInfo.sysBattery && SysInfo.sysBattery !== "-")
            out.push({ key: key("Battery"), value: SysInfo.sysBattery });
        return out;
    }

    // a control reached with the keys comes into view
    readonly property Item focusedItem: root.Window.activeFocusItem
    onFocusedItemChanged: {
        var item = root.focusedItem;
        var p = item;
        while (p && p !== flick.contentItem)
            p = p.parent;
        if (!p)
            return;
        var y = item.mapToItem(flick.contentItem, 0, 0).y;
        var margin = LithicTokens.space.s5;
        var foot = margin + (root.hub.noticeInset || 0);
        var end = Math.max(0, flick.contentHeight - flick.height);
        if (y < flick.contentY + margin)
            flick.contentY = Math.max(0, y - margin);
        else if (y + item.height > flick.contentY + flick.height - foot)
            flick.contentY = Math.min(end, y + item.height - flick.height + foot);
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: plate.height
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Item {
            id: plate
            width: flick.width
            height: Math.max(flick.height, column.y + column.height + LithicTokens.space.s5 + (root.hub.noticeInset || 0))

            // the word carved behind the statue, as on ryoku.dev's cover: Ryoku, or the
            // reader's own first big word. The one line set in capitals. On the black paper
            // it is oxblood, the sun mixed into the paper; on the light paper the same mix
            // reads salmon, so there it is cut into the stone, a step darker than the paper
            Text {
                id: carved
                visible: root.roomy && root.blockOn("epithets")
                readonly property var words: root.f("text.epithets", [])
                readonly property real room: root.heroLeft ? column.x - LithicTokens.space.s3 - x
                                                           : plate.width - root.gutter - x
                x: root.heroLeft ? root.gutter : column.x + column.width + LithicTokens.space.s3
                y: Math.round(figure.y + figure.height * 0.2 - height / 2)
                width: Math.max(0, carved.room)
                horizontalAlignment: Text.AlignHCenter
                text: String(carved.words.length > 0 ? carved.words[0] : "Ryoku")
                color: Lithic.dark ? Lithic.mix(Lithic.ground, Lithic.gold, 0.34) : Lithic.mix(Lithic.ground, Lithic.fg, 0.18)
                font.family: Lithic.font.sans
                font.weight: Font.Bold
                font.capitalization: Font.AllUppercase
                font.pixelSize: Math.round(figure.height * 0.34)
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: 48
            }

            CardFigure {
                id: figure
                readonly property real aspect: root.stockFigure ? figure.stockAspect : 900 / 1157
                // the room between the column and the label's gutter
                readonly property real room: Math.max(0, root.heroLeft ? column.x - LithicTokens.space.s5 - root.gutter
                                                                       : plate.width - root.gutter - column.x - column.width - LithicTokens.space.s5)
                // beside the column it is as tall as the plate holds under the top bar and no
                // wider than its room, and stands on the plate's foot, where the bust's own cut
                // meets the window's edge. On a narrow plate it stands beside the name instead
                readonly property real crown: topBar.y + topBar.height + LithicTokens.space.s2
                readonly property real sole: column.y + waits.y + waits.height
                visible: root.roomy || root.beside
                height: root.roomy ? Math.round(Math.min(flick.height - LithicTokens.space.s6, figure.room / figure.aspect))
                                   : Math.round(Math.max(0, Math.min(root.besideWidth / figure.aspect, figure.sole - figure.crown)))
                width: Math.round(height * figure.aspect)
                x: !root.roomy ? plate.width - root.side - width
                    : Math.round((root.heroLeft ? root.gutter : column.x + column.width + LithicTokens.space.s5) + (figure.room - width) / 2)
                y: root.roomy ? flick.height - height : Math.round(figure.sole - height)
                soft: true
                // a figure of dots is the lit side of the stone: bone on the black paper, and
                // the lightest stone on the light paper, never a negative
                ink: Lithic.dark ? Lithic.fg : Lithic.stone4
            }

            // the museum label the photograph's licence asks for, upright in the gutter beside
            // the figure as ryoku.dev sets its edge line: off the marble and whole at any
            // roomy size. A narrow plate leaves it to About's credits
            Item {
                visible: root.labelled
                width: creditLabel.height
                height: creditLabel.width
                x: root.heroLeft ? LithicTokens.space.s4 : plate.width - LithicTokens.space.s4 - width
                y: flick.height - LithicTokens.space.s4 - height
                Label {
                    id: creditLabel
                    anchors.centerIn: parent
                    rotation: 90
                    text: I18n.tr("Caracalla Farnese, marble, Naples. Photo Marie-Lan Nguyen, CC BY 2.5")
                    typeRole: "monoSm"
                    inkRole: "fg3"
                }
            }

            // the top bar: the search, where the reader left off, and the look
            Item {
                id: topBar
                x: root.side
                y: LithicTokens.space.s3
                // the window's close control stands in the pane's top right corner
                width: plate.width - root.side - LithicTokens.size.iconPlaqueSmall - LithicTokens.space.s2 - LithicTokens.space.s3
                height: LithicTokens.size.searchHeight

                SearchDoor {
                    id: door
                    width: Math.min(LithicTokens.size.searchWidth, topBar.width - customisePlaque.width - LithicTokens.space.s2)
                    onClicked: root.hub.focusSearch()
                }
                Plaque {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.continuePage !== "" && door.x + door.width + LithicTokens.space.s2 + implicitWidth < customisePlaque.x - LithicTokens.space.s2
                    x: door.x + door.width + LithicTokens.space.s2
                    kind: "quiet"
                    small: true
                    glyph: "arrow_forward"
                    text: root.continuePage !== "" ? I18n.tr("Continue in %1").replace("%1", I18n.tr(Registry.page(root.continuePage).title)) : ""
                    onClicked: root.hub.navigate(root.continuePage, "")
                }
                Plaque {
                    id: customisePlaque
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: "edit"
                    text: topBar.width >= 420 ? I18n.tr("Customise") : ""
                    accessibleName: I18n.tr("Customise")
                    onClicked: root.customise()
                }
            }

            Column {
                id: column
                x: root.roomy && root.heroLeft ? plate.width - root.side - width : root.side
                // centred in the plate when the column is shorter than it, so the room
                // sits above and below alike instead of one void under the last block
                y: root.roomy ? Math.max(LithicTokens.space.s6 + LithicTokens.space.s5,
                                         Math.round((flick.height - column.height) / 2))
                              : LithicTokens.space.s6 + LithicTokens.space.s4
                width: root.columnWidth
                spacing: LithicTokens.space.s5

                // who this is
                Column {
                    id: who
                    width: parent.width - root.besideRoom
                    spacing: LithicTokens.space.s2

                    Label {
                        width: parent.width
                        text: root.f("text.name", SysInfo.sysUser)
                        typeRole: root.width >= 640 ? "name" : "sentence"
                    }
                    Row {
                        spacing: LithicTokens.space.s3
                        Label {
                            anchors.baseline: clock.baseline
                            text: SysInfo.sysUser + "@" + SysInfo.sysHost
                            typeRole: "mono"
                            inkRole: "fg2"
                        }
                        Label {
                            id: clock
                            text: TimeFormat.format(root.now)
                            typeRole: "caption"
                            inkRole: "fg2"
                            numeric: true
                        }
                    }
                    Label {
                        width: parent.width
                        visible: text.length > 0
                        text: root.f("text.tagline", I18n.tr("For the sake of power and beauty."))
                        typeRole: "proof"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Label {
                        visible: UiPreferences.showJapaneseGloss && !root.f("text.tagline", "")
                        text: "力と美のために"
                        typeRole: "caption"
                        inkRole: "fg3"
                    }
                    // the tick ruler under the motto; on a cover its first segment is the sun
                    TickRuler {
                        width: Math.min(parent.width, implicitWidth)
                        lead: Lithic.gold
                    }
                }

                // what waits on the reader stands on the one toned slab; with nothing waiting, one line
                Column {
                    id: waits
                    width: parent.width - root.besideRoom
                    spacing: LithicTokens.space.s2

                    Row {
                        visible: !root.needsYou
                        spacing: LithicTokens.space.s3
                        Claim {
                            anchors.verticalCenter: parent.verticalCenter
                            tone: root.serviceDown && !root.updateKnown ? "failed" : root.updateKnown ? "alive" : "becoming"
                            text: root.serviceDown && !root.updateKnown ? I18n.tr("Updates can't be checked")
                                : root.updateKnown ? I18n.tr("Up to date") : I18n.tr("Checking for updates")
                        }
                        Label {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: root.updateKnown && text.length > 0
                            text: Updates.currentName && Updates.currentVersion && Updates.currentName !== Updates.currentVersion
                                ? I18n.tr("Ryoku %1, %2").replace("%1", Updates.currentName).replace("%2", Updates.currentVersion)
                                : I18n.tr("Ryoku %1").replace("%1", Updates.currentName || Updates.currentVersion)
                            typeRole: "detail"
                        }
                    }
                    // the daemon gone for a few seconds: say so, rather than show stale facts as live
                    Label {
                        width: parent.width
                        visible: root.serviceDown
                        text: I18n.tr("The desktop service is not answering, so what Settings shows may be out of date and changes may not take. Settings picks it up again by itself once it is back.")
                        typeRole: "detail"
                        ink: Lithic.jasper
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }

                    Slab {
                        visible: root.needsYou
                        width: parent.width
                        tone: "attention"
                        Column {
                            width: parent.width
                            spacing: LithicTokens.space.s2
                            Claim {
                                tone: "attention"
                                text: I18n.tr("Needs you")
                            }
                            Column {
                                width: parent.width
                                SettingRow {
                                    visible: Updates.available
                                    width: parent.width
                                    first: true
                                    plane: Lithic.mineralField("attention")
                                    title: I18n.tr("An update is ready")
                                    detail: Updates.latestName ? Updates.latestName : Updates.latestVersion
                                    Plaque {
                                        kind: "gold"
                                        small: true
                                        text: I18n.tr("Open Updates")
                                        onClicked: root.hub.navigate("updates", "")
                                    }
                                }
                                Repeater {
                                    model: Attention.items
                                    delegate: SettingRow {
                                        required property var modelData
                                        required property int index
                                        width: parent.width
                                        first: index === 0 && !Updates.available
                                        plane: Lithic.mineralField("attention")
                                        title: root.filled(modelData.title, modelData.args)
                                        detail: root.filled(modelData.detail, modelData.args)
                                        activatable: modelData.page !== "" && root.hub.pageAvailable(modelData.page)
                                        onActivated: root.hub.navigate(modelData.page, modelData.row || "")
                                        Plaque {
                                            // one gold on the plate: the first thing that waits
                                            kind: index === 0 && !Updates.available ? "gold" : "line"
                                            small: true
                                            text: modelData.actionText ? I18n.tr(modelData.actionText) : I18n.tr("Open")
                                            onClicked: {
                                                if (modelData.action === "log-out")
                                                    root.hub.logOut();
                                                else if (modelData.command && modelData.command.length > 0)
                                                    Spawn.run(modelData.command);
                                                else
                                                    root.hub.navigate(modelData.page, modelData.row || "");
                                            }
                                        }
                                    }
                                }
                                Repeater {
                                    model: root.failures
                                    delegate: SettingRow {
                                        required property var modelData
                                        required property int index
                                        width: parent.width
                                        first: index === 0 && !Updates.available && Attention.items.length === 0
                                        plane: Lithic.mineralField("attention")
                                        title: modelData.entry ? I18n.tr("%1 did not change").replace("%1", I18n.tr(modelData.entry.label)) : ""
                                        detail: modelData.error
                                        Plaque {
                                            kind: "line"
                                            small: true
                                            glyph: "refresh"
                                            text: I18n.tr("Retry")
                                            onClicked: Changes.retry(modelData.entry.bind)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                // a window without the rail lists the categories here, the one place they stand
                Column {
                    visible: root.compact
                    width: parent.width
                    spacing: LithicTokens.size.rowListGap
                    Repeater {
                        model: root.compact ? Registry.categories : []
                        delegate: PageRow {
                            required property var modelData
                            required property int index
                            width: parent.width
                            glyph: modelData.glyph
                            text: I18n.tr(modelData.label)
                            summary: Summaries.categorySentence(modelData.id, root.hub.summaries)
                            onClicked: root.hub.openCategory(index)
                        }
                    }
                }

                // this computer: what stands still in words, then its live readings as lines
                Column {
                    width: parent.width
                    spacing: LithicTokens.space.s3

                    Label {
                        width: parent.width
                        text: I18n.tr("This computer")
                        typeRole: "section"
                    }
                    // the standing facts as a datasheet: keys in mono, one vein rule, values
                    Row {
                        visible: root.blockOn("specs") && root.datasheet.length > 0
                        spacing: LithicTokens.space.s3
                        Column {
                            id: sheetKeys
                            Repeater {
                                model: root.datasheet
                                delegate: Label {
                                    required property var modelData
                                    height: Lithic.type.detail.lineHeight + LithicTokens.space.s1
                                    verticalAlignment: Text.AlignVCenter
                                    text: modelData.key
                                    typeRole: "monoSm"
                                    inkRole: "fg3"
                                }
                            }
                        }
                        Rectangle {
                            width: LithicTokens.size.hairline
                            height: sheetKeys.height
                            color: Lithic.vein
                        }
                        Column {
                            Repeater {
                                model: root.datasheet
                                delegate: Label {
                                    required property var modelData
                                    width: Math.max(0, column.width - sheetKeys.width - 2 * LithicTokens.space.s3 - LithicTokens.size.hairline)
                                    height: Lithic.type.detail.lineHeight + LithicTokens.space.s1
                                    verticalAlignment: Text.AlignVCenter
                                    text: modelData.value
                                    typeRole: "detail"
                                    inkRole: "fg"
                                }
                            }
                        }
                    }
                    Grid {
                        id: readings
                        visible: root.readingsOn && root.readingKeys.length > 0
                        width: parent.width
                        readonly property int perRow: width >= 4 * 112 + 3 * LithicTokens.space.s4 ? Math.min(4, root.readingKeys.length) : Math.min(2, root.readingKeys.length)
                        readonly property real cell: perRow > 0 ? Math.floor((width - (perRow - 1) * LithicTokens.space.s4) / perRow) : width
                        columns: Math.max(1, perRow)
                        columnSpacing: LithicTokens.space.s4
                        rowSpacing: LithicTokens.space.s4
                        Repeater {
                            model: root.readingKeys
                            delegate: Column {
                                required property string modelData
                                width: readings.cell
                                spacing: LithicTokens.space.s1
                                Label {
                                    width: parent.width
                                    text: root.readingNames[modelData]
                                    typeRole: "caption"
                                    inkRole: "fg2"
                                }
                                Label {
                                    width: parent.width
                                    text: root.readingValue(modelData)
                                    typeRole: "section"
                                    numeric: true
                                }
                                EngravedChart {
                                    width: parent.width
                                    height: LithicTokens.size.searchHeight
                                    values: root.series[modelData]
                                    minimum: 0
                                    maximum: modelData === "net" ? Number.NaN : 100
                                    emptyText: ""
                                    Accessible.name: root.readingNames[modelData]
                                }
                                Label {
                                    width: parent.width
                                    text: root.readingProof(modelData)
                                    typeRole: "caption"
                                    inkRole: "fg3"
                                    numeric: true
                                }
                            }
                        }
                    }
                    Plaque {
                        // a quiet plaque draws no edge at rest: its words line up with the column
                        x: -sideInset
                        kind: "quiet"
                        small: true
                        glyph: "arrow_forward"
                        text: I18n.tr("About this computer")
                        onClicked: root.hub.navigate("about", "")
                    }
                }

                Column {
                    visible: Changes.recent.length > 0
                    width: parent.width
                    spacing: LithicTokens.space.s3

                    Label {
                        width: parent.width
                        text: I18n.tr("Recently changed")
                        typeRole: "section"
                    }
                    Column {
                        width: parent.width
                        Repeater {
                            id: recentRows
                            model: Changes.recent
                            delegate: SettingRow {
                                required property var modelData
                                required property int index
                                readonly property alias undoButton: undoPlaque
                                width: parent.width
                                first: index === 0
                                plane: Lithic.ground
                                title: I18n.tr(modelData.label)
                                detail: modelData.page && Registry.page(modelData.page) ? I18n.tr(Registry.page(modelData.page).title) : ""
                                activatable: !!modelData.page
                                onActivated: root.hub.navigate(modelData.page, "")
                                Plaque {
                                    id: undoPlaque
                                    kind: "quiet"
                                    small: true
                                    glyph: "undo"
                                    text: I18n.tr("Undo")
                                    // the keys pressed it when it shows the keyboard mark
                                    onClicked: root.undoRecent(modelData, undoPlaque.visualFocus)
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // the Undo removes its own row, so this runs from the plate, not from the row
    function undoRecent(entry, fromKeys) {
        Changes.undoEntry(entry);
        if (fromKeys)
            refocusRecent.restart();
    }

    // the row an Undo came from is gone: the keyboard moves to the next Undo, or stays on Home
    Timer {
        id: refocusRecent
        interval: 0
        onTriggered: {
            var next = recentRows.count > 0 ? recentRows.itemAt(0) : null;
            if (next)
                next.undoButton.forceActiveFocus(Qt.TabFocusReason);
            else
                root.forceActiveFocus(Qt.OtherFocusReason);
        }
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.topMargin: LithicTokens.size.paneDecorStrip
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
}
