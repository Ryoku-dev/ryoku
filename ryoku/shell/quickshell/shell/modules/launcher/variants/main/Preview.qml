pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property var settings: ({})
    signal editRequested(string key, var value)

    implicitWidth: 720
    implicitHeight: 250

    readonly property var values: settings || ({})
    readonly property url heroSource: values.heroImage !== undefined
        && values.heroImage !== null ? values.heroImage : ""
    readonly property real heroStrength: boundedNumber(values.heroStrength, 0.6)
    readonly property real heroPosX: boundedNumber(values.heroPosX, 0.5)
    readonly property real heroPosY: boundedNumber(values.heroPosY, 0.5)
    readonly property bool showGreeting: values.showGreeting === undefined
        ? true : !!values.showGreeting
    readonly property bool showWeather: values.showWeather === undefined
        ? true : !!values.showWeather

    function boundedNumber(value, fallback) {
        const number = Number(value);
        return isFinite(number) ? Math.max(0, Math.min(1, number)) : fallback;
    }

    LayoutMirroring.enabled: Qt.application.layoutDirection === Qt.RightToLeft
    LayoutMirroring.childrenInherit: true

    Rectangle {
        id: searchRow

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        height: Tokens.rowH
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        Text {
            id: brand

            anchors.left: parent.left
            anchors.leftMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            text: UiPreferences.brandMark("力")
            color: Tokens.sun
            font.family: Tokens.jp
            font.pixelSize: Tokens.fRow
            font.weight: Font.DemiBold
        }

        Text {
            anchors.left: brand.right
            anchors.leftMargin: Tokens.s3
            anchors.right: helpKey.left
            anchors.rightMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Search apps, type / for commands")
            color: Tokens.inkMuted
            elide: Text.ElideRight
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
        }

        Rectangle {
            id: helpKey

            anchors.right: appGrid.left
            anchors.rightMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.s5
            height: Tokens.s5
            radius: Tokens.radius
            color: Tokens.tint5
            border.width: Tokens.border
            border.color: Tokens.line

            Text {
                anchors.centerIn: parent
                text: "?"
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.DemiBold
            }
        }

        Grid {
            id: appGrid

            anchors.right: parent.right
            anchors.rightMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            columns: 3
            rowSpacing: Tokens.s1
            columnSpacing: Tokens.s1

            Repeater {
                model: 9

                Rectangle {
                    width: Tokens.s1
                    height: Tokens.s1
                    radius: Tokens.radius
                    color: Tokens.inkMuted
                }
            }
        }
    }

    Rectangle {
        id: dashboard

        anchors.top: searchRow.bottom
        anchors.topMargin: Tokens.s3
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        radius: Tokens.radius
        color: Tokens.paperLift

        HeroCrop {
            id: hero

            anchors.fill: parent
            anchors.margins: Tokens.border
            source: root.heroSource
            fallbackSource: Qt.resolvedUrl("../../shared/art/hands-adam.png")
            focalX: root.heroPosX
            focalY: root.heroPosY
            strength: root.heroStrength
        }

        Column {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s5
            anchors.top: parent.top
            anchors.topMargin: Tokens.s4
            spacing: Tokens.s1

            Text {
                visible: root.showGreeting
                text: I18n.tr("GOOD MORNING")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.weight: Font.DemiBold
                font.letterSpacing: Tokens.trackLabel
            }

            Text {
                text: "09:41"
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fHero
                font.weight: Font.Medium
                font.features: ({ "tnum": 1 })
            }
        }

        Column {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s5
            anchors.top: parent.top
            anchors.topMargin: Tokens.s4
            spacing: Tokens.s1

            Text {
                anchors.right: parent.right
                visible: root.showWeather
                text: I18n.tr("21°C")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fValue
                font.weight: Font.Medium
                font.features: ({ "tnum": 1 })
            }

            Text {
                anchors.right: parent.right
                visible: root.showWeather
                text: I18n.tr("Clear sky")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }

            Text {
                anchors.right: parent.right
                text: I18n.tr("THURSDAY, JUL 30")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: Tokens.s5
            anchors.rightMargin: Tokens.s5
            anchors.bottomMargin: Tokens.s4
            height: Tokens.border
            color: Tokens.lineStrong
        }

        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.lineStrong
        }
    }
}
