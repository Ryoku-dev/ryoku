import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    spacing: LithicTokens.space.s4

    Label {
        text: I18n.tr("Your collection")
        typeRole: "title"
    }
    Label {
        width: parent.width
        text: I18n.tr("Complete milestones to earn credits. Rewards are one-time, offline and purely cosmetic.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Label {
        text: I18n.tr("%1 credits available").replace("%1", Progression.balance)
        typeRole: "sentence"
        ink: Lithic.gold
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
        text: I18n.tr("Achievements")
        typeRole: "section"
    }
    Repeater {
        model: Progression.achievements
        delegate: SettingRow {
            id: achievement
            required property var modelData
            readonly property bool complete: Progression.data.earned[modelData.id] === true
            width: root.width
            title: I18n.tr(modelData.title)
            detail: I18n.tr(modelData.detail)
            Label {
                text: achievement.complete ? I18n.tr("Complete") : "+" + achievement.modelData.reward
                typeRole: "mono"
                ink: achievement.complete ? Lithic.gold : Lithic.fg2
            }
        }
    }
    Label {
        text: I18n.tr("Card palettes")
        typeRole: "section"
    }
    Label {
        width: parent.width
        text: I18n.tr("Palettes recolour your stock card locally. Approved designs keep their own colours. Rarity and battle stats never change.")
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
