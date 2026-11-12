import QtQuick
import QtQuick.Layouts
import stage
import stage.services
import stage.modules.common
import stage.modules.common.widgets
import Ryoku.Ui as Ui
import Ryoku.Ui.Singletons

StyledFlickable {
    id: root

    contentHeight: column.implicitHeight
    clip: true

    signal fieldFocusRequested(Item field)
    signal fieldFocusReleased()

    function schemeName(value) {
        const name = String(value).replace(/^scheme-/, "").replace(/-/g, " ");
        return name.charAt(0).toUpperCase() + name.slice(1);
    }

    readonly property string colourSummary: MaterialThemeLoader.followsWallpaper
        ? root.schemeName(MaterialThemeLoader.schemeType)
            + " · " + Translation.tr("source %1")
                .arg(String(MaterialThemeLoader.sourceColorIndex + 1))
        : MaterialThemeLoader.themeName

    ColumnLayout {
        id: column
        width: root.width
        spacing: Tokens.s3

        EditStylePresets {
            Layout.fillWidth: true
            onFieldFocusRequested: field => root.fieldFocusRequested(field)
            onFieldFocusReleased: root.fieldFocusReleased()
        }

        // Colours have one home, Ryogami's Colours tab; this row reports them and hands off.
        Ui.SettingCard {
            Layout.fillWidth: true
            title: Translation.tr("COLOURS")
            collapsible: false

            Ui.SettingRow {
                width: parent.width
                label: Translation.tr("Wallpaper and colours")
                desc: Translation.tr("%1. Mode, scheme and app colours live in the picker.")
                    .arg(root.colourSummary)
                controlWidth: openButton.implicitWidth

                Ui.Btn {
                    id: openButton
                    anchors.fill: parent
                    text: Translation.tr("Open colours")
                    compact: true
                    onAct: {
                        Spawn.run(["ryogami", "wallpaper", "settings", "colours"]);
                        if (!GlobalStates.editGuideActive)
                            GlobalStates.editMode = false;
                    }
                }
            }
        }

        EditPanelNotice {
            Layout.fillWidth: true
            visible: MaterialThemeLoader.lastError !== ""
            symbol: "error"
            text: MaterialThemeLoader.lastError
        }

        Item {
            Layout.fillWidth: true
            implicitHeight: Tokens.s2
        }
    }
}
