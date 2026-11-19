import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    spacing: LithicTokens.space.s4

    readonly property var featured: Progression.achievements.find(a => a.id === Progression.featuredBadge)

    Label {
        text: I18n.tr("Your Ryoku achievements")
        typeRole: "title"
    }
    Label {
        width: parent.width
        text: I18n.tr("Badges celebrate your milestones across Ryoku. Choose one to feature on your profile.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Row {
        spacing: LithicTokens.space.s5
        Label {
            text: I18n.tr("%1 / %2 badges").replace("%1", Object.keys(Progression.data.earned || {}).length)
                .replace("%2", Progression.achievements.length)
            typeRole: "sentence"
        }
        Label {
            text: I18n.tr("%1 credits").replace("%1", Progression.balance)
            typeRole: "sentence"
            ink: Lithic.gold
        }
    }
    Label {
        width: parent.width
        text: root.featured ? I18n.tr("Featured badge: %1").replace("%1", I18n.tr(root.featured.title))
            : I18n.tr("No featured badge yet")
        typeRole: "body"
        ink: Lithic.gold
    }
    Plaque {
        visible: Progression.featuredBadge !== ""
        small: true
        kind: "quiet"
        text: I18n.tr("Remove featured badge")
        onClicked: Progression.featureBadge("")
    }
    Label {
        width: parent.width
        visible: Progression.error !== ""
        text: Progression.error
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Label {
        text: I18n.tr("Across Ryoku")
        typeRole: "section"
    }
    Repeater {
        model: Progression.achievements.filter(a => a.category === "Ryoku")
        delegate: SettingRow {
            id: milestone
            required property var modelData
            readonly property bool complete: Progression.data.earned[modelData.id] === true
            width: root.width
            title: I18n.tr(modelData.title)
            detail: I18n.tr(modelData.detail)
            Row {
                spacing: LithicTokens.space.s2
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: milestone.complete ? I18n.tr("Earned") : "+" + milestone.modelData.reward
                    typeRole: "mono"
                    ink: milestone.complete ? Lithic.gold : Lithic.fg2
                }
                Plaque {
                    visible: milestone.complete && Progression.featuredBadge !== milestone.modelData.id
                    small: true
                    kind: "line"
                    text: I18n.tr("Feature")
                    onClicked: Progression.featureBadge(milestone.modelData.id)
                }
            }
        }
    }
    Label {
        text: I18n.tr("Card studio")
        typeRole: "section"
    }
    Repeater {
        model: Progression.achievements.filter(a => a.category === "Card studio")
        delegate: SettingRow {
            id: milestone
            required property var modelData
            readonly property bool complete: Progression.data.earned[modelData.id] === true
            width: root.width
            title: I18n.tr(modelData.title)
            detail: I18n.tr(modelData.detail)
            Row {
                spacing: LithicTokens.space.s2
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: milestone.complete ? I18n.tr("Earned") : "+" + milestone.modelData.reward
                    typeRole: "mono"
                    ink: milestone.complete ? Lithic.gold : Lithic.fg2
                }
                Plaque {
                    visible: milestone.complete && Progression.featuredBadge !== milestone.modelData.id
                    small: true
                    kind: "line"
                    text: I18n.tr("Feature")
                    onClicked: Progression.featureBadge(milestone.modelData.id)
                }
            }
        }
    }
    Label {
        text: I18n.tr("Cosmetic collection")
        typeRole: "section"
    }
    Label {
        width: parent.width
        text: I18n.tr("Credits unlock visual rewards. Stock card palettes are the first collection, with more Ryoku-wide cosmetics planned.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Repeater {
        model: Progression.cosmetics
        delegate: SettingRow {
            id: cosmetic
            required property var modelData
            readonly property bool owned: Progression.data.owned[modelData.id] === true
            readonly property bool equipped: Progression.data.active === modelData.id
            width: root.width
            title: I18n.tr(modelData.name)
            detail: cosmetic.owned ? I18n.tr("Unlocked") : I18n.tr("%1 credits").replace("%1", modelData.price)
            Row {
                spacing: LithicTokens.space.s2
                Rectangle {
                    width: 24
                    height: 24
                    radius: 4
                    color: cosmetic.modelData.palette.accent
                    border.color: cosmetic.modelData.palette.rim
                    anchors.verticalCenter: parent.verticalCenter
                }
                Plaque {
                    small: true
                    kind: cosmetic.equipped ? "gold" : "line"
                    text: cosmetic.equipped ? I18n.tr("Equipped") : cosmetic.owned ? I18n.tr("Equip") : I18n.tr("Unlock")
                    enabled: Progression.ready && Progression.error === "" &&
                        !cosmetic.equipped && (cosmetic.owned || Progression.balance >= cosmetic.modelData.price)
                    onClicked: {
                        if (cosmetic.owned)
                            Progression.equip(cosmetic.modelData.id);
                        else
                            Progression.purchase(cosmetic.modelData.id);
                    }
                }
            }
        }
    }
    Plaque {
        kind: "quiet"
        small: true
        text: I18n.tr("Use original card palette")
        enabled: Progression.data.active !== ""
        onClicked: Progression.equip("")
    }
}
