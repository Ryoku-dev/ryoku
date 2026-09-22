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
            description: I18n.tr("Slices, Hex, Wall, Mosaic, Hand, Sandy, or Grid.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "slices",  label: I18n.tr("Slices") },
                        { key: "hex",     label: I18n.tr("Hex") },
                        { key: "wall",    label: I18n.tr("Wall") },
                        { key: "mosaic",  label: I18n.tr("Mosaic") },
                        { key: "hand",    label: I18n.tr("Hand") },
                        { key: "sandy",   label: I18n.tr("Sandy") },
                        { key: "grid",    label: I18n.tr("Grid") }
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

        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Slice height"); value: Config.wallpaperSliceHeight; min: 200; max: 1200; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("sliceHeight", v) } }
        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Visible items"); value: Config.wallpaperVisibleCount; min: 3; max: 30; onCommit: function(v) { if (root.saveField) root.saveField("visibleCount", v) } }
        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Selected width"); value: Config.wallpaperExpandedWidth; min: 50; max: 1800; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("expandedWidth", v) } }
        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Slice width"); value: Config.wallpaperSliceWidth; min: 50; max: 500; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("sliceWidth", v) } }
        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Gap"); value: Config.wallpaperSliceSpacing; min: -500; max: 500; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("sliceSpacing", v) } }
        RowSlider { visible: Config.displayMode === "slices"; colors: root.colors; title: I18n.tr("Skew"); value: Config.wallpaperSkewOffset; min: -500; max: 500; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("skewOffset", v) } }

        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Radius"); value: Config.hexRadius; min: 60; max: 300; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("hexRadius", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Rows"); value: Config.hexRows; min: 1; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("hexRows", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Columns"); value: Config.hexCols; min: 3; max: 20; onCommit: function(v) { if (root.saveField) root.saveField("hexCols", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Scroll step"); value: Config.hexScrollStep; min: 1; max: 10; onCommit: function(v) { if (root.saveField) root.saveField("hexScrollStep", v) } }
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
        RowSlider { visible: Config.displayMode === "hex" && Config.hexCurve !== "flat"; colors: root.colors; title: I18n.tr("Bend strength"); value: Config.hexArcIntensity; min: 0.1; max: 3.0; decimals: 1; onCommit: function(v) { if (root.saveField) root.saveField("hexArcIntensity", v) } }
        RowSlider { visible: Config.displayMode === "hex" && Config.hexCurve === "wave"; colors: root.colors; title: I18n.tr("Waves"); value: Config.hexWaves; min: 0.1; max: 5.0; decimals: 1; onCommit: function(v) { if (root.saveField) root.saveField("hexWaves", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Gap X"); value: Config.hexGapX; min: -40; max: 60; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("hexGapX", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Gap Y"); value: Config.hexGapY; min: -40; max: 60; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("hexGapY", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Column stagger"); value: Config.hexStagger; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("hexStagger", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Lens"); value: Config.hexLens; min: 0; max: 1.5; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("hexLens", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Twist"); value: Config.hexTwist; min: -30; max: 30; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("hexTwist", v) } }
        RowSlider { visible: Config.displayMode === "hex"; colors: root.colors; title: I18n.tr("Scatter"); value: Config.hexScatter; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("hexScatter", v) } }

        RowSlider { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Columns"); value: Config.gridColumns; min: 2; max: 12; onCommit: function(v) { if (root.saveField) root.saveField("gridColumns", v) } }
        RowSlider { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Rows"); value: Config.gridRows; min: 1; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("gridRows", v) } }
        RowSlider { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Thumb width"); value: Config.gridThumbWidth; min: 100; max: 600; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbWidth", v) } }
        RowSlider { visible: Config.displayMode === "wall"; colors: root.colors; title: I18n.tr("Thumb height"); value: Config.gridThumbHeight; min: 50; max: 400; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbHeight", v) } }

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

        RowSlider {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Top-left")
            description: I18n.tr("Top-left corner radius. 0 = square.")
            value: Config.wallpaperSliceCornerTL
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerTL", v) }
        }

        RowSlider {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Top-right")
            description: I18n.tr("Top-right corner radius. 0 = square.")
            value: Config.wallpaperSliceCornerTR
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerTR", v) }
        }

        RowSlider {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Bottom-right")
            description: I18n.tr("Bottom-right corner radius. 0 = square.")
            value: Config.wallpaperSliceCornerBR
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerBR", v) }
        }

        RowSlider {
            visible: Config.wallpaperSliceRoundCorners
            colors: root.colors
            title: I18n.tr("Bottom-left")
            description: I18n.tr("Bottom-left corner radius. 0 = square.")
            value: Config.wallpaperSliceCornerBL
            min: 0; max: 80; suffix: "px"
            onCommit: function(v) { if (root.saveField) root.saveField("cornerBL", v) }
        }
    }

    SettingsCard {
        visible: Config.displayMode === "hand"
        colors: root.colors
        title: I18n.tr("Hand")
        width: (parent.width - parent.spacing) / 2

        RowSlider { colors: root.colors; title: I18n.tr("Cards"); value: Config.handCount; min: 3; max: 15; onCommit: function(v) { if (root.saveField) root.saveField("handCount", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Card width"); value: Config.handCardWidth; min: 80; max: 400; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("handCardWidth", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Card height"); value: Config.handCardHeight; min: 160; max: 900; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("handCardHeight", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Fan angle"); value: Config.handFanAngle; min: -30; max: 30; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("handFanAngle", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Fan roll"); value: Config.handFanRoll; min: -30; max: 30; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("handFanRoll", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Arch"); value: Config.handArch; min: -120; max: 200; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("handArch", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Spread"); value: Config.handSpread; min: 40; max: 320; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("handSpread", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Skew"); value: Config.handSkew; min: -40; max: 40; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("handSkew", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Corner radius"); value: Config.handCornerRadius; min: 0; max: 60; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("handCornerRadius", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Tilt"); value: Config.handTilt; min: 0; max: 40; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("handTilt", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Perspective"); value: Config.handPerspective; min: 0; max: 60; onCommit: function(v) { if (root.saveField) root.saveField("handPerspective", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Speed"); value: Config.handSpeed; min: 0.2; max: 4; decimals: 1; onCommit: function(v) { if (root.saveField) root.saveField("handSpeed", v) } }
        RowToggle { colors: root.colors; title: I18n.tr("Motion ghosts"); description: I18n.tr("Trailing blur copies on moving cards."); checked: Config.handGhosts; onToggle: function(v) { if (root.saveField) root.saveField("handGhosts", v) } }
        RowToggle { colors: root.colors; title: I18n.tr("Idle bob"); description: I18n.tr("The centre card breathes when at rest."); checked: Config.handBob; onToggle: function(v) { if (root.saveField) root.saveField("handBob", v) } }
        RowToggle { colors: root.colors; title: I18n.tr("Backdrop"); description: I18n.tr("Dim, blurred hero behind the fan."); checked: Config.handBackdrop; onToggle: function(v) { if (root.saveField) root.saveField("handBackdrop", v) } }
    }

    SettingsCard {
        visible: Config.displayMode === "sandy"
        colors: root.colors
        title: I18n.tr("Sandy")
        width: (parent.width - parent.spacing) / 2

        RowSlider { colors: root.colors; title: I18n.tr("Centre"); description: I18n.tr("Where the strand sits across the stage (0 = left, 0.5 = centre)."); value: Config.sandyCenter; min: 0.2; max: 0.8; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyCenter", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Slice width"); value: Config.sandySliceWidth; min: 30; max: 260; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("sandySliceWidth", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Slice height"); value: Config.sandySliceHeight; min: 60; max: 520; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("sandySliceHeight", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Skew"); value: Config.sandySkew; min: -40; max: 40; suffix: "°"; onCommit: function(v) { if (root.saveField) root.saveField("sandySkew", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Spacing"); value: Config.sandySpacing; min: 0; max: 100; onCommit: function(v) { if (root.saveField) root.saveField("sandySpacing", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Duration"); value: Config.sandyDuration; min: 200; max: 3000; suffix: "ms"; onCommit: function(v) { if (root.saveField) root.saveField("sandyDuration", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Strands"); value: Config.sandyStrands; min: 1; max: 40; onCommit: function(v) { if (root.saveField) root.saveField("sandyStrands", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Twist"); value: Config.sandyTwist; min: 0; max: 3; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyTwist", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Orbit"); value: Config.sandyOrbit; min: 0; max: 3; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyOrbit", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Turbulence"); value: Config.sandyTurbulence; min: 0; max: 3; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyTurbulence", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Waist"); value: Config.sandyWaist; min: 0; max: 2; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyWaist", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Front"); value: Config.sandyFront; min: 0; max: 2; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyFront", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Arc"); value: Config.sandyArc; min: 0; max: 2; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyArc", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Edge speed"); value: Config.sandyEdgeSpeed; min: 0; max: 56; onCommit: function(v) { if (root.saveField) root.saveField("sandyEdgeSpeed", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Grain"); value: Config.sandyGrain; min: 0; max: 10; onCommit: function(v) { if (root.saveField) root.saveField("sandyGrain", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Fan"); value: Config.sandyFan; min: 0; max: 2; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("sandyFan", v) } }
    }

    SettingsCard {
        visible: Config.displayMode === "grid"
        colors: root.colors
        title: I18n.tr("Grid layout")
        width: (parent.width - parent.spacing) / 2

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Arrangement")
            description: I18n.tr("How the thumbnails are packed.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "uniform",   label: I18n.tr("Uniform") },
                        { key: "brick",     label: I18n.tr("Brick") },
                        { key: "masonry",   label: I18n.tr("Masonry") },
                        { key: "justified", label: I18n.tr("Justified") },
                        { key: "editorial", label: I18n.tr("Editorial") },
                        { key: "cylinder",  label: I18n.tr("Cylinder") }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: modelData.label
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.gridLayout === modelData.key
                        onClicked: if (root.saveField) root.saveField("gridLayout", modelData.key)
                    }
                }
            }
        }
        RowSlider { colors: root.colors; title: I18n.tr("Columns"); value: Config.gridColumns; min: 2; max: 12; onCommit: function(v) { if (root.saveField) root.saveField("gridColumns", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Rows"); value: Config.gridRows; min: 1; max: 8; onCommit: function(v) { if (root.saveField) root.saveField("gridRows", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Thumb width"); value: Config.gridThumbWidth; min: 100; max: 600; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbWidth", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Thumb height"); value: Config.gridThumbHeight; min: 50; max: 400; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("gridThumbHeight", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Stagger"); value: Config.gridStagger; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridStagger", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Selected scale"); value: Config.gridSelectedScale; min: 1; max: 1.6; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridSelectedScale", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Flow wave"); value: Config.gridFlowWave; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridFlowWave", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Flow frequency"); value: Config.gridFlowFrequency; min: 0.1; max: 4; decimals: 1; onCommit: function(v) { if (root.saveField) root.saveField("gridFlowFrequency", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Scatter"); value: Config.gridScatter; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridScatter", v) } }
        RowSlider { colors: root.colors; title: I18n.tr("Scale variance"); value: Config.gridScaleVariance; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridScaleVariance", v) } }
        RowSlider { visible: Config.gridLayout === "cylinder"; colors: root.colors; title: I18n.tr("Cylinder bend"); value: Config.gridCylinderBend; min: 0; max: 1; decimals: 2; onCommit: function(v) { if (root.saveField) root.saveField("gridCylinderBend", v) } }
        RowSlider { visible: Config.gridLayout === "cylinder"; colors: root.colors; title: I18n.tr("Cylinder radius"); value: Config.gridCylinderRadius; min: 300; max: 2000; suffix: "px"; onCommit: function(v) { if (root.saveField) root.saveField("gridCylinderRadius", v) } }
    }

}
