import QtQuick
import Ryoku.Ui.Singletons

// The controls above the front card, ported from kwepaperpicker's Wallpaper.qml.
// They act through Ryogami's picker state, so wall.apply stays the only owner
// of the wallpaper.
Item {
    id: actions

    property int row: -1
    property var entry: null
    property var dress: ({})
    property real u: 1
    property real reveal: 1
    property bool favourite: false
    property bool active: false

    signal favouriteClicked
    signal applyClicked

    readonly property color accent: actions.dress && actions.dress.accent ? actions.dress.accent : Theme.primary
    readonly property color accentInk: actions.dress && actions.dress.accentInk ? actions.dress.accentInk : Theme.primaryText
    readonly property color surface: actions.dress && actions.dress.surface ? actions.dress.surface : Theme.surface

    width: actionRow.implicitWidth
    height: 40 * u

    visible: actions.reveal > 0.05 && actions.row >= 0 && !!actions.entry
    opacity: actions.reveal

    Row {
        id: actionRow
        anchors.centerIn: parent
        spacing: 8 * actions.u

        Rectangle {
            id: favouriteButton
            width: 40 * actions.u
            height: 40 * actions.u
            radius: height / 2
            color: favouriteMouse.containsMouse
                ? Qt.lighter(actions.surface, 1.1)
                : Theme.withAlpha(actions.surface, 0.78)
            border.width: 1
            border.color: Theme.withAlpha(actions.dress && actions.dress.ink ? actions.dress.ink : Theme.surfaceText, 0.14)

            Text {
                anchors.centerIn: parent
                anchors.verticalCenterOffset: 0.5 * actions.u
                text: actions.favourite ? "\u{f02d1}" : "\u{f02d5}"
                font.family: Theme.icon
                font.pixelSize: 20 * actions.u
                color: actions.favourite ? Theme.primary : actions.dress && actions.dress.ink ? actions.dress.ink : Theme.surfaceText
            }

            MouseArea {
                id: favouriteMouse
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: actions.favouriteClicked()
            }
        }

        Rectangle {
            id: setButton
            implicitWidth: setRow.implicitWidth + 32 * actions.u
            implicitHeight: 40 * actions.u
            radius: Theme.radius * 3
            clip: true
            color: actions.active ? Theme.withAlpha(actions.surface, 0.82)
                : setMouse.containsMouse ? Qt.lighter(actions.accent, 1.08)
                : actions.accent
            border.width: actions.active ? 1 : 0
            border.color: Theme.withAlpha(actions.accentInk, 0.14)

            Row {
                id: setRow
                anchors.centerIn: parent
                spacing: 8 * actions.u

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: actions.active ? "\uf058" : "\uf03e"
                    font.family: Theme.icon
                    font.pixelSize: 19 * actions.u
                    color: actions.active ? actions.accent : actions.accentInk
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: actions.active ? I18n.tr("On your desktop") : I18n.tr("Set wallpaper")
                    font.family: Theme.sans
                    font.pixelSize: 14 * actions.u
                    font.weight: Font.DemiBold
                    color: actions.active
                        ? (actions.dress && actions.dress.ink ? actions.dress.ink : Theme.surfaceText)
                        : actions.accentInk
                }

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: enterKey.implicitWidth + 10 * actions.u
                    height: 20 * actions.u
                    radius: Theme.radius
                    color: Theme.withAlpha(actions.accentInk, 0.14)

                    Text {
                        id: enterKey
                        anchors.centerIn: parent
                        text: "\u23ce"
                        font.family: Theme.sans
                        font.pixelSize: 12 * actions.u
                        font.weight: Font.Bold
                        color: actions.active
                            ? (actions.dress && actions.dress.ink ? actions.dress.ink : Theme.surfaceText)
                            : actions.accentInk
                    }
                }
            }

            MouseArea {
                id: setMouse
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: actions.applyClicked()
            }
        }
    }
}
