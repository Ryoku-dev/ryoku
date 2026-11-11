// Written by go run ./cmd/lithic-tokens from tokens.json. Do not edit by hand.
package tokens

import "time"

type Type struct {
	Family string
	Weight int
	Size   float32
	Line   float32
}

const (
	SpaceS1 = 4
	SpaceS2 = 8
	SpaceS3 = 13
	SpaceS4 = 21
	SpaceS5 = 34
	SpaceS6 = 55
)

const (
	CutLarge       = 16
	CutMenu        = 8
	CutMid         = 7
	CutNotch       = 12
	CutOverlay     = 8
	CutPlacard     = 8
	CutSegment     = 5
	CutSmall       = 6
	CutToggleKnob  = 2
	CutToggleTrack = 4
	CutToken       = 3
	CutTooltip     = 4
)

const (
	BreakpointExpanded = 1280
	BreakpointMedium   = 760
)

const (
	SizeBladeIndeterminateFraction = 0.25
	SizeBladeMarker                = 2
	SizeBladeMeterAlertAt          = 0.9
	SizeBladeTrack                 = 6
	SizeBoardColumnsAt             = 860
	SizeBoardMax                   = 1240
	SizeBrandCutRatio              = 0.22
	SizeBrandMark                  = 40
	SizeBrandMarkCompact           = 21
	SizeBrandOctagonApothemRatio   = 0.28
	SizeBrandOutline               = 1.5
	SizeBrandPointRatio            = 0.045
	SizeBrandPointerFarRatio       = 0.31
	SizeBrandPointerMidRatio       = 0.14
	SizeBrandPointerNearRatio      = 0.06
	SizeCatalogueDialogWidth       = 720
	SizeCatalogueGridGap           = 4
	SizeCatalogueTileHeight        = 128
	SizeCatalogueTileWidthMax      = 260
	SizeCatalogueTileWidthMin      = 200
	SizeChartBaseline              = 1
	SizeChartHatchGap              = 6
	SizeChartHatchLine             = 1
	SizeChartLine                  = 1.5
	SizeChartTick                  = 4
	SizeControlPressCutGrow        = 4
	SizeControlPressSink           = 1
	SizeDialogActionGap            = 8
	SizeDialogActionsGap           = 34
	SizeDialogConfirmFieldGap      = 21
	SizeDialogEnterTravel          = 12
	SizeDialogExitTravel           = 8
	SizeDialogMargin               = 16
	SizeDialogPadding              = 34
	SizeDialogTitleGap             = 21
	SizeDialogWidth                = 520
	SizeDraftAsideBelow            = 600
	SizeDraftAsideFraction         = 0.36
	SizeDraftAsideMax              = 400
	SizeDraftAsideMin              = 300
	SizeDraftFormMax               = 600
	SizeDraftWideAt                = 820
	SizeEmptyBodyMax               = 420
	SizeEmptyFrame                 = 72
	SizeEmptyIcon                  = 30
	SizeFactsLabelWidth            = 140
	SizeFactsRowHeight             = 44
	SizeFilterCountGap             = 6
	SizeFilterGap                  = 4
	SizeFilterHeight               = 32
	SizeFilterInset                = 13
	SizeFlakeHeightRatio           = 0.7
	SizeFlakeNominal               = 9
	SizeFlakeOutline               = 1.5
	SizeFlakeWidthRatio            = 1.4
	SizeFocusCutGrow               = 2
	SizeFocusRingGap               = 2
	SizeFocusRingWidth             = 2
	SizeGroupHeaderBottom          = 6
	SizeGroupHeaderLeft            = 6
	SizeGroupHeaderRight           = 4
	SizeGroupHeaderTop             = 14
	SizeHairline                   = 1
	SizeHeaderActionsGap           = 21
	SizeHeaderBottom               = 26
	SizeHeaderContentGap           = 21
	SizeHeaderMark                 = 44
	SizeHeaderMarkGap              = 13
	SizeHeaderMarksGap             = 4
	SizeHeaderNameCompactAt        = 640
	SizeHeaderNoteGap              = 4
	SizeHeaderProofGap             = 8
	SizeHeaderSide                 = 36
	SizeHeaderTop                  = 30
	SizeIconPlaque                 = 40
	SizeIconPlaqueIcon             = 20
	SizeIconPlaqueIconSmall        = 18
	SizeIconPlaqueSmall            = 32
	SizeListExpanded               = 384
	SizeListInset                  = 10
	SizeListMedium                 = 320
	SizeMenuDividerInset           = 8
	SizeMenuIcon                   = 20
	SizeMenuIconGap                = 12
	SizeMenuInset                  = 16
	SizeMenuItem                   = 40
	SizeMenuPaddingVertical        = 6
	SizeMenuShortcutGap            = 13
	SizeMenuWidthMax               = 360
	SizeMenuWidthMin               = 220
	SizeNewPlaqueHeight            = 52
	SizeNewPlaqueHeightCompact     = 40
	SizeNewPlaqueIcon              = 21
	SizeNewPlaqueIconCompact       = 20
	SizeNewPlaqueLabelGap          = 2
	SizeNewPlaqueWidth             = 62
	SizeNewPlaqueWidthCompact      = 48
	SizePaneBodyGap                = 8
	SizePaneDecorStrip             = 32
	SizePaneGap                    = 8
	SizePaneRightWeb               = 10
	SizePaneTop                    = 10
	SizePickerArrow                = 20
	SizePickerArrowGap             = 10
	SizePickerHeight               = 44
	SizePickerInset                = 14
	SizePickerWidthMin             = 160
	SizePinnedTileMinHeight        = 154
	SizePlacardActionHeight        = 32
	SizePlacardActionInset         = 13
	SizePlacardBlade               = 2
	SizePlacardBottom              = 24
	SizePlacardHeight              = 48
	SizePlacardInset               = 16
	SizePlacardRise                = 8
	SizePlacardStackCount          = 3
	SizePlacardStackGap            = 8
	SizePlacardTrailingInset       = 8
	SizePlacardWidthMax            = 560
	SizePlaqueGap                  = 8
	SizePlaqueGapSmall             = 6
	SizePlaqueHeight               = 40
	SizePlaqueHeightSmall          = 32
	SizePlaqueIcon                 = 19
	SizePlaqueIconSmall            = 17
	SizePlaqueInset                = 18
	SizePlaqueInsetSmall           = 13
	SizePopoverAnchorGap           = 8
	SizePopoverEnterTravel         = 6
	SizePopoverExitTravel          = 4
	SizeRailBottomColumns          = 5
	SizeRailBottomHeight           = 64
	SizeRailDestinationCompact     = 48
	SizeRailDestinationExpanded    = 64
	SizeRailDestinationIcon        = 21
	SizeRailDestinationMedium      = 56
	SizeRailExpanded               = 88
	SizeRailHomeExpanded           = 56
	SizeRailHomeMedium             = 48
	SizeRailHomeTop                = 16
	SizeRailItemBottomInset        = 16
	SizeRailItemLabelGap           = 5
	SizeRailMedium                 = 72
	SizeRailNewYExpanded           = 168
	SizeRailNewYMedium             = 152
	SizeRailStartExpanded          = 96
	SizeRailStartMedium            = 88
	SizeRailTileGap                = 8
	SizeRowListGap                 = 4
	SizeRowListWheelNotch          = 120
	SizeScrollThumb                = 3
	SizeScrollThumbMin             = 12
	SizeSearchClear                = 32
	SizeSearchClearInset           = 6
	SizeSearchHeight               = 44
	SizeSearchIcon                 = 20
	SizeSearchIconInset            = 14
	SizeSearchLeading              = 42
	SizeSearchWidth                = 280
	SizeSectionIndexGap            = 34
	SizeSectionIndexInset          = 13
	SizeSectionIndexRow            = 48
	SizeSectionIndexRowGap         = 4
	SizeSectionIndexWidth          = 200
	SizeSegmentHeight              = 36
	SizeSegmentInset               = 4
	SizeSelBlade                   = 3
	SizeSettingsControlMax         = 320
	SizeSettingsDetailGap          = 2
	SizeSettingsExpandedMax        = 994
	SizeSettingsInsetSide          = 36
	SizeSettingsInsetVertical      = 34
	SizeSettingsMediumMax          = 760
	SizeSettingsRowGap             = 13
	SizeSkeletonBarGap             = 8
	SizeSkeletonBarHeight          = 8
	SizeSkeletonBarLongFraction    = 0.6
	SizeSkeletonBarShortFraction   = 0.35
	SizeSkeletonInset              = 16
	SizeSlabBelow                  = 21
	SizeSlabInset                  = 21
	SizeSlabTitleGap               = 13
	SizeSliderHandleHeight         = 20
	SizeSliderHandleWidth          = 8
	SizeSliderHeight               = 32
	SizeSliderKeyboardStep         = 0.05
	SizeSliderTrack                = 6
	SizeSliderWidth                = 160
	SizeSplitPlaqueGap             = 2
	SizeSplitPlaqueMoreWidth       = 40
	SizeStepperGap                 = 8
	SizeStepperUnitGap             = 4
	SizeTabletHeight               = 56
	SizeTabletHeightCompact        = 48
	SizeTabletInsetLeft            = 16
	SizeTabletInsetRight           = 12
	SizeTabletMark                 = 32
	SizeTabletMarkGap              = 13
	SizeTabletProgress             = 2
	SizeTabletTrailingMin          = 72
	SizeTabsBaseline               = 1
	SizeTabsBlade                  = 3
	SizeTabsHeight                 = 44
	SizeTabsLabelExtra             = 24
	SizeTabsMaxCount               = 5
	SizeTextFieldDefaultWidth      = 240
	SizeTextFieldHeight            = 44
	SizeTextFieldHelpGap           = 4
	SizeTextFieldInset             = 14
	SizeTextFieldLabelGap          = 4
	SizeTextFieldSecretAction      = 32
	SizeTextFieldSecretInset       = 6
	SizeTextFieldStroke            = 1
	SizeToggleHeight               = 22
	SizeToggleInset                = 4
	SizeToggleKnob                 = 14
	SizeToggleLabelGap             = 8
	SizeToggleWidth                = 40
	SizeTooltipAnchorGap           = 8
	SizeTooltipPaddingHorizontal   = 8
	SizeTooltipPaddingVertical     = 6
	SizeTooltipTravel              = 8
	SizeTooltipWidthMax            = 320
	SizeWindowDefaultHeight        = 800
	SizeWindowDefaultWidth         = 1280
	SizeWindowMinHeight            = 520
	SizeWindowMinWidth             = 640
	SizeWorkContentInset           = 34
	SizeWorkPaneRise               = 8
	SizeWorkingFlakeCount          = 3
	SizeWorkingFlakeGapRatio       = 0.1
	SizeWorkingFlakeHeightRatio    = 0.5
	SizeWorkingFlakeWidthRatio     = 0.2
)

