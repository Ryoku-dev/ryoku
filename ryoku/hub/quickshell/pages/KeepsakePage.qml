import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../components"

Item {
    id: pg
    property var hub: null
    readonly property bool fullBleed: true
    property real reveal: 0
    Component.onCompleted: {
        enter.start();
        Qt.callLater(() => scroll.contentY = 0);
    }

    property string beginning: [
        "Ryoku on NixOS started out as me wanting Ryoku on my desktop but it didn't support NixOS so I made it a",
        "reality and what I thought would be a simple QML port to the correct folders and a few commands turned",
        "into an entire repository with over 30,000 lines of translation code, removing arch assumptions, wrappers,",
        "scripts, compiling the whole 9 yards and throughout all of it, I never actually thought many people would",
        "actually use it but not even in the first 2 months of it being public theres been over 2,000 unique git",
        "clones and that to me is still insane to this day, so to each and every one of you, thank you, not only",
        "for installing and using this project of mine but also all the feedback, PR's and even all the issue",
        "reporting <3",
    ].join(" ")
    property string people: "The contributors behind Ryoku on NixOS, the ones who make this entire project a reality."
    property string letter: "Thank you from all of us for using Ryoku on NixOS, it means alot to us and I hope Ryoku is just as good as you expected it to be. Thank you for believing in us."
    readonly property var contributorMessages: [
        { name: "aethctl", role: "Lead Developer", quote: "Does tel aviv finally approve of my larp?", text: "", avatar: "../art/maintainers/aethctl.png", github: "https://github.com/aethctl" },
        { name: "feelyourwarmth", role: "Maintainer", quote: "I didnt do shit to Ryoku, why am I here", text: "", avatar: "../art/maintainers/feelyourwarmth.png", github: "https://github.com/feelyourwarmth" },
        { name: "PrayToWin", role: "Contributor", quote: "One more PR then im gonna sleep - 4:17 am", text: "", avatar: "../art/maintainers/praytowin.png", github: "https://github.com/eximora-private" },
        { name: "Mo", role: "Tester", quote: "I just clicked things until something broke.", text: "", avatar: "../art/maintainers/mo.png", github: "" }
    ]
    readonly property var earlyAdopters: [
        { name: "@Prime_79" },
        { name: "@ᴘʀᴀʏᴛᴏᴡɪɴ@ɴɪxᴏs" },
        { name: "@bob" },
        { name: "@Mo" },
        { name: "@Daman Weylin" },
        { name: "@tiogilito" },
        { name: "@Gamegyf" },
        { name: "@᲼᲼᲼᲼᲼᲼᲼᲼᲼᲼" },
        { name: "@Greed" },
        { name: "@ZTZ-V1D" },
        { name: "@༺ 𝕺𝖓𝖞𝖝 ༻" },
        { name: "@Philia" },
        { name: "@the random" },
        { name: "@✦ Panicat ✦" },
        { name: "@feel@nixos" }
    ]
    NumberAnimation {
        id: enter
        target: pg
        property: "reveal"
        from: 0
        to: 1
        duration: 950
        easing.type: Easing.OutCubic
    }

    Rectangle {
        anchors.fill: parent
        color: Lithic.ground
    }
    Image {
        anchors.fill: parent
        source: "../art/maintainers/paper-grain.png"
        fillMode: Image.Tile
        opacity: 0.07
    }
    Text {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.rightMargin: 25
        text: "力"
        color: Lithic.alpha(Lithic.gold, 0.11)
        font.pixelSize: Math.min(pg.width * 0.3, 280)
    }

    Flickable {
        id: scroll
        anchors.fill: parent
        anchors.margins: LithicTokens.space.s4
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        contentWidth: width
        contentHeight: content.implicitHeight + LithicTokens.space.s7

        Column {
            id: content
            x: Math.max(0, Math.round((scroll.width - width) / 2))
            y: LithicTokens.space.s5 + (1 - pg.reveal) * 28
            width: Math.min(840, Math.max(1, scroll.width - LithicTokens.space.s5 * 2))
            spacing: LithicTokens.space.s5
            opacity: pg.reveal

            Label {
                text: "RYOKU / KEEPSAKE 01"
                typeRole: "mono"
                ink: Lithic.gold
            }
            Label {
                width: parent.width
                text: "The Human Side of Ryoku"
                typeRole: "title"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                text: "Behind every line of code, there was a person."
                typeRole: "detail"
                inkRole: "fg2"
            }
            Rectangle {
                width: parent.width
                height: 1
                color: Lithic.mix(Lithic.gold, Lithic.ground, 0.4)
            }
            Label {
                text: "How Ryoku on NixOS was made"
                typeRole: "section"
            }
            Text {
                width: parent.width
                text: pg.beginning
                color: Lithic.fg2
                font.pixelSize: 17
                font.weight: Font.Normal
                lineHeight: 1.48
                wrapMode: Text.WordWrap
            }
            Label {
                text: "The people behind Ryoku on NixOS"
                typeRole: "section"
            }
            Label {
                width: parent.width
                text: pg.people
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Repeater {
                model: pg.contributorMessages
                delegate: Item {
                    id: contributor
                    required property var modelData
                    required property int index
                    width: content.width
                    height: Math.max(112, quote.implicitHeight + 68)

                    CutRect {
                        anchors.fill: parent
                        cut: LithicTokens.cut.small
                        fill: Lithic.mix(Lithic.ground, Lithic.fg, 0.035)
                        strokeColor: Lithic.mix(Lithic.ground, Lithic.gold, 0.35)
                        strokeWidth: 1
                    }
                    Image {
                        id: face
                        width: 82
                        height: 82
                        x: 14
                        anchors.verticalCenter: parent.verticalCenter
                        source: contributor.modelData.avatar
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                    Column {
                        x: 110
                        y: 15
                        width: Math.max(20, parent.width - 126)
                        spacing: 8
                        Label {
                            text: contributor.modelData.name + "  /  " + contributor.modelData.role
                            typeRole: "mono"
                            ink: Lithic.gold
                        }
                        Text {
                            id: quote
                            width: parent.width
                            text: "“" + contributor.modelData.quote + "”"
                            color: Lithic.fg
                            font.pixelSize: 19
                            font.italic: true
                            lineHeight: 1.3
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }
            Label {
                text: "A note to you"
                typeRole: "section"
                ink: Lithic.gold
            }
            Text {
                width: parent.width
                text: pg.letter
                color: Lithic.fg
                font.pixelSize: 20
                font.italic: true
                lineHeight: 1.42
                wrapMode: Text.WordWrap
            }
            Label {
                text: "EARLY ADOPTERS"
                typeRole: "mono"
                ink: Lithic.gold
            }
            Flow {
                width: parent.width
                spacing: LithicTokens.space.s2
                Repeater {
                    model: pg.earlyAdopters
                    delegate: Rectangle {
                        id: adopter
                        required property var modelData
                        radius: 4
                        width: adopterLabel.implicitWidth + 26
                        height: 34
                        color: Lithic.mix(Lithic.ground, Lithic.fg, 0.055)
                        border.width: 1
                        border.color: Lithic.mix(Lithic.ground, Lithic.fg, 0.22)
                        Label {
                            id: adopterLabel
                            anchors.centerIn: parent
                            text: adopter.modelData.name
                            typeRole: "monoSm"
                        }
                    }
                }
            }
            Label {
                text: "ESC TO RETURN"
                typeRole: "monoSm"
                inkRole: "fg2"
            }
        }
    }
}
