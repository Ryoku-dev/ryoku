import QtQuick
import QtQuick.Layouts
import "../"
import "../reusables"
import Ryoku.Ui.Singletons as Ui

GuideTab {
    id: dockTabRoot
    required property var rootObj
    required property int tabIndex

    function text(key, fallback) {
        const translated = I18n.t(key)
        return translated === key ? fallback : translated
    }

    property var themeSettings: Config.getSetting("theme", ({}))
    property string colorMode: {
        const mode = themeSettings.dockLauncherColorMode;
        return mode === "accent" || mode === "custom" ? mode : "bar";
    }
    property string customColor: {
        const value = themeSettings.dockLauncherCustomColor;
        return typeof value === "string" && /^#[0-9a-fA-F]{6}$/.test(value)
            ? value : "#20242a";
    }

    function setColorMode(mode) {
        const current = Config.getSetting("theme", {});
        current.dockLauncherColorMode = mode;
        if (mode === "custom")
            current.dockLauncherCustomColor = dockTabRoot.customColor;
        Config.setSetting("theme", current);
    }

    function setCustomColor(value) {
        const normalized = String(value).trim();
        if (!/^#[0-9a-fA-F]{6}$/.test(normalized))
            return;
        dockTabRoot.customColor = normalized;
        const current = Config.getSetting("theme", {});
        current.dockLauncherColorMode = "custom";
        current.dockLauncherCustomColor = normalized;
        Config.setSetting("theme", current);
    }

    function refreshThemeSettings() {
        dockTabRoot.themeSettings = Config.getSetting("theme", {});
    }

    Connections {
        target: Config
        function onSettingsLoaded() { dockTabRoot.refreshThemeSettings(); }
        function onRawSettingsChanged() { dockTabRoot.refreshThemeSettings(); }
    }

    anchors.fill: parent
    visible: rootObj.currentTab === tabIndex
    opacity: visible ? 1 : 0

    ColumnLayout {
        anchors.centerIn: parent
        width: Math.min(parent.width - rootObj.s(48), rootObj.s(620))
        spacing: rootObj.s(14)

        Text {
            Layout.fillWidth: true
            text: dockTabRoot.text("guide.dock.colour_heading", "Dock and launcher appearance")
            color: ThemeBackend.text
            font.family: ThemeBackend.fontFamily
            font.pixelSize: rootObj.s(22)
            font.bold: true
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
        }

        Text {
            text: dockTabRoot.text("guide.dock.colour_desc", "Match the active bar by default, use the palette accent, or choose one colour for both surfaces.")
            color: ThemeBackend.subtext0
            font.family: ThemeBackend.fontFamily
            font.pixelSize: rootObj.s(12)
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
        }

        SettingsRow {
            rootObj: dockTabRoot.rootObj
            settingId: "dock_launcher_colour"
            icon: "󰏘"
            title: dockTabRoot.text("guide.dock.colour_title", "Dock and launcher colour")
            description: dockTabRoot.text("guide.dock.colour_hint", "One palette choice keeps the Python dock and launcher together.")
            searchKeywords: "dock launcher color colour palette accent custom bar"

            Dropdown {
                Layout.preferredWidth: rootObj.s(170)
                Layout.preferredHeight: rootObj.s(32)
                options: [
                    dockTabRoot.text("guide.dock.colour_bar", "Follow bar"),
                    dockTabRoot.text("guide.dock.colour_accent", "Accent"),
                    dockTabRoot.text("guide.dock.colour_custom", "Custom")
                ]
                currentIndex: ["bar", "accent", "custom"].indexOf(dockTabRoot.colorMode)
                baseColor: ThemeBackend.surface0
                hoverColor: ThemeBackend.surface1
                dropdownColor: ThemeBackend.surface0
                borderColor: Qt.alpha(ThemeBackend.surface2, 0.6)
                accentColor: ThemeBackend.mauve
                textColor: ThemeBackend.text
                activeTextColor: ThemeBackend.crust
                cornerRadius: ThemeBackend.borderRadius
                fontPixelSize: rootObj.s(11)
                onValueChanged: function(index, value) {
                    const modes = ["bar", "accent", "custom"];
                    if (index >= 0 && index < modes.length)
                        dockTabRoot.setColorMode(modes[index]);
                }
            }

            bottomContent: RowLayout {
                width: parent ? parent.width : implicitWidth
                spacing: rootObj.s(10)
                visible: dockTabRoot.colorMode === "custom"

                Rectangle {
                    Layout.preferredWidth: rootObj.s(28)
                    Layout.preferredHeight: rootObj.s(28)
                    radius: ThemeBackend.borderRadius
                    color: dockTabRoot.customColor
                    border.width: 1
                    border.color: ThemeBackend.dockLauncherText
                }

                Input {
                    Layout.fillWidth: true
                    implicitHeight: rootObj.s(32)
                    text: dockTabRoot.customColor
                    placeholderText: "#20242a"
                    baseColor: ThemeBackend.surface0
                    accentColor: ThemeBackend.mauve
                    textColor: ThemeBackend.text
                    subTextColor: ThemeBackend.subtext0
                    borderColor: Qt.alpha(ThemeBackend.surface2, 0.6)
                    cornerRadius: ThemeBackend.borderRadius
                    fontPixelSize: rootObj.s(11)
                    onAccepted: function(value) { dockTabRoot.setCustomColor(value); }
                }
            }
        }

        Text {
            Layout.fillWidth: true
            text: dockTabRoot.text("guide.dock.stage_desc", "Design, placement, behaviour and pinned apps remain in Stage Editor.")
            color: ThemeBackend.subtext0
            font.family: ThemeBackend.fontFamily
            font.pixelSize: rootObj.s(11)
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.WordWrap
        }

        ClickButton {
            Layout.alignment: Qt.AlignHCenter
            implicitHeight: rootObj.s(40)
            horizontalPadding: rootObj.s(18)
            buttonText: dockTabRoot.text("guide.dock.open_stage", "Open Stage Editor")
            buttonIcon: "󰏘"
            accentColor: ThemeBackend.mauve
            textColor: ThemeBackend.crust
            cornerRadius: ThemeBackend.borderRadius
            onClicked: {
                rootObj.closePopup()
                Ui.Spawn.run(["qs", "-c", "shell", "ipc", "call", "desktop", "editSection", "dock", ""])
            }
        }
    }
}
