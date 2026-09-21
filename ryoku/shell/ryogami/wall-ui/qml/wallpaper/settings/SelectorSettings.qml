import QtQuick
import ".."
import "../.."
import "../../components"
import Ryoku.Ui.Singletons

Flow {
    id: root
    property var colors
    property var saveField
    property var saveConfigKey
    property var showWarning
    property var applyPreset
    property var saveCustomPreset
    property var loadCustomPreset

    width: parent ? parent.width : 0
    spacing: 8

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Layout")
        width: parent.width

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Display mode")
            description: I18n.tr("Slices, Hex grid, Wall grid, or Mosaic.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "slices",  label: I18n.tr("Slices") },
                        { key: "hex",     label: I18n.tr("Hex") },
                        { key: "wall",    label: I18n.tr("Wall") },
                        { key: "mosaic",  label: I18n.tr("Mosaic") }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: I18n.tr(modelData.label)
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.displayMode === modelData.key
                        onClicked: {
                            if (modelData.key === "mosaic" && Config.displayMode !== "mosaic" && root.showWarning)
                                root.showWarning(I18n.tr("MOSAIC IS EXPERIMENTAL"), I18n.tr("Not all features work yet. Please do not expect everything to function correctly."))
                            if (root.saveField) root.saveField("displayMode", modelData.key)
                        }
                    }
                }
            }
        }

        SettingsRow {
            visible: Config.displayMode === "slices"
            colors: root.colors
            title: I18n.tr("Size preset")
            description: I18n.tr("Pick a quick slice size.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { label: "XS", expanded: 360,  sliceH: 200, sliceW: 52,  visible: 20, gap: -30, skew: 16 },
                        { label: "S",  expanded: 480,  sliceH: 270, sliceW: 68,  visible: 18, gap: -30, skew: 20 },
                        { label: "M",  expanded: 768,  sliceH: 432, sliceW: 108, visible: 14, gap: -30, skew: 28 },
                        { label: "L",  expanded: 924,  sliceH: 520, sliceW: 135, visible: 12, gap: -30, skew: 35 },
                        { label: "XL", expanded: 1280, sliceH: 720, sliceW: 180, visible: 9,  gap: -30, skew: 45 }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: I18n.tr(modelData.label)
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.wallpaperExpandedWidth === modelData.expanded && Config.wallpaperSliceHeight === modelData.sliceH
                        onClicked: if (root.applyPreset) root.applyPreset(modelData.expanded, modelData.sliceH, modelData.sliceW, modelData.visible, modelData.gap, modelData.skew)
                        tooltip: modelData.expanded + "×" + modelData.sliceH + " (16:9)"
                    }
                }
            }
        }

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Custom presets")
            description: I18n.tr("Click to apply, right-click an empty slot to save the current geometry.")
            Row {
                spacing: 4
                Repeater {
                    model: ["C1", "C2", "C3", "C4"]
                    FilterButton {
                        property string presetKey: modelData + "_" + Config.displayMode
                        property var presetData: Config.wallpaperCustomPresets[presetKey] || null
                        property bool isEmpty: !presetData
                        colors: root.colors
                        label: modelData
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: {
                            if (isEmpty) return false
                            if (Config.displayMode === "slices") return Config.wallpaperExpandedWidth === presetData.expandedWidth && Config.wallpaperSliceHeight === presetData.sliceHeight
                            if (Config.displayMode === "hex")    return Config.hexRadius === presetData.hexRadius && Config.hexRows === presetData.hexRows && Config.hexCols === presetData.hexCols
                            if (Config.displayMode === "wall")   return Config.gridColumns === presetData.gridColumns && Config.gridRows === presetData.gridRows
                            return false
                        }
                        activeOpacity: isEmpty ? 0.35 : 1.0
                        tooltip: {
                            if (isEmpty) return I18n.tr("Click to save current")
                            if (Config.displayMode === "slices") return I18n.tr("%1×%2 - Right-click to overwrite").arg(presetData.expandedWidth).arg(presetData.sliceHeight)
                            if (Config.displayMode === "hex")    return I18n.tr("r%1 %2×%3 - Right-click to overwrite").arg(presetData.hexRadius).arg(presetData.hexRows).arg(presetData.hexCols)
                            if (Config.displayMode === "wall")   return I18n.tr("%1×%2 %3×%4 - Right-click to overwrite").arg(presetData.gridColumns).arg(presetData.gridRows).arg(presetData.gridThumbWidth).arg(presetData.gridThumbHeight)
                            return ""
                        }
                        onClicked: {
                            if (isEmpty) { if (root.saveCustomPreset) root.saveCustomPreset(modelData) }
                            else { if (root.loadCustomPreset) root.loadCustomPreset(modelData) }
                        }
                        MouseArea {
                            anchors.fill: parent; acceptedButtons: Qt.RightButton
                            cursorShape: Qt.PointingHandCursor
                            onClicked: if (root.saveCustomPreset) root.saveCustomPreset(modelData)
                        }
                    }
                }
            }
        }
    }

    SettingsCard {
        colors: root.colors
        title: Config.displayMode === "hex" ? I18n.tr("Hex grid") : (Config.displayMode === "wall" ? I18n.tr("Wall") : (Config.displayMode === "mosaic" ? I18n.tr("Mosaic") : I18n.tr("Slice size")))
        width: (parent.width - parent.spacing) / 2

        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Slice height"); value: Config.wallpaperSliceHeight; min: 200; max: 1200; onCommit: function(v) { if (root.saveField) root.saveField("sliceHeight", v) } }
        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Visible items"); value: Config.wallpaperVisibleCount; min: 3; max: 30; onCommit: function(v) { if (root.saveField) root.saveField("visibleCount", v) } }
        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Selected width"); value: Config.wallpaperExpandedWidth; min: 50; max: 1800; onCommit: function(v) { if (root.saveField) root.saveField("expandedWidth", v) } }
        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Slice width"); value: Config.wallpaperSliceWidth; min: 50; max: 500; onCommit: function(v) { if (root.saveField) root.saveField("sliceWidth", v) } }
        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Gap"); value: Config.wallpaperSliceSpacing; min: -500; max: 500; onCommit: function(v) { if (root.saveField) root.saveField("sliceSpacing", v) } }
        RowInput { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Skew"); value: Config.wallpaperSkewOffset; min: -500; max: 500; onCommit: function(v) { if (root.saveField) root.saveField("skewOffset", v) } }

        RowInput { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Radius"); value: Config.hexRadius; min: 60; max: 300; onCommit: function(v) { if (root.saveField) root.saveField("hexRadius", v) } }
        RowInput { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Rows"); value: Config.hexRows; min: 1; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("hexRows", v) } }
        RowInput { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Columns"); value: Config.hexCols; min: 3; max: 20; onCommit: function(v) { if (root.saveField) root.saveField("hexCols", v) } }
        RowInput { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Scroll step"); value: Config.hexScrollStep; min: 1; max: 10; onCommit: function(v) { if (root.saveField) root.saveField("hexScrollStep", v) } }
        SettingsRow {
            visible: Config.displayMode === "hex"
            colors: root.colors
            title: I18n.tr("Field curve")
            description: I18n.tr("Shape the columns across the picker.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "flat", label: I18n.tr("Plane") },
                        { key: "arc", label: I18n.tr("Bow") },
                        { key: "wave", label: I18n.tr("Ribbon") },
                        { key: "s", label: I18n.tr("S-sweep") }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: modelData.label
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.hexCurve === modelData.key
                        onClicked: if (root.saveField) root.saveField("hexCurve", modelData.key)
                    }
                }
            }
        }
        SettingsRow {
            visible: Config.displayMode === "hex"
            colors: root.colors
            title: I18n.tr("Tile family")
            description: I18n.tr("Choose the geometry used for wallpaper cards.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "hexagon", label: I18n.tr("Hexagon") },
                        { key: "triangle", label: I18n.tr("Triangle") },
                        { key: "diamond", label: I18n.tr("Diamond") },
                        { key: "rhombus", label: I18n.tr("Rhombus") }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: modelData.label
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.hexShape === modelData.key
                        onClicked: if (root.saveField) root.saveField("hexShape", modelData.key)
                    }
                }
            }
        }
        RowInput { visible: Config.displayMode === "hex" && Config.hexCurve !== "flat"; colors: root.colors; title: I18n.tr("Bend strength (×10)"); value: Math.round(Config.hexArcIntensity * 10); min: 1; max: 30; onCommit: function(v) { if (root.saveField) root.saveField("hexArcIntensity", v / 10) } }
        RowInput { visible: Config.displayMode === "hex" && Config.hexCurve === "wave"; colors: root.colors; title: I18n.tr("Waves (×10)"); value: Math.round(Config.hexWaves * 10); min: 1; max: 50; onCommit: function(v) { if (root.saveField) root.saveField("hexWaves", v / 10) } }

        RowInput { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Columns"); value: Config.gridColumns; min: 2; max: 12; onCommit: function(v) { if (root.saveField) root.saveField("gridColumns", v) } }
        RowInput { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Rows"); value: Config.gridRows; min: 1; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("gridRows", v) } }
        RowInput { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Thumb width"); value: Config.gridThumbWidth; min: 100; max: 600; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbWidth", v) } }
        RowInput { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Thumb height"); value: Config.gridThumbHeight; min: 50; max: 400; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbHeight", v) } }

        RowInput { visible: Config.displayMode === "mosaic"; colors: root.colors; title: I18n.tr("Cells"); value: Config.mosaicCells; min: 4; max: 200; onCommit: function(v) { if (root.saveField) root.saveField("mosaicCells", v) } }
        RowInput { visible: Config.displayMode === "mosaic"; colors: root.colors; title: I18n.tr("Seed"); value: Config.mosaicSeed; min: 1; max: 99999; onCommit: function(v) { if (root.saveField) root.saveField("mosaicSeed", v) } }
        RowInput { visible: Config.displayMode === "mosaic"; colors: root.colors; title: I18n.tr("Relax iterations"); value: Config.mosaicRelaxation; min: 0; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("mosaicRelaxation", v) } }
        RowInput { visible: Config.displayMode === "mosaic"; colors: root.colors; title: I18n.tr("Width"); value: Config.mosaicWidth; min: 400; max: 3000; onCommit: function(v) { if (root.saveField) root.saveField("mosaicWidth", v) } }
        RowInput { visible: Config.displayMode === "mosaic"; colors: root.colors; title: I18n.tr("Height"); value: Config.mosaicHeight; min: 200; max: 2000; onCommit: function(v) { if (root.saveField) root.saveField("mosaicHeight", v) } }
    }

    SettingsCard {
        visible: Config.displayMode === "slices"
        colors: root.colors
        title: I18n.tr("Corners")
        width: (parent.width - parent.spacing) / 2

        RowToggle {
            colors: root.colors
            title: I18n.tr("Round corners")
            description: I18n.tr("Apply a corner radius to slice edges.")
            checked: Config.wallpaperSliceRoundCorners
            onToggle: function(v) { if (root.saveField) root.saveField("roundCorners", v) }
        }

        RowInput {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Top-left")
            description: I18n.tr("Top-left corner radius (px). 0 = square.")
            value: Config.wallpaperSliceCornerTL
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerTL", v) }
        }

        RowInput {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Top-right")
            description: I18n.tr("Top-right corner radius (px). 0 = square.")
            value: Config.wallpaperSliceCornerTR
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerTR", v) }
        }

        RowInput {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Bottom-right")
            description: I18n.tr("Bottom-right corner radius (px). 0 = square.")
            value: Config.wallpaperSliceCornerBR
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerBR", v) }
        }

        RowInput {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Bottom-left")
            description: I18n.tr("Bottom-left corner radius (px). 0 = square.")
            value: Config.wallpaperSliceCornerBL
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerBL", v) }
        }
    }

}
