import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the desktop's colour scheme as the round designs wear it: where it comes from, then its
// key colours side by side with their roles, read live from the palette files
Column {
    id: root

    readonly property var desktop: Lithic.desktopRoles
    readonly property var roles: root.desktop.roles
    readonly property var swatches: [
        { key: "primary", label: "Primary" },
        { key: "secondary", label: "Secondary" },
        { key: "tertiary", label: "Tertiary" },
        { key: "primaryContainer", label: "Container" },
        { key: "surfaceContainerHigh", label: "Surface" },
        { key: "error", label: "Error" }
    ].filter(s => typeof root.roles[s.key] === "string")

    spacing: 10

    Label {
        width: root.width
        text: root.desktop.source === "scheme" ? I18n.tr("A fixed colour scheme, the same whatever the wallpaper.")
            : root.desktop.source === "wallpaper" ? I18n.tr("Drawn from your wallpaper, and redrawn when it changes.")
            : I18n.tr("No desktop palette yet, so the round designs wear Ryoku's own colours.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        visible: root.swatches.length > 0
        spacing: 8

        Repeater {
            model: root.swatches
            delegate: Column {
                required property var modelData
                spacing: 4
                CutRect {
                    width: 64
                    height: 40
                    cut: Tone.rowCut
                    fill: root.roles[modelData.key]
                    strokeWidth: 1
                    strokeColor: Tone.line
                    Behavior on fill { ColorAnimation { duration: Tone.move } }
                }
                Label {
                    width: 64
                    text: I18n.tr(modelData.label)
                    typeRole: "caption"
                    inkRole: "fg3"
                }
            }
        }
    }
}
