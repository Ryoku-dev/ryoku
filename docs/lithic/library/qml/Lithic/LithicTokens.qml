// Written by go run ./cmd/lithic-tokens from tokens.json. Do not edit by hand.
pragma Singleton

import QtQuick

QtObject {
    readonly property var alpha: ({"disabled":0.5,"hatch":0.3,"hoverFill":0.1,"hoverInk":0.1,"hoverPlane":0.06,"onSel2":0.2,"pressPlane":0.06,"pressRow":0.08,"scrimDark":0.62,"scrimLight":0.45,"selMix":0.5,"workingBase":0.55})
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
    readonly property var dye: ({"boost":{"gold":1.4,"ground":1.4,"vein":1.2},"chroma":{"gold":0.15,"ground":0.02,"ink":0.016,"jasper":0.17,"onGold":0.03,"onSel":0.03,"sel":0.03,"stone":0.014},"fallback":{"goldChroma":0.15,"goldHue":41,"groundChroma":0.004,"inkChroma":0.016,"jasperHue":16,"neutralHue":72,"selChroma":0.017,"stoneChroma":0.007},"mineral":{"amber":0.05,"jade":0.045,"jasper":0.055,"lapis":0.05},"tint":{"low":1.8,"medium":2.6}})
    readonly property var edition: ({"fedora":"#6ea5cf","nixos":"#9299d3","void":"#6daf8c"})
    readonly property QtObject font: QtObject {
        readonly property var icons: ["Material Symbols Sharp"]
        readonly property var mono: ["Martian Mono"]
        readonly property var sans: ["Geologica"]
    }
    readonly property var minerals: ({"dark":{"amber":"#d8ad52","jade":"#7ab8a0","lapis":"#8aa2d6"},"light":{"amber":"#855d00","jade":"#25715c","lapis":"#3d5f99"}})
    readonly property QtObject motion: QtObject {
        readonly property var curve: [0.2,0,0,1]
        readonly property int dialogIn: 300
        readonly property int dialogOut: 160
        readonly property int field: 320
        readonly property int focusIn: 120
        readonly property int focusOut: 100
        readonly property int highlight: 1600
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
    readonly property var palette: ({"accents":{"gold":"primary","jasper":"error","scrim":"scrim","sel":"secondaryContainer","selMix":"primaryContainer"},"dye":{"ink":"onSurface","neutral":"surface"}})
    readonly property var signature: ({"dark":{"edge":"#66625e","fg":"#cdc4ba","fg2":"#b0a9a0","fg3":"#958f87","field":"#303030","gold":"#ce5e32","goldInk":"#dc6a3e","ground":"#000000","jasper":"#eb596c","onGold":"#000000","onSel":"#000000","onSel2":"#292725","scrim":"#9e000000","sel":"#cdc4ba","slab":"#232323","stone1":"#141414","stone2":"#1b1b1b","stone3":"#232323","stone4":"#2c2c2c","vein":"#393939"},"darkHigh":{"edge":"#87827c","fg":"#e6dcd2","fg2":"#cdc4ba","fg3":"#b0a9a0","field":"#3a3a3a","gold":"#ce5e32","goldInk":"#ea784c","ground":"#000000","jasper":"#fa6777","onGold":"#000000","onSel":"#000000","onSel2":"#2e2c2a","scrim":"#9e000000","sel":"#e6dcd2","slab":"#292929","stone1":"#171717","stone2":"#1f1f1f","stone3":"#292929","stone4":"#333333","vein":"#4a4a4a"},"light":{"edge":"#c7c3be","fg":"#1e1a15","fg2":"#56504a","fg3":"#6b655f","field":"#e7e5e3","gold":"#ce5e32","goldInk":"#b04313","ground":"#dfdddb","jasper":"#ba334a","onGold":"#000000","onSel":"#f3f1ef","onSel2":"#c8c6c3","scrim":"#73000000","sel":"#1e1a15","slab":"#fbf9f7","stone1":"#e8e6e4","stone2":"#f3f1ef","stone3":"#e7e5e3","stone4":"#fbf9f7","vein":"#d6d3cf"},"lightHigh":{"edge":"#a29e99","fg":"#110c08","fg2":"#4c4641","fg3":"#605b55","field":"#e9e7e5","gold":"#ce5e32","goldInk":"#aa3d07","ground":"#d9d7d5","jasper":"#b32d45","onGold":"#000000","onSel":"#fdfbf9","onSel2":"#cecbc9","scrim":"#73000000","sel":"#110c08","slab":"#fefdfc","stone1":"#ebe9e7","stone2":"#fdfbf9","stone3":"#e9e7e5","stone4":"#fefdfc","vein":"#c0bdba"}})
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
        readonly property real designPreviewRatio: 1.6
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
    readonly property var tone: ({"dark":{"edge":0.45,"fg":0.951,"fg2":0.801,"fg3":0.689,"field":0.31,"gold":0.785,"goldInk":0.785,"ground":0.158,"jasper":0.74,"mineralField":0.305,"onGold":0.1,"onSel":0.1,"sel":0.825,"slab":0.246,"stone1":0.188,"stone2":0.216,"stone3":0.246,"stone4":0.278,"vein":0.391},"darkHigh":{"edge":0.535,"fg":0.976,"fg2":0.854,"fg3":0.772,"field":0.401,"gold":0.8,"goldInk":0.8,"ground":0.158,"jasper":0.78,"mineralField":0.345,"onGold":0.1,"onSel":0.08,"sel":0.9,"slab":0.299,"stone1":0.206,"stone2":0.251,"stone3":0.299,"stone4":0.35,"vein":0.45},"light":{"edge":0.818,"fg":0.221,"fg2":0.435,"fg3":0.508,"field":0.923,"gold":0.78,"goldInk":0.492,"ground":0.899,"jasper":0.495,"mineralField":0.925,"onGold":0.12,"onSel":0.959,"sel":0.221,"slab":0.983,"stone1":0.926,"stone2":0.959,"stone3":0.923,"stone4":0.983,"vein":0.867},"lightHigh":{"edge":0.78,"fg":0.18,"fg2":0.398,"fg3":0.474,"field":0.937,"gold":0.78,"goldInk":0.487,"ground":0.899,"jasper":0.486,"mineralField":0.92,"onGold":0.1,"onSel":0.989,"sel":0.159,"slab":0.999,"stone1":0.942,"stone2":0.995,"stone3":0.937,"stone4":0.999,"vein":0.86}})
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
            readonly property int size: 10
            readonly property int weight: 400
        }
        readonly property QtObject monoSm: QtObject {
            readonly property string family: "mono"
            readonly property int line: 15
            readonly property int size: 9
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
