pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    property string selectedStyle: "orpheus"
    signal chose(string styleId)

    readonly property var styles: [
        { id: "orpheus", name: I18n.tr("Orpheus"), description: I18n.tr("A liquid bar that moves with your music."), recommended: true },
        { id: "kairos", name: I18n.tr("Kairos"), description: I18n.tr("A clock island that grows with you.") }
    ]

    function styleName(styleId) {
        for (let i = 0; i < styles.length; i++) {
            if (styles[i].id === styleId)
                return styles[i].name
        }
        return ""
    }

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s4

        PageTitle {
            width: parent.width
            title: I18n.tr("Choose your shell")
            description: I18n.tr("Try either included subshell. The desktop switches while this window stays open.")
        }

        BarPreviewStage {
            width: parent.width
            styleId: root.selectedStyle
            styleName: root.styleName(root.selectedStyle)
        }

        Row {
            id: selectors
            width: parent.width
            spacing: Tokens.s2

            Repeater {
                model: root.styles

                BarCard {
                    required property var modelData
                    width: Math.floor((selectors.width - selectors.spacing) / 2)
                    styleId: modelData.id
                    name: modelData.name
                    description: modelData.description
                    recommended: modelData.recommended === true
                    selected: root.selectedStyle === modelData.id
                    onChosen: styleId => root.chose(styleId)
                }
            }
        }

        Text {
            width: parent.width
            text: I18n.tr("Find more subshells and bar styles in Ryostore.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            horizontalAlignment: Text.AlignHCenter
        }
    }
}
