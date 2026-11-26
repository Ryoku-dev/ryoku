import QtQuick
import QtQuick.Layouts
import stage.services
import stage.modules.common
import stage.modules.common.widgets
ContentPage {
    id: root
    forceWidth: false

    signal goBack

    RowLayout {
        spacing: 12

        RippleButton {
            implicitWidth: implicitHeight
            implicitHeight: 40
            topLeftRadius: Appearance.rounding.full
            topRightRadius: Appearance.rounding.full
            bottomLeftRadius: Appearance.rounding.full
            bottomRightRadius: Appearance.rounding.full
            colBackground: Appearance.colors.colSecondaryContainer
            colBackgroundHover: Appearance.colors.colSecondaryContainerHover
            colRipple: Appearance.colors.colSecondaryContainerActive

            MaterialSymbol {
                anchors.centerIn: parent
                text: "arrow_back"
                iconSize: Appearance.font.pixelSize.large
                color: Appearance.colors.colOnSecondaryContainer
            }

            onClicked: root.goBack()
        }

        StyledText {
            text: Translation.tr("Search Pill Options")
            font.pixelSize: Appearance.font.pixelSize.large
            font.family: Appearance.font.family.title
            color: Appearance.colors.colOnLayer0
        }
    }

    ContentSection {
        title: Translation.tr("Search Pill Settings")
        icon: "auto_awesome"

        Item {
            Layout.fillWidth: true
            implicitHeight: 250
            visible: !Config.isWidgetActive("search_pill")

            PagePlaceholder {
                anchors.fill: parent
                icon: "auto_awesome"
                shape: MaterialShape.Shape.Circle
                title: Translation.tr("Search Pill disabled")
                description: Translation.tr("Enable Search Pill in Desktop Widgets settings to configure options.")
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 4
            visible: Config.isWidgetActive("search_pill")

            ContentSubsectionLabel { text: Translation.tr("Widget Aspect Ratio & Size") }

            ConfigSelectionArray {
                currentValue: Config.options.background.widgets.search_pill.aspectRatio ?? "0.5x2"
                onSelected: value => Config.options.background.widgets.search_pill.aspectRatio = value
                options: [
                    { displayName: "0.5x2 (Compact)", icon: "crop_landscape", value: "0.5x2" },
                    { displayName: "2x1 (Medium)", icon: "crop_16_9", value: "2x1" },
                    { displayName: "3x1 (Wide)", icon: "crop_free", value: "3x1" },
                    { displayName: "4x1 (Full Width)", icon: "fit_screen", value: "4x1" }
                ]
            }

            ContentSubsectionLabel { text: Translation.tr("Outer Left Icon") }

            ConfigSwitch {
                buttonIcon: "text_fields"
                text: Translation.tr("Use Material Symbol for outer icon")
                checked: Config.options.background.widgets.search_pill.useMaterialSymbolForOuterLeftIcon ?? false
                onCheckedChanged: {
                    Config.options.background.widgets.search_pill.useMaterialSymbolForOuterLeftIcon = checked;
                }
            }

            ConfigTextField {
                id: outerLeftIconField
                text: Translation.tr("Outer icon identifier")
                icon: "image"
                tooltip: Translation.tr("Use a Material Symbol name or an SVG/distro name from the icons folder.")
                placeholderText: Translation.tr("spark, distro, arch...")
                Component.onCompleted: {
                    inputText = Config.options.background.widgets.search_pill.outerLeftIcon ?? "spark";
                }
                textField.onTextChanged: {
                    var value = textField.text.trim();
                    if (value !== "" && textField.activeFocus)
                        Config.options.background.widgets.search_pill.outerLeftIcon = value;
                }
                Connections {
                    target: Config.options.background.widgets.search_pill
                    function onOuterLeftIconChanged() {
                        outerLeftIconField.textField.text = Config.options.background.widgets.search_pill.outerLeftIcon;
                    }
                }
            }

            ContentSubsectionLabel { text: Translation.tr("AI Chat Logo") }

            ConfigSelectionArray {
                currentValue: Config.options.background.widgets.search_pill.aiLogo ?? "symbol"
                onSelected: value => Config.options.background.widgets.search_pill.aiLogo = value
                options: [
                    { displayName: Translation.tr("Symbol"), icon: "neurology", value: "symbol" },
                    { displayName: Translation.tr("Gemini"), icon: "auto_awesome", value: "gemini" },
                    { displayName: Translation.tr("OpenAI"), icon: "neurology", value: "openai" },
                    { displayName: Translation.tr("Claude"), icon: "psychology", value: "claude" },
                    { displayName: Translation.tr("DeepSeek"), icon: "smart_toy", value: "deepseek" },
                    { displayName: Translation.tr("OpenCode"), icon: "code", value: "opencode" },
                    { displayName: Translation.tr("Ollama"), icon: "memory", value: "ollama" },
                    { displayName: Translation.tr("Mistral"), icon: "air", value: "mistral" },
                    { displayName: Translation.tr("OpenRouter"), icon: "route", value: "openrouter" },
                    { displayName: Translation.tr("Antigravity"), icon: "rocket_launch", value: "antigravity" },
                    { displayName: Translation.tr("Arch"), icon: "computer", value: "arch" },
                    { displayName: Translation.tr("CachyOS"), icon: "computer", value: "cachyos" },
                    { displayName: Translation.tr("Debian"), icon: "computer", value: "debian" },
                    { displayName: Translation.tr("EndeavourOS"), icon: "computer", value: "endeavouros" },
                    { displayName: Translation.tr("Fedora"), icon: "computer", value: "fedora" },
                    { displayName: Translation.tr("Gentoo"), icon: "computer", value: "gentoo" },
                    { displayName: Translation.tr("NixOS"), icon: "computer", value: "nixos" },
                    { displayName: Translation.tr("Ubuntu"), icon: "computer", value: "ubuntu" },
                    { displayName: Translation.tr("Linux"), icon: "computer", value: "linux" }
                ]
            }

            ContentSubsectionLabel { text: Translation.tr("Button 1 Action (Left)") }

            ConfigSelectionArray {
                currentValue: Config.options.background.widgets.search_pill.action1 ?? "ai_chat"
                onSelected: value => Config.options.background.widgets.search_pill.action1 = value
                options: [
                    { displayName: Translation.tr("Music Recognition"), icon: "music_note", value: "music_rec" },
                    { displayName: Translation.tr("AI Chat"), icon: "neurology", value: "ai_chat" },
                    { displayName: Translation.tr("Translator"), icon: "translate", value: "translator" },
                    { displayName: Translation.tr("Search Overlay"), icon: "search", value: "search" },
                    { displayName: Translation.tr("Wallpapers"), icon: "wallpaper", value: "wallpapers" },
                    { displayName: Translation.tr("Phone / KDE Connect"), icon: "smartphone", value: "phone" },
                    { displayName: Translation.tr("Kit"), icon: "apps", value: "cheatsheet" },
                    { displayName: Translation.tr("Clipboard"), icon: "content_paste", value: "clipboard" },
                    { displayName: Translation.tr("Color Picker"), icon: "colorize", value: "color_picker" },
                    { displayName: Translation.tr("Screenshot"), icon: "crop", value: "screenshot" }
                ]
            }

            ContentSubsectionLabel { text: Translation.tr("Button 2 Action (Center)") }

            ConfigSelectionArray {
                currentValue: Config.options.background.widgets.search_pill.action2 ?? "music_rec"
                onSelected: value => Config.options.background.widgets.search_pill.action2 = value
                options: [
                    { displayName: Translation.tr("AI Chat"), icon: "neurology", value: "ai_chat" },
                    { displayName: Translation.tr("Music Recognition"), icon: "music_note", value: "music_rec" },
                    { displayName: Translation.tr("Translator"), icon: "translate", value: "translator" },
                    { displayName: Translation.tr("Search Overlay"), icon: "search", value: "search" },
                    { displayName: Translation.tr("Wallpapers"), icon: "wallpaper", value: "wallpapers" },
                    { displayName: Translation.tr("Phone / KDE Connect"), icon: "smartphone", value: "phone" },
                    { displayName: Translation.tr("Kit"), icon: "apps", value: "cheatsheet" },
                    { displayName: Translation.tr("Clipboard"), icon: "content_paste", value: "clipboard" },
                    { displayName: Translation.tr("Color Picker"), icon: "colorize", value: "color_picker" },
                    { displayName: Translation.tr("Screenshot"), icon: "crop", value: "screenshot" }
                ]
            }

            ContentSubsectionLabel { text: Translation.tr("Button 3 Action (Right)") }

            ConfigSelectionArray {
                currentValue: Config.options.background.widgets.search_pill.action3 ?? "search"
                onSelected: value => Config.options.background.widgets.search_pill.action3 = value
                options: [
                    { displayName: Translation.tr("Search Overlay"), icon: "search", value: "search" },
                    { displayName: Translation.tr("AI Chat"), icon: "neurology", value: "ai_chat" },
                    { displayName: Translation.tr("Music Recognition"), icon: "music_note", value: "music_rec" },
                    { displayName: Translation.tr("Translator"), icon: "translate", value: "translator" },
                    { displayName: Translation.tr("Wallpapers"), icon: "wallpaper", value: "wallpapers" },
                    { displayName: Translation.tr("Phone / KDE Connect"), icon: "smartphone", value: "phone" },
                    { displayName: Translation.tr("Kit"), icon: "apps", value: "cheatsheet" },
                    { displayName: Translation.tr("Clipboard"), icon: "content_paste", value: "clipboard" },
                    { displayName: Translation.tr("Color Picker"), icon: "colorize", value: "color_picker" },
                    { displayName: Translation.tr("Screenshot"), icon: "crop", value: "screenshot" }
                ]
            }

            ContentSubsectionLabel { text: Translation.tr("Scale & Appearance") }

            ConfigSlider {
                buttonIcon: "aspect_ratio"
                text: Translation.tr("Widget Scale")
                value: Config.options.background.widgets.search_pill.widgetSize ?? 100
                from: 50
                to: 200
                stepSize: 10
                onValueChanged: Config.options.background.widgets.search_pill.widgetSize = value
            }

            ConfigSwitch {
                buttonIcon: "wb_sunny"
                text: Translation.tr("Enable Shadows")
                checked: Config.options.background.widgets.enableShadows ?? true
                onCheckedChanged: Config.options.background.widgets.enableShadows = checked
            }
        }
    }
}