const (
	MotionDialogIn     time.Duration = 300 * time.Millisecond
	MotionDialogOut    time.Duration = 160 * time.Millisecond
	MotionField        time.Duration = 320 * time.Millisecond
	MotionFocusIn      time.Duration = 120 * time.Millisecond
	MotionFocusOut     time.Duration = 100 * time.Millisecond
	MotionHoverIn      time.Duration = 160 * time.Millisecond
	MotionHoverOut     time.Duration = 100 * time.Millisecond
	MotionMenuIn       time.Duration = 160 * time.Millisecond
	MotionMenuOut      time.Duration = 100 * time.Millisecond
	MotionNavigate     time.Duration = 200 * time.Millisecond
	MotionNotch        time.Duration = 200 * time.Millisecond
	MotionPlacardIn    time.Duration = 300 * time.Millisecond
	MotionPlacardLife  time.Duration = 5000 * time.Millisecond
	MotionPress        time.Duration = 90 * time.Millisecond
	MotionSelect       time.Duration = 240 * time.Millisecond
	MotionSweep        time.Duration = 1280 * time.Millisecond
	MotionTooltipDelay time.Duration = 600 * time.Millisecond
	MotionTooltipIn    time.Duration = 300 * time.Millisecond
	MotionUnfold       time.Duration = 200 * time.Millisecond
	MotionWheel        time.Duration = 140 * time.Millisecond
	MotionWorking      time.Duration = 900 * time.Millisecond
)

