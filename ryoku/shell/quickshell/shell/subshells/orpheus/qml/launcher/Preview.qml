// visual only: the launcher takes no settings that change this picture
pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Lithic
import Ryoku.Blobs
import Ryoku.Ui.Singletons

// Settings' still of the Orpheus launcher in the design in use: the bar's voice
// island at the top centre with the launcher poured out of it as one liquid
// body, the search line, four tracks with the bead on the first, and the lead.
// It runs inside Settings, so it draws from the Lithic library, not the shell.
Item {
    id: root

    property var settings: ({})
    signal editRequested(string key, var value)

    implicitWidth: 720
    implicitHeight: 250
    clip: true

    readonly property string design: Lithic.design
    readonly property bool lithic: root.design === "lithic"
    readonly property bool material: root.design === "material"
    readonly property bool adwaita: root.design === "adwaita"
    readonly property bool fluent: root.design === "fluent"
    readonly property bool hairlines: root.lithic || root.fluent

    readonly property color paper: Lithic.stone2
    readonly property color ink: Lithic.fg
    readonly property color inkDim: Lithic.fg2
    readonly property color inkFaint: Lithic.fg3
    readonly property color line: Lithic.vein
    readonly property color tint5: Lithic.alpha(Lithic.fg, 0.05)
    readonly property color tint10: Lithic.alpha(Lithic.fg, 0.10)
    readonly property color tint16: Lithic.alpha(Lithic.fg, 0.16)
    // the print language's bone plate, the same pair the launcher's selection wears
    readonly property color bone: Tokens.bone
    readonly property color inkOnBone: Tokens.inkOnBone

    readonly property real barH: 30
    readonly property real smoothing: root.material ? 16 : root.adwaita ? 13 : root.fluent ? 9 : 12
    readonly property real dropCorner: Lithic.cornerRadius(16, 400, root.barH, root.design)
    readonly property real cardCorner: Lithic.cornerRadius(16, card.width, card.height, root.design)

    // The card's face, in preview px.
    readonly property real cardX: 110
    readonly property real cardW: 500
    readonly property real cardBottom: 242
    readonly property real pad: 14
    readonly property real innerX: root.cardX + root.pad
    readonly property real innerW: root.cardW - root.pad * 2
    readonly property real listW: Math.round(root.innerW * 0.57)
    readonly property real leadX: root.innerX + root.listW + 12
    readonly property real leadW: root.innerX + root.innerW - root.leadX
    readonly property real rowsY: 96
    readonly property real rowH: 34

    readonly property var tracks: [
        { "glyph": "public", "title": "Firefox", "kind": I18n.tr("Browser"), "meta": "2" },
        { "glyph": "folder", "title": "Files", "kind": I18n.tr("System"), "meta": "" },
        { "glyph": "security", "title": "Firewall", "kind": I18n.tr("System"), "meta": "" },
        { "glyph": "terminal", "title": "Fish", "kind": I18n.tr("Terminal"), "meta": "" }
    ]
    readonly property var tabs: [I18n.tr("All"), I18n.tr("Apps"), I18n.tr("Windows"), I18n.tr("Files"), I18n.tr("Actions")]
    readonly property var voiceLevels: [0.3, 0.55, 0.8, 0.5, 0.95, 0.65, 1, 0.7, 0.45, 0.75, 0.35]

    function numeral(index) {
        return root.lithic ? "0" + (index + 1) : String(index + 1);
    }

    Rectangle {
        anchors.fill: parent
        color: Lithic.ground
    }

    // ── The liquid body: the bar's drops and the launcher, one blob ─────────
    BlobGroup {
        id: liquid
        color: root.paper
        smoothing: root.smoothing
        cutCorners: root.lithic
        borderWidth: root.hairlines ? 1 : 0
        borderColor: root.line
        shadowStrength: 0
        shadowSize: 0
    }

    // the screen's top edge, so every drop melts into it with a fillet
    BlobRect {
        group: liquid
        x: -40
        y: -40
        width: root.width + 80
        height: 40
        radius: 0
        sinks: false
        deformScale: 0
    }

    BlobRect {
        id: leftDrop
        group: liquid
        x: 18
        y: 0
        width: 168
        height: root.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: root.dropCorner
        bottomRightRadius: root.dropCorner
        sinks: false
        deformScale: 0
    }

    BlobRect {
        id: rightDrop
        group: liquid
        x: root.width - 18 - 168
        y: 0
        width: 168
        height: root.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: root.dropCorner
        bottomRightRadius: root.dropCorner
        sinks: false
        deformScale: 0
    }

    BlobRect {
        id: card
        group: liquid
        x: root.cardX
        y: 0
        width: root.cardW
        height: root.cardBottom
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: root.cardCorner
        bottomRightRadius: root.cardCorner
        sinks: false
        deformScale: 0
    }

    // ── The bar sliver ──────────────────────────────────────────────────────
    Row {
        x: leftDrop.x + 14
        height: root.barH
        spacing: 10

        Repeater {
            model: 4

            delegate: Text {
                required property int index
                anchors.verticalCenter: parent.verticalCenter
                text: String(index + 1)
                color: index === 0 ? root.ink : root.inkFaint
                font.family: Lithic.font.mono
                font.pixelSize: 11
                font.weight: index === 0 ? Font.Bold : Font.Normal
            }
        }
    }

    Row {
        anchors.right: rightDrop.right
        anchors.rightMargin: 14
        height: root.barH
        spacing: 8

        Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: "wifi"
            size: 14
            ink: root.inkDim
        }
        Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: "battery_5_bar"
            size: 14
            ink: root.inkDim
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "12:07"
            color: root.ink
            font.family: Lithic.font.mono
            font.pixelSize: 12
            font.features: ({ "tnum": 1 })
        }
    }

    // the voice island at the top centre, the launcher pouring out under it
    Row {
        anchors.horizontalCenter: card.horizontalCenter
        height: root.barH
        spacing: 3

        Repeater {
            model: root.voiceLevels

            delegate: Rectangle {
                required property real modelData
                anchors.verticalCenter: parent.verticalCenter
                width: 3
                height: Math.max(2, modelData * 14)
                color: root.inkDim
            }
        }
    }

    // ── The search line ─────────────────────────────────────────────────────
    Item {
        id: search
        x: root.innerX
        y: root.barH + 8
        width: root.innerW
        height: 32

        CutRect {
            anchors.fill: parent
            visible: !root.lithic
            cut: root.material ? 16 : root.adwaita ? 8 : 4
            fill: root.fluent ? root.tint5 : root.tint10
            strokeWidth: root.fluent ? 1 : 0
            strokeColor: Lithic.alpha(root.line, 0.7)
        }

        Rectangle {
            visible: root.lithic || root.fluent
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: root.fluent ? 2 : 1
            color: root.fluent ? root.ink : root.line
        }

        Text {
            id: seal
            visible: root.lithic
            x: 4
            anchors.verticalCenter: parent.verticalCenter
            text: "力"
            color: root.ink
            font.family: Tokens.jp
            font.pixelSize: 17
        }

        Glyph {
            id: lens
            visible: !root.lithic
            x: 12
            anchors.verticalCenter: parent.verticalCenter
            name: "search"
            size: 17
            ink: root.inkDim
        }

        Text {
            id: queryText
            x: root.lithic ? seal.x + seal.width + 10 : lens.x + lens.width + 10
            anchors.verticalCenter: parent.verticalCenter
            text: "fi"
            color: root.ink
            font.family: Lithic.font.sans
            font.pixelSize: 17
        }

        Text {
            anchors.left: queryText.right
            anchors.baseline: queryText.baseline
            text: "refox"
            color: root.inkFaint
            font.family: Lithic.font.sans
            font.pixelSize: 17
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: root.lithic ? 0 : 6
            anchors.verticalCenter: parent.verticalCenter
            spacing: root.fluent ? 10 : 2

            Repeater {
                model: root.tabs

                delegate: Item {
                    id: tab
                    required property string modelData
                    required property int index
                    readonly property bool selected: tab.index === 0

                    width: tabLabel.implicitWidth + (root.fluent ? 0 : 14)
                    height: 22

                    CutRect {
                        anchors.fill: parent
                        visible: tab.selected && !root.fluent
                        cut: root.material ? 12 : root.adwaita ? 6 : 3
                        fill: root.lithic ? root.bone : root.material ? root.tint16 : root.tint10
                    }

                    CutRect {
                        visible: tab.selected && root.fluent
                        anchors.horizontalCenter: parent.horizontalCenter
                        anchors.bottom: parent.bottom
                        width: 14
                        height: 3
                        cut: 2
                        fill: root.ink
                    }

                    Text {
                        id: tabLabel
                        anchors.centerIn: parent
                        text: root.lithic ? tab.modelData.toUpperCase() : tab.modelData
                        color: tab.selected ? (root.lithic ? root.inkOnBone : root.ink) : root.inkDim
                        font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
                        font.pixelSize: root.lithic ? 9 : 11
                        font.letterSpacing: root.lithic ? 0.8 : 0
                        font.weight: tab.selected ? Font.Medium : Font.Normal
                    }
                }
            }
        }
    }

    // ── The tracks ──────────────────────────────────────────────────────────
    Text {
        x: root.innerX + 8
        y: root.rowsY - 15
        text: root.lithic ? I18n.tr("Pinned").toUpperCase() : I18n.tr("Pinned")
        color: root.inkFaint
        font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
        font.pixelSize: 9
        font.letterSpacing: root.lithic ? 1 : 0
    }

    CutRect {
        x: root.innerX
        y: root.rowsY
        width: root.listW
        height: root.rowH
        cut: root.material ? 16 : root.adwaita ? 7 : root.fluent ? 6 : 6
        fill: root.lithic ? root.bone : root.material ? root.tint16 : root.tint10
        strokeWidth: root.fluent ? 1 : 0
        strokeColor: Lithic.alpha(root.line, 0.7)
    }

    Repeater {
        model: root.tracks

        delegate: Item {
            id: track
            required property var modelData
            required property int index
            readonly property bool onBone: track.index === 0 && root.lithic
            readonly property color trackInk: track.onBone ? root.inkOnBone : root.ink

            x: root.innerX
            y: root.rowsY + track.index * root.rowH
            width: root.listW
            height: root.rowH

            Text {
                x: 8
                anchors.verticalCenter: parent.verticalCenter
                text: root.numeral(track.index)
                color: track.onBone ? root.inkOnBone : root.inkFaint
                font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
                font.pixelSize: 9
                font.features: ({ "tnum": 1 })
            }

            Glyph {
                x: 28
                anchors.verticalCenter: parent.verticalCenter
                name: track.modelData.glyph
                size: 18
                ink: track.trackInk
            }

            Column {
                x: 54
                anchors.verticalCenter: parent.verticalCenter
                spacing: 1

                Text {
                    text: track.modelData.title
                    color: track.trackInk
                    font.family: Lithic.font.sans
                    font.pixelSize: 12
                    font.weight: Font.Medium
                }
                Text {
                    text: track.modelData.kind
                    color: track.onBone ? Lithic.alpha(root.inkOnBone, 0.72) : root.inkFaint
                    font.family: Lithic.font.sans
                    font.pixelSize: 9
                }
            }

            Row {
                anchors.right: parent.right
                anchors.rightMargin: 8
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6

                CutRect {
                    visible: track.modelData.meta.length > 0
                    anchors.verticalCenter: parent.verticalCenter
                    width: 5
                    height: 5
                    cut: root.lithic ? 1 : 3
                    fill: track.trackInk
                }
                Text {
                    visible: track.modelData.meta.length > 0
                    anchors.verticalCenter: parent.verticalCenter
                    text: track.modelData.meta
                    color: track.onBone ? root.inkOnBone : root.inkDim
                    font.family: Lithic.font.mono
                    font.pixelSize: 9
                }

                CutRect {
                    visible: track.index === 0
                    anchors.verticalCenter: parent.verticalCenter
                    width: enterCap.implicitWidth + 10
                    height: 15
                    cut: root.material ? 8 : root.adwaita ? 4 : 2
                    fill: root.lithic ? "transparent" : root.tint10
                    strokeWidth: root.lithic || root.fluent ? 1 : 0
                    strokeColor: root.lithic ? Lithic.alpha(root.inkOnBone, 0.5) : Lithic.alpha(root.line, 0.7)

                    Text {
                        id: enterCap
                        anchors.centerIn: parent
                        text: root.lithic ? "ENTER" : "Enter"
                        color: track.onBone ? root.inkOnBone : root.inkDim
                        font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
                        font.pixelSize: 8
                    }
                }
            }
        }
    }

    // ── The lead ────────────────────────────────────────────────────────────
    Rectangle {
        visible: root.lithic
        x: root.leadX
        y: root.rowsY - 16
        width: 1
        height: root.rowH * 4 + 16
        color: root.line
    }

    CutRect {
        visible: !root.lithic
        x: root.leadX
        y: root.rowsY - 16
        width: root.leadW
        height: root.rowH * 4 + 16
        cut: root.material ? 16 : root.adwaita ? 8 : 6
        fill: root.tint5
        strokeWidth: root.fluent ? 1 : 0
        strokeColor: Lithic.alpha(root.line, 0.7)
    }

    Item {
        id: lead
        x: root.leadX + 12
        y: root.rowsY - 8
        width: root.leadW - 20
        height: root.rowH * 4

        Text {
            visible: root.lithic
            x: 62
            y: -6
            text: "01"
            color: root.inkFaint
            opacity: 0.6
            font.family: Lithic.font.mono
            font.pixelSize: 46
            font.weight: Font.Light
        }

        CutRect {
            id: plate
            visible: !root.adwaita
            width: 52
            height: 52
            cut: root.material ? 26 : root.lithic ? 8 : 4
            fill: root.lithic ? root.paper : root.material ? root.tint10 : root.tint5
            strokeWidth: root.hairlines ? 1 : 0
            strokeColor: root.lithic ? root.line : Lithic.alpha(root.line, 0.7)
        }

        MultiEffect {
            visible: !root.lithic
            anchors.fill: icon
            source: icon
            scale: 1.5
            opacity: 0.28
            blurEnabled: true
            blur: 1
            blurMax: 24
            autoPaddingEnabled: true
        }

        Glyph {
            id: icon
            anchors.centerIn: plate
            name: "public"
            size: root.adwaita ? 40 : 32
            ink: root.ink
        }

        Text {
            id: leadTitle
            y: plate.height + 6
            text: "Firefox"
            color: root.ink
            font.family: Lithic.font.sans
            font.pixelSize: 15
            font.weight: Font.DemiBold
        }

        Text {
            id: leadSub
            anchors.top: leadTitle.bottom
            text: I18n.tr("Web browser")
            color: root.inkDim
            font.family: Lithic.font.sans
            font.pixelSize: 10
        }

        Column {
            anchors.top: leadSub.bottom
            anchors.topMargin: 6
            width: parent.width

            Repeater {
                model: [I18n.tr("Open"), I18n.tr("New private window")]

                delegate: Item {
                    id: action
                    required property string modelData
                    required property int index

                    width: parent.width
                    height: 22

                    Text {
                        x: 2
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.numeral(action.index)
                        color: root.inkFaint
                        font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
                        font.pixelSize: 9
                    }

                    Text {
                        x: 22
                        anchors.verticalCenter: parent.verticalCenter
                        text: action.modelData
                        color: action.index === 0 ? root.ink : root.inkDim
                        font.family: Lithic.font.sans
                        font.pixelSize: 11
                        font.weight: action.index === 0 ? Font.Medium : Font.Normal
                    }

                    CutRect {
                        visible: action.index === 0
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: actionCap.implicitWidth + 10
                        height: 15
                        cut: root.material ? 8 : root.adwaita ? 4 : 2
                        fill: root.lithic ? "transparent" : root.tint10
                        strokeWidth: root.hairlines ? 1 : 0
                        strokeColor: Lithic.alpha(root.line, 0.7)

                        Text {
                            id: actionCap
                            anchors.centerIn: parent
                            text: root.lithic ? "ENTER" : "Enter"
                            color: root.inkDim
                            font.family: root.lithic ? Lithic.font.mono : Lithic.font.sans
                            font.pixelSize: 8
                        }
                    }
                }
            }
        }
    }
}
