pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui.Singletons
import shell.services

QQC.Popup {
    id: root

    required property real s
    required property bool active
    signal openExtensions()
    signal requestClose()

    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    readonly property var pluginCards: pluginHost.cards()

    width: (Tokens.railW + Tokens.s6) * s
    padding: Tokens.s4 * s
    modal: false
    focus: true
    transformOrigin: Item.BottomRight
    closePolicy: QQC.Popup.CloseOnEscape | QQC.Popup.CloseOnPressOutside

    function openHub(section): void {
        root.close();
        root.requestClose();
        Spawn.run(["ryoku-shell", "hub", "open", section]);
    }

    SidebarPlugins {
        id: pluginHost
        active: root.active && SidebarState.elementVisible("pluginCards")
    }

    background: Rectangle {
        radius: Tokens.radius * root.s * 1.5
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
    }

    contentItem: Column {
        id: content
        spacing: Tokens.s2 * root.s

        Text {
            width: parent.width
            text: I18n.tr("Motion")
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro * root.s
            font.capitalization: Font.AllUppercase
            font.letterSpacing: Tokens.trackLabel
        }

        SidebarSegments {
            width: parent.width
            s: root.s
            options: ["quick", "standard", "calm"]
            labels: ({ "quick": "Quick", "standard": "Standard", "calm": "Calm" })
            current: SidebarState.speed
            onChose: key => SidebarState.setSpeed(key)
        }

        Text {
            width: parent.width
            text: I18n.tr("Open, close, and section reveal speed")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            wrapMode: Text.WordWrap
        }

        Item { width: parent.width; height: Tokens.s1 * root.s }

        Text {
            width: parent.width
            visible: extensionRow.visible
            text: I18n.tr("EXTENSIONS")
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro * root.s
            font.capitalization: Font.AllUppercase
            font.letterSpacing: Tokens.trackLabel
        }

        QQC.AbstractButton {
            id: extensionRow
            width: parent.width
            implicitHeight: (Tokens.ctlH + Tokens.s2) * root.s
            visible: SidebarState.elementVisible("pluginCards") && root.pluginCards.length > 0
            hoverEnabled: true
            Accessible.name: I18n.tr("Extensions")
            onClicked: {
                root.close();
                root.openExtensions();
            }
            background: Rectangle {
                radius: Tokens.radius * root.s
                color: extensionRow.down ? Tokens.tint16 : extensionRow.hovered ? Tokens.tint10 : Tokens.tint5
                border.width: Tokens.border
                border.color: extensionRow.visualFocus ? Tokens.sun : Tokens.lineSoft
                Behavior on color {
                    enabled: root.motionAllowed
                    ColorAnimation { duration: Tokens.snap }
                }
            }
            contentItem: Item {
                Text {
                    id: extensionIcon
                    anchors.left: parent.left
                    anchors.leftMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: "extension"
                    color: Tokens.inkDim
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: (Tokens.fBody + Tokens.s1) * root.s
                    Accessible.ignored: true
                }
                Text {
                    id: extensionArrow
                    anchors.right: parent.right
                    anchors.rightMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: "arrow_forward"
                    color: Tokens.inkMuted
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: (Tokens.fBody + Tokens.s1) * root.s
                    Accessible.ignored: true
                }
                Text {
                    anchors.left: extensionIcon.right
                    anchors.right: extensionArrow.left
                    anchors.leftMargin: Tokens.s2 * root.s
                    anchors.rightMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Manage extensions")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }

        Item { width: parent.width; height: extensionRow.visible ? Tokens.s1 * root.s : 0 }

        Text {
            width: parent.width
            text: I18n.tr("OPEN IN RYOKU HUB")
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro * root.s
            font.capitalization: Font.AllUppercase
            font.letterSpacing: Tokens.trackLabel
        }

        Repeater {
            model: [
                { label: "Controls", glyph: "tune", section: "controls" },
                { label: "Displays", glyph: "display_settings", section: "displays" },
                { label: "Sound", glyph: "volume_up", section: "connections" },
                { label: "Network", glyph: "wifi", section: "connections" },
                { label: "Graphics & Power", glyph: "memory", section: "gpu" },
                { label: "Keybinds", glyph: "keyboard", section: "keybinds" }
            ]
            delegate: QQC.AbstractButton {
                id: hubRow
                required property var modelData
                width: content.width
                implicitHeight: (Tokens.ctlH + Tokens.s1) * root.s
                hoverEnabled: true
                Accessible.name: I18n.tr(modelData.label)
                onClicked: root.openHub(modelData.section)
                background: Rectangle {
                    radius: Tokens.radius * root.s
                    color: hubRow.down ? Tokens.tint16 : hubRow.hovered ? Tokens.tint10 : "transparent"
                    border.width: Tokens.border
                    border.color: hubRow.visualFocus ? Tokens.sun : "transparent"
                    Behavior on color {
                        enabled: root.motionAllowed
                        ColorAnimation { duration: Tokens.snap }
                    }
                }
                contentItem: Item {
                    Text {
                        id: hubIcon
                        anchors.left: parent.left
                        anchors.leftMargin: Tokens.s2 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: hubRow.modelData.glyph
                        color: Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: (Tokens.fBody + Tokens.s1) * root.s
                        Accessible.ignored: true
                    }
                    Text {
                        id: hubArrow
                        anchors.right: parent.right
                        anchors.rightMargin: Tokens.s2 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: "north_east"
                        color: Tokens.inkFaint
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: (Tokens.fSmall + Tokens.s1) * root.s
                        Accessible.ignored: true
                    }
                    Text {
                        anchors.left: hubIcon.right
                        anchors.right: hubArrow.left
                        anchors.leftMargin: Tokens.s2 * root.s
                        anchors.rightMargin: Tokens.s2 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr(hubRow.modelData.label)
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        elide: Text.ElideRight
                    }
                }
                HoverHandler { cursorShape: Qt.PointingHandCursor }
            }
        }
    }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation {
                property: "opacity"
                from: 0
                to: 1
                duration: SidebarState.motionDuration(Tokens.swap)
                easing.type: Tokens.ease
            }
            NumberAnimation {
                property: "scale"
                from: 0.94
                to: 1
                duration: SidebarState.motionDuration(Tokens.swap)
                easing.type: Tokens.ease
            }
        }
    }
    exit: Transition {
        ParallelAnimation {
            NumberAnimation {
                property: "opacity"
                from: 1
                to: 0
                duration: SidebarState.motionDuration(Tokens.snap)
                easing.type: Tokens.easeSnap
            }
            NumberAnimation {
                property: "scale"
                from: 1
                to: 0.96
                duration: SidebarState.motionDuration(Tokens.snap)
                easing.type: Tokens.easeSnap
            }
        }
    }
}
