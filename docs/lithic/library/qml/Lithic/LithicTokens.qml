// Written by go run ./cmd/lithic-tokens from tokens.json. Do not edit by hand.
pragma Singleton

import QtQuick

QtObject {
    readonly property var alpha: ({"contrastHigh":0.6,"contrastHighFg3":0.3,"disabled":0.5,"edge":0.5,"hatch":0.3,"hoverFill":0.1,"hoverInk":0.1,"hoverPlane":0.06,"mineral":0.15,"mineralJade":0.17,"onSel2":0.45,"pressPlane":0.06,"pressRow":0.08,"scrimDark":0.62,"scrimLight":0.45,"selMix":0.5,"tintLow":0.06,"tintMedium":0.1,"workingBase":0.55})
    readonly property QtObject breakpoint: QtObject {
        readonly property int expanded: 1280
        readonly property int medium: 760
    }
    readonly property QtObject contrast: QtObject {
        readonly property real minimum: 4.5
    }
    readonly property QtObject cut: QtObject {
        readonly property int large: 16
        readonly property int menu: 8
        readonly property int mid: 7
        readonly property int notch: 12
        readonly property int overlay: 8
        readonly property int placard: 8
        readonly property int segment: 5
        readonly property int small: 6
        readonly property int toggleKnob: 2
        readonly property int toggleTrack: 4
        readonly property int token: 3
        readonly property int tooltip: 4
    }
    readonly property var edition: ({"alpine":"#0d7fb0","arch":"#1793d1","cachyos":"#00b3a1","debian":"#d70a53","endeavouros":"#7f3fbf","fedora":"#51a2da","gentoo":"#7a6bbd","linuxmint":"#86be43","manjaro":"#35bf5c","nixos":"#7ebae4","opensuse":"#73ba25","pop":"#48b9c7","ubuntu":"#e95420","void":"#478061"})
    readonly property QtObject font: QtObject {
        readonly property var icons: ["Material Symbols Sharp"]
        readonly property var mono: ["Space Mono","SpaceMono Nerd Font","SpaceMono Nerd Font Mono"]
        readonly property var sans: ["Space Grotesk"]
    }
    readonly property var minerals: ({"dark":{"amber":"#e0954a","jade":"#7ab8a0","lapis":"#8aa2d6"},"light":{"amber":"#94520f","jade":"#2f7a64","lapis":"#3d5f99"}})
    readonly property QtObject motion: QtObject {
        readonly property var curve: [0.2,0,0,1]
        readonly property int dialogIn: 300
        readonly property int dialogOut: 160
        readonly property int field: 320
        readonly property int focusIn: 120
        readonly property int focusOut: 100
        readonly property int hoverIn: 160
        readonly property int hoverOut: 100
        readonly property int menuIn: 160
        readonly property int menuOut: 100
        readonly property int navigate: 200
        readonly property int notch: 200
        readonly property int placardIn: 300
        readonly property int placardLife: 5000
        readonly property int press: 90
        readonly property int select: 240
        readonly property int sweep: 1280
        readonly property int tooltipDelay: 600
        readonly property int tooltipIn: 300
        readonly property int unfold: 200
        readonly property int wheel: 140
        readonly property int working: 900
    }
    readonly property var palette: ({"accents":{"gold":"primary","jasper":"error","onGold":"onPrimary","onSel":"onSecondaryContainer","scrim":"scrim","sel":"secondaryContainer","selMix":"primaryContainer","tint":"primaryContainer"},"dark":{"fg":"onSurface","fg2":"onSurfaceVariant","fg3":"outline","field":"surfaceContainerHighest","ground":"surfaceContainerLowest","slab":"surfaceContainer","stone1":"surface","stone2":"surfaceContainerLow","stone3":"surfaceContainer","stone4":"surfaceContainerHigh","vein":"outlineVariant"},"light":{"fg":"onSurface","fg2":"onSurfaceVariant","fg3":"outline","field":"surfaceContainerHighest","ground":"surfaceContainerHighest","slab":"surfaceContainerLowest","stone1":"surfaceContainerHigh","stone2":"surfaceContainerLowest","stone3":"surfaceContainer","stone4":"surfaceContainerLowest","vein":"outlineVariant"},"toned":{"goldInk":"gold","jasper":"jasper"}})
    readonly property var signature: ({"dark":{"fg":"#e3e3e3","fg2":"#c6c6c6","fg3":"#8f8f8f","field":"#393939","gold":"#c9a861","goldInk":"#d6b877","ground":"#0e0e0e","jasper":"#e0553a","onGold":"#17120a","onSel":"#121212","onSel2":"#474747","scrim":"#9e000000","sel":"#e3e3e3","slab":"#232323","stone1":"#141414","stone2":"#1b1b1b","stone3":"#232323","stone4":"#2c2c2c","vein":"#444444"},"light":{"fg":"#1d1b18","fg2":"#4a4640","fg3":"#6f6a62","field":"#e9e5de","gold":"#c9a45a","goldInk":"#85651f","ground":"#e2ddd4","jasper":"#b8321e","onGold":"#1d1b18","onSel":"#f4f1eb","onSel2":"#b9b2a7","scrim":"#73000000","sel":"#1d1b18","slab":"#fbf9f5","stone1":"#eae6df","stone2":"#f4f1eb","stone3":"#e9e5de","stone4":"#fbf9f5","vein":"#cbc4b8"}})
    readonly property QtObject size: QtObject {
        readonly property real bladeIndeterminateFraction: 0.25
        readonly property int bladeMarker: 2
        readonly property real bladeMeterAlertAt: 0.9
        readonly property int bladeTrack: 6
        readonly property int boardColumnsAt: 860
        readonly property int boardMax: 1240
        readonly property real brandCutRatio: 0.22
        readonly property int brandMark: 40
        readonly property int brandMarkCompact: 21
        readonly property real brandOctagonApothemRatio: 0.28
        readonly property real brandOutline: 1.5
        readonly property real brandPointRatio: 0.045
        readonly property real brandPointerFarRatio: 0.31
        readonly property real brandPointerMidRatio: 0.14
        readonly property real brandPointerNearRatio: 0.06
        readonly property int catalogueDialogWidth: 720
        readonly property int catalogueGridGap: 4
        readonly property int catalogueTileHeight: 128
        readonly property int catalogueTileWidthMax: 260
        readonly property int catalogueTileWidthMin: 200
        readonly property int chartBaseline: 1
        readonly property int chartHatchGap: 6
        readonly property int chartHatchLine: 1
        readonly property real chartLine: 1.5
        readonly property int chartTick: 4
        readonly property int controlPressCutGrow: 4
        readonly property int controlPressSink: 1
        readonly property int dialogActionGap: 8
        readonly property int dialogActionsGap: 34
        readonly property int dialogConfirmFieldGap: 21
        readonly property int dialogEnterTravel: 12
        readonly property int dialogExitTravel: 8
        readonly property int dialogMargin: 16
        readonly property int dialogPadding: 34
        readonly property int dialogTitleGap: 21
        readonly property int dialogWidth: 520
        readonly property int draftAsideBelow: 600
        readonly property real draftAsideFraction: 0.36
        readonly property int draftAsideMax: 400
        readonly property int draftAsideMin: 300
        readonly property int draftFormMax: 600
        readonly property int draftWideAt: 820
        readonly property int emptyBodyMax: 420
        readonly property int emptyFrame: 72
        readonly property int emptyIcon: 30
        readonly property int factsLabelWidth: 140
        readonly property int factsRowHeight: 44
        readonly property int filterCountGap: 6
        readonly property int filterGap: 4
        readonly property int filterHeight: 32
        readonly property int filterInset: 13
        readonly property real flakeHeightRatio: 0.7
        readonly property int flakeNominal: 9
        readonly property real flakeOutline: 1.5
        readonly property real flakeWidthRatio: 1.4
        readonly property int focusCutGrow: 2
        readonly property int focusRingGap: 2
        readonly property int focusRingWidth: 2
        readonly property int groupHeaderBottom: 6
        readonly property int groupHeaderLeft: 6
        readonly property int groupHeaderRight: 4
        readonly property int groupHeaderTop: 14
        readonly property int hairline: 1
        readonly property int headerActionsGap: 21
        readonly property int headerBottom: 26
        readonly property int headerContentGap: 21
        readonly property int headerMark: 44
        readonly property int headerMarkGap: 13
        readonly property int headerMarksGap: 4
        readonly property int headerNameCompactAt: 640
        readonly property int headerNoteGap: 4
        readonly property int headerProofGap: 8
        readonly property int headerSide: 36
        readonly property int headerTop: 30
        readonly property int iconPlaque: 40
        readonly property int iconPlaqueIcon: 20
        readonly property int iconPlaqueIconSmall: 18
        readonly property int iconPlaqueSmall: 32
        readonly property int listExpanded: 384
        readonly property int listInset: 10
        readonly property int listMedium: 320
        readonly property int menuDividerInset: 8
        readonly property int menuIcon: 20
        readonly property int menuIconGap: 12
        readonly property int menuInset: 16
        readonly property int menuItem: 40
        readonly property int menuPaddingVertical: 6
        readonly property int menuShortcutGap: 13
        readonly property int menuWidthMax: 360
        readonly property int menuWidthMin: 220
        readonly property int newPlaqueHeight: 52
        readonly property int newPlaqueHeightCompact: 40
        readonly property int newPlaqueIcon: 21
        readonly property int newPlaqueIconCompact: 20
        readonly property int newPlaqueLabelGap: 2
        readonly property int newPlaqueWidth: 62
        readonly property int newPlaqueWidthCompact: 48
        readonly property int paneBodyGap: 8
        readonly property int paneDecorStrip: 32
        readonly property int paneGap: 8
        readonly property int paneRightWeb: 10
        readonly property int paneTop: 10
        readonly property int pickerArrow: 20
        readonly property int pickerArrowGap: 10
        readonly property int pickerHeight: 44
        readonly property int pickerInset: 14
        readonly property int pickerWidthMin: 160
        readonly property int pinnedTileMinHeight: 154
        readonly property int placardActionHeight: 32
        readonly property int placardActionInset: 13
        readonly property int placardBlade: 2
        readonly property int placardBottom: 24
        readonly property int placardHeight: 48
        readonly property int placardInset: 16
        readonly property int placardRise: 8
        readonly property int placardStackCount: 3
        readonly property int placardStackGap: 8
        readonly property int placardTrailingInset: 8
        readonly property int placardWidthMax: 560
        readonly property int plaqueGap: 8
        readonly property int plaqueGapSmall: 6
        readonly property int plaqueHeight: 40
        readonly property int plaqueHeightSmall: 32
        readonly property int plaqueIcon: 19
        readonly property int plaqueIconSmall: 17
        readonly property int plaqueInset: 18
        readonly property int plaqueInsetSmall: 13
        readonly property int popoverAnchorGap: 8
        readonly property int popoverEnterTravel: 6
        readonly property int popoverExitTravel: 4
        readonly property int railBottomColumns: 5
        readonly property int railBottomHeight: 64
        readonly property int railDestinationCompact: 48
        readonly property int railDestinationExpanded: 64
        readonly property int railDestinationIcon: 21
        readonly property int railDestinationMedium: 56
        readonly property int railExpanded: 88
        readonly property int railHomeExpanded: 56
        readonly property int railHomeMedium: 48
        readonly property int railHomeTop: 16
        readonly property int railItemBottomInset: 16
        readonly property int railItemLabelGap: 5
        readonly property int railMedium: 72
        readonly property int railNewYExpanded: 168
        readonly property int railNewYMedium: 152
        readonly property int railStartExpanded: 96
        readonly property int railStartMedium: 88
        readonly property int railTileGap: 8
        readonly property int rowListGap: 4
        readonly property int rowListWheelNotch: 120
        readonly property int scrollThumb: 3
        readonly property int scrollThumbMin: 12
        readonly property int searchClear: 32
        readonly property int searchClearInset: 6
        readonly property int searchHeight: 44
        readonly property int searchIcon: 20
        readonly property int searchIconInset: 14
        readonly property int searchLeading: 42
        readonly property int searchWidth: 280
        readonly property int sectionIndexGap: 34
        readonly property int sectionIndexInset: 13
        readonly property int sectionIndexRow: 48
        readonly property int sectionIndexRowGap: 4
        readonly property int sectionIndexWidth: 200
        readonly property int segmentHeight: 36
        readonly property int segmentInset: 4
        readonly property int selBlade: 3
        readonly property int settingsControlMax: 320
        readonly property int settingsDetailGap: 2
        readonly property int settingsExpandedMax: 994
        readonly property int settingsInsetSide: 36
        readonly property int settingsInsetVertical: 34
        readonly property int settingsMediumMax: 760
        readonly property int settingsRowGap: 13
        readonly property int skeletonBarGap: 8
        readonly property int skeletonBarHeight: 8
        readonly property real skeletonBarLongFraction: 0.6
        readonly property real skeletonBarShortFraction: 0.35
        readonly property int skeletonInset: 16
        readonly property int slabBelow: 21
        readonly property int slabInset: 21
        readonly property int slabTitleGap: 13
        readonly property int sliderHandleHeight: 20
        readonly property int sliderHandleWidth: 8
        readonly property int sliderHeight: 32
        readonly property real sliderKeyboardStep: 0.05
        readonly property int sliderTrack: 6
        readonly property int sliderWidth: 160
        readonly property int splitPlaqueGap: 2
        readonly property int splitPlaqueMoreWidth: 40
        readonly property int stepperGap: 8
        readonly property int stepperUnitGap: 4
        readonly property int tabletHeight: 56
        readonly property int tabletHeightCompact: 48
        readonly property int tabletInsetLeft: 16
        readonly property int tabletInsetRight: 12
        readonly property int tabletMark: 32
        readonly property int tabletMarkGap: 13
        readonly property int tabletProgress: 2
        readonly property int tabletTrailingMin: 72
        readonly property int tabsBaseline: 1
        readonly property int tabsBlade: 3
        readonly property int tabsHeight: 44
        readonly property int tabsLabelExtra: 24
        readonly property int tabsMaxCount: 5
        readonly property int textFieldDefaultWidth: 240
        readonly property int textFieldHeight: 44
        readonly property int textFieldHelpGap: 4
        readonly property int textFieldInset: 14
        readonly property int textFieldLabelGap: 4
        readonly property int textFieldSecretAction: 32
        readonly property int textFieldSecretInset: 6
        readonly property int textFieldStroke: 1
        readonly property int toggleHeight: 22
        readonly property int toggleInset: 4
        readonly property int toggleKnob: 14
        readonly property int toggleLabelGap: 8
        readonly property int toggleWidth: 40
        readonly property int tooltipAnchorGap: 8
        readonly property int tooltipPaddingHorizontal: 8
        readonly property int tooltipPaddingVertical: 6
        readonly property int tooltipTravel: 8
        readonly property int tooltipWidthMax: 320
        readonly property int windowDefaultHeight: 800
        readonly property int windowDefaultWidth: 1280
        readonly property int windowMinHeight: 520
        readonly property int windowMinWidth: 640
        readonly property int workContentInset: 34
        readonly property int workPaneRise: 8
        readonly property int workingFlakeCount: 3
        readonly property real workingFlakeGapRatio: 0.1
        readonly property real workingFlakeHeightRatio: 0.5
        readonly property real workingFlakeWidthRatio: 0.2
    }
    readonly property QtObject space: QtObject {
        readonly property int s1: 4
        readonly property int s2: 8
        readonly property int s3: 13
        readonly property int s4: 21
        readonly property int s5: 34
        readonly property int s6: 55
    }
    readonly property QtObject type: QtObject {
        readonly property QtObject body: QtObject {
            readonly property string family: "sans"
            readonly property int line: 20
            readonly property int size: 14
            readonly property int weight: 400
        }
        readonly property QtObject caption: QtObject {
            readonly property string family: "sans"
            readonly property int line: 16
            readonly property int size: 12
            readonly property int weight: 400
        }
        readonly property QtObject detail: QtObject {
            readonly property string family: "sans"
            readonly property int line: 18
            readonly property int size: 13
            readonly property int weight: 400
        }
        readonly property QtObject group: QtObject {
            readonly property string family: "sans"
            readonly property int line: 16
            readonly property int size: 12
            readonly property int weight: 500
        }
        readonly property QtObject label: QtObject {
            readonly property string family: "sans"
            readonly property int line: 16
            readonly property int size: 13
            readonly property int weight: 500
        }
        readonly property QtObject mono: QtObject {
            readonly property string family: "mono"
            readonly property int line: 16
            readonly property int size: 12
            readonly property int weight: 400
        }
        readonly property QtObject monoSm: QtObject {
            readonly property string family: "mono"
            readonly property int line: 15
            readonly property int size: 11
            readonly property int weight: 400
        }
        readonly property QtObject name: QtObject {
            readonly property string family: "sans"
            readonly property int line: 56
            readonly property int size: 52
            readonly property int weight: 700
        }
        readonly property QtObject proof: QtObject {
            readonly property string family: "sans"
            readonly property int line: 21
            readonly property int size: 15
            readonly property int weight: 400
        }
        readonly property QtObject row: QtObject {
            readonly property string family: "sans"
            readonly property int line: 20
            readonly property int size: 15
            readonly property int weight: 500
        }
        readonly property QtObject section: QtObject {
            readonly property string family: "sans"
            readonly property int line: 28
            readonly property int size: 21
            readonly property int weight: 700
        }
        readonly property QtObject sentence: QtObject {
            readonly property string family: "sans"
            readonly property int line: 40
            readonly property int size: 34
            readonly property int weight: 700
        }
        readonly property QtObject title: QtObject {
            readonly property string family: "sans"
            readonly property int line: 20
            readonly property int size: 15
            readonly property int weight: 500
        }
    }
}
