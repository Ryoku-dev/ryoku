import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Item {
    id: card

    required property var badge
    signal claimRequested(var badge)
    signal featureRequested(string id)

    readonly property bool earned: Progression.data.earned[badge.id] === true
    readonly property bool claimed: Progression.data.claimed[badge.id] === true
    readonly property bool mastery: badge.category === "Mastery"
    readonly property bool classified: badge.category === "Classified"
    readonly property var progress: Progression.progressInfoFor(badge.id)
    readonly property color metal: Progression.rarityColours[badge.rarity] || Lithic.fg2
    readonly property bool brilliant: badge.rarity === "holographic"
    readonly property bool prestigious: mastery || classified
    readonly property string status: claimed ? "ARCHIVED" : earned ? "READY TO CLAIM" : "UNDISCOVERED"
    readonly property real cutSize: prestigious ? 22 : 12

    implicitHeight: prestigious ? 314 : 294
    clip: true

    HoverHandler { id: hover; enabled: !Lithic.reduceMotion }

    CutRect {
        anchors.fill: parent
        cut: card.cutSize
        fill: card.prestigious ? Lithic.mix(Lithic.ground, card.metal, card.earned ? 0.12 : 0.055)
            : Lithic.mix(Lithic.ground, Lithic.fg, card.earned ? 0.065 : 0.026)
        strokeColor: card.earned ? card.metal : Lithic.mix(Lithic.ground, Lithic.fg, 0.22)
        strokeWidth: card.prestigious ? 2 : 1
    }

    Rectangle {
        x: 3
        y: 30
        width: card.earned ? 3 : 1
        height: parent.height - 61
        color: card.metal
        opacity: card.earned ? 0.9 : 0.23
    }

    Rectangle {
        x: 22
        y: 36
        width: parent.width - 44
        height: 1
        color: card.metal
        opacity: card.prestigious ? 0.55 : 0.25
    }

    Rectangle {
        x: 22
        y: parent.height - 62
        width: parent.width - 44
        height: 1
        color: card.metal
        opacity: card.prestigious ? 0.6 : 0.22
    }

    Row {
        x: 20
        y: 10
        width: parent.width - 40
        spacing: 7

        Label {
            text: card.classified ? "DECRYPTED / 07" :
                card.mastery ? "MASTERY / ARCHIVE" : "RYOKU / " + card.badge.category.toUpperCase()
            typeRole: "monoSm"
            ink: card.metal
        }
        Item { width: Math.max(0, parent.width - 260); height: 1 }
        Label {
            text: card.badge.rarity.toUpperCase()
            typeRole: "monoSm"
            ink: card.metal
        }
    }

    Item {
        id: insignia
        x: 17
        y: 50
        width: card.prestigious ? 137 : 125
        height: width

        Rectangle {
            visible: card.prestigious && card.earned
            anchors.centerIn: parent
            width: parent.width * 0.91
            height: width
            rotation: 45
            color: "transparent"
            border.color: card.metal
            border.width: 1
            opacity: card.brilliant ? 0.45 : 0.28
        }

        ArchiveSeal {
            anchors.fill: parent
            title: card.badge.title
            sigil: card.badge.sigil
            rarity: card.badge.rarity
            category: card.badge.category
            locked: !card.earned
            animateFoil: card.earned && card.brilliant && card.visible &&
                !Lithic.reduceMotion
        }
    }

    Column {
        x: insignia.x + insignia.width + 10
        y: 54
        width: Math.max(10, parent.width - x - 16)
        spacing: 9

        Label {
            width: parent.width
            text: card.badge.title
            typeRole: "section"
            wrapMode: Text.Wrap
            elide: Text.ElideRight
            maximumLineCount: 2
        }
        Label {
            width: parent.width
            text: card.status
            typeRole: "monoSm"
            ink: card.earned ? card.metal : Lithic.fg3
            elide: Text.ElideRight
        }
        Label {
            width: parent.width
            text: "+" + card.badge.reward + " ◆"
            typeRole: "mono"
            ink: Lithic.gold
        }
    }

    Column {
        x: 21
        y: 200
        width: parent.width - 42
        spacing: 8

        Label {
            width: parent.width
            text: card.badge.detail
            typeRole: "detail"
            inkRole: card.earned ? "fg" : "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideRight
            maximumLineCount: 2
        }

        Item {
            width: parent.width
            height: 24
            visible: card.progress !== null && !card.earned

            Row {
                anchors.top: parent.top
                width: parent.width
                Label {
                    text: card.progress ? card.progress.label : ""
                    typeRole: "monoSm"
                    ink: card.metal
                }
                Item { width: Math.max(0, parent.width - 200); height: 1 }
                Label {
                    text: card.progress ? card.progress.current + " / " + card.progress.total : ""
                    typeRole: "monoSm"
                    ink: card.metal
                }
            }
            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: 3
                color: Lithic.mix(Lithic.ground, card.metal, 0.16)
                Rectangle {
                    width: parent.width * (card.progress ? card.progress.fraction : 0)
                    height: parent.height
                    color: card.metal
                    Behavior on width {
                        enabled: !Lithic.reduceMotion
                        NumberAnimation { duration: 320; easing.type: Easing.OutCubic }
                    }
                }
            }
        }
    }

    Row {
        x: 20
        y: parent.height - 52
        width: parent.width - 40
        spacing: 7

        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: card.claimed ? "IN YOUR ARCHIVE" : card.earned ? "REWARD AVAILABLE" : "CHALLENGE ACTIVE"
            typeRole: "monoSm"
            ink: card.earned ? card.metal : Lithic.fg3
        }
        Item { width: Math.max(0, parent.width - (card.claimed ? 300 : 270)); height: 1 }
        Plaque {
            small: true
            kind: card.earned && !card.claimed ? "gold" : "quiet"
            text: card.claimed ? "Claimed" : card.earned ? "Claim" : "Locked"
            enabled: card.earned && !card.claimed
            onClicked: card.claimRequested(card.badge)
        }
        Plaque {
            visible: card.claimed
            small: true
            kind: Progression.featuredBadge === card.badge.id ? "gold" : "line"
            text: Progression.featuredBadge === card.badge.id ? "Featured" : "Feature"
            enabled: Progression.featuredBadge !== card.badge.id
            onClicked: card.featureRequested(card.badge.id)
        }
    }
}
