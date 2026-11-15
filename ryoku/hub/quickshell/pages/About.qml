import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/about"
import "../schema/about.js" as Schema
import "../registry/rarity.js" as Rarity

// what this computer runs and is made of, the system card, and the credits
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property string edition: ""
    property string copied: ""
    property string cardMessage: ""
    property string cardError: ""
    property string cardFile: ""
    property string cardJob: ""
    readonly property var cardAll: ({ store: "card", key: "*" })
    readonly property string home: Quickshell.env("HOME") || ""

    function tilde(p) {
        p = String(p || "");
        return root.home.length > 0 && p.indexOf(root.home) === 0 ? "~" + p.slice(root.home.length) : p;
    }

    function localPath(url) {
        var s = String(url || "");
        return decodeURIComponent(s.indexOf("file://") === 0 ? s.slice(7) : s);
    }

    function known(v) {
        return v !== undefined && v !== null && String(v).length > 0 && String(v) !== "-" && String(v) !== "- / -";
    }

    // the full read ended; a fact still empty then could not be read
    readonly property bool readEnded: SysInfo.loaded === true && !SysInfo.loading
    readonly property bool factsFailed: root.readEnded && (SysInfo.failed === true || Version.rawVersion.length === 0)
    function readAgain() {
        SysInfo.load();
        Version.refresh();
    }

    readonly property var facts: ({
        version: Version.rawVersion.length > 0 ? Version.rawVersion.replace(/^v/, "") + "  ·  " + root.channel : "",
        edition: root.edition,
        wm: root.known(SysInfo.sysWM) ? SysInfo.sysWM + (root.known(SysInfo.sysWmVer) ? " " + SysInfo.sysWmVer.replace(/^v+/, "") : "") : "",
        cpu: root.known(SysInfo.sysCpu) ? SysInfo.sysCpu + (root.known(SysInfo.sysCpuCores) ? "  ·  " + SysInfo.sysCpuCores : "") : "",
        gpu: root.known(SysInfo.sysGpu) ? [SysInfo.sysGpu, SysInfo.sysGpu2].filter(g => root.known(g)).join("  ·  ") : "",
        ram: root.known(SysInfo.sysRam) ? I18n.tr("%1 in use").replace("%1", SysInfo.sysRam) : "",
        disk: root.known(SysInfo.sysDisk) ? I18n.tr("%1 used").replace("%1", SysInfo.sysDisk) : "",
        display: root.known(SysInfo.sysResolution) ? SysInfo.sysResolution + (root.known(SysInfo.sysRefresh) ? " @ " + SysInfo.sysRefresh : "") : "",
        kernel: root.known(SysInfo.sysKernel) ? SysInfo.sysKernel : "",
        packages: root.known(SysInfo.sysPackages) ? (root.known(SysInfo.sysPkgExplicit)
            ? I18n.tr("%1 installed, %2 chosen by you").replace("%1", SysInfo.sysPackages).replace("%2", SysInfo.sysPkgExplicit)
            : SysInfo.sysPackages) : "",
        uptime: root.known(SysInfo.sysUptime) ? SysInfo.sysUptime : ""
    })
    readonly property var literal: ["version", "kernel"]
    readonly property string channel: {
        var idx = Version.editionIndex;
        return idx.length > 0 ? idx.charAt(0) + idx.slice(1).toLowerCase() + (Version.editionNumber.length > 0 ? " " + Version.editionNumber : "") : "";
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        // the reset row carries the whole card, so a refused write shows on it with Retry
        rows: Schema.rows.filter(r => r.template !== true).map(r => r.id === "card-reset" ? Object.assign({}, r, { bind: root.cardAll }) : r)
    })

    readonly property string stateLine: Version.rawVersion.length > 0
        ? I18n.tr("Ryoku %1 on %2").replace("%1", Version.rawVersion.replace(/^v/, "")).replace("%2", root.edition || SysInfo.sysDistro)
        : ""

    function details() {
        var lines = ["Ryoku " + root.facts.version];
        var labels = { edition: "Edition", wm: "Window manager", cpu: "Processor", gpu: "Graphics", ram: "Memory",
                       disk: "Storage", display: "Display", kernel: "Kernel", packages: "Packages", uptime: "Up for" };
        for (var k in labels)
            if (root.facts[k])
                lines.push(labels[k] + ": " + root.facts[k]);
        return lines.join("\n");
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (page.layout.length === 0) {
            root.pendingRow = rowId;
            return;
        }
        // the crew lives in the credits footer, so its link lands there, on the first tester
        if (rowId === "testers" && root.crewItem) {
            page.reveal(root.crewItem);
            var first = root.crewItem.nextItemInFocusChain(true);
            if (first && page.contains(root.crewItem, first))
                first.forceActiveFocus(Qt.TabFocusReason);
            return;
        }
        if (page.sectionOfRow(rowId)) {
            page.focusRow(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        // a deep link may name a section rather than a row
        var at = root.sectionIndex(row ? row.section : rowId);
        if (at >= 0)
            page.chooseSection(at);
    }

    // the trading card on the page is the one the pictures are taken from
    property Item tradingCard: null
    // the credits footer holding the testers, where their deep link lands
    property Item crewItem: null

    // copy ("copy") or save ("save") the card: it is drawn into a picture at twice its
    // size, and the card tool writes the card's design into that picture
    function takeCard(job) {
        root.cardError = "";
        root.cardMessage = "";
        root.cardLast = job;
        if (!root.tradingCard) {
            root.cardError = I18n.tr("The card is not drawn yet. Scroll to it, then try again.");
            return;
        }
        var rendered = (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-card-render.png";
        var name = String(SysInfo.sysUser || "card").replace(/[^A-Za-z0-9._-]/g, "") || "card";
        var dst = job === "copy" ? "--copy" : Paths.picturesDir + "/ryoku-card-" + name + "-" + Qt.formatDateTime(new Date(), "yyyyMMdd-HHmmss") + ".png";
        root.cardJob = job;
        var card = root.tradingCard;
        var ok = card.grabToImage(function (result) {
            if (!result.saveToFile(rendered)) {
                root.cardJob = "";
                root.cardError = I18n.tr("The card could not be drawn into a picture. Try again.");
                return;
            }
            root.runCard(job, dst, rendered);
        }, Qt.size(card.designWidth * 2, card.designHeight * 2));
        if (!ok) {
            root.cardJob = "";
            root.cardError = I18n.tr("The card could not be drawn into a picture. Try again.");
        }
    }

    // use someone's card: a card picture or .ryoprofile file ("use"), or the card picture on
    // the clipboard ("paste")
    function useDesign(job, file) {
        root.hub.confirm({
            title: I18n.tr("Use this card's design?"),
            text: I18n.tr("Its figure, words and choices replace your card's. To keep yours, copy or save it first."),
            actionText: I18n.tr("Use it"),
            destructive: false
        }, () => root.runCard(job, file, ""));
    }

    // a refused card job, in plain words with its next step; the tool's own words
    // ("open …: permission denied", JSON parse errors) are not shown
    function cardRefusal(job, err) {
        var e = String(err || "");
        var use = job === "use" || job === "paste";
        if (/no space left/i.test(e))
            return use ? I18n.tr("The disk is full, so the design could not be used. Free some space, then press Retry.")
                       : I18n.tr("The disk is full, so the card could not be saved. Free some space, then press Retry.");
        if (job === "paste" && /no picture/i.test(e))
            return I18n.tr("The clipboard holds no picture. Copy a Ryoku card first, then press Paste.");
        if (use && /not a Ryoku card|stripped/i.test(e))
            return I18n.tr("That picture is not a Ryoku card, or the app it came through stripped the design from it. Ask for the card as a file.");
        if (use && /ryoprofile|hero|not a PNG|cut short|no end|invalid character|unexpected end|cannot unmarshal/i.test(e))
            return I18n.tr("That file is not a card, or it is damaged. Choose a card picture or a .ryoprofile with Open.");
        if (job === "copy" && /wl-copy/i.test(e))
            return I18n.tr("The card could not be put on the clipboard. Save it instead, or press Retry.");
        if (/permission denied|read-only file system|operation not permitted/i.test(e))
            return use ? I18n.tr("Settings is not allowed to read that file or save your card. Check the file's permissions, then press Retry, or choose another file with Open.")
                       : I18n.tr("Settings is not allowed to write in %1. Check the folder's permissions, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        if (/no such file|not a directory/i.test(e))
            return use ? I18n.tr("That file is no longer there. Choose another with Open.")
                       : I18n.tr("%1 is not there. Create it, then press Retry.").replace("%1", root.tilde(Paths.picturesDir));
        return use ? I18n.tr("That design could not be used. Press Retry, or choose another file with Open.")
                   : I18n.tr("The card could not be made. Press Retry.");
    }

    // the last card job, so Retry runs it again
    property string cardLast: ""
    property string cardArg: ""
    function runCard(job, arg, rendered) {
        root.cardError = "";
        root.cardMessage = "";
        root.cardLast = job;
        root.cardArg = arg;
        root.cardJob = job;
        var argv = job === "use" ? ["ryoku-hub", "profile", "import", arg]
            : job === "paste" ? ["ryoku-hub", "profile", "import", "--clipboard"]
            : ["ryoku-hub", "profile", "card", rendered, arg];
        Changes.run(argv, function (ok, out, err, code) {
            root.cardJob = "";
            if (code === -1) {
                root.cardError = err === Changes.missingText(argv[0])
                    ? I18n.tr("The card tool is missing from this Ryoku install. Update Ryoku, then try again.")
                    : I18n.tr("That took too long, so Settings stopped waiting. Try again.");
                return;
            }
            if (!ok) {
                root.cardError = root.cardRefusal(job, err);
                return;
            }
            if (job === "copy")
                root.cardMessage = I18n.tr("Copied. Paste it in a chat or anywhere; Ryoku Settings opens it as a card.");
            else if (job === "save") {
                root.cardFile = arg;
                root.cardMessage = I18n.tr("Saved to %1").replace("%1", root.tilde(arg));
            } else
                root.cardMessage = I18n.tr("Your card uses the shared design");
        }, { timeout: 30000 });
    }

    function retryCard() {
        if (root.cardLast === "copy" || root.cardLast === "save")
            root.takeCard(root.cardLast);
        else if (root.cardLast !== "")
            root.runCard(root.cardLast, root.cardArg, "");
    }

    function resetCard() {
        root.hub.confirm({
            title: I18n.tr("Back to the stock card?"),
            text: I18n.tr("Your figure, words and choices are forgotten. Undo is offered right after."),
            actionText: I18n.tr("Reset the card"),
            destructive: true
        }, () => Changes.set(root.cardAll, ({}), I18n.tr("System card"), "about", I18n.tr("The card is back to stock")));
    }

    Component.onCompleted: {
        Changes.ensureStore("card", Qt.resolvedUrl("../components/about/CardStore.qml"));
        LiveStats.active = true;
    }
    Component.onDestruction: {
        if (!editor.opened)
            LiveStats.active = false;
    }

    // one reading for the still card, then the sampler stops
    Connections {
        target: LiveStats
        function onTickChanged() {
            if (LiveStats.tick >= 2 && !editor.opened)
                LiveStats.active = false;
        }
    }

    FileView {
        path: "/etc/os-release"
        onLoaded: {
            var m = /^PRETTY_NAME="?([^"\n]*)"?/m.exec(text());
            root.edition = m ? m[1] : "";
        }
    }

    RyokuUi.FileDialogHost {
        id: openDesign
        title: I18n.tr("Open a card")
        nameFilters: [I18n.tr("Ryoku cards") + " (*.png *.ryoprofile)"]
        onAccepted: root.useDesign("use", root.localPath(openDesign.selectedFile))
    }

    Timer {
        id: copiedClear
        interval: LithicTokens.motion.placardLife
        onTriggered: root.copied = ""
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "about"
        schema: root.schema
        stateLine: root.stateLine
        controls: {
            var c = { "copy-details": copyControl, "card-edit": editControl, "card-copy": copyCardControl,
                      "card-image": imageControl, "card-use": useControl, "card-paste": pasteControl,
                      "card-reset": resetControl };
            Schema.rows.forEach(r => { if (r.fact) c[r.id] = factControl; });
            return c;
        }
        bodies: ({ card: cardBody })
        footers: ({ computer: factsFooter, card: cardFooter, credits: crewFooter })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    CardEditor {
        id: editor
        parent: root.hub.overlayItem
        hub: root.hub
    }

    Component {
        id: factControl
        Label {
            property var row: null
            readonly property string fact: row ? (root.facts[row.fact] || "") : ""
            width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
            horizontalAlignment: Text.AlignRight
            text: fact.length > 0 ? fact : (root.readEnded ? I18n.tr("Could not be read") : I18n.tr("Reading"))
            typeRole: fact.length > 0 && row && root.literal.indexOf(row.fact) >= 0 ? "mono" : "label"
            inkRole: fact.length > 0 ? "fg" : "fg3"
            elide: Text.ElideMiddle
        }
    }

    // a helper that gave no answer: say so, with a way to read again
    Component {
        id: factsFooter
        Row {
            visible: root.factsFailed
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Some details could not be read. Try again, and if they stay missing, update Ryoku from Updates.")
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                width: Math.min(implicitWidth, parent.parent ? parent.parent.width - 160 : implicitWidth)
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.readAgain()
            }
        }
    }

    Component {
        id: copyControl
        Plaque {
            function activate() {
                Quickshell.clipboardText = root.details();
                root.copied = "yes";
                copiedClear.restart();
            }
            kind: "quiet"
            small: true
            glyph: root.copied !== "" ? "check" : "content_copy"
            text: root.copied !== "" ? I18n.tr("Copied") : I18n.tr("Copy")
            onClicked: activate()
        }
    }

    Component {
        id: editControl
        Plaque {
            id: customize
            function activate() {
                editor.opener = customize;
                editor.open();
            }
            kind: "line"
            small: true
            glyph: "edit"
            text: I18n.tr("Customise")
            onClicked: activate()
        }
    }

    Component {
        id: copyCardControl
        Plaque {
            function activate() { if (root.cardJob === "") root.takeCard("copy"); }
            kind: "gold"
            small: true
            glyph: "content_copy"
            text: I18n.tr("Copy")
            busy: root.cardJob === "copy"
            onClicked: activate()
        }
    }

    Component {
        id: imageControl
        Plaque {
            function activate() { if (root.cardJob === "") root.takeCard("save"); }
            kind: "quiet"
            small: true
            glyph: "image"
            text: I18n.tr("Save")
            busy: root.cardJob === "save"
            onClicked: activate()
        }
    }

    Component {
        id: useControl
        Plaque {
            function activate() { if (root.cardJob === "") openDesign.open(); }
            kind: "quiet"
            small: true
            glyph: "file_open"
            text: I18n.tr("Open")
            busy: root.cardJob === "use"
            onClicked: activate()
        }
    }

    Component {
        id: pasteControl
        Plaque {
            function activate() { if (root.cardJob === "") root.useDesign("paste", ""); }
            kind: "quiet"
            small: true
            glyph: "content_paste"
            text: I18n.tr("Paste")
            busy: root.cardJob === "paste"
            onClicked: activate()
        }
    }

    Component {
        id: resetControl
        Plaque {
            function activate() { root.resetCard(); }
            kind: "dangerLine"
            small: true
            text: I18n.tr("Reset")
            onClicked: activate()
        }
    }

    // the card itself, standing in the section with its rarity beside it. It tilts toward the
    // pointer and its foil slides; resting on a tier in the ladder shows that tier's look.
    // A click opens the editor
    Component {
        id: cardBody
        Item {
            id: stage
            readonly property real cardHeight: Math.min(600, Math.max(420, width * 0.62))
            readonly property real cardScale: stage.cardHeight / trading.designHeight
            height: stage.cardHeight + 24
            property int preview: -1

            Item {
                id: holder
                x: 4
                y: 12
                width: trading.designWidth * stage.cardScale
                height: stage.cardHeight
                readonly property bool pointed: tilt.hovered && !Lithic.reduceMotion
                property real tx: pointed ? (tilt.point.position.x / width - 0.5) * 2 : 0
                property real ty: pointed ? (tilt.point.position.y / height - 0.5) * 2 : 0
                // a stiff spring: the card keeps up with the pointer and only eases the end
                Behavior on tx { SpringAnimation { spring: 30; damping: 1; mass: 0.5; epsilon: 0.002 } }
                Behavior on ty { SpringAnimation { spring: 30; damping: 1; mass: 0.5; epsilon: 0.002 } }

                TradingCard {
                    id: trading
                    transformOrigin: Item.TopLeft
                    scale: stage.cardScale
                    foil: Qt.vector2d(holder.tx, holder.ty)
                    live: holder.pointed || stage.preview >= 0
                    showLevel: stage.preview
                    Component.onCompleted: root.tradingCard = trading
                    Component.onDestruction: if (root.tradingCard === trading) root.tradingCard = null
                }
                transform: [
                    Rotation {
                        origin.x: holder.width / 2
                        origin.y: holder.height / 2
                        axis { x: 0; y: 1; z: 0 }
                        angle: holder.tx * 10
                    },
                    Rotation {
                        origin.x: holder.width / 2
                        origin.y: holder.height / 2
                        axis { x: 1; y: 0; z: 0 }
                        angle: -holder.ty * 8
                    }
                ]
                HoverHandler {
                    id: tilt
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    onTapped: {
                        editor.opener = null;
                        editor.open();
                    }
                }
                Accessible.role: Accessible.Button
                Accessible.name: I18n.tr("System card, customise it")
            }

            // how rare this machine's card is, and what the next tier takes
            Column {
                x: holder.x + holder.width + LithicTokens.space.s6
                y: 18
                width: Math.max(0, stage.width - x)
                spacing: LithicTokens.space.s2

                Label {
                    text: I18n.tr("Rarity")
                    typeRole: "group"
                    inkRole: "fg3"
                    font.capitalization: Font.AllUppercase
                    font.letterSpacing: 1.6
                }
                Label {
                    width: parent.width
                    text: I18n.tr("Your machine draws a %1 card").replace("%1", I18n.tr(trading.rarity.tier.name))
                    typeRole: "sentence"
                    font.pixelSize: 24
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Label {
                    width: parent.width
                    text: trading.rarity.next
                        ? I18n.tr("%1 points. %2 more for %3.").replace("%1", trading.rarity.points).replace("%2", trading.rarity.toNext).replace("%3", I18n.tr(trading.rarity.next.name))
                        : I18n.tr("%1 points, the rarest card there is.").replace("%1", trading.rarity.points)
                    typeRole: "body"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                // where the points come from
                Row {
                    topPadding: LithicTokens.space.s2
                    spacing: LithicTokens.space.s5
                    Repeater {
                        model: trading.rarity.parts
                        delegate: Column {
                            required property var modelData
                            Label {
                                text: ({ cpu: I18n.tr("Processor"), ram: I18n.tr("Memory"), gpu: I18n.tr("Graphics"), display: I18n.tr("Display") })[modelData.id]
                                typeRole: "caption"
                                inkRole: "fg3"
                            }
                            Label {
                                text: "+" + modelData.points
                                typeRole: "mono"
                                font.pixelSize: 20
                                ink: modelData.points > 0 ? Lithic.fg : Lithic.fg3
                            }
                        }
                    }
                }

                // the ladder: every tier, this card's lit; resting on one shows its look
                Column {
                    topPadding: LithicTokens.space.s4
                    width: parent.width
                    Repeater {
                        model: Rarity.tiers
                        delegate: Item {
                            id: rung
                            required property var modelData
                            required property int index
                            readonly property bool mine: index === trading.rarity.level
                            readonly property bool reached: index <= trading.rarity.level
                            width: parent.width
                            height: 30
                            CutRect {
                                anchors.fill: parent
                                cut: LithicTokens.cut.small
                                fill: rung.mine ? Tone.select : over.hovered ? Tone.hover : "transparent"
                            }
                            Label {
                                x: 10
                                width: 44
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.mark
                                typeRole: "label"
                                ink: rung.reached && rung.index >= 2 ? Lithic.gold : rung.reached ? Lithic.fg : Lithic.fg3
                            }
                            Label {
                                x: 60
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr(modelData.name)
                                typeRole: "label"
                                font.weight: rung.mine ? Font.DemiBold : Font.Normal
                                ink: rung.mine ? Tone.selectInk : rung.reached ? Lithic.fg : Lithic.fg3
                            }
                            Label {
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("%1 pts").replace("%1", modelData.min)
                                typeRole: "monoSm"
                                inkRole: "fg3"
                            }
                            HoverHandler {
                                id: over
                                onHoveredChanged: stage.preview = hovered ? rung.index : (stage.preview === rung.index ? -1 : stage.preview)
                            }
                        }
                    }
                }
                Label {
                    width: parent.width
                    text: I18n.tr("Rest on a tier to see its look. Copy the card to show it off; anyone with Ryoku can open it as a design.")
                    typeRole: "caption"
                    inkRole: "fg3"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
        }
    }

    Component {
        id: cardFooter
        Row {
            visible: root.cardMessage.length > 0 || root.cardError.length > 0
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: root.cardError.length > 0 ? root.cardError : root.cardMessage
                typeRole: "detail"
                ink: root.cardError.length > 0 ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                width: Math.min(implicitWidth, parent.parent ? parent.parent.width - 160 : implicitWidth)
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.cardError.length === 0 && root.cardFile.length > 0
                kind: "quiet"
                small: true
                glyph: "open_in_new"
                text: I18n.tr("Show")
                onClicked: Spawn.runApp(["xdg-open", root.cardFile.replace(/\/[^/]*$/, "")], { label: "files" })
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.cardError.length > 0 && root.cardLast !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.retryCard()
            }
        }
    }

    Component {
        id: crewFooter
        Column {
            id: crew
            Component.onCompleted: root.crewItem = crew
            Component.onDestruction: if (root.crewItem === crew) root.crewItem = null
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("The crash test crew, the alpha and beta testers who break it first")
                typeRole: "row"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Flow {
                width: parent.width
                spacing: LithicTokens.space.s2
                Repeater {
                    model: Schema.testers
                    delegate: Plaque {
                        required property var modelData
                        kind: "tonal"
                        small: true
                        glyph: "open_in_new"
                        text: modelData.name
                        onClicked: Spawn.runApp(["xdg-open", modelData.url], { label: "browser" })
                    }
                }
            }
            Label {
                width: parent.width
                text: I18n.tr("The figures on the cards and posters are generated during development and finished by hand.")
                typeRole: "detail"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }
}
