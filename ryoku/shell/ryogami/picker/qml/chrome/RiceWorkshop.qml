import QtQuick
import Lithic
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Item {
    id: root

    required property PickerState state

    anchors.fill: parent

    readonly property bool shown: root.state ? root.state.riceWorkshopOpen : false
    readonly property bool busy: filesProc.running
    readonly property var rice: root.state ? root.state.riceEntry : null

    readonly property string slug: rice ? String(rice.slug || "") : ""
    readonly property string name: rice ? String(rice.name || rice.slug || "") : ""
    readonly property string author: rice ? String(rice.author || "") : ""
    readonly property string blurb: rice ? String(rice.blurb || "") : ""
    readonly property string compat: rice ? String(rice.compat || "") : ""
    readonly property string createdWith: rice ? String(rice.createdWith || "") : ""
    readonly property string preview: rice ? String(rice.preview || "") : ""
    readonly property bool live: rice ? rice.live === true : false
    readonly property bool active: rice ? rice.active === true : false
    readonly property var tags: (rice && rice.tags)
        ? (Array.isArray(rice.tags) ? rice.tags : String(rice.tags).split(","))
        : []

    property real reveal: 0
    states: State { name: "open"; when: root.shown; PropertyChanges { target: root; reveal: 1 } }
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: reveal > 0.01

    function close() { if (root.state) root.state.riceWorkshopOpen = false }
    Keys.onEscapePressed: root.close()

    onShownChanged: {
        if (root.shown) { root._loadTouches(); forceActiveFocus() }
    }

    ListModel { id: touchModel }
    property string _touchBuf: ""
    function _loadTouches() {
        touchModel.clear()
        _touchBuf = ""
        if (root.slug === "") return
        filesProc.command = ["ryoku-hub", "rice", "files", root.slug]
        filesProc.running = true
    }
    Process {
        id: filesProc
        stdout: SplitParser { splitMarker: ""; onRead: data => root._touchBuf += data }
        onExited: {
            touchModel.clear()
            var d = null
            try { d = JSON.parse(root._touchBuf) } catch (e) { d = null }
            root.summary = root._summarize(d && d.config ? d.config : "")
            var t = (d && d.touches) ? d.touches : []
            for (var i = 0; i < t.length; ++i)
                if (t[i].provided === true)
                    touchModel.append({ touchLabel: String(t[i].label || "") })
        }
    }

    // What the look is, read from the rice itself: its style, colours, contrast and prompt.
    property var summary: []
    function _summarize(text) {
        var r = null
        try { r = JSON.parse(text) } catch (e) { return [] }
        var out = [], look = (r && r.look) || {}
        var theme = (look.shell && look.shell.theme) || {}
        var styles = { lithic: I18n.tr("Lithic style"), material: I18n.tr("Material style"),
                       adwaita: I18n.tr("Adwaita style"), fluent: I18n.tr("Fluent style") }
        out.push(styles[theme.design] || styles.lithic)
        if (theme.colours === "ryoku") out.push(I18n.tr("Ryoku colours"))
        else if (r.color && r.color.mode === "fixed") out.push(I18n.tr("Fixed palette"))
        else out.push(I18n.tr("Wallpaper colours"))
        if (theme.contrast === "high") out.push(I18n.tr("High contrast"))
        if (look.starship && look.starship.layout) out.push(I18n.tr("Prompt: %1").arg(look.starship.layout))
        if (look.shell && look.shell.barStyle) out.push(I18n.tr("Bar: %1").arg(look.shell.barStyle))
        return out
    }
    readonly property string compatText: root.compat === "ok" ? I18n.tr("Made for this Ryoku")
        : root.compat === "older" ? I18n.tr("Made with an older Ryoku")
        : root.compat === "newer" ? I18n.tr("Made with a newer Ryoku") : ""

    function _run(args) { Quickshell.execDetached(args) }

    function applyRice() {
        if (root.slug === "") return
        root._run(["ryoku-hub", "rice", "apply", root.slug, "all"])
        if (root.state) root.state.toast(I18n.tr("Applying rice"), "info")
        root.close()
    }
    function forkRice() {
        if (root.slug === "") return
        root._run(["ryoku-hub", "rice", "fork", root.slug])
        if (root.state) root.state.toast(I18n.tr("Forked rice"), "success")
    }
    function undoRice() {
        root._run(["ryoku-hub", "rice", "restore", "previous"])
        if (root.state) root.state.toast(I18n.tr("Undoing the last rice"), "info")
        root.close()
    }
    function restoreRice() {
        root._run(["ryoku-hub", "rice", "restore"])
        if (root.state) root.state.toast(I18n.tr("Restoring your desktop"), "info")
    }
    function deleteRice() {
        if (root.slug === "") return
        root._run(["ryoku-hub", "rice", "delete", root.slug])
        if (root.state) root.state.toast(I18n.tr("Deleted rice"), "success")
        root.close()
    }

    Scrim {
        anchors.fill: parent
        alpha: 0.55
        reveal: root.reveal
        onDismissed: root.close()
    }

    CutRect {
        id: card
        anchors.centerIn: parent
        width: Math.min(parent.width - 100 * Theme.scale, 880 * Theme.scale)
        height: Math.min(parent.height - 100 * Theme.scale, 600 * Theme.scale)
        opacity: root.reveal
        transform: Translate { y: (1 - root.reveal) * 12 * Theme.scale }

        cut: LithicTokens.cut.large * Theme.scale
        fill: Lithic.stone4
        strokeWidth: 1
        strokeColor: Lithic.vein

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

        Row {
            anchors.fill: parent
            anchors.margins: 22 * Theme.scale
            spacing: 22 * Theme.scale

            CutRect {
                id: preview
                width: (parent.width - parent.spacing) * 0.42
                height: parent.height
                clip: true
                fill: Theme.withAlpha(Theme.surfaceContainer, 0.9)

                Image {
                    anchors.fill: parent
                    visible: root.preview !== "" && status === Image.Ready
                    source: Library.fileUrl(root.preview)
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: false
                    sourceSize.width: 560
                    sourceSize.height: 680
                }
                Text {
                    anchors.centerIn: parent
                    visible: root.preview === ""
                    text: root.name.length > 0 ? root.name.charAt(0).toUpperCase() : "?"
                    font.family: Theme.display
                    font.pixelSize: 120 * Theme.scale
                    color: Theme.withAlpha(Theme.surfaceText, 0.22)
                    renderType: Text.NativeRendering
                }
                CutRect {
                    anchors.fill: parent
                    fill: "transparent"
                    strokeWidth: 1
                    strokeColor: Theme.withAlpha(Theme.outline, 0.4)
                }
            }

            Item {
                width: parent.width - preview.width - parent.spacing
                height: parent.height

                Column {
                    id: infoCol
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: 8 * Theme.scale

                    Text {
                        visible: root.author !== ""
                        text: root.author.toUpperCase()
                        font.family: Theme.sans; font.weight: Font.Medium
                        font.pixelSize: Theme.fontFine
                        font.letterSpacing: 2
                        color: Theme.withAlpha(Theme.surfaceText, 0.6)
                        renderType: Text.NativeRendering
                    }
                    Text {
                        width: parent.width
                        text: root.name
                        font.family: Theme.display
                        font.pixelSize: Theme.fontSegment
                        color: Theme.surfaceText
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        renderType: Text.NativeRendering
                    }
                    Flow {
                        width: parent.width
                        spacing: 6 * Theme.scale
                        Repeater {
                            model: root.summary
                            delegate: RiceChip {
                                required property var modelData
                                label: String(modelData)
                                tint: Lithic.sel
                                strong: true
                            }
                        }
                        RiceChip { visible: root.compatText !== ""; label: root.compatText }
                        RiceChip { visible: root.createdWith !== ""; label: "v" + root.createdWith }
                        RiceChip { visible: root.live; label: I18n.tr("LIVE"); tint: Theme.surfaceText; strong: true }
                        RiceChip { visible: root.active; label: I18n.tr("ACTIVE"); tint: Theme.surfaceText; strong: true }
                        Repeater {
                            model: root.tags
                            delegate: RiceChip {
                                required property var modelData
                                label: String(modelData).trim()
                                visible: label.length > 0
                            }
                        }
                    }
                    Text {
                        visible: root.blurb !== ""
                        width: parent.width
                        text: root.blurb
                        wrapMode: Text.WordWrap
                        maximumLineCount: 3
                        elide: Text.ElideRight
                        font.family: Theme.sans; font.weight: Font.Medium
                        font.pixelSize: Theme.fontBase
                        color: Theme.withAlpha(Theme.surfaceText, 0.62)
                        renderType: Text.NativeRendering
                    }
                    Item { width: 1; height: 6 * Theme.scale }
                    SectionLabel { width: parent.width; text: I18n.tr("Touches") }
                    Flow {
                        width: parent.width
                        spacing: 6 * Theme.scale
                        Repeater {
                            model: touchModel
                            delegate: RiceChip {
                                required property string touchLabel
                                label: touchLabel
                            }
                        }
                        Text {
                            visible: touchModel.count === 0
                            text: "\u2014"
                            font.family: Theme.sans; font.weight: Font.Medium
                            font.pixelSize: Theme.fontBase
                            color: Theme.withAlpha(Theme.surfaceText, 0.4)
                            renderType: Text.NativeRendering
                        }
                    }
                }

                Column {
                    anchors.bottom: parent.bottom
                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: 10 * Theme.scale

                    Row {
                        spacing: 8 * Theme.scale
                        FixedButton {
                            mode: "action"
                            label: I18n.tr("Apply")
                            gold: true
                            onTriggered: root.applyRice()
                        }
                        FixedButton {
                            visible: root.active
                            mode: "action"
                            label: I18n.tr("Undo")
                            onTriggered: root.undoRice()
                        }
                        FixedButton {
                            mode: "action"
                            label: I18n.tr("Fork")
                            onTriggered: root.forkRice()
                        }
                        FixedButton {
                            mode: "action"
                            label: I18n.tr("Restore original")
                            onTriggered: root.restoreRice()
                        }
                        FolioDestructiveAction {
                            label: I18n.tr("Delete")
                            confirm: true
                            onTriggered: root.deleteRice()
                        }
                    }

                    Item { width: 1; height: 4 * Theme.scale }
                    SectionLabel { width: parent.width; text: I18n.tr("Share") }

                    // Export and import open the rice share dialog, which runs the command
                    // tracked and reports what really happened.
                    Row {
                        spacing: 8 * Theme.scale
                        FixedButton {
                            mode: "action"
                            glyph: "\u{f0193}"
                            label: I18n.tr("Save look")
                            onTriggered: if (root.state) root.state.openRiceShare("save", null)
                        }
                        FixedButton {
                            mode: "action"
                            glyph: "\u{f0207}"
                            label: I18n.tr("Export")
                            enabled: root.slug !== ""
                            onTriggered: if (root.state) root.state.openRiceShare("export", root.rice)
                        }
                        FixedButton {
                            mode: "action"
                            glyph: "\u{f02fa}"
                            label: I18n.tr("Import")
                            onTriggered: if (root.state) root.state.openRiceShare("import", null)
                        }
                    }
                }
            }
        }
    }
}
