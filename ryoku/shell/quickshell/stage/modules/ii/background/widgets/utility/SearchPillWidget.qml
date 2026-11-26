pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import stage
import Ryoku.Ui.Singletons
import stage.services
import stage.modules.common
import stage.modules.common.functions
import stage.modules.common.widgets
import stage.modules.ii.background.widgets
AbstractBackgroundWidget {
    id: root

    configEntryName: "search_pill"

    visibleWhenLocked: root.lockBehavior === "keep" || root.lockBehavior === "custom"
                    || root.lockBehavior === "center"
                    || root.lockBehavior === "lockOnly"
                    || (Config.options.lock.centerWidget === "search_pill")

    opacity: {
        if (root.lockBehavior === "lockOnly")
            return GlobalStates.screenLocked ? 1 : 0;
        if (GlobalStates.screenLocked && !visibleWhenLocked)
            return 0;
        return 1;
    }

    readonly property real contentScale: (Config.options.background.widgets.search_pill.widgetSize ?? 100) / 100.0
    readonly property string aspectRatio: Config.options.background.widgets.search_pill.aspectRatio ?? "0.5x2"
    readonly property var options: Config.options.background.widgets.search_pill
    readonly property string action1Key: options.action1 ?? "ai_chat"
    readonly property string action2Key: options.action2 ?? "music_rec"
    readonly property string action3Key: options.action3 ?? "search"
    readonly property string aiLogo: options.aiLogo ?? "symbol"
    readonly property string outerLeftIcon: options.outerLeftIcon ?? "spark"
    readonly property bool useMaterialSymbolForOuterLeftIcon: options.useMaterialSymbolForOuterLeftIcon ?? false
    readonly property string outerLeftIconSource: {
        if (root.outerLeftIcon === "distro") return SystemInfo.distroIcon;
        if (root.outerLeftIcon.endsWith(".svg")) return root.outerLeftIcon;
        return root.outerLeftIcon + "-symbolic";
    }
    readonly property list<string> actionKeys: [action1Key, action2Key, action3Key]

    readonly property color colOuterBg: WidgetColorScheme.cardBgColor
    readonly property color colOuterText: WidgetColorScheme.textColorOnBg
    readonly property color colInnerBg: WidgetColorScheme.pillBgColor
    readonly property color colIconText: WidgetColorScheme.textColorOnPillTrack
    readonly property real actionDiameter: 48 * root.contentScale

    implicitWidth: {
        if (root.aspectRatio === "4x1") return 960 * root.contentScale;
        if (root.aspectRatio === "3x1") return 720 * root.contentScale;
        if (root.aspectRatio === "2x1") return 480 * root.contentScale;
        return 340 * root.contentScale;
    }
    implicitHeight: 60 * root.contentScale

    readonly property string activeAiLogo: root.aiLogo === "auto" ? "symbol" : root.aiLogo

    function getAiIconAsset() {
        const logo = root.activeAiLogo;
        if (logo === "gemini") return "google-gemini-symbolic.svg";
        if (logo === "openai") return "ai-openai-symbolic.svg";
        if (logo === "claude") return "bootstrap_claude.svg";
        if (logo === "deepseek") return "deepseek-symbolic.svg";
        if (logo === "opencode") return "opencode-logo-light.svg";
        if (logo === "ollama") return "ollama-symbolic.svg";
        if (logo === "mistral") return "mistral-symbolic.svg";
        if (logo === "openrouter") return "openrouter-symbolic.svg";
        if (logo === "antigravity") return "material-symbols_antigravity.svg";
        if (logo === "arch") return "arch-symbolic.svg";
        if (logo === "cachyos") return "cachyos-symbolic.svg";
        if (logo === "debian") return "debian-symbolic.svg";
        if (logo === "endeavouros") return "endeavouros-symbolic.svg";
        if (logo === "fedora") return "fedora-symbolic.svg";
        if (logo === "gentoo") return "gentoo-symbolic.svg";
        if (logo === "nixos") return "nixos-symbolic.svg";
        if (logo === "ubuntu") return "ubuntu-symbolic.svg";
        if (logo === "linux") return "linux-symbolic.svg";
        return "";
    }

    function getActionDetails(key) {
        if (key === "ai_chat") return {
            defaultIcon: "neurology",
            iconAsset: root.getAiIconAsset(),
            label: Translation.tr("Search and downloads")
        };
        if (key === "music_rec") return { defaultIcon: "music_note", iconAsset: "", label: Translation.tr("Music Recognition") };
        if (key === "translator") return { defaultIcon: "translate", iconAsset: "", label: Translation.tr("Translator") };
        if (key === "wallpapers") return { defaultIcon: "wallpaper", iconAsset: "", label: Translation.tr("Wallpapers") };
        if (key === "phone") return { defaultIcon: "smartphone", iconAsset: "", label: Translation.tr("Phone / KDE Connect") };
        if (key === "cheatsheet") return { defaultIcon: "apps", iconAsset: "", label: Translation.tr("Kit") };
        if (key === "clipboard") return { defaultIcon: "content_paste", iconAsset: "", label: Translation.tr("Clipboard") };
        if (key === "color_picker") return { defaultIcon: "colorize", iconAsset: "", label: Translation.tr("Color Picker") };
        if (key === "screenshot") return { defaultIcon: "crop", iconAsset: "", label: Translation.tr("Screenshot") };
        return { defaultIcon: "search", iconAsset: "", label: Translation.tr("Search") };
    }

    function runAction(key) {
        if (key === "search") {
            Spawn.run(["ryoku-shell", "launcher"]);
        } else if (key === "music_rec") {
            Spawn.run(["ryoku-cmd-songrec"]);
        } else if (key === "ai_chat") {
            Spawn.run(["ryoku-shell", "ask"]);
        } else if (key === "translator") {
            Spawn.run(["ryoku-shell", "quicksettings"]);
        } else if (key === "wallpapers") {
            Spawn.run(["ryogami", "wallpaper", "ui"]);
        } else if (key === "phone") {
            Spawn.runApp(["ryoku-kit", "phone"], { label: "ryoku-kit" });
        } else if (key === "cheatsheet") {
            Spawn.runApp(["ryoku-kit"], { label: "ryoku-kit" });
        } else if (key === "clipboard") {
            Spawn.run(["ryoku-shell", "clipboard"]);
        } else if (key === "color_picker") {
            Spawn.run(["ryoku-cmd-color-picker"]);
        } else if (key === "screenshot") {
            Spawn.run(["ryoku-cmd-ryoshot"]);
        } else {
            Spawn.run(["ryoku-shell", "launcher"]);
        }
    }


    StyledDropShadow {
        target: outerCapsule
        visible: Config.options.background.widgets.enableShadows ?? true
    }

    Rectangle {
        id: outerCapsule
        anchors.fill: parent
        radius: Appearance.rounding.full
        color: WidgetColorScheme.tintBackground(root.colOuterBg)

        RowLayout {
            anchors.fill: parent
            anchors.margins: 4 * root.contentScale
            spacing: 4 * root.contentScale

            RippleButton {
                Layout.preferredWidth: root.actionDiameter
                Layout.minimumWidth: root.actionDiameter
                Layout.maximumWidth: root.actionDiameter
                Layout.preferredHeight: root.actionDiameter
                Layout.minimumHeight: root.actionDiameter
                Layout.maximumHeight: root.actionDiameter
                Layout.alignment: Qt.AlignVCenter
                buttonRadius: Appearance.rounding.full
                colBackground: "transparent"
                colBackgroundHover: ColorUtils.applyAlpha(root.colOuterText, 0.12)
                colRipple: ColorUtils.applyAlpha(root.colOuterText, 0.24)

                CustomIcon {
                    anchors.centerIn: parent
                    visible: !root.useMaterialSymbolForOuterLeftIcon
                    width: Math.min(parent.height * 0.62, 42 * root.contentScale)
                    height: width
                    source: root.outerLeftIconSource
                    colorize: true
                    color: root.colOuterText
                }

                MaterialSymbol {
                    anchors.centerIn: parent
                    visible: root.useMaterialSymbolForOuterLeftIcon
                    text: root.outerLeftIcon
                    fill: 1
                    iconSize: Math.round(Math.min(parent.height * 0.62, 42 * root.contentScale))
                    color: root.colOuterText
                }

                onClicked: Spawn.run(["ryoku-shell", "launcher"])
            }

            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true
            }

            Item {
                id: innerSlot
                Layout.fillHeight: true
                Layout.preferredWidth: (root.actionDiameter * 3) + (root.contentScale * 20)
                Layout.minimumWidth: (root.actionDiameter * 3) + (root.contentScale * 20)
                Layout.maximumWidth: (root.actionDiameter * 3) + (root.contentScale * 20)

                Rectangle {
                    id: innerCapsule
                    anchors.fill: parent
                    radius: Appearance.rounding.full
                    color: WidgetColorScheme.tintBackground(root.colInnerBg)
                    clip: true

                    Row {
                        anchors.fill: parent

                        Repeater {
                            model: root.actionKeys

                            delegate: Item {
                                required property string modelData
                                readonly property bool isAiAction: modelData === "ai_chat"
                                readonly property var actionDetails: root.getActionDetails(modelData)
                                readonly property string resolvedIconAsset: isAiAction ? root.getAiIconAsset() : ""
                                readonly property string resolvedDefaultIcon: isAiAction ? "neurology" : actionDetails.defaultIcon

                                width: innerCapsule.width / 3
                                height: innerCapsule.height

                                RippleButton {
                                    anchors.centerIn: parent
                                    width: Math.min(parent.height - (8 * root.contentScale), parent.width - (8 * root.contentScale))
                                    height: width
                                    buttonRadius: Appearance.rounding.full
                                    colBackground: "transparent"
                                    colBackgroundHover: ColorUtils.applyAlpha(root.colIconText, 0.14)
                                    colRipple: ColorUtils.applyAlpha(root.colIconText, 0.26)

                                    CustomIcon {
                                        anchors.centerIn: parent
                                        visible: resolvedIconAsset !== ""
                                        width: parent.width * 0.58
                                        height: width
                                        source: resolvedIconAsset || "spark-symbolic.svg"
                                        colorize: true
                                        color: root.colIconText
                                    }

                                    MaterialSymbol {
                                        anchors.centerIn: parent
                                        visible: resolvedIconAsset === ""
                                        text: resolvedDefaultIcon
                                        fill: 1
                                        iconSize: Math.round(parent.width * 0.58)
                                        color: root.colIconText
                                    }

                                    onClicked: root.runAction(modelData)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
