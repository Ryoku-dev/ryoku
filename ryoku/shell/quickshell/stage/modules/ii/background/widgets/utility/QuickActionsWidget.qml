import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Qt5Compat.GraphicalEffects
import Ryoku.Ui.Singletons
import stage
import stage.services
import stage.modules.common
import stage.modules.common.widgets
import stage.modules.ii.background.widgets
AbstractBackgroundWidget {
    id: root

    configEntryName: "quick_actions"

    implicitWidth: 240
    implicitHeight: 240

    // Color tokens from WidgetColorScheme
    readonly property color cardBgColor: WidgetColorScheme.cardBgColor
    readonly property color textColorOnBg: WidgetColorScheme.textColorOnBg
    readonly property color subtextColorOnBg: WidgetColorScheme.subtextColorOnBg
    readonly property color accentColor: WidgetColorScheme.accentColor
    readonly property color onAccentColor: WidgetColorScheme.onAccentColor
    readonly property color innerShapeColor: WidgetColorScheme.innerShapeColor

    // Module definitions for configurable bottom buttons
    readonly property var moduleMap: ({
        "translator": {
            "icon": "translate",
            "label": Translation.tr("Translator"),
            "action": function() { Spawn.run(["ryoku-shell", "quicksettings"]); }
        },
        "phone": {
            "icon": "smartphone",
            "label": Translation.tr("Phone"),
            "action": function() { Spawn.runApp(["ryoku-kit", "phone"], { label: "ryoku-kit" }); }
        },
        "wallpapers": {
            "icon": "wallpaper",
            "label": Translation.tr("Wallpapers"),
            "action": function() { Spawn.run(["ryogami", "wallpaper", "ui"]); }
        },
        "media": {
            "icon": "play_circle",
            "label": Translation.tr("Media"),
            "action": function() { Spawn.run(["ryoku-shell", "quicksettings"]); }
        },
        "sidebar_dashboard": {
            "icon": "dashboard",
            "label": Translation.tr("Dashboard"),
            "action": function() { Spawn.run(["ryoku-shell", "quicksettings"]); }
        },
        "cheatsheet": {
            "icon": "apps",
            "label": Translation.tr("Kit"),
            "action": function() { Spawn.runApp(["ryoku-kit"], { label: "ryoku-kit" }); }
        }
    })

    readonly property string configuredButton1: Config.options.background.widgets.quick_actions.bottomButton1 || "translator"
    readonly property string configuredButton2: Config.options.background.widgets.quick_actions.bottomButton2 || "phone"
    readonly property string button1Module: moduleMap[configuredButton1] ? configuredButton1 : "translator"
    readonly property string button2Module: moduleMap[configuredButton2] ? configuredButton2 : "phone"
    readonly property var button1Def: moduleMap[button1Module]
    readonly property var button2Def: moduleMap[button2Module]

    function triggerModule(moduleId) {
        const def = moduleMap[moduleId];
        if (def && def.action) def.action();
    }

    StyledRectangularShadow {
        id: bgShadow
        target: bgRect
        visible: Config.options.background.widgets.enableShadows ?? false
    }

    // Outer Container
    Rectangle {
        id: bgRect
        anchors.fill: parent
        color: WidgetColorScheme.tintBackground(root.cardBgColor)
        topLeftRadius: 48
        topRightRadius: 48
        bottomLeftRadius: Appearance.rounding.windowRounding
        bottomRightRadius: Appearance.rounding.windowRounding

        layer.enabled: Config.options.background.widgets.enableInnerShadow ?? false
        layer.effect: InnerShadow {
            color: Qt.rgba(0, 0, 0, 0.15)
            radius: 8.0
            samples: 16
            horizontalOffset: 0
            verticalOffset: 1
            spread: 0.0
        }

        // Mask container for asymmetric rounding (top=large, bottom=windowRounding)
        Item {
            id: contentContainer
            anchors.fill: parent
            layer.enabled: true
            layer.effect: OpacityMask {
                maskSource: Rectangle {
                    width: contentContainer.width
                    height: contentContainer.height
                    topLeftRadius: Appearance.rounding.large + 36
                    topRightRadius: Appearance.rounding.large + 46
                    bottomLeftRadius: bgRect.radius
                    bottomRightRadius: bgRect.radius
                }
            }

            ColumnLayout {
                anchors.fill: parent
                anchors.margins: 16
                spacing: 16

                // Top: the Alt+Space bar (pill shape, accent color)
                RippleButton {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 64
                    buttonRadius: Appearance.rounding.full
                    colBackground: root.accentColor
                    colBackgroundHover: Qt.darker(root.accentColor, 1.08)
                    colRipple: root.onAccentColor

                    RowLayout {
                        anchors.centerIn: parent
                        spacing: 10

                        CustomIcon {
                            width: 22
                            height: 22
                            source: "spark-symbolic.svg"
                            colorize: true
                            color: root.onAccentColor
                        }

                        StyledText {
                            text: Translation.tr("Search and downloads")
                            font.pixelSize: Appearance.font.pixelSize.large
                            font.weight: Font.Medium
                            color: root.onAccentColor
                        }
                    }

                    onClicked: Spawn.run(["ryoku-shell", "ask"])
                }

                // Bottom row: two configurable action buttons
                RowLayout {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    spacing: 12

                    // Button 1
                    RippleButton {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        buttonRadius: Appearance.rounding.normal
                        colBackground: root.innerShapeColor
                        colBackgroundHover: Qt.darker(root.innerShapeColor, 1.1)
                        colRipple: Qt.darker(root.innerShapeColor, 1.2)

                        ColumnLayout {
                            anchors.centerIn: parent
                            spacing: 4

                            MaterialSymbol {
                                Layout.alignment: Qt.AlignHCenter
                                text: root.button1Def.icon
                                iconSize: 22
                                fill: 1
                                color: root.textColorOnBg
                            }

                            StyledText {
                                Layout.alignment: Qt.AlignHCenter
                                text: root.button1Def.label
                                font.pixelSize: Appearance.font.pixelSize.small
                                color: root.subtextColorOnBg
                            }
                        }

                        onClicked: root.triggerModule(root.button1Module)
                    }

                    // Button 2
                    RippleButton {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        buttonRadius: Appearance.rounding.normal
                        colBackground: root.innerShapeColor
                        colBackgroundHover: Qt.darker(root.innerShapeColor, 1.1)
                        colRipple: Qt.darker(root.innerShapeColor, 1.2)

                        ColumnLayout {
                            anchors.centerIn: parent
                            spacing: 4

                            MaterialSymbol {
                                Layout.alignment: Qt.AlignHCenter
                                text: root.button2Def.icon
                                iconSize: 22
                                fill: 1
                                color: root.textColorOnBg
                            }

                            StyledText {
                                Layout.alignment: Qt.AlignHCenter
                                text: root.button2Def.label
                                font.pixelSize: Appearance.font.pixelSize.small
                                color: root.subtextColorOnBg
                            }
                        }

                        onClicked: root.triggerModule(root.button2Module)
                    }
                }
            }
        }
    }
}
