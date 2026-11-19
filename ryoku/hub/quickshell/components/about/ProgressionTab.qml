import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    spacing: LithicTokens.space.s4
    property int pageIndex: 0
    readonly property var group: Progression.groups[Math.max(0, Math.min(pageIndex, Progression.groups.length - 1))]
    readonly property var entries: Progression.visibleBadges(group.id)
    readonly property int earnedCount: entries.filter(a => Progression.data.earned[a.id] === true).length
    readonly property int claimableCount: entries.filter(a =>
        Progression.data.earned[a.id] === true && Progression.data.claimed[a.id] !== true).length

    Row {
        width: parent.width
        spacing: LithicTokens.space.s3
        Column {
            width: parent.width - 160
            spacing: LithicTokens.space.s2
            Label {
                text: I18n.tr("THE ACHIEVEMENT ARCHIVE")
                typeRole: "mono"
                ink: Lithic.gold
            }
            Label {
                text: I18n.tr("Every badge tells a story.")
                typeRole: "title"
            }
            Label {
                text: I18n.tr("%1 of %2 achievements completed").replace("%1",
                    Object.keys(Progression.data.earned || {}).length).replace("%2", Progression.achievements.length)
                typeRole: "detail"
                inkRole: "fg2"
            }
        }
        Column {
            width: 150
            spacing: LithicTokens.space.s2
            Label {
                text: String(Progression.balance) + " ◆"
                typeRole: "title"
                ink: Lithic.gold
            }
            Label {
                text: I18n.tr("AVAILABLE CREDITS")
                typeRole: "monoSm"
                inkRole: "fg2"
            }
        }
    }

    Rectangle {
        width: parent.width
        height: 1
        color: Lithic.gold
        opacity: 0.5
    }

    Row {
        width: parent.width
        spacing: LithicTokens.space.s3
        Plaque {
            kind: "line"
            small: true
            text: "◀"
            accessibleName: I18n.tr("Previous achievement category")
            enabled: root.pageIndex > 0
            onClicked: root.pageIndex--
        }
        Repeater {
            model: Progression.groups
            delegate: Plaque {
                required property var modelData
                required property int index
                kind: root.pageIndex === index ? "gold" : "quiet"
                small: true
                text: I18n.tr(modelData.title)
                onClicked: root.pageIndex = index
            }
        }
        Plaque {
            kind: "line"
            small: true
            text: "▶"
            accessibleName: I18n.tr("Next achievement category")
            enabled: root.pageIndex < Progression.groups.length - 1
            onClicked: root.pageIndex++
        }
    }

    Rectangle {
        width: parent.width
        height: 76
        radius: 6
        color: Lithic.mix(Lithic.ground, Lithic.gold, 0.07)
        border.width: 1
        border.color: Lithic.mix(Lithic.ground, Lithic.gold, 0.35)

        Row {
            anchors.fill: parent
            anchors.margins: 18
            spacing: 24
            Column {
                width: parent.width - 120
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5
                Label {
                    text: I18n.tr(root.group.title).toUpperCase()
                    typeRole: "section"
                }
                Label {
                    text: I18n.tr(root.group.subtitle)
                    typeRole: "detail"
                    inkRole: "fg2"
                }
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                Label {
                    text: root.earnedCount + " / " + root.entries.length
                    typeRole: "title"
                    ink: Lithic.gold
                }
                Label {
                    text: I18n.tr("RECORDED")
                    typeRole: "monoSm"
                    inkRole: "fg2"
                }
            }
        }
    }

    Item {
        width: parent.width
        height: root.group.id === "Classified" ? 112 : 0
        visible: root.group.id === "Classified"

        Column {
            anchors.centerIn: parent
            spacing: 9
            Label {
                anchors.horizontalCenter: parent.horizontalCenter
                text: Progression.discoveredSecrets > 0 ?
                    I18n.tr("CLASSIFIED ARCHIVE · %1 SIGNALS DECODED").replace("%1", Progression.discoveredSecrets) :
                    I18n.tr("THE ARCHIVE DOES NOT ANSWER EVERY QUESTION")
                typeRole: "monoSm"
                inkRole: "fg2"
            }
            Plaque {
                anchors.horizontalCenter: parent.horizontalCenter
                small: true
                kind: "quiet"
                text: "力 ◇ 力"
                accessibleName: I18n.tr("Listen for the archive signal")
                onClicked: Progression.recordSignal()
            }
        }
    }

    Grid {
        id: album
        width: parent.width
        columns: 2
        columnSpacing: LithicTokens.space.s3
        rowSpacing: LithicTokens.space.s3

        Repeater {
            model: root.entries
            delegate: Rectangle {
                id: achievementTile
                required property var modelData
                readonly property bool complete: Progression.data.earned[modelData.id] === true
                readonly property bool claimed: Progression.data.claimed[modelData.id] === true
                width: (album.width - album.columnSpacing) / 2
                height: 236
                radius: 6
                color: Lithic.mix(Lithic.ground, Lithic.fg, claimed ? 0.052 : 0.025)
                border.width: 1
                border.color: complete ? Progression.rarityColours[modelData.rarity] : Lithic.mix(Lithic.fg, Lithic.ground, 0.85)

                Column {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 7
                    Row {
                        width: parent.width
                        spacing: 12
                        ArchiveSeal {
                            width: 104
                            height: 104
                            title: achievementTile.modelData.title
                            sigil: achievementTile.modelData.sigil
                            rarity: achievementTile.modelData.rarity
                            category: achievementTile.modelData.category
                            locked: !achievementTile.complete
                            animateFoil: achievementTile.modelData.rarity === "holographic"
                        }
                        Column {
                            width: parent.width - 116
                            spacing: 4
                            Label {
                                width: parent.width
                                text: I18n.tr(achievementTile.modelData.title)
                                typeRole: "sentence"
                                elide: Text.ElideRight
                            }
                            Label {
                                text: achievementTile.modelData.rarity.toUpperCase()
                                typeRole: "monoSm"
                                ink: Progression.rarityColours[achievementTile.modelData.rarity]
                            }
                            Label {
                                text: achievementTile.claimed ? I18n.tr("CLAIMED") :
                                    achievementTile.complete ? I18n.tr("READY TO CLAIM") : I18n.tr("LOCKED")
                                typeRole: "monoSm"
                                ink: achievementTile.complete ? Lithic.gold : Lithic.fg3
                            }
                        }
                    }
                    Label {
                        width: parent.width
                        height: 36
                        text: I18n.tr(achievementTile.modelData.detail)
                        typeRole: "detail"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideRight
                    }
                    Label {
                        visible: text !== ""
                        text: Progression.progressFor(achievementTile.modelData.id)
                        typeRole: "monoSm"
                        ink: Progression.rarityColours[achievementTile.modelData.rarity]
                    }
                    Row {
                        width: parent.width
                        spacing: 8
                        Label {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "+" + achievementTile.modelData.reward + " ◆"
                            typeRole: "mono"
                            ink: Lithic.gold
                        }
                        Item { width: Math.max(0, parent.width - 255); height: 1 }
                        Plaque {
                            small: true
                            kind: achievementTile.complete && !achievementTile.claimed ? "gold" : "line"
                            text: achievementTile.claimed ? I18n.tr("Collected") :
                                achievementTile.complete ? I18n.tr("Claim") : I18n.tr("Locked")
                            enabled: achievementTile.complete && !achievementTile.claimed
                            onClicked: Progression.claim(achievementTile.modelData.id)
                        }
                        Plaque {
                            visible: achievementTile.claimed
                            small: true
                            kind: Progression.featuredBadge === achievementTile.modelData.id ? "gold" : "quiet"
                            text: Progression.featuredBadge === achievementTile.modelData.id ? I18n.tr("Featured") : I18n.tr("Feature")
                            enabled: Progression.featuredBadge !== achievementTile.modelData.id
                            onClicked: Progression.featureBadge(achievementTile.modelData.id)
                        }
                    }
                }
            }
        }
    }

    Label {
        width: parent.width
        visible: root.claimableCount > 0
        text: I18n.tr("%1 badge rewards waiting to be claimed on this page").replace("%1", root.claimableCount)
        typeRole: "detail"
        ink: Lithic.gold
    }
    Label {
        width: parent.width
        text: I18n.tr("Claiming an achievement adds its badge to your sticker collection. All rewards are local and cosmetic.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
