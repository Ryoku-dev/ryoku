import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    spacing: LithicTokens.space.s4

    property int pageIndex: 0
    signal categorySelected()
    signal badgeClaimed(var badge)

    readonly property var group: Progression.groups[Math.max(0, Math.min(pageIndex, Progression.groups.length - 1))]
    readonly property var entries: Progression.visibleBadges(group.id)
    readonly property int totalInCategory: Progression.achievements.filter(a => a.category === group.id).length
    readonly property int earnedCount: entries.filter(a => Progression.data.earned[a.id] === true).length

    onPageIndexChanged: {
        root.categorySelected();
        if (!Lithic.reduceMotion) pageTurn.restart();
    }

    NumberAnimation {
        id: pageTurn
        target: album
        property: "opacity"
        from: 0.35
        to: 1
        duration: 220
        easing.type: Easing.OutCubic
    }

    Row {
        width: parent.width
        height: 56
        spacing: LithicTokens.space.s3

        Column {
            width: Math.max(0, parent.width - credits.width - 16)
            anchors.verticalCenter: parent.verticalCenter
            spacing: 5
            Label {
                text: Object.keys(Progression.data.earned || {}).length + " / " +
                    Progression.achievements.length
                typeRole: "title"
            }
            Label {
                text: I18n.tr("Achievements completed")
                typeRole: "detail"
                inkRole: "fg2"
            }
        }
        Column {
            id: credits
            anchors.verticalCenter: parent.verticalCenter
            width: 155
            spacing: 5
            Label {
                text: Progression.balance + " ◆"
                typeRole: "title"
                ink: Lithic.gold
            }
            Label {
                text: I18n.tr("Points")
                typeRole: "detail"
                inkRole: "fg2"
            }
        }
    }

    Flickable {
        width: parent.width
        height: 42
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        contentHeight: height
        contentWidth: categories.implicitWidth
        Row {
            id: categories
            height: parent.height
            spacing: LithicTokens.space.s2
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
        }
    }

    Row {
        width: parent.width
        height: 42
        spacing: 12
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(root.group.title)
            typeRole: "section"
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.earnedCount + " / " + root.totalInCategory
            typeRole: "mono"
            ink: Lithic.gold
        }
    }

    Item {
        width: parent.width
        height: root.group.id === "Classified" ? 66 : 0
        visible: root.group.id === "Classified"
        Row {
            anchors.centerIn: parent
            spacing: 7
            Repeater {
                model: root.totalInCategory
                delegate: Rectangle {
                    required property int index
                    width: 8
                    height: 8
                    rotation: 45
                    color: index < Progression.discoveredSecrets ? Lithic.gold : "transparent"
                    border.width: 1
                    border.color: index < Progression.discoveredSecrets ? Lithic.gold : Lithic.fg3
                }
            }
        }
        Plaque {
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            small: true
            kind: "quiet"
            text: "力 ◇ 力"
            accessibleName: I18n.tr("Listen for the archive signal")
            onClicked: Progression.recordSignal()
        }
    }

    Grid {
        id: album
        width: parent.width
        columns: width >= 680 ? 2 : 1
        columnSpacing: LithicTokens.space.s3
        rowSpacing: LithicTokens.space.s3

        Repeater {
            model: root.entries
            delegate: AchievementCard {
                required property var modelData
                badge: modelData
                width: (album.width - (album.columns - 1) * album.columnSpacing) / album.columns
                height: implicitHeight
                onClaimRequested: badge => {
                    Progression.claim(badge.id);
                    if (Progression.data.claimed[badge.id] === true)
                        root.badgeClaimed(badge);
                }
                onFeatureRequested: id => Progression.featureBadge(id)
            }
        }
    }
}
