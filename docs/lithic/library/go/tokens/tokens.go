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
	SizeDesignPreviewRatio         = 1.6
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
	MotionHighlight    time.Duration = 1600 * time.Millisecond
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
	AlphaDisabled    float32 = 0.5
	AlphaHatch       float32 = 0.3
	AlphaHoverFill   float32 = 0.1
	AlphaHoverInk    float32 = 0.1
	AlphaHoverPlane  float32 = 0.06
	AlphaOnSel2      float32 = 0.2
	AlphaPressPlane  float32 = 0.06
	AlphaPressRow    float32 = 0.08
	AlphaScrimDark   float32 = 0.62
	AlphaScrimLight  float32 = 0.45
	AlphaSelMix      float32 = 0.5
	AlphaWorkingBase float32 = 0.55
)

const (
	ContrastMinimum float32 = 4.5
)

var (
	FontIcons = []string{"Material Symbols Sharp"}
	FontMono  = []string{"Martian Mono"}
	FontSans  = []string{"Geologica"}
)

var (
	TypeBody     = Type{Family: "sans", Weight: 400, Size: 14, Line: 20}
	TypeCaption  = Type{Family: "sans", Weight: 400, Size: 12, Line: 16}
	TypeDetail   = Type{Family: "sans", Weight: 400, Size: 13, Line: 18}
	TypeGroup    = Type{Family: "sans", Weight: 500, Size: 12, Line: 16}
	TypeLabel    = Type{Family: "sans", Weight: 500, Size: 13, Line: 16}
	TypeMono     = Type{Family: "mono", Weight: 400, Size: 10, Line: 16}
	TypeMonoSm   = Type{Family: "mono", Weight: 400, Size: 9, Line: 15}
	TypeName     = Type{Family: "sans", Weight: 700, Size: 52, Line: 56}
	TypeProof    = Type{Family: "sans", Weight: 400, Size: 15, Line: 21}
	TypeRow      = Type{Family: "sans", Weight: 500, Size: 15, Line: 20}
	TypeSection  = Type{Family: "sans", Weight: 700, Size: 21, Line: 28}
	TypeSentence = Type{Family: "sans", Weight: 700, Size: 34, Line: 40}
	TypeTitle    = Type{Family: "sans", Weight: 500, Size: 15, Line: 20}
)

var SignatureDark = map[string]string{
	"edge":    "#66625e",
	"fg":      "#cdc4ba",
	"fg2":     "#b0a9a0",
	"fg3":     "#958f87",
	"field":   "#303030",
	"gold":    "#ce5e32",
	"goldInk": "#dc6a3e",
	"ground":  "#000000",
	"jasper":  "#eb596c",
	"onGold":  "#000000",
	"onSel":   "#000000",
	"onSel2":  "#292725",
	"scrim":   "#0000009e",
	"sel":     "#cdc4ba",
	"slab":    "#232323",
	"stone1":  "#141414",
	"stone2":  "#1b1b1b",
	"stone3":  "#232323",
	"stone4":  "#2c2c2c",
	"vein":    "#393939",
}

var SignatureDarkHigh = map[string]string{
	"edge":    "#87827c",
	"fg":      "#e6dcd2",
	"fg2":     "#cdc4ba",
	"fg3":     "#b0a9a0",
	"field":   "#3a3a3a",
	"gold":    "#ce5e32",
	"goldInk": "#ea784c",
	"ground":  "#000000",
	"jasper":  "#fa6777",
	"onGold":  "#000000",
	"onSel":   "#000000",
	"onSel2":  "#2e2c2a",
	"scrim":   "#0000009e",
	"sel":     "#e6dcd2",
	"slab":    "#292929",
	"stone1":  "#171717",
	"stone2":  "#1f1f1f",
	"stone3":  "#292929",
	"stone4":  "#333333",
	"vein":    "#4a4a4a",
}

var SignatureLight = map[string]string{
	"edge":    "#c7c3be",
	"fg":      "#1e1a15",
	"fg2":     "#56504a",
	"fg3":     "#6b655f",
	"field":   "#e7e5e3",
	"gold":    "#ce5e32",
	"goldInk": "#b04313",
	"ground":  "#dfdddb",
	"jasper":  "#ba334a",
	"onGold":  "#000000",
	"onSel":   "#f3f1ef",
	"onSel2":  "#c8c6c3",
	"scrim":   "#00000073",
	"sel":     "#1e1a15",
	"slab":    "#fbf9f7",
	"stone1":  "#e8e6e4",
	"stone2":  "#f3f1ef",
	"stone3":  "#e7e5e3",
	"stone4":  "#fbf9f7",
	"vein":    "#d6d3cf",
}

