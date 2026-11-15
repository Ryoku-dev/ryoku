import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the colours Ryoku's surfaces wear now, side by side with their roles: Ryoku's own on its
// colours, the desktop's scheme (the wallpaper's, or a fixed one) on the wallpaper's. The
// bars, terminal and apps wear the desktop's scheme either way, so that is said too
Column {
    id: root

    readonly property bool ryoku: {
        Changes.revision;
        return Changes.value({ store: "settings", key: "theme.colours", fallback: "ryoku" }) !== "wallpaper";
    }
    readonly property var desktop: Lithic.desktopRoles
    readonly property var roles: root.desktop.roles
    readonly property var swatches: root.ryoku
        ? [
            { fill: Lithic.sel, label: "Accent" },
            { fill: Lithic.ground, label: "Paper" },
            { fill: Lithic.stone4, label: "Raised" },
            { fill: Lithic.fg, label: "Ink" },
            { fill: Lithic.fg3, label: "Quiet ink" },
            { fill: Lithic.jasper, label: "Error" }
        ]
        : [
            { key: "primary", label: "Primary" },
            { key: "secondary", label: "Secondary" },
            { key: "tertiary", label: "Tertiary" },
            { key: "primaryContainer", label: "Container" },
            { key: "surfaceContainerHigh", label: "Surface" },
            { key: "error", label: "Error" }
        ].filter(s => typeof root.roles[s.key] === "string").map(s => ({ fill: root.roles[s.key], label: s.label }))

    spacing: 10

    Label {
        width: root.width
        text: root.ryoku
            ? I18n.tr("Ryoku's own colours, in light or dark. Your wallpaper's scheme still colours the bars, terminal and apps.")
            : root.desktop.source === "scheme" ? I18n.tr("A fixed colour scheme, the same whatever the wallpaper.")
            : root.desktop.source === "wallpaper" ? I18n.tr("Drawn from your wallpaper, and redrawn when it changes.")
            : I18n.tr("No desktop palette yet; it appears once a wallpaper is set.")
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
                    fill: modelData.fill
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
