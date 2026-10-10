pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Credits, the kansha poster. Gratitude as a plate: an ancient tree, its roots
// gripping the black, dissolves off the right; the left column carries the
// bilingual title, a Fraunces deck, the projects Ryoku grows from as editorial
// type lines (never cards), the one bone placard in the whole app for the
// crash-test crew, and a self-documenting colophon. Pure space-bone-grotesk --
// bone on black, no colour, no marble; the tree is fal-generated and graded to
// the bone duotone by hand. Every value is a Token.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // people + projects Ryoku is built on, reused verbatim from the old page.
    // an empty url is a quiet, unlinked credit: no home was given, none invented.
    readonly property var projects: [
        { "name": "qylock",       "by": "Darkkal44",      "role": I18n.tr("lockscreen"),          "url": "https://github.com/Darkkal44/qylock" },
        { "name": "caelestia",    "by": "caelestia-dots", "role": I18n.tr("Quickshell craft, motion + browser theming"), "url": "https://github.com/caelestia-dots/shell" },
        { "name": "dusky",        "by": "dusklinux",      "role": I18n.tr("recolor templates + animation presets"), "url": "https://github.com/dusklinux/dusky" },
        { "name": "OkShell",      "by": "John Oberhauser", "role": I18n.tr("frame and bar design (GPL-3.0)"), "url": "https://github.com/JohnOberhauser/OkShell/tree/87a9d09d923163cd03f396a395be3bb02b335b6e" },
        { "name": "rishot",       "by": "Gakuseei",       "role": I18n.tr("screenshot flow"),      "url": "https://github.com/Gakuseei/rishot" },
        { "name": "cava-bg",      "by": "leriart",        "role": I18n.tr("audio-reactive walls"), "url": "https://github.com/leriart/cava-bg" },
        { "name": "Brain_Shell",  "by": "Brainitech",     "role": I18n.tr("shell craft"),          "url": "https://github.com/Brainitech/Brain_Shell" },
        { "name": "ActivSpot",    "by": "Devvvmn",        "role": I18n.tr("window spotlight"),     "url": "https://github.com/Devvvmn/ActivSpot" },
        { "name": "hyprmod",      "by": "BlueManCZ",      "role": I18n.tr("Hyprland tooling"),     "url": "https://github.com/BlueManCZ/hyprmod" },
        { "name": "dotfiles",     "by": "matteogini",     "role": I18n.tr("dotfile craft"),        "url": "https://github.com/matteogini/dotfiles" },
        { "name": "inir",         "by": "snowarch",       "role": I18n.tr("launcher grid"),        "url": "" },
        { "name": "noctalia",     "by": "noctalia-dev",   "role": I18n.tr("shell polish"),         "url": "https://github.com/noctalia-dev/noctalia-shell" },
        { "name": "DankMaterial", "by": "AvengeMedia",    "role": I18n.tr("material shell"),       "url": "https://github.com/AvengeMedia/DankMaterialShell" },
        { "name": "Omarchy",      "by": "37signals / DHH", "role": I18n.tr("Nomarchy shell source (MIT)"), "url": "https://github.com/basecamp/omarchy" },
        { "name": "CachyOS",      "by": "CachyOS team",   "role": I18n.tr("performance Arch"),     "url": "" },
        { "name": "Ricelin",      "by": "Gakuseei",       "role": I18n.tr("washi warping pill"),   "url": "https://github.com/Gakuseei/Ricelin" },
        { "name": "nixos-configuration", "by": "ilyamiro", "role": I18n.tr("legacy island bar"),   "url": "https://github.com/ilyamiro/nixos-configuration" },
        { "name": "dotfiles",     "by": "Jules3182",     "role": I18n.tr("dyad dual-edge bar"),   "url": "https://github.com/Jules3182/dotfiles" },
        { "name": "NibrasShell",  "by": "Ahmed Saadi",    "role": I18n.tr("depth effect engine"),  "url": "https://github.com/AhmedSaadi0/NibrasShell" },
        { "name": "skwd-wall",    "by": "liixini",        "role": I18n.tr("ryogami wallpaper engine"), "url": "https://github.com/liixini/skwd-wall" },
        { "name": "ii-p3drovfx",  "by": "P3DROVFX",       "role": I18n.tr("Stage Editor (GPL-3.0)"), "url": "https://github.com/P3DROVFX/ii-p3drovfx" }
    ]

    // the alpha/beta crew, constantly stress-testing and filing bugs. each name
    // links to its GitHub home; names verbatim.
    readonly property var testers: [
        { "name": "bhimio1",      "url": "https://github.com/bhimio1" },
        { "name": "povargg",      "url": "https://github.com/povargg" },
        { "name": "godspeed1709", "url": "https://github.com/godspeed1709" },
        { "name": "VortexVirus",  "url": "https://github.com/VortexVirus" },
        { "name": "Lowingx",      "url": "https://github.com/Lowingx" },
        { "name": "ΛVΛLON",       "url": "https://github.com/Aval0n-01" },
        { "name": "JaviBOT96",    "url": "https://github.com/JaviBOT96" }
    ]

    // the ink ramp, for the colophon specimen. ratios are the measured AA
    // contrast against pure-black paper, from the section 1 palette table.
    readonly property var ramp: [
        { "label": I18n.tr("INK"),   "swatch": Tokens.ink,      "ratio": "12.0:1" },
        { "label": I18n.tr("DIM"),   "swatch": Tokens.inkDim,   "ratio": "9.0:1" },
        { "label": I18n.tr("MUTED"), "swatch": Tokens.inkMuted, "ratio": "6.6:1" },
        { "label": I18n.tr("FAINT"), "swatch": Tokens.inkFaint, "ratio": "4.6:1" }
    ]

    // a token colour rendered as its literal hex, so the colophon prints the
    // real value without a hardcoded string.
    function hex(c) {
        function o(v) { return Math.round(v * 255).toString(16).padStart(2, "0"); }
        return "#" + o(c.r) + o(c.g) + o(c.b);
    }

    // display sizes the token scale does not name, built only from tokens
    // (mirrors the Profile plate); commented with the section 9 target.
    readonly property int fKanji: Tokens.fHero + Tokens.fValue + Tokens.s1   // 64: the 感謝 hero
    readonly property int fByline: (Tokens.fMicro + Tokens.fTiny) / 2        // 10: the mono author
    readonly property int fRole: (Tokens.fSmall + Tokens.fMicro) / 2         // 12: role / body desc

    readonly property int margin: Tokens.s7
    readonly property real colW: Math.min(560, pg.width * 0.56 - pg.margin)

    // the plate.
    Rectangle {
        anchors.fill: parent
        color: Tokens.paper
    }

    // stage: the Three Graces, right-anchored, dissolving left through the alpha
    // baked into the asset. no colour lives on this page, so the marble is
    // desaturated to grey and dimmed so it recedes behind the type.
    Image {
        id: hero
        source: "../art/roots.png"
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: parent.height
        width: height * 0.75
        fillMode: Image.PreserveAspectCrop
        smooth: true
        asynchronous: true
        opacity: 0.92
    }

    // the poster spine: a rotated edge label reading bottom-up (the Berserk move).
    Item {
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s4
        anchors.verticalCenter: parent.verticalCenter
        width: Tokens.fTiny
        height: 1
        Text {
            anchors.centerIn: parent
            rotation: -90
            text: "RYOKU  \u00b7  \u611f\u8b1d KANSHA  \u00b7  SHOT ON BLACK"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: 2
        }
    }

    // left column: masthead, the standing-on list, the placard, the colophon.
    Flickable {
        id: flick
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.leftMargin: pg.margin
        anchors.topMargin: pg.margin
        anchors.bottomMargin: pg.margin
        width: pg.colW
        contentHeight: col.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        flickableDirection: Flickable.VerticalFlick
        ScrollBar.vertical: ScrollRail {}
        WheelScroll { }

        Column {
            id: col
            width: flick.width
            spacing: Tokens.s7

            // masthead: eyebrow over the bilingual title, ink on paper.
            Column {
                width: parent.width
                spacing: Tokens.s4

                Row {
                    spacing: Tokens.s2
                    Rectangle {
                        width: Tokens.s4
                        height: 1
                        color: Tokens.ink
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: "力"
                        color: Tokens.ink
                        font.family: Tokens.jp
                        font.pixelSize: Tokens.fMicro
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: "·"
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Text {
                        text: I18n.tr("KANSHA")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                        font.capitalization: Font.AllUppercase
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                Column {
                    width: parent.width
                    spacing: 0
                    Text {
                        text: "感謝"
                        color: Tokens.ink
                        font.family: Tokens.jp
                        font.pixelSize: pg.fKanji
                        font.weight: Font.Medium
                    }
                    Text {
                        text: I18n.tr("GRATITUDE")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fTitle
                    }
                }
                Text {
                    width: parent.width
                    text: I18n.tr("The roots we grow from - every project, distro, and hand this build stands on.")
                    color: Tokens.inkMuted
                    font.family: Tokens.display
                    font.italic: true
                    font.pixelSize: pg.fRole + 3
                    wrapMode: Text.WordWrap
                }
            }

            // the standing-on list: one editorial type line per project.
            Column {
                width: parent.width
                spacing: 0
                Repeater {
                    model: pg.projects
                    delegate: CreditRow {
                        required property var modelData
                        width: col.width
                        name: modelData.name
                        by: modelData.by
                        role: modelData.role
                        url: modelData.url
                    }
                }
            }

            // the crash-test placard: the one bone plate in the whole app.
            Rectangle {
                width: parent.width
                height: crew.implicitHeight + Tokens.s4 * 2
                radius: Tokens.radius
                color: Tokens.bone

                Column {
                    id: crew
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Tokens.s4
                    anchors.rightMargin: Tokens.s4
                    spacing: Tokens.s3

                    Text {
                        text: I18n.tr("THE CRASH TEST CREW")
                        color: Tokens.inkOnBoneDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                        font.capitalization: Font.AllUppercase
                    }
                    Flow {
                        width: parent.width
                        Repeater {
                            model: pg.testers
                            delegate: Row {
                                required property var modelData
                                required property int index
                                Text {
                                    text: modelData.name.toUpperCase()
                                    color: Tokens.inkOnBone
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                    font.underline: crewHov.hovered
                                    HoverHandler { id: crewHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler { onTapped: Qt.openUrlExternally(modelData.url) }
                                }
                                Text {
                                    visible: index < pg.testers.length - 1
                                    text: "  \u25c6  "
                                    color: Tokens.inkOnBoneDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                }
                            }
                        }
                    }
                }
            }

            // colophon: the self-documenting block. each face set in itself at
            // its role size, then the ink ramp, the disclosure.
            Column {
                width: parent.width
                spacing: Tokens.s4

                Text {
                    text: I18n.tr("Fraunces 44")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                }
                Text {
                    text: I18n.tr("Space Grotesk 14")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fBody
                }
                Text {
                    text: I18n.tr("SpaceMono 12")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: pg.fRole
                }

                Row {
                    spacing: Tokens.s4
                    Repeater {
                        model: pg.ramp
                        delegate: RampChip {
                            required property var modelData
                            label: I18n.tr(modelData.label)
                            swatch: modelData.swatch
                            ratio: modelData.ratio
                        }
                    }
                }

                Text {
                    width: parent.width
                    text: I18n.tr("Figurative art is AI-generated at dev time and graded by hand.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: pg.fRole
                    wrapMode: Text.WordWrap
                }
            }
        }
    }

    // one project = an editorial type line: name and mono byline on the left, a
    // right-aligned role, a lineSoft leader eating the gap between them. rows
    // with a url invert fully under the cursor (the transient bone of the
    // bone-stock rule) and open the home; rows without a url stay quiet.
    component CreditRow: Item {
        id: cr
        property string name: ""
        property string by: ""
        property string role: ""
        property string url: ""
        readonly property bool linked: cr.url.length > 0
        readonly property bool inv: cr.linked && hov.hovered

        height: Tokens.rowH

        HoverHandler { id: hov; enabled: cr.linked; cursorShape: Qt.PointingHandCursor }
        TapHandler { enabled: cr.linked; onTapped: Qt.openUrlExternally(cr.url) }

        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius
            color: cr.inv ? Tokens.bone : "transparent"
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }

        Text {
            id: nameT
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            text: cr.name
            color: cr.inv ? Tokens.inkOnBone : Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fRow
            font.weight: Font.Medium
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        Text {
            id: byT
            anchors.left: nameT.right
            anchors.leftMargin: Tokens.s2
            anchors.baseline: nameT.baseline
            text: cr.by
            color: cr.inv ? Tokens.inkOnBoneDim : Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: pg.fByline
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        Text {
            id: roleT
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            text: cr.role
            color: cr.inv ? Tokens.inkOnBoneDim : Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: pg.fRole
            // a role never walks over the name cluster: it yields and elides
            // when the row runs out of room, keeping every credit legible.
            width: Math.min(implicitWidth, cr.width - nameT.width - byT.width - Tokens.s2 - Tokens.s3 * 4)
            horizontalAlignment: Text.AlignRight
            elide: Text.ElideRight
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
        Rectangle {
            anchors.left: byT.right
            anchors.leftMargin: Tokens.s3
            anchors.right: roleT.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            height: 1
            color: cr.inv ? Tokens.lineOnBone : Tokens.lineSoft
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
        }
    }

    // one ink-ramp chip: the value as a swatch, its name, its literal hex, and
    // its measured contrast, the last two in mono at tag size.
    component RampChip: Column {
        id: rcp
        property string label: ""
        property color swatch: Tokens.ink
        property string ratio: ""
        spacing: Tokens.s1

        Rectangle {
            width: Tokens.s7
            height: Tokens.s5
            radius: Tokens.radius
            color: rcp.swatch
        }
        Text {
            text: I18n.tr(rcp.label)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: pg.fByline
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
            font.capitalization: Font.AllUppercase
        }
        Text {
            text: pg.hex(rcp.swatch)
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
        }
        Text {
            text: rcp.ratio
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
        }
    }
}
