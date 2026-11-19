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
    readonly property bool prestigious: badge.category === "Mastery" || badge.category === "Classified"
    readonly property var progress: Progression.progressInfoFor(badge.id)
    readonly property color metal: Progression.rarityColours[badge.rarity] || Lithic.fg2

    implicitHeight: 304

    CutRect {
        anchors.fill: parent
        cut: card.prestigious ? LithicTokens.cut.large : LithicTokens.cut.small
        fill: Lithic.mix(Lithic.ground, card.metal, card.prestigious ? 0.075 : 0.025)
        strokeColor: card.earned ? card.metal : Lithic.mix(Lithic.ground, Lithic.fg, 0.24)
        strokeWidth: card.prestigious ? 2 : 1
    }

    ArchiveSeal {
        id: emblem
        badgeId: card.badge.id
        x: 19
        y: 27
        width: 124
        height: 124
        title: card.badge.title
        sigil: card.badge.sigil
        rarity: card.badge.rarity
        category: card.badge.category
        locked: !card.earned
        animateFoil: card.visible && card.earned &&
            card.badge.rarity === "holographic" && !Lithic.reduceMotion
    }

    Column {
        x: 158
        y: 29
        width: Math.max(0, parent.width - x - 18)
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
            text: card.badge.rarity.toUpperCase()
            typeRole: "monoSm"
            ink: card.metal
        }
        Label {
            text: "+" + card.badge.reward + " ◆"
            typeRole: "mono"
            ink: Lithic.gold
        }
    }

    Label {
        x: 20
        y: 167
        width: parent.width - 40
        height: 43
        text: card.badge.detail
        typeRole: "detail"
        inkRole: card.earned ? "fg" : "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideRight
        maximumLineCount: 2
        verticalAlignment: Text.AlignVCenter
    }

    Item {
        id: tracker
        x: 20
        y: 218
        width: parent.width - 40
        height: 31
        visible: card.progress !== null && !card.earned

        Label {
            anchors.left: parent.left
            anchors.top: parent.top
            text: card.progress ? card.progress.label : ""
            typeRole: "monoSm"
            inkRole: "fg2"
        }
        Label {
            anchors.right: parent.right
            anchors.top: parent.top
            text: card.progress ? card.progress.current + " / " + card.progress.total : ""
            typeRole: "monoSm"
            ink: card.metal
        }
        Rectangle {
            anchors.bottom: parent.bottom
            width: parent.width
            height: 3
            color: Lithic.mix(Lithic.ground, card.metal, 0.18)
            Rectangle {
                width: parent.width * (card.progress ? card.progress.fraction : 0)
                height: parent.height
                color: card.metal
                Behavior on width {
                    enabled: !Lithic.reduceMotion
                    NumberAnimation { duration: 240; easing.type: Easing.OutCubic }
                }
            }
        }
    }

    Row {
        anchors.right: parent.right
        anchors.rightMargin: 19
        y: parent.height - 47
        height: 32
        spacing: 8
        Plaque {
            small: true
            visible: card.earned && !card.claimed
            kind: "gold"
            text: I18n.tr("Claim")
            onClicked: card.claimRequested(card.badge)
        }
        Plaque {
            small: true
            visible: card.claimed
            kind: Progression.featuredBadge === card.badge.id ? "gold" : "line"
            text: Progression.featuredBadge === card.badge.id ? I18n.tr("Featured") : I18n.tr("Feature")
            enabled: Progression.featuredBadge !== card.badge.id
            onClicked: card.featureRequested(card.badge.id)
        }
    }

    Label {
        x: 20
        y: parent.height - 38
        visible: card.earned && !card.claimed
        text: I18n.tr("Reward ready")
        typeRole: "detail"
        ink: card.metal
    }
}
