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
    // drawn as the live card's preview (Customise, About): the plate at its design
    // size, its own controls inert, so the preview IS Home and cannot drift from it
    property bool preview: false
    readonly property real designWidth: 1280
    readonly property real designHeight: 800

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

    // the look's editor, made the first time it is asked for; the loader builds
    // it on a later frame, so the open waits for it
    // the card studio opens on a tab: "picture", "adjust", "words", "layout" or "share"
    property string wantCustomise: ""
    function customise(tab) {
        Changes.ensureStore("card", Qt.resolvedUrl("about/CardStore.qml"));
        if (editorLoader.item) {
            editorLoader.item.opener = customisePlaque;
            editorLoader.item.openOn(tab || "picture");
            return;
        }
        wantCustomise = tab || "picture";
        editorLoader.active = true;
    }

    // Share on the top bar copies the system card as a picture that carries its design
    property bool shareDone: false
    function shareCard() {
        root.shareDone = false;
        homeShare.take("copy");
    }
    CardShare {
        id: homeShare
        hub: root.hub
        onJobChanged: {
            if (homeShare.job === "" && homeShare.last === "copy") {
                root.shareDone = homeShare.error === "";
                shareSaid.restart();
            }
        }
    }
    Timer {
        id: shareSaid
        interval: 4000
        onTriggered: {
            root.shareDone = false;
            homeShare.error = "";
        }
    }

    // the card large, to read it whole; made the first time it is asked for
    function expandCard() {
        viewerLoader.active = true;
        if (viewerLoader.item)
            viewerLoader.item.open();
    }
    Loader {
        id: viewerLoader
        active: false
        parent: root.hub ? root.hub.overlayItem : root
        anchors.fill: parent
        sourceComponent: CardViewer {}
        onLoaded: item.open()
    }

    function openBinder() {
        binderLoader.active = true;
        if (binderLoader.item)
            binderLoader.item.open();
    }
    Loader {
        id: binderLoader
        active: false
        parent: root.hub ? root.hub.overlayItem : root
        anchors.fill: parent
        sourceComponent: Binder { hub: root.hub }
        onLoaded: item.open()
    }

    Loader {
        id: editorLoader
        active: false
        // loaded by URL, not imported: the editor draws this plate, so a static
        // import here would be a cycle
        source: Qt.resolvedUrl("about/CardEditor.qml")
        onLoaded: {
            item.parent = root.hub.overlayItem;
            item.hub = root.hub;
            item.pageId = "";
            item.binderOpener = () => root.openBinder();
            if (root.wantCustomise !== "") {
                var tab = root.wantCustomise;
                root.wantCustomise = "";
                item.opener = customisePlaque;
                item.openOn(tab);
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
    // only the live plate drives the sampler; a preview reads it, it does not
    // set its pace (two bindings to one property would fight)
    Binding {
        target: LiveStats
        property: "active"
        value: true
        when: !root.preview && root.shown && root.readingsOn
    }
    Binding {
        target: LiveStats
        property: "interval"
        value: root.focused ? 3000 : 15000
        when: !root.preview && root.shown
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

    // the plate answers the pointer in depth: the figure turns toward it and catches its
    // light on the edges that face it, the carved word drifts behind and the column a little
    // in front, and a dotted light follows it. Left alone, everything eases back and the
    // light rests behind the figure's head. A still card (About, a saved picture) stands flat
    readonly property bool depthOn: !root.preview && root.roomy
    // Home shows the plate, or the reader's system card in the figure's place (the studio's
    // Layout); the card needs the room only a roomy plate has
    readonly property bool cardView: root.roomy && root.f("homeView", "plate") === "card"
    // the figure, for the studio to measure while the reader drags it
    readonly property Item figureItem: figure
    readonly property bool pointed: root.depthOn && pointer.hovered
    // the pointer's place, -1..1 each way, and the light's, in the plate's pixels; each
    // follows its target at once and settles without overshoot (Chase)
    Chase { id: chaseX }
    Chase { id: chaseY }
    Chase { id: chaseLightX; epsilon: 0.5 }
    Chase { id: chaseLightY; epsilon: 0.5 }
    readonly property real tiltX: chaseX.value
    readonly property real tiltY: chaseY.value
    readonly property real lightX: chaseLightX.value
    readonly property real lightY: chaseLightY.value
    property bool lightPlaced: false
    property real lightStrength: root.pointed ? 1 : 0.8
    Behavior on lightStrength {
        NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
    }
    function point(p) {
        if (!root.pointed || !p || plate.width <= 0 || plate.height <= 0) {
            chaseX.target = 0;
            chaseY.target = 0;
            // the light rests behind the figure's head; set there at once the first time
            var rx = figure.x + figure.width * 0.5, ry = figure.y + figure.height * 0.22;
            if (!root.lightPlaced) {
                chaseLightX.snap(rx);
                chaseLightY.snap(ry);
                root.lightPlaced = figure.height > 0;
            } else {
                chaseLightX.target = rx;
                chaseLightY.target = ry;
            }
            return;
        }
        chaseX.target = Math.max(-1, Math.min(1, (p.x - plate.width / 2) / (plate.width / 2)));
        chaseY.target = Math.max(-1, Math.min(1, (p.y - plate.height / 2) / (plate.height / 2)));
        chaseLightX.target = p.x;
        chaseLightY.target = p.y;
    }
    onPointedChanged: root.point(pointer.point.position)
    Connections {
        target: figure
        function onXChanged() { if (!root.pointed) root.point(null); }
        function onYChanged() { if (!root.pointed) root.point(null); }
        function onHeightChanged() { if (!root.pointed) root.point(null); }
    }

    // the dust in the light drifts only while Settings is in front and Home on show
    property real moteTime: 0
    Timer {
        interval: 66
        repeat: true
        running: root.depthOn && root.shown && Ambient.on
        onTriggered: root.moteTime += 0.066
    }

    // a still card (About's, a saved picture) carries its own paper; Home lies on the pane's
    Rectangle {
        anchors.fill: parent
        visible: root.preview
        color: Lithic.ground
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

            HoverHandler {
                id: pointer
                enabled: root.depthOn
                onPointChanged: root.point(pointer.point.position)
            }

            // the word carved behind the statue, as on ryoku.dev's cover: Ryoku, or the
            // reader's own first big word. The one line set in capitals. On the black paper
            // it is oxblood, the sun mixed into the paper; on the light paper the same mix
            // reads salmon, so there it is cut into the stone, a step darker than the paper
            Text {
                id: carved
                visible: root.roomy && root.blockOn("epithets") && !root.cardView
                readonly property var words: root.f("text.epithets", [])
                readonly property real room: root.heroLeft ? column.x - LithicTokens.space.s3 - x
                                                           : plate.width - root.gutter - x
                x: root.heroLeft ? root.gutter : column.x + column.width + LithicTokens.space.s3
                y: Math.round(figure.y + figure.height * 0.2 - height / 2)
                width: Math.max(0, carved.room)
                horizontalAlignment: Text.AlignHCenter
                text: String(carved.words.length > 0 ? carved.words[0] : "Ryoku")
                color: Lithic.dark ? Lithic.mix(Lithic.ground, Lithic.gold, 0.34) : Lithic.mix(Lithic.ground, Lithic.fg, 0.18)
                // far back: it drifts against the pointer
                transform: Translate { x: -root.tiltX * 22; y: -root.tiltY * 10 }
                font.family: Lithic.font.sans
                font.weight: Font.Bold
                font.capitalization: Font.AllUppercase
                font.pixelSize: Math.round(figure.height * 0.34)
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: 48
            }

            // the light the figure stands in, drawn only in a box around itself so a moving
            // light repaints a patch of the plate, not all of it
            ShaderEffect {
                visible: root.depthOn && !root.cardView
                readonly property real radius: Math.max(160, figure.height * 0.42)
                x: Math.round(root.lightX - width / 2)
                y: Math.round(root.lightY - height / 2)
                width: Math.round(radius * 3.4)
                height: width
                property color tint: Lithic.alpha(Lithic.dark ? Lithic.sel : Lithic.fg, Lithic.dark ? 0.55 : 0.32)
                property vector2d size: Qt.vector2d(width, height)
                property vector2d center: Qt.vector2d(0.5, 0.5)
                property real strength: root.lightStrength
                property real time: root.moteTime
                property real motes: Ambient.on ? 1 : 0
                fragmentShader: Qt.resolvedUrl("../shaders/light.frag.qsb")
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
                visible: (root.roomy || root.beside) && !root.cardView
                height: root.roomy ? Math.round(Math.min(flick.height - LithicTokens.space.s6, figure.room / figure.aspect))
                                   : Math.round(Math.max(0, Math.min(root.besideWidth / figure.aspect, figure.sole - figure.crown)))
                width: Math.round(height * figure.aspect)
                x: !root.roomy ? plate.width - root.side - width
                    : Math.round((root.heroLeft ? root.gutter : column.x + column.width + LithicTokens.space.s5) + (figure.room - width) / 2)
                y: root.roomy ? flick.height - height : Math.round(figure.sole - height)
                soft: true
                live: root.pointed
                // a figure of dots is the lit side of the stone: bone on the black paper, and
                // the lightest stone on the light paper, never a negative
                ink: Lithic.dark ? Lithic.fg : Lithic.stone4
                // the middle plane: it turns toward the pointer in perspective on its own foot
                transform: [
                    Rotation {
                        origin.x: figure.width / 2
                        origin.y: figure.height
                        axis { x: 0; y: 1; z: 0 }
                        angle: root.tiltX * 9
                    },
                    Rotation {
                        origin.x: figure.width / 2
                        origin.y: figure.height
                        axis { x: 1; y: 0; z: 0 }
                        angle: -root.tiltY * 5
                    },
                    Translate { x: root.tiltX * 10; y: root.tiltY * 5 }
                ]
                // lit from the pointer while it is on the plate
                layer.enabled: root.depthOn && root.lightStrength > 0.6
                layer.effect: ShaderEffect {
                    property color tint: Lithic.sel
                    property vector2d texel: Qt.vector2d(1 / Math.max(1, figure.width), 1 / Math.max(1, figure.height))
                    property vector2d lightDir: {
                        var dx = root.lightX - (figure.x + figure.width / 2);
                        var dy = root.lightY - (figure.y + figure.height / 2);
                        var l = Math.max(1, Math.sqrt(dx * dx + dy * dy));
                        return Qt.vector2d(dx / l, dy / l);
                    }
                    property real strength: (root.lightStrength - 0.55) / 0.45
                    fragmentShader: Qt.resolvedUrl("../shaders/rim.frag.qsb")
                }
            }

            // the museum label the photograph's licence asks for, upright in the gutter beside
            // the figure as ryoku.dev sets its edge line: off the marble and whole at any
            // roomy size. A narrow plate leaves it to About's credits
            Item {
                visible: root.labelled && !root.cardView
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

            // the system card in the figure's place, when Home shows the card; it turns toward
            // the pointer and its foil slides, as on the studio's preview
            Item {
                id: homeCard
                visible: root.cardView
                readonly property real room: Math.max(0, root.heroLeft ? column.x - LithicTokens.space.s5 - root.gutter
                                                                       : plate.width - root.gutter - column.x - column.width - LithicTokens.space.s5)
                readonly property real crown: topBar.y + topBar.height + LithicTokens.space.s3
                // a playing card's shape, TradingCard's design size
                readonly property size design: Qt.size(630, 880)
                readonly property real scaleTo: Math.min(homeCard.room / homeCard.design.width,
                                                         (flick.height - homeCard.crown - LithicTokens.space.s5) / homeCard.design.height)
                width: homeCard.design.width * homeCard.scaleTo
                height: homeCard.design.height * homeCard.scaleTo
                x: Math.round((root.heroLeft ? root.gutter : column.x + column.width + LithicTokens.space.s5) + (homeCard.room - width) / 2)
                y: Math.round(homeCard.crown + (flick.height - homeCard.crown - LithicTokens.space.s5 - height) / 2)
                transform: [
                    Rotation { origin.x: homeCard.width / 2; origin.y: homeCard.height / 2; axis { x: 0; y: 1; z: 0 } angle: root.tiltX * 9 },
                    Rotation { origin.x: homeCard.width / 2; origin.y: homeCard.height / 2; axis { x: 1; y: 0; z: 0 } angle: -root.tiltY * 6 }
                ]
                Loader {
                    active: root.cardView
                    sourceComponent: TradingCard {
                        transformOrigin: Item.TopLeft
                        scale: homeCard.scaleTo
                        foil: Qt.vector2d(root.tiltX, root.tiltY)
                        live: root.pointed && Ambient.on
                    }
                }
            }

            // the top bar: where the reader left off, and the look; a narrow window, whose
            // sidebar is put away, keeps the search here too, clear of the drawer's button
            Item {
                id: topBar
                visible: !root.preview
                x: root.side + (root.hub.wide ? 0 : 40)
                y: LithicTokens.space.s3
                // the window's close control stands in the pane's top right corner
                width: plate.width - x - LithicTokens.size.iconPlaqueSmall - LithicTokens.space.s2 - LithicTokens.space.s3
                height: LithicTokens.size.searchHeight

                SearchDoor {
                    id: door
                    visible: !root.hub.wide
                    width: Math.min(LithicTokens.size.searchWidth, topBar.width - customisePlaque.width - LithicTokens.space.s2)
                    onClicked: root.hub.focusSearch()
                }
                Plaque {
                    id: continuePlaque
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.continuePage !== "" && !shareError.visible && x + implicitWidth < expandPlaque.x - LithicTokens.space.s2
                    x: door.visible ? door.x + door.width + LithicTokens.space.s2 : 0
                    kind: "quiet"
                    small: true
                    glyph: "arrow_forward"
                    text: root.continuePage !== "" ? I18n.tr("Continue in %1").replace("%1", I18n.tr(Registry.page(root.continuePage).title)) : ""
                    onClicked: root.hub.navigate(root.continuePage, "")
                }
                Plaque {
                    id: expandPlaque
                    anchors.right: binderPlaque.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: "open_in_full"
                    text: ""
                    accessibleName: I18n.tr("Show your system card large")
                    onClicked: root.expandCard()
                }
                Plaque {
                    id: binderPlaque
                    anchors.right: sharePlaque.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: "collections_bookmark"
                    text: I18n.tr("Binder")
                    onClicked: root.openBinder()
                }
                Plaque {
                    id: sharePlaque
                    anchors.right: customisePlaque.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    kind: "quiet"
                    small: true
                    glyph: homeShare.error !== "" && shareSaid.running ? "error" : root.shareDone ? "check" : "share"
                    text: topBar.width < 420 ? "" : root.shareDone ? I18n.tr("Copied") : I18n.tr("Share")
                    accessibleName: I18n.tr("Copy your system card to share it")
                    busy: homeShare.job === "copy"
                    onClicked: root.shareCard()
                }
                // a copy that failed says why on the bar itself, in the room left of the buttons;
                // one that worked says so on its button
                Label {
                    id: shareError
                    anchors.right: expandPlaque.left
                    anchors.rightMargin: LithicTokens.space.s3
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, Math.max(0, expandPlaque.x - LithicTokens.space.s3))
                    visible: shareSaid.running && homeShare.error !== ""
                    text: homeShare.error
                    typeRole: "caption"
                    ink: Lithic.jasper
                    horizontalAlignment: Text.AlignRight
                    elide: Text.ElideRight
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
                // the near plane: a little with the pointer
                transform: Translate { x: root.tiltX * 4; y: root.tiltY * 2 }
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
                                        enabled: !root.preview
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
                                        activatable: !root.preview && modelData.page !== "" && root.hub.pageAvailable(modelData.page)
                                        onActivated: root.hub.navigate(modelData.page, modelData.row || "")
                                        Plaque {
                                            // one gold on the plate: the first thing that waits
                                            kind: index === 0 && !Updates.available ? "gold" : "line"
                                            small: true
                                            enabled: !root.preview
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
                                            enabled: !root.preview
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

                // a window whose sidebar is put away lists the categories here too
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
                        enabled: !root.preview
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
                                    enabled: !root.preview
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
