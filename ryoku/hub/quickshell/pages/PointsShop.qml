import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../components"
import "../components/about"

Item {
    id: root
    required property var hub

    Column {
        anchors.fill: parent
        spacing: LithicTokens.space.s3

        Row {
            spacing: LithicTokens.space.s3
            Plaque {
                small: true
                kind: "quiet"
                glyph: "auto_stories"
                text: I18n.tr("Achievements")
                onClicked: root.hub.navigate("achievements", "")
            }
            Plaque {
                small: true
                kind: "gold"
                glyph: "storefront"
                text: I18n.tr("Points Shop")
            }
            Plaque {
                small: true
                kind: "line"
                text: I18n.tr("View profile")
                onClicked: root.hub.goHome()
            }
        }

        Flickable {
            id: scroll
            width: parent.width
            height: parent.height - y - LithicTokens.space.s3
            contentHeight: studio.implicitHeight + LithicTokens.space.s5 * 2
            boundsBehavior: Flickable.StopAtBounds
            clip: true

            ProfileStudio {
                id: studio
                x: LithicTokens.space.s5
                y: LithicTokens.space.s5
                width: Math.max(0, scroll.width - 2 * LithicTokens.space.s5 - 18)
                hub: root.hub
            }
        }
    }
}
