import QtQuick
import "../kit"
import "../../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

// Bar route (帯) on the QS Bar Settings kit. A live bar-surface panel: every
// control writes straight to `root` (the qsbar Theme), so the running bar updates
// live and mirrors to Bar Studio. The page opens with the live bar silhouette,
// then numbered sections carry the surface knobs. Arranging the widgets is the
// Layout route's job, so this page holds no layout controls. Root reads stay
// null-guarded - the page may briefly exist before `root` is assigned, and some
// tokens live on the V2 Theme first (guard the barShellStyleValid call in
// particular).
Item {
    id: page
    property var root: null
    property var cc: null
    readonly property var tk: page.cc ? page.cc.tokens : null

    // The content column is capped so a label and its control stay related. This
    // cap is the fix for the old panel's 600px voids: never stretch a row to the
    // panel's full width.
    readonly property real colW: Math.min(page.width, page.tk ? page.tk.contentW : 640)
    implicitHeight: col.implicitHeight

    // the five bar forms + their captions (drives the FORM grid)
    readonly property var formModel: [
        { form: "islands", label: I18n.tr("Islands"), detail: I18n.tr("Split pills") },
        { form: "full",    label: I18n.tr("Full"),    detail: I18n.tr("Edge to edge") },
        { form: "fit",     label: I18n.tr("Fit"),     detail: I18n.tr("Inset frame") },
        { form: "dock",    label: I18n.tr("Dock"),    detail: I18n.tr("Open edge") },
        { form: "notch",   label: I18n.tr("Notch"),   detail: I18n.tr("Flowing shoulders") }
    ]

    // One shape picker over the single bar: islands is the split-pill form, the
    // rest are the continuous shell forms. Selecting a form just records it.
    readonly property string activeForm:
        page.root && page.root.barShellStyle ? String(page.root.barShellStyle) : "full"
    function selectForm(f) {
        if (page.root && page.root.barShellStyleValid && page.root.barShellStyleValid(f))
            page.root.barShellStyle = f
    }

    // gap-animation mode: each option sets barAnim to its exact mode value (0-8).
    readonly property int curAnim: (page.root && page.root.barAnim !== undefined) ? page.root.barAnim : -1
    function setAnim(v) { if (page.root && page.root.barAnim !== undefined) page.root.barAnim = v }

    // gap-animation presets, label <-> mode value both ways (mirrors the Hub).
    readonly property var animModes: [
        { v: 0, label: I18n.tr("Off") },
        { v: 1, label: I18n.tr("Stream") },
        { v: 2, label: I18n.tr("Surge") },
        { v: 3, label: I18n.tr("Bolt") },
        { v: 4, label: I18n.tr("Bolt-2") },
        { v: 7, label: I18n.tr("Reactor") },
        { v: 8, label: I18n.tr("Quotes") }
    ]
    function animLabel(v) {
        for (var i = 0; i < page.animModes.length; i++)
            if (page.animModes[i].v === v) return page.animModes[i].label;
        return "";
    }
    function animValue(label) {
        for (var i = 0; i < page.animModes.length; i++)
            if (page.animModes[i].label === label) return page.animModes[i].v;
        return 0;
    }

    // The last palette slot the bar actually wore, so switching Follow wallpaper
    // off puts back the colour that was there instead of a default.
    property string pinnedSlot: "color01"
    Binding {
        target: page
        property: "pinnedSlot"
        value: page.root ? page.root.barColor : "color01"
        when: page.root && !page.root.barColorIsAccent
        restoreMode: Binding.RestoreNone
    }

    // what is still below the plate's edge; the stage reads it for the fade.
    readonly property real scrollRemaining: Math.max(0, flick.contentHeight - flick.height - flick.contentY)

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        // a page that overflows gets a tail, so its last row can scroll clear
        // of the bottom fade instead of living inside it.
        contentHeight: col.implicitHeight + (col.implicitHeight > height && page.tk ? page.tk.tailPad : 0)
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: col
            width: page.colW
            spacing: page.tk ? page.tk.sectionGap : 16

            // ── live bar preview: a single full-width plate over the sections ──
            Rectangle {
                width: page.colW
                height: sil.implicitHeight + (page.tk ? page.tk.gap * 2 : 24)
                radius: page.tk ? Tokens.radius : 6
                color: page.tk ? Tokens.paperLift : "#161616"
                border.width: 1
                border.color: page.tk ? Tokens.line : "#333333"

                BarSilhouette {
                    id: sil
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: page.tk ? page.tk.pad : 24
                    anchors.rightMargin: page.tk ? page.tk.pad : 24
                    root: page.root
                    form: page.activeForm
                }
            }

            // ── 01 POSITION ──
            Entrance {
                width: page.colW
                index: 0
                SettingCard {
                    width: page.colW
                    title: I18n.tr("01 POSITION")
                    kana: "\u4f4d\u7f6e"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        controlWidth: 130
                        label: I18n.tr("Edge")
                        source: "shell.json"
                        Seg {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["top", "bottom"]
                            current: page.root ? page.root.barPosition : "top"
                            onChose: key => { if (page.root) page.root.barPosition = key }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 54
                        label: I18n.tr("Auto-hide")
                        desc: I18n.tr("Reveal on edge hover")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.barAutoHide === true : false
                            onToggled: value => { if (page.root && page.root.barAutoHide !== undefined) page.root.barAutoHide = value }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 58
                        label: I18n.tr("Size")
                        unit: "%"
                        value: String(Math.round(100 * (page.root ? page.root.barScale : 1)))
                        desc: I18n.tr("Bar scale, apart from the display")
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 100
                            to: 200
                            stepBy: 10
                            value: 100 * (page.root ? page.root.barScale : 1)
                            onModified: v => {
                                if (page.root && page.root.barScale !== undefined)
                                    page.root.barScale = v / 100
                            }
                        }
                    }
                }
            }

            // ── 02 MOTION: the stream flowing in the gaps between widgets. Right
            // under position, so it is the second thing on the page, not the last ──
            Entrance {
                width: page.colW
                index: 1
                SettingCard {
                    width: page.colW
                    title: I18n.tr("02 MOTION")
                    kana: "\u52d5\u304d"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        block: true
                        label: I18n.tr("Gap animation")
                        desc: I18n.tr("Motion in the gaps between widgets")
                        source: "shell.json"
                        Seg {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            options: page.animModes.map(m => m.label)
                            current: page.animLabel(page.curAnim)
                            onChose: key => page.setAnim(page.animValue(key))
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        visible: page.curAnim >= 1 && page.curAnim <= 6
                        divider: true
                        controlWidth: 54
                        label: I18n.tr("Drift when silent")
                        desc: I18n.tr("Keep it moving with no audio, on any power profile")
                        source: "performance.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: Perf.ambientBarMotionPref
                            onToggled: value => Perf.setAmbientBarMotion(value)
                        }
                    }
                }
            }

            // ── 03 FORM: one row of named tiles, the way the Hub picks a bar
            // style. The live silhouette sits at the top of this page already, so a
            // tile only has to name the shape.
            Entrance {
                width: page.colW
                index: 2
                SettingCard {
                    width: page.colW
                    title: I18n.tr("03 FORM")
                    kana: "\u5f62"

                    Item {
                        width: parent.width
                        height: formRow.height + (page.tk ? page.tk.gap * 2 : 24)

                        Row {
                            id: formRow
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: page.tk ? page.tk.gap : 12
                            anchors.rightMargin: page.tk ? page.tk.gap : 12
                            spacing: Tokens.s2
                            readonly property real cellW: (width - (page.formModel.length - 1) * Tokens.s2)
                                / Math.max(1, page.formModel.length)

                            Repeater {
                                model: page.formModel

                                delegate: Rectangle {
                                    id: ftile
                                    required property var modelData
                                    readonly property bool on: page.activeForm === modelData.form

                                    width: formRow.cellW
                                    height: Tokens.px(58)
                                    radius: Tokens.radius
                                    color: ftile.on ? Tokens.bone : (fma.containsMouse ? Tokens.tint5 : "transparent")
                                    border.width: Tokens.border
                                    border.color: ftile.on ? Tokens.bone : Tokens.line
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                    Column {
                                        anchors.left: parent.left
                                        anchors.right: parent.right
                                        anchors.margins: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: 2

                                        UiText {
                                            width: parent.width
                                            text: I18n.tr(ftile.modelData.label).toUpperCase()
                                            color: ftile.on ? Tokens.inkOnBone : Tokens.inkDim
                                            elide: Text.ElideRight
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            font.weight: Font.Medium
                                            font.letterSpacing: Tokens.trackLabel
                                        }
                                        UiText {
                                            width: parent.width
                                            text: I18n.tr(ftile.modelData.detail)
                                            color: ftile.on ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                            elide: Text.ElideRight
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny
                                        }
                                    }

                                    MouseArea {
                                        id: fma
                                        anchors.fill: parent
                                        hoverEnabled: true
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: page.selectForm(ftile.modelData.form)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ── 04 SURFACE ──
            Entrance {
                width: page.colW
                index: 3
                SettingCard {
                    width: page.colW
                    title: I18n.tr("04 SURFACE")
                    kana: "\u8868\u9762"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        controlWidth: 54
                        label: I18n.tr("Bar border")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.barBorderEnabled === true : false
                            onToggled: value => { if (page.root && page.root.barBorderEnabled !== undefined) page.root.barBorderEnabled = value }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 58
                        label: I18n.tr("Corners")
                        unit: "px"
                        value: String(page.root ? page.root.barCornerRadius : 0)
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 40
                            value: page.root ? page.root.barCornerRadius : 0
                            onModified: v => { if (page.root && page.root.barCornerRadius !== undefined) page.root.barCornerRadius = v }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 54
                        label: I18n.tr("Panel + tooltip")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.panelTooltipBorderEnabled === true : false
                            onToggled: value => { if (page.root && page.root.panelTooltipBorderEnabled !== undefined) page.root.panelTooltipBorderEnabled = value }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 54
                        label: I18n.tr("Depth")
                        desc: I18n.tr("Lift the bar and its panels")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.barShadowEnabled === true : false
                            onToggled: value => { if (page.root && page.root.barShadowEnabled !== undefined) page.root.barShadowEnabled = value }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 54
                        label: I18n.tr("Frost")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.barFrostEnabled === true : false
                            onToggled: value => { if (page.root && page.root.barFrostEnabled !== undefined) page.root.barFrostEnabled = value }
                        }
                    }
                }
            }

            // ── 05 GAPS: how far the shell stays off each output edge ──
            Entrance {
                width: page.colW
                index: 4
                SettingCard {
                    width: page.colW
                    title: I18n.tr("05 GAPS")
                    kana: "\u9593\u9694"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        controlWidth: 58
                        label: I18n.tr("Top")
                        unit: "px"
                        value: String(page.root && page.root.barGapTop !== undefined ? page.root.barGapTop : 0)
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 64
                            stepBy: 1
                            value: page.root && page.root.barGapTop !== undefined ? page.root.barGapTop : 0
                            onModified: v => { if (page.root && page.root.barGapTop !== undefined) page.root.barGapTop = v }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 58
                        label: I18n.tr("Bottom")
                        unit: "px"
                        value: String(page.root && page.root.barGapBottom !== undefined ? page.root.barGapBottom : 0)
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 64
                            stepBy: 1
                            value: page.root && page.root.barGapBottom !== undefined ? page.root.barGapBottom : 0
                            onModified: v => { if (page.root && page.root.barGapBottom !== undefined) page.root.barGapBottom = v }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 58
                        label: I18n.tr("Left")
                        unit: "px"
                        value: String(page.root && page.root.barGapLeft !== undefined ? page.root.barGapLeft : 0)
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 64
                            stepBy: 1
                            value: page.root && page.root.barGapLeft !== undefined ? page.root.barGapLeft : 0
                            onModified: v => { if (page.root && page.root.barGapLeft !== undefined) page.root.barGapLeft = v }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        controlWidth: 58
                        label: I18n.tr("Right")
                        unit: "px"
                        value: String(page.root && page.root.barGapRight !== undefined ? page.root.barGapRight : 0)
                        source: "shell.json"
                        Step {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 64
                            stepBy: 1
                            value: page.root && page.root.barGapRight !== undefined ? page.root.barGapRight : 0
                            onModified: v => { if (page.root && page.root.barGapRight !== undefined) page.root.barGapRight = v }
                        }
                    }
                }
            }

            // ── 06 ACCENT: the bar's data colour, so the swatches keep their hue ──
            Entrance {
                width: page.colW
                index: 5
                SettingCard {
                    width: page.colW
                    title: I18n.tr("06 ACCENT")
                    kana: "\u8272"

                    // Following the wallpaper is the honest default: matugen already
                    // derives an accent from the image, and "accent" is a real stored
                    // value, not a fixed slot pretending to track the picture.
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        controlWidth: 54
                        label: I18n.tr("Follow wallpaper")
                        desc: I18n.tr("Use the wallpaper's accent")
                        source: "shell.json"
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: page.root ? page.root.barColorIsAccent : false
                            // turning it off restores the slot that was pinned before,
                            // not a hardcoded colour01: the switch must not eat a choice
                            onToggled: v => { if (page.root) page.root.barColor = v ? "accent" : page.pinnedSlot }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        divider: true
                        block: true
                        label: I18n.tr("Slot")
                        desc: I18n.tr("Or pin one palette colour")
                        source: "shell.json"
                        enabled: page.root ? !page.root.barColorIsAccent : true
                        CcSwatchGrid {
                            id: accentGrid
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            root: page.root
                            tk: page.tk
                            options: page.root ? page.root.barColorOptions : []
                            current: page.root ? page.root.barColor : ""
                            onChose: id => { if (page.root) page.root.barColor = id }
                        }
                    }
                }
            }

            // ── 07 PICKERS: the visual form shared by the theme, wallpaper and
            // media browsers that open from this bar.
            Entrance {
                width: page.colW
                index: 6
                SettingCard {
                    width: page.colW
                    title: I18n.tr("07 PICKERS")
                    kana: "\u9078"

                    SettingRow {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        block: true
                        label: I18n.tr("Picker style")
                        desc: I18n.tr("How the theme, wallpaper and media pickers are laid out.")
                        source: "shell.json"
                        Seg {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["tanzaku", "hearthstone", "carousel"]
                            labels: ({
                                "tanzaku": "Tanzaku",
                                "hearthstone": "Hearthstone",
                                "carousel": "Carousel"
                            })
                            current: page.root ? page.root.pickerStyle : "tanzaku"
                            onChose: key => { if (page.root) page.root.pickerStyle = key }
                        }
                    }
                }
            }
        }
    }

    CcScrollRail { root: page.root; tk: page.tk; flick: flick; z: 5 }
}
