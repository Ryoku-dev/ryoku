pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property var bar
    required property Brightness.Monitor monitor
    property color colour: Colours.palette.m3primary

    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real scaleFactor: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int barThickness: Math.round(Tokens.sizes.bar.innerWidth * scaleFactor)
    readonly property int baseThickness: Tokens.sizes.bar.innerWidth
    readonly property int effectiveThickness: rawScale < 1.0 ? baseThickness : barThickness
    readonly property int iconSize: Math.round(effectiveThickness * 0.42)
    readonly property int textSize: Math.round(effectiveThickness * 0.32)

    readonly property string windowTitle: {
        const username = Quickshell.env("USER") || qsTr("User");
        const formattedUser = username.charAt(0).toUpperCase() + username.slice(1);

        const mode = Config.bar.greeter.mode;
        if (mode === "slideshow" && (Config.bar.greeter.slideshowText || "").length > 0) {
            const raw = Config.bar.greeter.slideshowText;
            if (raw.includes("{user}")) {
                return raw.replace(/{user}/g, formattedUser);
            }
            return raw;
        }

        const hr = Time.hours;
        const mStart = Config.bar.greeter.morningStart;
        const aStart = Config.bar.greeter.afternoonStart;
        const eStart = Config.bar.greeter.eveningStart;
        const nStart = Config.bar.greeter.nightStart;

        let msg = Config.bar.greeter.nightText || qsTr("Good Night");
        if (hr >= mStart && hr < aStart) {
            msg = Config.bar.greeter.morningText || qsTr("Good Morning");
        } else if (hr >= aStart && hr < eStart) {
            msg = Config.bar.greeter.afternoonText || qsTr("Good Afternoon");
        } else if (hr >= eStart && hr < nStart) {
            msg = Config.bar.greeter.eveningText || qsTr("Good Evening");
        } else {
            msg = Config.bar.greeter.nightText || qsTr("Good Night");
        }

        if (msg.includes("{user}")) {
            return msg.replace(/{user}/g, formattedUser);
        }
        return `${msg}, ${formattedUser}!`;
    }

    readonly property int maxSize: {
        const otherModules = bar.children.filter(c => c.id && c.item !== this && c.id !== "spacer");
        if (bar.isHorizontal) {
            const otherWidth = otherModules.reduce((acc, curr) => acc + (curr.item.nonAnimWidth ?? curr.width), 0);
            return bar.width - otherWidth - bar.spacing * (bar.children.length - 1) - bar.vPadding * 2;
        } else {
            const otherHeight = otherModules.reduce((acc, curr) => acc + (curr.item.nonAnimHeight ?? curr.height), 0);
            return bar.height - otherHeight - bar.spacing * (bar.children.length - 1) - bar.vPadding * 2;
        }
    }
    property Title current: text1

    clip: true
    implicitWidth: bar.isHorizontal ? (icon.implicitWidth + current.width + current.anchors.leftMargin) : Math.max(icon.implicitWidth, current.width)
    implicitHeight: bar.isHorizontal ? Math.max(icon.implicitHeight, current.height) : (icon.implicitHeight + current.height + current.anchors.topMargin)

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton | Qt.RightButton

        onPositionChanged: {
            if (root.bar.popouts.hasCurrent && root.bar.popouts.currentName === "greetercontext") return;
            const popouts = root.bar.popouts;
            if (popouts.hasCurrent && popouts.currentName !== "greeter" && popouts.currentName !== "activewindow")
                popouts.hasCurrent = false;
        }
        onClicked: mouse => {
            const popouts = root.bar.popouts;
            if (mouse.button === Qt.RightButton) {
                popouts.currentName = "greetercontext";
                popouts.currentCenter = bar.isHorizontal ? root.mapToItem(null, root.implicitWidth / 2, 0).x : (root.mapToItem(null, 0, root.implicitHeight / 2).y ?? 0);
                popouts.hasCurrent = true;
            } else if (mouse.button === Qt.LeftButton) {
                if (!Config.bar.greeter.showOnHover) {
                    if (popouts.hasCurrent && (popouts.currentName === "greeter" || popouts.currentName === "activewindow")) {
                        popouts.hasCurrent = false;
                    } else {
                        popouts.currentName = "greeter";
                        popouts.currentCenter = bar.isHorizontal ? root.mapToItem(null, root.implicitWidth / 2, 0).x : (root.mapToItem(null, 0, root.implicitHeight / 2).y ?? 0);
                        popouts.hasCurrent = true;
                    }
                } else if (popouts.hasCurrent && popouts.currentName === "greetercontext") {
                    popouts.hasCurrent = false;
                }
            }
        }
    }

    MaterialIcon {
        id: icon

        anchors.horizontalCenter: bar.isHorizontal ? undefined : parent.horizontalCenter
        anchors.verticalCenter: bar.isHorizontal ? parent.verticalCenter : undefined

        fontStyle: Tokens.font.icon.builders.small.size(root.iconSize).build()
        animate: true
        text: (Config.bar.greeter.mode === "slideshow" && (Config.bar.greeter.slideshowIcon || "").length > 0)
            ? Config.bar.greeter.slideshowIcon
            : "waving_hand"
        color: root.colour
    }

    Title {
        id: text1
    }

    Title {
        id: text2
    }

    TextMetrics {
        id: metrics

        text: root.windowTitle
        font: Tokens.font.body.builders.small.letterSpacing(1.4).build()

        onTextChanged: {
            const next = root.current === text1 ? text2 : text1;
            next.text = text;
            root.current = next;
        }
    }

    Behavior on implicitHeight {
        enabled: !bar.isHorizontal

        Anim {
            type: Anim.DefaultSpatial
        }
    }

    Behavior on implicitWidth {
        enabled: bar.isHorizontal

        Anim {
            type: Anim.DefaultSpatial
        }
    }

    component Title: Item {
        id: textContainer

        property alias text: styledText.text

        width: bar.isHorizontal ? styledText.implicitWidth : styledText.implicitHeight
        height: bar.isHorizontal ? styledText.implicitHeight : styledText.implicitWidth

        anchors.horizontalCenter: bar.isHorizontal ? undefined : icon.horizontalCenter
        anchors.verticalCenter: bar.isHorizontal ? parent.verticalCenter : undefined
        anchors.top: bar.isHorizontal ? undefined : icon.bottom
        anchors.topMargin: bar.isHorizontal ? 0 : Tokens.spacing.small
        anchors.left: bar.isHorizontal ? icon.right : undefined
        anchors.leftMargin: bar.isHorizontal ? Tokens.spacing.small : 0

        opacity: root.current === this ? 1 : 0

        StyledText {
            id: styledText

            anchors.centerIn: parent

            font.pointSize: metrics.font.pointSize
            font.family: metrics.font.family
            color: root.colour

            rotation: bar.isHorizontal ? 0 : (Config.bar.greeter.inverted ? 270 : 90)
        }

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }
    }
}
