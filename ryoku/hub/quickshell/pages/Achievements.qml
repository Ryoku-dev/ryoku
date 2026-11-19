import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/about"

Item {
    id: root
    required property var hub
    property string section: "book"

    Component.onCompleted: Progression.award("ryoku-first-steps")

    Column {
        anchors.fill: parent
        spacing: LithicTokens.space.s3

        Row {
            width: parent.width
            spacing: LithicTokens.space.s3
            Plaque {
                kind: root.section === "book" ? "gold" : "quiet"
                small: true
                glyph: "auto_stories"
                text: I18n.tr("Achievement book")
                onClicked: root.section = "book"
            }
            Plaque {
                kind: root.section === "workshop" ? "gold" : "quiet"
                small: true
                glyph: "palette"
                text: I18n.tr("Profile workshop")
                onClicked: root.section = "workshop"
            }
            Plaque {
                kind: "line"
                small: true
                text: I18n.tr("View profile")
                onClicked: root.hub.goHome()
            }
        }

        Flickable {
            id: scroll
            width: parent.width
            height: parent.height - y - LithicTokens.space.s3
            contentHeight: sectionBody.implicitHeight + LithicTokens.space.s5 * 2
            clip: true
            boundsBehavior: Flickable.StopAtBounds

            Column {
                id: sectionBody
                x: LithicTokens.space.s5
                y: LithicTokens.space.s5
                width: Math.max(0, scroll.width - 2 * LithicTokens.space.s5 - 18)
                spacing: LithicTokens.space.s5

                ProgressionTab {
                    width: parent.width
                    visible: root.section === "book"
                }
                ProfileStudio {
                    width: parent.width
                    hub: root.hub
                    visible: root.section === "workshop"
                }
            }
        }
    }

}