var SignatureLightHigh = map[string]string{
	"edge":    "#a29e99",
	"fg":      "#110c08",
	"fg2":     "#4c4641",
	"fg3":     "#605b55",
	"field":   "#e9e7e5",
	"gold":    "#ce5e32",
	"goldInk": "#aa3d07",
	"ground":  "#d9d7d5",
	"jasper":  "#b32d45",
	"onGold":  "#000000",
	"onSel":   "#fdfbf9",
	"onSel2":  "#cecbc9",
	"scrim":   "#00000073",
	"sel":     "#110c08",
	"slab":    "#fefdfc",
	"stone1":  "#ebe9e7",
	"stone2":  "#fdfbf9",
	"stone3":  "#e9e7e5",
	"stone4":  "#fefdfc",
	"vein":    "#c0bdba",
}

var MineralsDark = map[string]string{
	"amber": "#d8ad52",
	"jade":  "#7ab8a0",
	"lapis": "#8aa2d6",
}

var MineralsLight = map[string]string{
	"amber": "#855d00",
	"jade":  "#25715c",
	"lapis": "#3d5f99",
}

var PaletteAccents = map[string]string{
	"gold":   "primary",
	"jasper": "error",
	"scrim":  "scrim",
	"sel":    "secondaryContainer",
	"selMix": "primaryContainer",
}

var PaletteDye = map[string]string{
	"ink":     "onSurface",
	"neutral": "surface",
}

var ToneDark = map[string]float64{
	"edge":         0.45,
	"fg":           0.951,
	"fg2":          0.801,
	"fg3":          0.689,
	"field":        0.31,
	"gold":         0.785,
	"goldInk":      0.785,
	"ground":       0.158,
	"jasper":       0.74,
	"mineralField": 0.305,
	"onGold":       0.1,
	"onSel":        0.1,
	"sel":          0.825,
	"slab":         0.246,
	"stone1":       0.188,
	"stone2":       0.216,
	"stone3":       0.246,
	"stone4":       0.278,
	"vein":         0.391,
}

var ToneDarkHigh = map[string]float64{
	"edge":         0.535,
	"fg":           0.976,
	"fg2":          0.854,
	"fg3":          0.772,
	"field":        0.401,
	"gold":         0.8,
	"goldInk":      0.8,
	"ground":       0.158,
	"jasper":       0.78,
	"mineralField": 0.345,
	"onGold":       0.1,
	"onSel":        0.08,
	"sel":          0.9,
	"slab":         0.299,
	"stone1":       0.206,
	"stone2":       0.251,
	"stone3":       0.299,
	"stone4":       0.35,
	"vein":         0.45,
}

var ToneLight = map[string]float64{
	"edge":         0.818,
	"fg":           0.221,
	"fg2":          0.435,
	"fg3":          0.508,
	"field":        0.923,
	"gold":         0.78,
	"goldInk":      0.492,
	"ground":       0.899,
	"jasper":       0.495,
	"mineralField": 0.925,
	"onGold":       0.12,
	"onSel":        0.959,
	"sel":          0.221,
	"slab":         0.983,
	"stone1":       0.926,
	"stone2":       0.959,
	"stone3":       0.923,
	"stone4":       0.983,
	"vein":         0.867,
}

var ToneLightHigh = map[string]float64{
	"edge":         0.78,
	"fg":           0.18,
	"fg2":          0.398,
	"fg3":          0.474,
	"field":        0.937,
	"gold":         0.78,
	"goldInk":      0.487,
	"ground":       0.899,
	"jasper":       0.486,
	"mineralField": 0.92,
	"onGold":       0.1,
	"onSel":        0.989,
	"sel":          0.159,
	"slab":         0.999,
	"stone1":       0.942,
	"stone2":       0.995,
	"stone3":       0.937,
	"stone4":       0.999,
	"vein":         0.86,
}

var DyeBoost = map[string]float64{
	"gold":   1.4,
	"ground": 1.4,
	"vein":   1.2,
}

var DyeChroma = map[string]float64{
	"gold":   0.15,
	"ground": 0.02,
	"ink":    0.016,
	"jasper": 0.17,
	"onGold": 0.03,
	"onSel":  0.03,
	"sel":    0.03,
	"stone":  0.014,
}

var DyeFallback = map[string]float64{
	"goldChroma":   0.15,
	"goldHue":      41,
	"groundChroma": 0.004,
	"inkChroma":    0.016,
	"jasperHue":    16,
	"neutralHue":   72,
	"selChroma":    0.017,
	"stoneChroma":  0.007,
}

var DyeMineral = map[string]float64{
	"amber":  0.05,
	"jade":   0.045,
	"jasper": 0.055,
	"lapis":  0.05,
}

var DyeTint = map[string]float64{
	"low":    1.8,
	"medium": 2.6,
}

var Edition = map[string]string{
	"fedora": "#6ea5cf",
	"nixos":  "#9299d3",
	"void":   "#6daf8c",
}
