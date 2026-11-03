pragma ComponentBehavior: Bound

import QtQuick
import qs.services
import qs.modules.common.widgets
import qs.modules.iris.style
import qs.modules.iris.frame
import qs.modules.iris.components
import qs.modules.iris.background
import qs.modules.iris.field as Field
import qs.modules.iris.preview.parts

// Texture is what the material is drawn as, so this shows three real bodies (the Island, a card and a panel) in one
// IrisField pass: Afterglow's key light, chrome bevel, bloom past the silhouette and signal come from the same shader the
// shell uses, and every row of the group moves them. "Grade the wallpaper" puts the real IrisAfterglowWallpaper over the
// preview's own decoded wallpaper (no second decode). Solid leaves both plain, so the difference is the point.
PreviewScene {
    id: tex
    // Laid tight, so the bodies and the glow between them read at the head's size.
    readonly property real naturalWidth: Math.round(420 * tex.d)
    readonly property real naturalHeight: Math.round(210 * tex.d)
    readonly property bool lit: IrisStyle.afterglow
    readonly property string gradeLabel: Translation.tr(({ dusk: "Dusk", cyber: "Cyber", fog: "Fog", wallpaper: "Wallpaper" })[IrisStyle.afterglowGrade] ?? "Dusk")

    readonly property real side: Math.round(20 * tex.d)
    readonly property real gap: Math.round(16 * tex.d)
    readonly property real bodyHeight: Math.round(92 * tex.d)
    readonly property var island: ({ x: Math.round((tex.width - 150 * tex.d) / 2), y: Math.round(14 * tex.d), width: Math.round(150 * tex.d), height: IrisFrame.islandBand })
    readonly property real rowY: tex.island.y + tex.island.height + tex.gap
    readonly property var card: ({ x: tex.side, y: tex.rowY, width: Math.round(204 * tex.d), height: tex.bodyHeight })
    readonly property var panel: ({ x: tex.card.x + tex.card.width + tex.gap, y: tex.rowY, width: tex.width - 2 * tex.side - tex.card.width - tex.gap, height: tex.bodyHeight })

    // The grade over the wallpaper, exactly as the desktop and the lock lay it. The filter hides its source, which is
    // this preview's wallpaper, so the graded copy stands in for it only while the grade is on.
    Loader {
        anchors.fill: parent
        active: IrisStyle.afterglowWallpaper
        sourceComponent: IrisAfterglowWallpaper {
            source: tex.wallpaperView
            live: false
        }
    }

    Field.IrisField {
        anchors.fill: parent
        framed: false
        shapes: [
            { id: "island", x: tex.island.x, y: tex.island.y, width: tex.island.width, height: tex.island.height,
                radius: tex.island.height / 2, fuse: IrisStyle.fuse, paints: true },
            { id: "card", x: tex.card.x, y: tex.card.y, width: tex.card.width, height: tex.card.height,
                radius: IrisStyle.radiusPlate, fuse: IrisStyle.fuse, paints: true },
            { id: "panel", x: tex.panel.x, y: tex.panel.y, width: tex.panel.width, height: tex.panel.height,
                radius: IrisStyle.radiusPlate, fuse: IrisStyle.fuse, paints: true }
        ]
    }

    IrisClock {
        x: tex.island.x + Math.round((tex.island.width - width) / 2)
        y: tex.island.y + Math.round((tex.island.height - height) / 2)
        pixelSize: IrisStyle.typeHeadline
        separatorColor: IrisStyle.secondaryAccent
    }

    Column {
        id: cardContent
        readonly property int pad: IrisStyle.concentricPad(IrisStyle.radiusPlate, 14 * tex.d)
        x: tex.card.x + cardContent.pad
        y: tex.card.y + Math.round((tex.card.height - height) / 2)
        width: tex.card.width - 2 * cardContent.pad
        spacing: Math.round(12 * tex.d)
        Row {
            spacing: Math.round(10 * tex.d)
            Rectangle {
                width: Math.round(36 * tex.d); height: width
                radius: IrisStyle.iconRadius(width); color: IrisStyle.fill
                MaterialSymbol { anchors.centerIn: parent; text: "music_note"; fill: 1; iconSize: Math.round(18 * tex.d); color: IrisStyle.textSecondary }
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                IrisText { text: Translation.tr("Now playing"); font.weight: IrisStyle.weight(Font.DemiBold); font.pixelSize: IrisStyle.typeLabel }
                IrisText { text: Translation.tr("Artist"); color: IrisStyle.muted; font.pixelSize: IrisStyle.typeMeta }
            }
        }
        Rectangle {
            width: parent.width; height: Math.max(2, Math.round(4 * tex.d)); radius: height / 2
            color: IrisStyle.fill
            Rectangle { width: parent.width * 0.42; height: parent.height; radius: parent.radius; color: IrisStyle.accent }
        }
    }

    Grid {
        id: panelContent
        readonly property int pad: IrisStyle.concentricPad(IrisStyle.radiusPlate, 14 * tex.d)
        readonly property real cell: Math.floor((tex.panel.width - 2 * panelContent.pad - panelContent.spacing) / 2)
        x: tex.panel.x + panelContent.pad
        y: tex.panel.y + Math.round((tex.panel.height - height) / 2)
        columns: 2
        spacing: Math.round(8 * tex.d)
        Repeater {
            model: ["wifi", "bluetooth", "dark_mode", "do_not_disturb_on"]
            Rectangle {
                id: tile
                required property int index
                required property string modelData
                width: panelContent.cell
                height: Math.round(34 * tex.d)
                radius: IrisStyle.radiusTile
                color: tile.index === 0 ? IrisStyle.tintFill(IrisStyle.accent) : IrisStyle.fill
                MaterialSymbol {
                    anchors.centerIn: parent
                    text: tile.modelData; fill: 1; iconSize: Math.round(18 * tex.d)
                    color: tile.index === 0 ? IrisStyle.accent : IrisStyle.textSecondary
                }
            }
        }
    }

    Caption {
        glyph: tex.lit ? "flare" : "crop_square"
        text: tex.lit ? Translation.tr("Afterglow") + " · " + tex.gradeLabel : Translation.tr("Solid")
    }
}
