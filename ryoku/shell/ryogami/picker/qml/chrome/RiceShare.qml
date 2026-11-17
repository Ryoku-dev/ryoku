import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// Save, export and import for rices. Every command runs through one tracked Process:
// the backend prints JSON on success and an error on failure, so the toast reports what
// actually happened and a refused folder keeps the dialog open to pick another.
Item {
    id: root

    required property PickerState state

    anchors.fill: parent

    readonly property string mode: root.state ? root.state.riceShare : ""
    readonly property var entry: root.state ? root.state.riceShareEntry : null
    readonly property string slug: root.entry ? String(root.entry.slug || "") : ""
    readonly property string riceName: root.entry ? String(root.entry.name || root.entry.slug || "") : ""
    readonly property string home: Quickshell.env("HOME") || ""

    // Holds the last mode through the fade-out, so the card does not blank as it leaves.
    property string shownMode: ""
    onModeChanged: {
        if (root.mode === "")
            return
        root.shownMode = root.mode
        root._prepare()
    }

    property real reveal: 0
    states: State { name: "open"; when: root.mode !== ""; PropertyChanges { target: root; reveal: 1 } }
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: reveal > 0.01

    function close() { if (root.state) root.state.closeRiceShare() }

    function tidy(p) {
        return (root.home !== "" && p.indexOf(root.home) === 0) ? "~" + p.substring(root.home.length) : p
    }
    // Mirrors the backend's slugify, so the dialog can say a name replaces a saved rice.
    function slugify(s) {
        return String(s).trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "")
    }
    readonly property string typedSlug: root.slugify(nameField.text)
    readonly property var clash: root.typedSlug !== "" ? Library.entry("rices", root.typedSlug) : null
    readonly property string clashName: (root.clash && root.clash.slug)
        ? String(root.clash.name || root.clash.slug) : ""

    function _prepare() {
        if (root.mode === "save") {
            nameField.text = ""
            root._loadPreflight()
        } else {
            browser.mode = root.mode
            browser.open(root._startFolder())
        }
        Qt.callLater(function () {
            if (root.mode === "save") nameField.forceActiveFocus()
            else root.forceActiveFocus()
        })
    }
    // Exports gather in ~/Rices; a rice someone shared usually arrives in ~/Downloads.
    function _startFolder() {
        var order = root.mode === "import" ? ["Downloads", "Rices"] : ["Rices"]
        for (var i = 0; i < order.length; ++i)
            if (browser.placeExists(order[i])) return root.home + "/" + order[i]
        return root.home
    }

    // What a save carries, read from the backend's own preflight so the chips never
    // promise more than the capture takes.
    ListModel { id: carryModel }
    property string _preBuf: ""
    function _loadPreflight() {
        carryModel.clear()
        root._preBuf = ""
        preProc.running = true
    }
    Process {
        id: preProc
        command: ["ryoku-hub", "rice", "preflight"]
        stdout: SplitParser { splitMarker: ""; onRead: data => root._preBuf += data }
        onExited: {
            var d = null
            try { d = JSON.parse(root._preBuf) } catch (e) { d = null }
            carryModel.clear()
            if (!d) return
            function add(label) { carryModel.append({ carry: label }) }
            if (d.wallpaper) add(d.live ? I18n.tr("Live wallpaper") : I18n.tr("Wallpaper"))
            add(d.fixed ? I18n.tr("Locked colours") : I18n.tr("Colours"))
            if (d.themeApps) add(I18n.tr("App colours"))
            add(I18n.tr("Shell and bar"))
            add(I18n.tr("Launcher"))
            add(I18n.tr("Window look"))
            if (d.widgets) add(I18n.tr("Desktop widgets"))
            if (d.plugins) add(I18n.tr("Plugin widgets"))
            if (d.stage) add(I18n.tr("Widget stage"))
            if (d.visualizer) add(I18n.tr("Visualiser"))
            if (d.profile) add(I18n.tr("Profile decor"))
            if (d.decors > 0) add(I18n.tr("%1 decors").arg(d.decors))
            if (d.lock) add(I18n.tr("Lock screen"))
            if (d.fastfetch) add(I18n.tr("Fastfetch"))
            var layers = d.layers || []
            if (layers.indexOf("input") >= 0) add(I18n.tr("Input"))
            if (layers.indexOf("brand") >= 0) add(I18n.tr("Brand mark"))
        }
    }

    property string job: ""
    property string jobBuf: ""
    readonly property bool busy: preProc.running || jobProc.running
    Process {
        id: jobProc
        stdout: SplitParser { splitMarker: ""; onRead: data => root.jobBuf += data }
        stderr: SplitParser { splitMarker: ""; onRead: data => root.jobBuf += data }
        onExited: (code) => root._finish(code)
    }
    function _run(kind, args) {
        if (jobProc.running) return
        root.jobBuf = ""
        root.job = kind
        jobProc.command = args
        jobProc.running = true
    }
    function _finish(code) {
        var kind = root.job
        var out = root.jobBuf.trim()
        root.job = ""
        root.jobBuf = ""
        if (!root.state) return
        if (code !== 0) {
            var lines = out.split("\n")
            var why = lines[lines.length - 1].replace(/^ryoku-hub:\s*/, "")
            var what = kind === "export" ? I18n.tr("Export failed")
                : kind === "import" ? I18n.tr("Import failed") : I18n.tr("Save failed")
            root.state.toast(why !== "" ? what + ": " + why : what, "error")
            return
        }
        var d = null
        try { d = JSON.parse(out) } catch (e) { d = null }
        if (kind === "export") {
            var p = d && d.path ? String(d.path) : ""
            root.state.toast(p !== "" ? I18n.tr("Exported to %1").arg(root.tidy(p)) : I18n.tr("Rice exported"), "success")
        } else {
            var name = d && d.name ? String(d.name) : ""
            root.state.toast(kind === "import"
                ? (name !== "" ? I18n.tr("Imported %1").arg(name) : I18n.tr("Rice imported"))
                : (name !== "" ? I18n.tr("Saved %1").arg(name) : I18n.tr("Look saved")), "success")
            root.pendingKey = d && d.slug ? String(d.slug) : ""
            root._focusPending()
        }
        root.close()
    }

    // A saved or imported rice lands in the list a moment later; bring it under the cursor.
    property string pendingKey: ""
    function _focusPending() {
        if (root.pendingKey === "" || !root.state || !root.state.view || !root.state.field) return
        if (root.state.collection !== "rices") return
        var i = root.state.view.indexOfKey(root.pendingKey)
        if (i < 0) return
        root.state.field.currentIndex = i
        root.pendingKey = ""
    }
    // The field re-anchors on its old card after a reload, so the jump waits a turn.
    Connections {
        target: root.state ? root.state.view : null
        function onCountChanged() { if (root.pendingKey !== "") Qt.callLater(root._focusPending) }
    }

    readonly property bool ready: {
        if (root.busy) return false
        if (root.shownMode === "save") return root.typedSlug !== ""
        if (root.shownMode === "export") return root.slug !== "" && browser.path !== ""
        if (root.shownMode === "import") return browser.riceFolder !== ""
        return false
    }
    function commit() {
        if (!root.ready) return
        if (root.shownMode === "save")
            root._run("capture", ["ryoku-hub", "rice", "capture", "--force", nameField.text.trim(), "all"])
        else if (root.shownMode === "export")
            root._run("export", ["ryoku-hub", "rice", "export", root.slug, browser.path])
        else if (root.shownMode === "import")
            root._run("import", ["ryoku-hub", "rice", "import", browser.riceFolder])
    }
    function importNow(path) {
        browser.selected = path
        root.commit()
    }

    Keys.onPressed: (event) => {
        if (event.key === Qt.Key_Escape) {
            root.close()
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            root.commit()
        } else if (event.key === Qt.Key_Backspace && root.shownMode !== "save") {
            browser.up()
        }
        event.accepted = true
    }

    readonly property string kicker: root.shownMode === "save" ? I18n.tr("Save look")
        : root.shownMode === "export" ? I18n.tr("Export")
        : I18n.tr("Import")
    readonly property string title: root.shownMode === "save" ? I18n.tr("Save this desktop as a rice")
        : root.shownMode === "export" ? root.riceName
        : I18n.tr("Import a rice")
    readonly property string lede: root.shownMode === "save"
        ? I18n.tr("Name it, and everything below is kept together so you can come back to it or pass it on.")
        : root.shownMode === "export"
        ? I18n.tr("Choose a folder to export into. The rice goes in a folder of its own, with a README and readable copies of its settings.")
        : I18n.tr("Open the folder someone shared with you. Folders that hold a rice show its preview; pick one and import it.")
    readonly property string footnote: {
        if (root.shownMode === "save") {
            if (root.typedSlug === "") return I18n.tr("A name is all it needs.")
            return root.clashName !== ""
                ? I18n.tr("Replaces your saved rice “%1”.").arg(root.clashName)
                : I18n.tr("Saved to your rices as “%1”.").arg(nameField.text.trim())
        }
        if (root.shownMode === "export")
            return I18n.tr("Writes %1").arg(root.tidy(browser.path.replace(/\/+$/, "") + "/" + root.slug))
        if (browser.riceFolder !== "")
            return I18n.tr("Imports %1").arg(root.tidy(browser.riceFolder))
        return I18n.tr("Pick a folder marked Rice.")
    }
    readonly property string actionLabel: root.busy
        ? (root.shownMode === "save" ? I18n.tr("Saving") : root.shownMode === "export" ? I18n.tr("Exporting") : I18n.tr("Importing"))
        : (root.shownMode === "save" ? (root.clashName !== "" ? I18n.tr("Replace") : I18n.tr("Save"))
           : root.shownMode === "export" ? I18n.tr("Export here") : I18n.tr("Import"))
    readonly property string actionGlyph: root.shownMode === "save" ? "\u{f0193}"
        : root.shownMode === "export" ? "\u{f0207}" : "\u{f02fa}"

    Scrim {
        anchors.fill: parent
        alpha: 0.55
        reveal: root.reveal
        onDismissed: root.close()
    }

    Rectangle {
        id: card
        anchors.centerIn: parent
        readonly property bool compact: root.shownMode === "save"
        width: Math.min(parent.width - 100 * Theme.scale, (compact ? 640 : 900) * Theme.scale)
        height: compact
            ? Math.min(parent.height - 100 * Theme.scale, saveBody.implicitHeight + header.implicitHeight + foot.height + 76 * Theme.scale)
            : Math.min(parent.height - 100 * Theme.scale, 620 * Theme.scale)
        opacity: root.reveal
        transform: Translate { y: (1 - root.reveal) * 12 * Theme.scale }

        color: Theme.withAlpha(Theme.surface, 0.99)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.58)

        MouseArea { anchors.fill: parent }

        Text {
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.topMargin: 12 * Theme.scale
            anchors.rightMargin: 14 * Theme.scale
            text: "\u00d7"
            font.family: Theme.sans; font.weight: Font.Medium
            font.pixelSize: Theme.fontTitle
            color: closeMouse.containsMouse ? Theme.surfaceText : Theme.withAlpha(Theme.surfaceText, 0.7)
            z: 5
            renderType: Text.NativeRendering
            MouseArea {
                id: closeMouse
                anchors.fill: parent
                anchors.margins: -8 * Theme.scale
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.close()
            }
        }

        Column {
            id: header
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: 22 * Theme.scale
            anchors.rightMargin: 48 * Theme.scale
            spacing: 6 * Theme.scale

            Text {
                text: root.kicker.toUpperCase()
                font.family: Theme.sans; font.weight: Font.Medium
                font.pixelSize: Theme.fontFine
                font.letterSpacing: 2
                color: Theme.withAlpha(Theme.surfaceText, 0.6)
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                text: root.title
                font.family: Theme.display
                font.pixelSize: Theme.fontSegment
                color: Theme.surfaceText
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                text: root.lede
                wrapMode: Text.WordWrap
                font.family: Theme.sans; font.weight: Font.Medium
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.62)
                renderType: Text.NativeRendering
            }
        }

        Item {
            id: body
            anchors.top: header.bottom
            anchors.topMargin: 18 * Theme.scale
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: 22 * Theme.scale
            anchors.rightMargin: 22 * Theme.scale
            anchors.bottom: foot.top
            anchors.bottomMargin: 14 * Theme.scale

            Column {
                id: saveBody
                visible: root.shownMode === "save"
                width: parent.width
                spacing: 10 * Theme.scale

                SectionLabel { width: parent.width; text: I18n.tr("Name") }
                TextField {
                    id: nameField
                    width: parent.width
                    height: 36 * Theme.scale
                    variant: "field"
                    placeholder: I18n.tr("Midnight, Frieren, Work desk…")
                    onCommitted: if (root.mode === "save" && nameField.editing) root.commit()
                }
                Item { width: 1; height: 6 * Theme.scale }
                SectionLabel { width: parent.width; text: I18n.tr("Carries") }
                Flow {
                    width: parent.width
                    spacing: 6 * Theme.scale
                    Repeater {
                        model: carryModel
                        delegate: RiceChip {
                            required property string carry
                            label: carry
                        }
                    }
                    Text {
                        visible: carryModel.count === 0
                        text: preProc.running ? I18n.tr("Reading your desktop…") : "\u2014"
                        font.family: Theme.sans; font.weight: Font.Medium
                        font.pixelSize: Theme.fontBase
                        color: Theme.withAlpha(Theme.surfaceText, 0.45)
                        renderType: Text.NativeRendering
                    }
                }
            }

            RiceFolderBrowser {
                id: browser
                visible: root.shownMode === "export" || root.shownMode === "import"
                anchors.fill: parent
                onRiceActivated: (path) => root.importNow(path)
            }
        }

        Item {
            id: foot
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 58 * Theme.scale

            FolioRule {
                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                alpha: 0.35
            }
            Text {
                anchors.left: parent.left
                anchors.leftMargin: 22 * Theme.scale
                anchors.right: actions.left
                anchors.rightMargin: 16 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                text: root.footnote
                elide: Text.ElideMiddle
                font.family: Theme.sans; font.weight: Font.Medium
                font.pixelSize: Theme.fontBody
                color: root.clashName !== "" && root.shownMode === "save"
                    ? Theme.tertiary
                    : Theme.withAlpha(Theme.surfaceText, 0.68)
                renderType: Text.NativeRendering
            }
            Row {
                id: actions
                anchors.right: parent.right
                anchors.rightMargin: 22 * Theme.scale
                anchors.verticalCenter: parent.verticalCenter
                spacing: 8 * Theme.scale
                FixedButton {
                    mode: "action"
                    label: I18n.tr("Cancel")
                    onTriggered: root.close()
                }
                FixedButton {
                    mode: "action"
                    glyph: root.actionGlyph
                    label: root.actionLabel
                    active: root.ready || root.busy
                    enabled: root.ready
                    minWidth: 110
                    onTriggered: root.commit()
                }
            }
        }
    }
}
