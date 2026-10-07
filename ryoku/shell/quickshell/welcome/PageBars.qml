pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    property string selectedStyle: "qsbar"
    signal chose(string styleId)

    readonly property var styles: [
        { id: "sumi", name: I18n.tr("Sumi"), description: I18n.tr("The monochrome painted frame.") },
        { id: "qsbar", name: I18n.tr("QS Bar"), description: I18n.tr("The full-colour wallpaper bar."), recommended: true },
        { id: "kairos", name: I18n.tr("Kairos"), description: I18n.tr("A clock island that grows with you.") },
        { id: "iris", name: I18n.tr("Shima"), description: I18n.tr("An edge island that morphs into tools.") },
        { id: "python", name: I18n.tr("Python"), description: I18n.tr("Pills that open into one stage.") }
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
            title: I18n.tr("Choose your edge")
            description: I18n.tr("Pick any bar to try it now. The desktop switches while this window stays open.")
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
                    width: Math.floor((selectors.width - selectors.spacing * 4) / 5)
                    styleId: modelData.id
                    name: modelData.name
                    description: modelData.description
                    recommended: modelData.recommended === true
                    selected: root.selectedStyle === modelData.id
                    onChosen: styleId => root.chose(styleId)
                }
            }
        }
    }
}
