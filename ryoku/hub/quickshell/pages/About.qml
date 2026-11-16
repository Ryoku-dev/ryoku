import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../schema/about.js" as Schema

// what this computer runs and is made of, and the credits
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property string edition: ""
    property string copied: ""
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
        rows: Schema.rows
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

    // the credits footer holding the testers, where their deep link lands
    property Item crewItem: null

    FileView {
        path: "/etc/os-release"
        onLoaded: {
            var m = /^PRETTY_NAME="?([^"\n]*)"?/m.exec(text());
            root.edition = m ? m[1] : "";
        }
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
            var c = { "copy-details": copyControl };
            Schema.rows.forEach(r => { if (r.fact) c[r.id] = factControl; });
            return c;
        }
        footers: ({ computer: factsFooter, credits: crewFooter })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
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
        id: crewFooter
        Column {
            id: crew
            Component.onCompleted: root.crewItem = crew
            Component.onDestruction: if (root.crewItem === crew) root.crewItem = null
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("The people who helped lift Ryoku up in its alpha days, testing it before anyone else")
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