var MotionCurve = [4]float32{0.2, 0, 0, 1}

const (
	AlphaContrastHigh    float32 = 0.6
	AlphaContrastHighFg3 float32 = 0.3
	AlphaDisabled        float32 = 0.5
	AlphaEdge            float32 = 0.5
	AlphaHatch           float32 = 0.3
	AlphaHoverFill       float32 = 0.1
	AlphaHoverInk        float32 = 0.1
	AlphaHoverPlane      float32 = 0.06
	AlphaMineral         float32 = 0.15
	AlphaMineralJade     float32 = 0.17
	AlphaOnSel2          float32 = 0.45
	AlphaPressPlane      float32 = 0.06
	AlphaPressRow        float32 = 0.08
	AlphaScrimDark       float32 = 0.62
	AlphaScrimLight      float32 = 0.45
	AlphaSelMix          float32 = 0.5
	AlphaTintLow         float32 = 0.06
	AlphaTintMedium      float32 = 0.1
	AlphaWorkingBase     float32 = 0.55
)

const (
	ContrastMinimum float32 = 4.5
)

var (
	FontIcons = []string{"Material Symbols Sharp"}
	FontMono  = []string{"Space Mono", "SpaceMono Nerd Font", "SpaceMono Nerd Font Mono"}
	FontSans  = []string{"Space Grotesk"}
)

