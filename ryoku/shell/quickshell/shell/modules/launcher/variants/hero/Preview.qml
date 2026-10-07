pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property var settings: ({})
    signal editRequested(string key, var value)

    implicitWidth: 720
    implicitHeight: 250
    clip: true

    property var now: new Date()
    readonly property string clockStr: TimeFormat.format(root.now)
    readonly property string dateStr: Qt.locale("en_US").toString(
        root.now, "dddd, MMM d")
    readonly property string greeting: {
        var hour = root.now.getHours();
        return hour < 5 ? I18n.tr("GOOD NIGHT") : hour < 12 ? I18n.tr("GOOD MORNING")
            : hour < 18 ? I18n.tr("GOOD AFTERNOON") : I18n.tr("GOOD EVENING");
    }
    readonly property string effectiveWeatherUnit: {
        var unit = String(root.setting("weatherUnit", "auto") || "auto");
        return unit === "auto" ? root.localeUnit() : unit;
    }

    function setting(key, fallback) {
        var source = root.settings || {};
        return source[key] !== undefined ? source[key] : fallback;
    }

    function finiteOr(value, fallback) {
        var number = Number(value);
        return isFinite(number) ? number : fallback;
    }

    function localeUnit() {
        var locale = String(Quickshell.env("LC_MEASUREMENT")
            || Quickshell.env("LANG") || "");
        return /(^|[_.@-])(US|LR|MM)([_.@-]|$)/.test(locale) ? "F" : "C";
    }

    function pad2(number) {
        return (number < 10 ? "0" : "") + number;
    }

    Timer {
        interval: 10000
        running: true
        repeat: true
        onTriggered: root.now = new Date()
    }

    HeroCrop {
        id: heroCrop
        anchors.fill: parent
        source: String(root.setting("heroImage", "") || "")
        fallbackSource: Qt.resolvedUrl("../../shared/art/hands-adam.png")
        focalX: root.finiteOr(root.setting("heroPosX", 0.5), 0.5)
        focalY: root.finiteOr(root.setting("heroPosY", 0.5), 0.5)
        strength: root.finiteOr(root.setting("heroStrength", 0.6), 0.6)
    }

    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            GradientStop {
                position: 0
                color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.52)
            }
            GradientStop {
                position: 0.34
                color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.06)
            }
            GradientStop {
                position: 0.68
                color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.10)
            }
            GradientStop {
                position: 1
                color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.64)
            }
        }
    }

    Column {
        x: 16
        y: 12
        spacing: 1

        Text {
            visible: !!root.setting("showGreeting", true)
            text: root.greeting
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 9
            font.weight: Font.DemiBold
            font.letterSpacing: 1.5
        }
        Text {
            text: root.clockStr
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: 28
            font.weight: Font.Light
            font.features: ({ "tnum": 1 })
        }
    }

    Column {
        anchors.right: parent.right
        anchors.rightMargin: 16
        y: 12
        spacing: 1

        Text {
            anchors.right: parent.right
            visible: !!root.setting("showWeather", true)
            text: (root.effectiveWeatherUnit === "F" ? "70" : "21")
                + (root.effectiveWeatherUnit === "F"
                    ? I18n.tr("\u00b0F") : I18n.tr("\u00b0C"))
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: 19
            font.weight: Font.Medium
            font.features: ({ "tnum": 1 })
        }
        Text {
            anchors.right: parent.right
            visible: !!root.setting("showWeather", true)
            text: I18n.tr("Clear sky")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 10
        }
        Text {
            anchors.right: parent.right
            text: root.dateStr
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 9
            font.letterSpacing: 0.6
        }
    }

    Item {
        x: 145
        y: 105
        width: 430
        height: 30

        Text {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: "⌕"
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: 25
        }
        Text {
            anchors.left: parent.left
            anchors.leftMargin: 26
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("TYPE TO SEARCH")
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: 11
            font.weight: Font.Medium
            font.letterSpacing: 1.3
        }
        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: Tokens.border
            color: Tokens.ink
        }
    }

    Row {
        anchors.horizontalCenter: parent.horizontalCenter
        y: 154
        spacing: 7

        Repeater {
            model: [I18n.tr("ALL"), I18n.tr("IMG"), I18n.tr("FILE"), I18n.tr("REC")]

            delegate: Rectangle {
                required property string modelData

                width: label.implicitWidth + 16
                height: 19
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.inkMuted

                Text {
                    id: label
                    anchors.centerIn: parent
                    text: modelData
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: 8
                    font.weight: Font.DemiBold
                    font.letterSpacing: 0.7
                }
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: Tokens.border
        color: Tokens.lineStrong
    }

    DragHandler {
        id: dragH
        target: null
        enabled: heroCrop.ready
            && (heroCrop.overflowX > 1 || heroCrop.overflowY > 1)
        cursorShape: Qt.SizeAllCursor

        property real originalX: 0.5
        property real originalY: 0.5

        onActiveChanged: if (dragH.active) {
            dragH.originalX = root.finiteOr(
                root.setting("heroPosX", 0.5), 0.5);
            dragH.originalY = root.finiteOr(
                root.setting("heroPosY", 0.5), 0.5);
        }
        onActiveTranslationChanged: {
            if (!dragH.active)
                return;
            if (heroCrop.overflowX > 1)
                root.editRequested("heroPosX", heroCrop.dragFocal(
                    dragH.originalX, dragH.activeTranslation.x,
                    heroCrop.overflowX));
            if (heroCrop.overflowY > 1)
                root.editRequested("heroPosY", heroCrop.dragFocal(
                    dragH.originalY, dragH.activeTranslation.y,
                    heroCrop.overflowY));
        }
    }

    HoverHandler {
        id: dragHover
        enabled: dragH.enabled
        cursorShape: Qt.SizeAllCursor
    }

    Rectangle {
        visible: dragH.enabled && dragHover.hovered
        anchors.left: parent.left
        anchors.bottom: parent.bottom
        anchors.margins: 8
        width: dragHint.implicitWidth + 16
        height: 22
        radius: Tokens.radius
        color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.72)

        Text {
            id: dragHint
            anchors.centerIn: parent
            text: I18n.tr("DRAG TO REPOSITION")
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 8
            font.weight: Font.DemiBold
            font.letterSpacing: 0.8
        }
    }
}
