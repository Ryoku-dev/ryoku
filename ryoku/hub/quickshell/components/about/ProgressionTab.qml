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

    onPageIndexChanged: {
        root.categorySelected();
        if (!Lithic.reduceMotion)
            turnPage.restart();
    }

    NumberAnimation {
        id: turnPage
        target: album
        property: "opacity"
        from: 0.25
        to: 1
        duration: 270
        easing.type: Easing.OutCubic
    }
    readonly property var group: Progression.groups[Math.max(0, Math.min(pageIndex, Progression.groups.length - 1))]
    readonly property var entries: Progression.visibleBadges(group.id)
    readonly property int totalInCategory: Progression.achievements.filter(a => a.category === group.id).length
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
                text: root.group.id === "Mastery" ? I18n.tr("The master archives.") :
                    root.group.id === "Classified" ? I18n.tr("Beyond the visible.") :
                    I18n.tr("Every badge tells a story.")
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
        spacing: LithicTokens.space.s2
        Plaque {
            kind: "line"
            small: true
            text: "◀"
            accessibleName: I18n.tr("Previous achievement category")
            enabled: root.pageIndex > 0
            onClicked: root.pageIndex--
        }
        Flickable {
            width: Math.max(0, parent.width - 120)
            height: 38
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            contentWidth: categoryTabs.implicitWidth
            contentHeight: height
            Row {
                id: categoryTabs
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
                    text: root.earnedCount + " / " + root.totalInCategory
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
        height: root.group.id === "Classified" ? 154 : 0
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
            Row {
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: 8
                Repeater {
                    model: root.totalInCategory
                    delegate: Rectangle {
                        required property int index
                        width: 9
                        height: 9
                        rotation: 45
                        color: index < Progression.discoveredSecrets ? Lithic.gold : "transparent"
                        border.color: index < Progression.discoveredSecrets ? Lithic.gold : Lithic.fg3
                        border.width: 1
                    }
                }
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

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3
        visible: root.group.id === "Mastery"

        Rectangle {
            width: parent.width
            height: 108
            color: Lithic.mix(Lithic.ground, Lithic.gold, 0.065)
            border.color: Lithic.mix(Lithic.ground, Lithic.gold, 0.6)
            border.width: 1

            Row {
                anchors.fill: parent
                anchors.margins: 17
                spacing: 16
                Label {
                    text: "道"
                    typeRole: "title"
                    ink: Lithic.gold
                    anchors.verticalCenter: parent.verticalCenter
                }
                Column {
                    width: Math.max(0, parent.width - 80)
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 5
                    Label { text: "THE FOUR PILLARS"; typeRole: "mono"; ink: Lithic.gold }
                    Label {
                        width: parent.width
                        text: "Earn each category's complete collection. Nothing here requires better hardware or paid items."
                        typeRole: "detail"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }
        }
    }

    Grid {
        id: album
        width: parent.width
        columns: width >= 660 ? 2 : 1
        columnSpacing: LithicTokens.space.s3
        rowSpacing: LithicTokens.space.s3

        Repeater {
            model: root.entries
            delegate: AchievementCard {
                required property var modelData
                badge: modelData
                width: (album.width - (album.columns - 1) * album.columnSpacing) /
                    album.columns
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