var (
	TypeBody     = Type{Family: "sans", Weight: 400, Size: 14, Line: 20}
	TypeCaption  = Type{Family: "sans", Weight: 400, Size: 12, Line: 16}
	TypeDetail   = Type{Family: "sans", Weight: 400, Size: 13, Line: 18}
	TypeGroup    = Type{Family: "sans", Weight: 500, Size: 12, Line: 16}
	TypeLabel    = Type{Family: "sans", Weight: 500, Size: 13, Line: 16}
	TypeMono     = Type{Family: "mono", Weight: 400, Size: 12, Line: 16}
	TypeMonoSm   = Type{Family: "mono", Weight: 400, Size: 11, Line: 15}
	TypeName     = Type{Family: "sans", Weight: 700, Size: 52, Line: 56}
	TypeProof    = Type{Family: "sans", Weight: 400, Size: 15, Line: 21}
	TypeRow      = Type{Family: "sans", Weight: 500, Size: 15, Line: 20}
	TypeSection  = Type{Family: "sans", Weight: 700, Size: 21, Line: 28}
	TypeSentence = Type{Family: "sans", Weight: 700, Size: 34, Line: 40}
	TypeTitle    = Type{Family: "sans", Weight: 500, Size: 15, Line: 20}
)

var SignatureDark = map[string]string{
	"fg":      "#e3e3e3",
	"fg2":     "#c6c6c6",
	"fg3":     "#8f8f8f",
	"field":   "#393939",
	"gold":    "#c9a861",
	"goldInk": "#d6b877",
	"ground":  "#0e0e0e",
	"jasper":  "#e0553a",
	"onGold":  "#17120a",
	"onSel":   "#121212",
	"onSel2":  "#474747",
	"scrim":   "#0000009e",
	"sel":     "#e3e3e3",
	"slab":    "#232323",
	"stone1":  "#141414",
	"stone2":  "#1b1b1b",
	"stone3":  "#232323",
	"stone4":  "#2c2c2c",
	"vein":    "#444444",
}

var SignatureLight = map[string]string{
	"fg":      "#1d1b18",
	"fg2":     "#4a4640",
	"fg3":     "#6f6a62",
	"field":   "#e9e5de",
	"gold":    "#c9a45a",
	"goldInk": "#85651f",
	"ground":  "#e2ddd4",
	"jasper":  "#b8321e",
	"onGold":  "#1d1b18",
	"onSel":   "#f4f1eb",
	"onSel2":  "#b9b2a7",
	"scrim":   "#00000073",
	"sel":     "#1d1b18",
	"slab":    "#fbf9f5",
	"stone1":  "#eae6df",
	"stone2":  "#f4f1eb",
	"stone3":  "#e9e5de",
	"stone4":  "#fbf9f5",
	"vein":    "#cbc4b8",
}

var MineralsDark = map[string]string{
	"amber": "#e0954a",
	"jade":  "#7ab8a0",
	"lapis": "#8aa2d6",
}

var MineralsLight = map[string]string{
	"amber": "#94520f",
	"jade":  "#2f7a64",
	"lapis": "#3d5f99",
}

var PaletteAccents = map[string]string{
	"gold":   "primary",
	"jasper": "error",
	"onGold": "onPrimary",
	"onSel":  "onSecondaryContainer",
	"scrim":  "scrim",
	"sel":    "secondaryContainer",
	"selMix": "primaryContainer",
	"tint":   "primaryContainer",
}

var PaletteDark = map[string]string{
	"fg":     "onSurface",
	"fg2":    "onSurfaceVariant",
	"fg3":    "outline",
	"field":  "surfaceContainerHighest",
	"ground": "surfaceContainerLowest",
	"slab":   "surfaceContainer",
	"stone1": "surface",
	"stone2": "surfaceContainerLow",
	"stone3": "surfaceContainer",
	"stone4": "surfaceContainerHigh",
	"vein":   "outlineVariant",
}

var PaletteLight = map[string]string{
	"fg":     "onSurface",
	"fg2":    "onSurfaceVariant",
	"fg3":    "outline",
	"field":  "surfaceContainerHighest",
	"ground": "surfaceContainerHighest",
	"slab":   "surfaceContainerLowest",
	"stone1": "surfaceContainerHigh",
	"stone2": "surfaceContainerLowest",
	"stone3": "surfaceContainer",
	"stone4": "surfaceContainerLowest",
	"vein":   "outlineVariant",
}

var PaletteToned = map[string]string{
	"goldInk": "gold",
	"jasper":  "jasper",
}

var Edition = map[string]string{
	"alpine":      "#0d7fb0",
	"arch":        "#1793d1",
	"cachyos":     "#00b3a1",
	"debian":      "#d70a53",
	"endeavouros": "#7f3fbf",
	"fedora":      "#51a2da",
	"gentoo":      "#7a6bbd",
	"linuxmint":   "#86be43",
	"manjaro":     "#35bf5c",
	"nixos":       "#7ebae4",
	"opensuse":    "#73ba25",
	"pop":         "#48b9c7",
	"ubuntu":      "#e95420",
	"void":        "#478061",
}
