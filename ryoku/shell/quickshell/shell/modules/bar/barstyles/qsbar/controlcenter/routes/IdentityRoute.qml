import QtQuick
import "../kit"
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Identity route (id "identity"): what makes this bar this user's. The launcher
// mark first - wordmark or kanji glyph (launcherLogoMode "text" | "icon"), then
// a grid of the chosen format's options (launcherLogoText / launcherLogoIcon),
// every tile drawn exactly as LauncherWidget.qml draws it - then the workspaces:
// how many the bar shows (workspaceMode), the marker each one wears
// (workspaceStyle), and a live preview in the bar's own colours. These were the
// old Logo and Spaces routes, folded for a while into the Widgets rows; they are
// the bar's identity, so they get their own door again, with the full option
// sets the rows never had room for.
Item {
    id: page
    property var root: null
    property var cc: null
    readonly property var tk: cc ? cc.tokens : null
    readonly property real colW: tk ? Math.min(page.width, tk.contentW) : page.width

    readonly property string mode: page.root ? String(page.root.launcherLogoMode || "text") : "text"
    readonly property var activeOptions: page.root
        ? (page.mode === "icon" ? page.root.launcherLogoIconOptions : page.root.launcherLogoTextOptions)
        : []

    implicitHeight: col.implicitHeight


    // A hairline preview card whose selection reads as a bone ring plus a bone
    // corner tag, so the chosen mark stays legible instead of being swallowed by
    // an inverted plate. Emphasis is inversion elsewhere; here the mark IS data,
    // so the card keeps paper and only the frame carries the state.
    component MarkTag: Rectangle {
        property bool on: false
        visible: on
        anchors.right: parent.right
        anchors.top: parent.top
        width: page.tk ? page.tk.gap : 12
        height: width
        radius: page.tk ? Tokens.radius / 2 : 2
        color: page.tk ? Tokens.bone : "#cdc4ba"
    }


    // Presentable captions for the segmented controls: the house Seg/Chips render
    // the option string itself, so the caption lives in `options` and is mapped to
    // the stored key here -- the backing write stays exactly what it was.
    function countCap(m) { return m === "active" ? "Active" : (m === "10" ? "1-10" : "1-5") }
    function countKey(c) { return c === "Active" ? "active" : (c === "1-10" ? "10" : "5") }
    function styleCap(key) {
        var opts = page.root ? page.root.workspaceStyleOptions : []
        for (var i = 0; i < opts.length; i++) if (opts[i].key === key) return opts[i].label
        return ""
    }
    function styleKey(label) {
        var opts = page.root ? page.root.workspaceStyleOptions : []
        for (var i = 0; i < opts.length; i++) if (opts[i].label === label) return opts[i].key
        return label
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: col.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: col
            width: page.colW
            spacing: page.tk ? page.tk.sectionGap : 24

            Entrance {
                width: page.colW
                index: 0
                SettingCard {
                    width: page.colW
                    title: I18n.tr("Mark")
                    kana: "\u5370"

                    Row {
                    width: parent.width
                        spacing: page.tk ? page.tk.colGap : 16

                        Repeater {
                            model: [
                                { mode: "text", label: I18n.tr("Wordmark"), glyph: "RYOKU" },
                                { mode: "icon", label: I18n.tr("Glyph"),    glyph: UiPreferences.brandMark("\u529b") }
                            ]

                            delegate: Rectangle {
                                id: markCard
                                required property var modelData
                                readonly property bool selected: page.mode === modelData.mode
                                readonly property bool iconMode: modelData.mode === "icon"

                            width: (parent.width - (page.tk ? page.tk.colGap : 16)) / 2
                                height: page.tk ? page.tk.rowH * 3 : 120
                                radius: page.tk ? Tokens.radius : 6
                                color: markCardMa.containsMouse ? (page.tk ? Tokens.tint5 : "#111111") : "transparent"
                                border.width: markCard.selected ? 2 : 1
                                border.color: markCard.selected ? (page.tk ? Tokens.bone : "#cdc4ba")
                                    : markCardMa.containsMouse ? (page.tk ? Tokens.ink : "#cccccc")
                                    : (page.tk ? Tokens.line : "#333333")
                                Behavior on border.color { ColorAnimation { duration: page.tk ? Tokens.move : 160 } }
                                Behavior on color { ColorAnimation { duration: page.tk ? Tokens.move : 160 } }

                                MarkTag { on: markCard.selected }

                                // The real launcher pill, faithful to LauncherWidget.qml (data).
                                Rectangle {
                                    id: pill
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.verticalCenterOffset: page.tk ? -page.tk.gap : -12
                                width: mark.implicitWidth + 12
                                    height: page.root ? page.root.pillH : 20
                                    radius: page.root ? page.root.pillRadius : 8
                                    color: page.root ? page.root.pill : "#222222"
                                    border.color: page.root ? page.root.pillBorder : "#333333"
                                    border.width: page.root ? page.root.pillBorderW : 1

                                    Text {
                                        id: mark
                                        anchors.centerIn: parent
                                        text: markCard.modelData.glyph
                                        color: page.root ? page.root.seal : "#c0392b"
                                        renderType: Text.NativeRendering
                                        font.family: markCard.iconMode
                                            ? "Noto Sans CJK JP"
                                            : (page.root ? page.root.mono : "monospace")
                                        // Matches LauncherWidget's real mark size per mode.
                                        font.pixelSize: markCard.iconMode ? 15 : 12
                                        font.weight: Font.Bold
                                        font.letterSpacing: markCard.iconMode ? 0 : 2
                                    }
                                }

                                // The format's name - the fact this card chooses, not
                                // a second copy of the mark.
                                UiText {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    anchors.bottom: parent.bottom
                                    anchors.bottomMargin: page.tk ? page.tk.gap : 12
                                    text: I18n.tr(markCard.modelData.label)
                                    color: page.tk ? (markCard.selected ? Tokens.ink : Tokens.inkMuted) : "#cccccc"
                                    font.family: page.tk ? Tokens.mono : "monospace"
                                    font.pixelSize: page.tk ? Tokens.fSmall : 13
                                    font.letterSpacing: page.tk ? Tokens.trackLabel : 1
                                    font.weight: markCard.selected ? Font.DemiBold : Font.Normal
                                }

                                MouseArea {
                                    id: markCardMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: {
                                        if (page.root) page.root.launcherLogoMode = markCard.modelData.mode
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ── the active format's options ───────────────────────────────────
            Entrance {
                width: page.colW
                index: 1
                SettingCard {
                    width: page.colW
                    title: page.mode === "icon" ? I18n.tr("GLYPH") : I18n.tr("WORDMARK")
                    kana: page.mode === "icon" ? "\u7d0b" : "\u6587\u5b57"

                    Grid {
                        id: optGrid
                    width: parent.width
                        columns: 4
                        columnSpacing: page.tk ? page.tk.colGap : 16
                        rowSpacing: page.tk ? page.tk.colGap : 16
                        readonly property real cellW: (width - columnSpacing * (columns - 1)) / columns

                        Repeater {
                            model: page.activeOptions

                            delegate: Rectangle {
                                id: optCard
                                required property string modelData
                                readonly property bool iconMode: page.mode === "icon"
                                readonly property bool selected: (iconMode
                                    ? (page.root ? page.root.launcherLogoIcon : "")
                                    : (page.root ? page.root.launcherLogoText : "")) === modelData

                            width: optGrid.cellW
                                height: page.tk ? page.tk.rowH * 2 : 80
                                radius: page.tk ? Tokens.radius : 6
                                color: optMa.containsMouse ? (page.tk ? Tokens.tint5 : "#111111") : "transparent"
                                border.width: optCard.selected ? 2 : 1
                                border.color: optCard.selected ? (page.tk ? Tokens.bone : "#cdc4ba")
                                    : optMa.containsMouse ? (page.tk ? Tokens.ink : "#cccccc")
                                    : (page.tk ? Tokens.line : "#333333")
                                Behavior on border.color { ColorAnimation { duration: page.tk ? Tokens.move : 160 } }
                                Behavior on color { ColorAnimation { duration: page.tk ? Tokens.move : 160 } }

                                MarkTag { on: optCard.selected }

                                // The mark, previewed once, exactly as the bar draws it (data).
                                Text {
                                    id: optMark
                                    anchors.centerIn: parent
                                width: parent.width - (page.tk ? page.tk.gap * 2 : 14)
                                    horizontalAlignment: Text.AlignHCenter
                                    text: optCard.iconMode
                                        ? (page.root ? page.root.launcherLogoIconGlyph(optCard.modelData) : "")
                                        : (page.root ? page.root.launcherLogoTextLabel(optCard.modelData) : optCard.modelData)
                                    color: page.root ? page.root.seal : "#c0392b"
                                    renderType: Text.NativeRendering
                                    font.family: optCard.iconMode
                                        ? (page.root ? page.root.launcherLogoIconFont(optCard.modelData) : "monospace")
                                        : (page.root ? page.root.mono : "monospace")
                                    font.pixelSize: optCard.iconMode
                                        ? (page.root ? page.root.launcherLogoIconSize(optCard.modelData) : 15)
                                        : 12
                                    font.weight: Font.Bold
                                    font.letterSpacing: optCard.iconMode ? 0 : 2
                                    fontSizeMode: Text.HorizontalFit
                                    minimumPixelSize: 7
                                }

                                MouseArea {
                                    id: optMa
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: {
                                        if (!page.root) return
                                        if (optCard.iconMode) page.root.launcherLogoIcon = optCard.modelData
                                        else page.root.launcherLogoText = optCard.modelData
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Entrance {
                width: page.colW
                index: 2
                SettingCard {
                    width: page.colW
                    title: I18n.tr("COUNT")
                    kana: "\u6570"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        label: I18n.tr("Workspaces")
                        desc: I18n.tr("Only the active workspace, or a fixed 1-5 / 1-10 row.")
                        source: "shell.json"
                        controlWidth: Math.max(120, 62 * 3)
                        Seg {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["Active", "1-5", "1-10"]
                            current: page.root ? page.countCap(page.root.workspaceMode) : ""
                            onChose: (key) => { if (page.root) page.root.workspaceMode = page.countKey(key) }
                        }
                    }
                }
            }

            // ── workspaces: MARKER ──
            Entrance {
                width: page.colW
                index: 3
                SettingCard {
                    width: page.colW
                    title: I18n.tr("MARKER")
                    kana: "\u70b9"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        block: true
                        label: I18n.tr("Style")
                        desc: I18n.tr("How the workspace indicators are drawn.")
                        source: "shell.json"
                        Chips {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: page.root ? page.root.workspaceStyleOptions.map(o => o.label) : []
                            current: page.root ? page.styleCap(page.root.workspaceStyle) : ""
                            onChose: (key) => { if (page.root) page.root.workspaceStyle = page.styleKey(key) }
                        }
                    }
                }
            }


            // ── PREVIEW ──
            Entrance {
                width: page.colW
                index: 4
                SettingCard {
                    width: page.colW
                    title: I18n.tr("PREVIEW")
                    kana: "\u898b\u672c"

                    CcWorkspacePreview {
                        width: parent.width
                        root: page.root
                        tk: page.tk
                    }
                }
            }
        }
    }

    CcScrollRail { root: page.root; tk: page.tk; flick: flick; z: 5 }
}
