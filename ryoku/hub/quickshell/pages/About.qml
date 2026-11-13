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

    // the still card on the page is the one the picture is taken from
    property Item stillCard: null
    // the credits footer holding the testers, where their deep link lands
    property Item crewItem: null

    function saveImage() {
        root.cardError = "";
        root.cardMessage = "";
        root.cardLast = "image";
        if (!root.stillCard) {
            root.cardError = I18n.tr("The card is not drawn yet. Scroll to it, then try again.");
            return;
        }
        var path = Paths.picturesDir + "/ryoku-card-" + Qt.formatDateTime(new Date(), "yyyyMMdd-HHmmss") + ".png";
        root.cardJob = "image";
        var card = root.stillCard;
        var ok = card.grabToImage(function (result) {
            root.cardJob = "";
            if (result.saveToFile(path)) {
                root.cardFile = path;
                root.cardMessage = I18n.tr("Saved to %1").replace("%1", root.tilde(path));
            } else {
                root.cardError = I18n.tr("The picture could not be saved in %1. Check that the folder exists and can be written to, then try again.").replace("%1", root.tilde(Paths.picturesDir));
            }
        }, Qt.size(card.designWidth * 2, card.designHeight * 2));
        if (!ok) {
            root.cardJob = "";
            root.cardError = I18n.tr("The card could not be drawn into a picture. Try again.");
        }
    }

    function shareDesign(folder) {
        var name = String(SysInfo.sysUser || "card").replace(/[^A-Za-z0-9._-]/g, "") || "card";
        root.cardFile = folder + "/ryoku-" + name + ".ryoprofile";
        root.runCard("share", root.cardFile);
    }

    function useDesign(file) {
        root.hub.confirm({
            title: I18n.tr("Use this card design?"),
            text: I18n.tr("Its figure, words and choices replace your card's. To keep yours, share it first."),
            actionText: I18n.tr("Use it"),
            destructive: false
        }, () => {
            root.cardFile = "";
            root.runCard("use", file);
        });
    }

    // a refused share or use, in plain words with its next step; the tool's own
    // words ("open …: permission denied", JSON parse errors) are not shown
    function cardRefusal(job, err) {
        var e = String(err || "");
        var use = job === "use";
        if (/no space left/i.test(e))
            return use ? I18n.tr("The disk is full, so the design could not be used. Free some space, then press Retry.")
                       : I18n.tr("The disk is full, so the design could not be written. Free some space, then press Retry.");
        if (use && /ryoprofile|hero|invalid character|unexpected end|cannot unmarshal/i.test(e))
            return I18n.tr("That file is not a card design, or it is damaged. Choose a .ryoprofile someone shared with Open.");
        if (/permission denied|read-only file system|operation not permitted/i.test(e))
            return use ? I18n.tr("Settings is not allowed to read that file or save your card. Check the file's permissions, then press Retry, or choose another file with Open.")
                       : I18n.tr("Settings is not allowed to write in that folder. Change the folder's permissions, then press Retry, or choose another folder with Share.");
        if (/no such file|not a directory/i.test(e))
            return use ? I18n.tr("That file is no longer there. Choose another with Open.")
                       : I18n.tr("That folder is no longer there. Choose another with Share.");
        return use ? I18n.tr("That design could not be used. Press Retry, or choose another file with Open.")
                   : I18n.tr("The design could not be written. Press Retry, or choose another folder with Share.");
    }

    // the last card job, so Retry runs it again
    property string cardLast: ""
    property string cardArg: ""
    function runCard(job, path) {
        root.cardError = "";
        root.cardMessage = "";
        root.cardLast = job;
        root.cardArg = path;
        root.cardJob = job;
        var argv = ["ryoku-hub", "profile", job === "use" ? "import" : "export", path];
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
            root.cardMessage = job === "use" ? I18n.tr("The card uses the shared design")
                : I18n.tr("Shared as %1").replace("%1", root.tilde(root.cardFile));
        }, { timeout: 30000 });
    }

    function retryCard() {
        if (root.cardLast === "image")
            root.saveImage();
        else if (root.cardLast !== "")
            root.runCard(root.cardLast, root.cardArg);
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

    FolderChooser {
        id: shareTo
        overlayParent: root.hub.overlayItem
        title: I18n.tr("Share the card's design into")
        onAccepted: root.shareDesign(root.localPath(shareTo.selectedFolder))
    }

    RyokuUi.FileDialogHost {
        id: openDesign
        title: I18n.tr("Open a card design")
        nameFilters: [I18n.tr("Card designs") + " (*.ryoprofile)"]
        onAccepted: root.useDesign(root.localPath(openDesign.selectedFile))
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
            var c = { "copy-details": copyControl, "card-edit": editControl, "card-image": imageControl,
                      "card-share": shareControl, "card-use": useControl, "card-reset": resetControl };
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
        id: imageControl
        Plaque {
            function activate() { if (root.cardJob === "") root.saveImage(); }
            kind: "quiet"
            small: true
            glyph: "image"
            text: I18n.tr("Save")
            busy: root.cardJob === "image"
            onClicked: activate()
        }
    }

    Component {
        id: shareControl
        Plaque {
            function activate() { if (root.cardJob === "") shareTo.open(); }
            kind: "quiet"
            small: true
            glyph: "share"
            text: I18n.tr("Share")
            busy: root.cardJob === "share"
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
        id: resetControl
        Plaque {
            function activate() { root.resetCard(); }
            kind: "dangerLine"
            small: true
            text: I18n.tr("Reset")
            onClicked: activate()
        }
    }

    // the card itself, still, sized to the column; a click opens the editor
    Component {
        id: cardBody
        CutRect {
            id: plate
            height: width * 10 / 16
            cut: LithicTokens.cut.small
            fill: Lithic.ground
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            Accessible.role: Accessible.Button
            Accessible.name: I18n.tr("System card, customise it")

            Item {
                anchors.fill: parent
                clip: true
                // the card IS Home's plate, loaded by URL (a static import would
                // clash with the components directory and cycle through the editor)
                Loader {
                    id: still
                    anchors.centerIn: parent
                    width: plate.width
                    height: plate.height
                    Component.onCompleted: setSource("../components/Home.qml", { hub: root.hub, preview: true })
                    onLoaded: {
                        item.width = item.designWidth;
                        item.height = item.designHeight;
                        item.transformOrigin = Item.TopLeft;
                        item.scale = plate.width / item.designWidth;
                        root.stillCard = item;
                    }
                    onItemChanged: if (!item)
                        root.stillCard = null;
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
            TapHandler {
                onTapped: {
                    editor.opener = null;
                    editor.open();
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
