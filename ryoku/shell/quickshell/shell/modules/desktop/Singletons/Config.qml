pragma Singleton
import QtQuick
import Ryoku.Ui.Singletons
import Quickshell
import Quickshell.Io
import "WidgetKeys.js" as WidgetKeys

// Live desktop widget config. The legacy top-level values remain the fallback;
// Stage edits add output-keyed overrides in the same widgets.json file.
// FileView watches it so every desktop follows the next write.
Singleton {
    id: root
    property bool ready: false

    // -- clock ---------------------------------------------------------------
    property alias clockEnabled: adapter.clockEnabled
    property alias clockDesign:  adapter.clockDesign   // digital | minimal | analog | flip | rings
    readonly property bool clock24h: TimeFormat.is24h
    property alias clockSeconds: adapter.clockSeconds
    property alias clockAccent:  adapter.clockAccent   // palette | brand | mono
    property alias clockScale:   adapter.clockScale
    property alias clockAnchor:  adapter.clockAnchor   // top-left .. center .. bottom-right | free
    property alias clockX:       adapter.clockX        // free placement, monitor pixels
    property alias clockY:       adapter.clockY
    property alias clockLocked:  adapter.clockLocked   // prevent drag/resize
    property alias clockOpacity: adapter.clockOpacity
    property alias clockBg:      adapter.clockBg        // none | card | glass
    property alias clockRadius:  adapter.clockRadius
    // per-widget ink colour: "" follows the wallpaper (adaptive), a hex pins a
    // solid fill, and Gradient blends <widget>Color -> <widget>Color2 across the
    // rendered glyphs. Only the bare (bg:none) widgets read these.
    property alias clockColor:    adapter.clockColor
    property alias clockColor2:   adapter.clockColor2
    property alias clockGradient: adapter.clockGradient
    property alias widgetDepths: adapter.widgetDepths
    property alias dateShow:     adapter.dateShow
    property alias dateDesign:   adapter.dateDesign     // inline | badge | stacked

    // widget typography: the sans family every clock face and Theme.font widget
    // renders with. Empty = the built-in default (Space Grotesk); a bundled or
    // installed family name overrides it. Picked in Hub -> Widgets.
    property alias widgetFont: adapter.widgetFont

    property alias calendarEnabled:       adapter.calendarEnabled
    property alias calendarStyle:         adapter.calendarStyle
    property alias calendarWeeks:         adapter.calendarWeeks
    property alias calendarWeekNumbers:   adapter.calendarWeekNumbers
    property alias calendarHolidayRegion: adapter.calendarHolidayRegion
    property alias calendarScale:         adapter.calendarScale
    property alias calendarAnchor:        adapter.calendarAnchor
    property alias calendarX:             adapter.calendarX
    property alias calendarY:             adapter.calendarY
    property alias calendarLocked:        adapter.calendarLocked
    property alias calendarOpacity:       adapter.calendarOpacity
    property alias calendarColor:    adapter.calendarColor
    property alias calendarColor2:   adapter.calendarColor2
    property alias calendarGradient: adapter.calendarGradient

    property alias musicEnabled: adapter.musicEnabled
    property alias musicStyle:   adapter.musicStyle    // cover | glass
    property alias musicLyrics:  adapter.musicLyrics   // show the synced lyric sheet
    property alias musicViz:     adapter.musicViz     // bars | wave (no-lyrics visualiser look)
    property alias musicScale:   adapter.musicScale
    property alias musicAnchor:  adapter.musicAnchor
    property alias musicX:       adapter.musicX
    property alias musicY:       adapter.musicY
    property alias musicLocked:  adapter.musicLocked
    property alias musicOpacity: adapter.musicOpacity
    property alias musicApp:     adapter.musicApp     // launch command for the corner button
    property alias musicShape:     adapter.musicShape      // wide | tall (9:16)
    property alias musicVideo:     adapter.musicVideo      // off | canvas | custom
    property alias musicVideoFile: adapter.musicVideoFile  // custom backdrop file
    property alias musicColor:    adapter.musicColor
    property alias musicColor2:   adapter.musicColor2
    property alias musicGradient: adapter.musicGradient

    property alias aioEnabled: adapter.aioEnabled
    property alias aioStyle:   adapter.aioStyle     // wide | tall
    property alias aioScale:   adapter.aioScale
    property alias aioAnchor:  adapter.aioAnchor
    property alias aioX:       adapter.aioX
    property alias aioY:       adapter.aioY
    property alias aioLocked:  adapter.aioLocked
    property alias aioOpacity: adapter.aioOpacity
    property alias aioColor:    adapter.aioColor
    property alias aioColor2:   adapter.aioColor2
    property alias aioGradient: adapter.aioGradient

    property alias statsEnabled: adapter.statsEnabled
    property alias statsScale:   adapter.statsScale
    property alias statsAnchor:  adapter.statsAnchor
    property alias statsX:       adapter.statsX
    property alias statsY:       adapter.statsY
    property alias statsLocked:  adapter.statsLocked
    property alias statsOpacity: adapter.statsOpacity
    property alias statsColor:    adapter.statsColor
    property alias statsColor2:   adapter.statsColor2
    property alias statsGradient: adapter.statsGradient

    property alias weatherEnabled: adapter.weatherEnabled
    property alias weatherDesign:  adapter.weatherDesign   // compact | full
    property alias weatherScale:   adapter.weatherScale
    property alias weatherAnchor:  adapter.weatherAnchor
    property alias weatherX:       adapter.weatherX
    property alias weatherY:       adapter.weatherY
    property alias weatherLocked:  adapter.weatherLocked
    property alias weatherOpacity: adapter.weatherOpacity
    property alias weatherColor:    adapter.weatherColor
    property alias weatherColor2:   adapter.weatherColor2
    property alias weatherGradient: adapter.weatherGradient

    property alias notesEnabled: adapter.notesEnabled
    property alias notesScale:   adapter.notesScale
    property alias notesAnchor:  adapter.notesAnchor
    property alias notesX:       adapter.notesX
    property alias notesY:       adapter.notesY
    property alias notesLocked:  adapter.notesLocked
    property alias notesOpacity: adapter.notesOpacity
    property alias notesWidth:   adapter.notesWidth   // pad size in logical px, before scale
    property alias notesHeight:  adapter.notesHeight
    property alias notesColor:    adapter.notesColor
    property alias notesColor2:   adapter.notesColor2
    property alias notesGradient: adapter.notesGradient

    // dayProgress + shape: the two iRiS desktop widgets folded into Ryoku's
    // roster. Both carry the shared must-have contract (enable, placement, size,
    // lock, opacity, colour) plus their own look keys.
    property alias dayprogressEnabled:  adapter.dayprogressEnabled
    property alias dayprogressStyle:    adapter.dayprogressStyle   // ring | arc
    property alias dayprogressShowDate: adapter.dayprogressShowDate
    property alias dayprogressScale:    adapter.dayprogressScale
    property alias dayprogressAnchor:   adapter.dayprogressAnchor
    property alias dayprogressX:        adapter.dayprogressX
    property alias dayprogressY:        adapter.dayprogressY
    property alias dayprogressLocked:   adapter.dayprogressLocked
    property alias dayprogressOpacity:  adapter.dayprogressOpacity
    property alias dayprogressColor:    adapter.dayprogressColor
    property alias dayprogressColor2:   adapter.dayprogressColor2
    property alias dayprogressGradient: adapter.dayprogressGradient

    property alias shapeEnabled:  adapter.shapeEnabled
    property alias shapeKind:     adapter.shapeKind          // dot | ring | diamond | square
    property alias shapeOutline:  adapter.shapeOutline
    property alias shapeScale:    adapter.shapeScale
    property alias shapeAnchor:   adapter.shapeAnchor
    property alias shapeX:        adapter.shapeX
    property alias shapeY:        adapter.shapeY
    property alias shapeLocked:   adapter.shapeLocked
    property alias shapeOpacity:  adapter.shapeOpacity
    property alias shapeColor:    adapter.shapeColor
    property alias shapeColor2:   adapter.shapeColor2
    property alias shapeGradient: adapter.shapeGradient

    // ii desktop widgets: the vendored ii roster, one slot contract each.
    property alias iiClockCookieEnabled: adapter.iiClockCookieEnabled; property alias iiClockCookieAnchor: adapter.iiClockCookieAnchor; property alias iiClockCookieX: adapter.iiClockCookieX; property alias iiClockCookieY: adapter.iiClockCookieY
    property alias iiClockCookieLocked: adapter.iiClockCookieLocked; property alias iiClockCookieScale: adapter.iiClockCookieScale; property alias iiClockCookieOpacity: adapter.iiClockCookieOpacity; property alias iiClockCookieBg: adapter.iiClockCookieBg
    property alias iiClockCookieColor: adapter.iiClockCookieColor; property alias iiClockCookieColor2: adapter.iiClockCookieColor2; property alias iiClockCookieStyle: adapter.iiClockCookieStyle; property alias iiClockCookieRadius: adapter.iiClockCookieRadius
    property alias iiClockCookiePad: adapter.iiClockCookiePad; property alias iiClockCookieBorder: adapter.iiClockCookieBorder; property alias iiClockCookieBorderOpacity: adapter.iiClockCookieBorderOpacity; property alias iiClockCookieBackingOpacity: adapter.iiClockCookieBackingOpacity; property alias iiClockCookieOpts: adapter.iiClockCookieOpts
    property alias iiClockDigitalEnabled: adapter.iiClockDigitalEnabled; property alias iiClockDigitalAnchor: adapter.iiClockDigitalAnchor; property alias iiClockDigitalX: adapter.iiClockDigitalX; property alias iiClockDigitalY: adapter.iiClockDigitalY
    property alias iiClockDigitalLocked: adapter.iiClockDigitalLocked; property alias iiClockDigitalScale: adapter.iiClockDigitalScale; property alias iiClockDigitalOpacity: adapter.iiClockDigitalOpacity; property alias iiClockDigitalBg: adapter.iiClockDigitalBg
    property alias iiClockDigitalColor: adapter.iiClockDigitalColor; property alias iiClockDigitalColor2: adapter.iiClockDigitalColor2; property alias iiClockDigitalStyle: adapter.iiClockDigitalStyle; property alias iiClockDigitalRadius: adapter.iiClockDigitalRadius
    property alias iiClockDigitalPad: adapter.iiClockDigitalPad; property alias iiClockDigitalBorder: adapter.iiClockDigitalBorder; property alias iiClockDigitalBorderOpacity: adapter.iiClockDigitalBorderOpacity; property alias iiClockDigitalBackingOpacity: adapter.iiClockDigitalBackingOpacity; property alias iiClockDigitalOpts: adapter.iiClockDigitalOpts
    property alias iiClockNagasakiEnabled: adapter.iiClockNagasakiEnabled; property alias iiClockNagasakiAnchor: adapter.iiClockNagasakiAnchor; property alias iiClockNagasakiX: adapter.iiClockNagasakiX; property alias iiClockNagasakiY: adapter.iiClockNagasakiY
    property alias iiClockNagasakiLocked: adapter.iiClockNagasakiLocked; property alias iiClockNagasakiScale: adapter.iiClockNagasakiScale; property alias iiClockNagasakiOpacity: adapter.iiClockNagasakiOpacity; property alias iiClockNagasakiBg: adapter.iiClockNagasakiBg
    property alias iiClockNagasakiColor: adapter.iiClockNagasakiColor; property alias iiClockNagasakiColor2: adapter.iiClockNagasakiColor2; property alias iiClockNagasakiStyle: adapter.iiClockNagasakiStyle; property alias iiClockNagasakiRadius: adapter.iiClockNagasakiRadius
    property alias iiClockNagasakiPad: adapter.iiClockNagasakiPad; property alias iiClockNagasakiBorder: adapter.iiClockNagasakiBorder; property alias iiClockNagasakiBorderOpacity: adapter.iiClockNagasakiBorderOpacity; property alias iiClockNagasakiBackingOpacity: adapter.iiClockNagasakiBackingOpacity; property alias iiClockNagasakiOpts: adapter.iiClockNagasakiOpts
    property alias iiClockFlexEnabled: adapter.iiClockFlexEnabled; property alias iiClockFlexAnchor: adapter.iiClockFlexAnchor; property alias iiClockFlexX: adapter.iiClockFlexX; property alias iiClockFlexY: adapter.iiClockFlexY
    property alias iiClockFlexLocked: adapter.iiClockFlexLocked; property alias iiClockFlexScale: adapter.iiClockFlexScale; property alias iiClockFlexOpacity: adapter.iiClockFlexOpacity; property alias iiClockFlexBg: adapter.iiClockFlexBg
    property alias iiClockFlexColor: adapter.iiClockFlexColor; property alias iiClockFlexColor2: adapter.iiClockFlexColor2; property alias iiClockFlexStyle: adapter.iiClockFlexStyle; property alias iiClockFlexRadius: adapter.iiClockFlexRadius
    property alias iiClockFlexPad: adapter.iiClockFlexPad; property alias iiClockFlexBorder: adapter.iiClockFlexBorder; property alias iiClockFlexBorderOpacity: adapter.iiClockFlexBorderOpacity; property alias iiClockFlexBackingOpacity: adapter.iiClockFlexBackingOpacity; property alias iiClockFlexOpts: adapter.iiClockFlexOpts
    property alias iiClockHoriEnabled: adapter.iiClockHoriEnabled; property alias iiClockHoriAnchor: adapter.iiClockHoriAnchor; property alias iiClockHoriX: adapter.iiClockHoriX; property alias iiClockHoriY: adapter.iiClockHoriY
    property alias iiClockHoriLocked: adapter.iiClockHoriLocked; property alias iiClockHoriScale: adapter.iiClockHoriScale; property alias iiClockHoriOpacity: adapter.iiClockHoriOpacity; property alias iiClockHoriBg: adapter.iiClockHoriBg
    property alias iiClockHoriColor: adapter.iiClockHoriColor; property alias iiClockHoriColor2: adapter.iiClockHoriColor2; property alias iiClockHoriStyle: adapter.iiClockHoriStyle; property alias iiClockHoriRadius: adapter.iiClockHoriRadius
    property alias iiClockHoriPad: adapter.iiClockHoriPad; property alias iiClockHoriBorder: adapter.iiClockHoriBorder; property alias iiClockHoriBorderOpacity: adapter.iiClockHoriBorderOpacity; property alias iiClockHoriBackingOpacity: adapter.iiClockHoriBackingOpacity; property alias iiClockHoriOpts: adapter.iiClockHoriOpts
    property alias iiClockIosEnabled: adapter.iiClockIosEnabled; property alias iiClockIosAnchor: adapter.iiClockIosAnchor; property alias iiClockIosX: adapter.iiClockIosX; property alias iiClockIosY: adapter.iiClockIosY
    property alias iiClockIosLocked: adapter.iiClockIosLocked; property alias iiClockIosScale: adapter.iiClockIosScale; property alias iiClockIosOpacity: adapter.iiClockIosOpacity; property alias iiClockIosBg: adapter.iiClockIosBg
    property alias iiClockIosColor: adapter.iiClockIosColor; property alias iiClockIosColor2: adapter.iiClockIosColor2; property alias iiClockIosStyle: adapter.iiClockIosStyle; property alias iiClockIosRadius: adapter.iiClockIosRadius
    property alias iiClockIosPad: adapter.iiClockIosPad; property alias iiClockIosBorder: adapter.iiClockIosBorder; property alias iiClockIosBorderOpacity: adapter.iiClockIosBorderOpacity; property alias iiClockIosBackingOpacity: adapter.iiClockIosBackingOpacity; property alias iiClockIosOpts: adapter.iiClockIosOpts
    property alias iiClockNothingEnabled: adapter.iiClockNothingEnabled; property alias iiClockNothingAnchor: adapter.iiClockNothingAnchor; property alias iiClockNothingX: adapter.iiClockNothingX; property alias iiClockNothingY: adapter.iiClockNothingY
    property alias iiClockNothingLocked: adapter.iiClockNothingLocked; property alias iiClockNothingScale: adapter.iiClockNothingScale; property alias iiClockNothingOpacity: adapter.iiClockNothingOpacity; property alias iiClockNothingBg: adapter.iiClockNothingBg
    property alias iiClockNothingColor: adapter.iiClockNothingColor; property alias iiClockNothingColor2: adapter.iiClockNothingColor2; property alias iiClockNothingStyle: adapter.iiClockNothingStyle; property alias iiClockNothingRadius: adapter.iiClockNothingRadius
    property alias iiClockNothingPad: adapter.iiClockNothingPad; property alias iiClockNothingBorder: adapter.iiClockNothingBorder; property alias iiClockNothingBorderOpacity: adapter.iiClockNothingBorderOpacity; property alias iiClockNothingBackingOpacity: adapter.iiClockNothingBackingOpacity; property alias iiClockNothingOpts: adapter.iiClockNothingOpts
    property alias iiNothingWheelClockEnabled: adapter.iiNothingWheelClockEnabled; property alias iiNothingWheelClockAnchor: adapter.iiNothingWheelClockAnchor; property alias iiNothingWheelClockX: adapter.iiNothingWheelClockX; property alias iiNothingWheelClockY: adapter.iiNothingWheelClockY
    property alias iiNothingWheelClockLocked: adapter.iiNothingWheelClockLocked; property alias iiNothingWheelClockScale: adapter.iiNothingWheelClockScale; property alias iiNothingWheelClockOpacity: adapter.iiNothingWheelClockOpacity; property alias iiNothingWheelClockBg: adapter.iiNothingWheelClockBg
    property alias iiNothingWheelClockColor: adapter.iiNothingWheelClockColor; property alias iiNothingWheelClockColor2: adapter.iiNothingWheelClockColor2; property alias iiNothingWheelClockStyle: adapter.iiNothingWheelClockStyle; property alias iiNothingWheelClockRadius: adapter.iiNothingWheelClockRadius
    property alias iiNothingWheelClockPad: adapter.iiNothingWheelClockPad; property alias iiNothingWheelClockBorder: adapter.iiNothingWheelClockBorder; property alias iiNothingWheelClockBorderOpacity: adapter.iiNothingWheelClockBorderOpacity; property alias iiNothingWheelClockBackingOpacity: adapter.iiNothingWheelClockBackingOpacity; property alias iiNothingWheelClockOpts: adapter.iiNothingWheelClockOpts
    property alias iiNagasakiTextEnabled: adapter.iiNagasakiTextEnabled; property alias iiNagasakiTextAnchor: adapter.iiNagasakiTextAnchor; property alias iiNagasakiTextX: adapter.iiNagasakiTextX; property alias iiNagasakiTextY: adapter.iiNagasakiTextY
    property alias iiNagasakiTextLocked: adapter.iiNagasakiTextLocked; property alias iiNagasakiTextScale: adapter.iiNagasakiTextScale; property alias iiNagasakiTextOpacity: adapter.iiNagasakiTextOpacity; property alias iiNagasakiTextBg: adapter.iiNagasakiTextBg
    property alias iiNagasakiTextColor: adapter.iiNagasakiTextColor; property alias iiNagasakiTextColor2: adapter.iiNagasakiTextColor2; property alias iiNagasakiTextStyle: adapter.iiNagasakiTextStyle; property alias iiNagasakiTextRadius: adapter.iiNagasakiTextRadius
    property alias iiNagasakiTextPad: adapter.iiNagasakiTextPad; property alias iiNagasakiTextBorder: adapter.iiNagasakiTextBorder; property alias iiNagasakiTextBorderOpacity: adapter.iiNagasakiTextBorderOpacity; property alias iiNagasakiTextBackingOpacity: adapter.iiNagasakiTextBackingOpacity; property alias iiNagasakiTextOpts: adapter.iiNagasakiTextOpts
    property alias iiClockWordEnabled: adapter.iiClockWordEnabled; property alias iiClockWordAnchor: adapter.iiClockWordAnchor; property alias iiClockWordX: adapter.iiClockWordX; property alias iiClockWordY: adapter.iiClockWordY
    property alias iiClockWordLocked: adapter.iiClockWordLocked; property alias iiClockWordScale: adapter.iiClockWordScale; property alias iiClockWordOpacity: adapter.iiClockWordOpacity; property alias iiClockWordBg: adapter.iiClockWordBg
    property alias iiClockWordColor: adapter.iiClockWordColor; property alias iiClockWordColor2: adapter.iiClockWordColor2; property alias iiClockWordStyle: adapter.iiClockWordStyle; property alias iiClockWordRadius: adapter.iiClockWordRadius
    property alias iiClockWordPad: adapter.iiClockWordPad; property alias iiClockWordBorder: adapter.iiClockWordBorder; property alias iiClockWordBorderOpacity: adapter.iiClockWordBorderOpacity; property alias iiClockWordBackingOpacity: adapter.iiClockWordBackingOpacity; property alias iiClockWordOpts: adapter.iiClockWordOpts
    property alias iiClockDialEnabled: adapter.iiClockDialEnabled; property alias iiClockDialAnchor: adapter.iiClockDialAnchor; property alias iiClockDialX: adapter.iiClockDialX; property alias iiClockDialY: adapter.iiClockDialY
    property alias iiClockDialLocked: adapter.iiClockDialLocked; property alias iiClockDialScale: adapter.iiClockDialScale; property alias iiClockDialOpacity: adapter.iiClockDialOpacity; property alias iiClockDialBg: adapter.iiClockDialBg
    property alias iiClockDialColor: adapter.iiClockDialColor; property alias iiClockDialColor2: adapter.iiClockDialColor2; property alias iiClockDialStyle: adapter.iiClockDialStyle; property alias iiClockDialRadius: adapter.iiClockDialRadius
    property alias iiClockDialPad: adapter.iiClockDialPad; property alias iiClockDialBorder: adapter.iiClockDialBorder; property alias iiClockDialBorderOpacity: adapter.iiClockDialBorderOpacity; property alias iiClockDialBackingOpacity: adapter.iiClockDialBackingOpacity; property alias iiClockDialOpts: adapter.iiClockDialOpts
    property alias iiClockWearosEnabled: adapter.iiClockWearosEnabled; property alias iiClockWearosAnchor: adapter.iiClockWearosAnchor; property alias iiClockWearosX: adapter.iiClockWearosX; property alias iiClockWearosY: adapter.iiClockWearosY
    property alias iiClockWearosLocked: adapter.iiClockWearosLocked; property alias iiClockWearosScale: adapter.iiClockWearosScale; property alias iiClockWearosOpacity: adapter.iiClockWearosOpacity; property alias iiClockWearosBg: adapter.iiClockWearosBg
    property alias iiClockWearosColor: adapter.iiClockWearosColor; property alias iiClockWearosColor2: adapter.iiClockWearosColor2; property alias iiClockWearosStyle: adapter.iiClockWearosStyle; property alias iiClockWearosRadius: adapter.iiClockWearosRadius
    property alias iiClockWearosPad: adapter.iiClockWearosPad; property alias iiClockWearosBorder: adapter.iiClockWearosBorder; property alias iiClockWearosBorderOpacity: adapter.iiClockWearosBorderOpacity; property alias iiClockWearosBackingOpacity: adapter.iiClockWearosBackingOpacity; property alias iiClockWearosOpts: adapter.iiClockWearosOpts
    property alias iiWearosArcClockEnabled: adapter.iiWearosArcClockEnabled; property alias iiWearosArcClockAnchor: adapter.iiWearosArcClockAnchor; property alias iiWearosArcClockX: adapter.iiWearosArcClockX; property alias iiWearosArcClockY: adapter.iiWearosArcClockY
    property alias iiWearosArcClockLocked: adapter.iiWearosArcClockLocked; property alias iiWearosArcClockScale: adapter.iiWearosArcClockScale; property alias iiWearosArcClockOpacity: adapter.iiWearosArcClockOpacity; property alias iiWearosArcClockBg: adapter.iiWearosArcClockBg
    property alias iiWearosArcClockColor: adapter.iiWearosArcClockColor; property alias iiWearosArcClockColor2: adapter.iiWearosArcClockColor2; property alias iiWearosArcClockStyle: adapter.iiWearosArcClockStyle; property alias iiWearosArcClockRadius: adapter.iiWearosArcClockRadius
    property alias iiWearosArcClockPad: adapter.iiWearosArcClockPad; property alias iiWearosArcClockBorder: adapter.iiWearosArcClockBorder; property alias iiWearosArcClockBorderOpacity: adapter.iiWearosArcClockBorderOpacity; property alias iiWearosArcClockBackingOpacity: adapter.iiWearosArcClockBackingOpacity; property alias iiWearosArcClockOpts: adapter.iiWearosArcClockOpts
    property alias iiConcentricClockEnabled: adapter.iiConcentricClockEnabled; property alias iiConcentricClockAnchor: adapter.iiConcentricClockAnchor; property alias iiConcentricClockX: adapter.iiConcentricClockX; property alias iiConcentricClockY: adapter.iiConcentricClockY
    property alias iiConcentricClockLocked: adapter.iiConcentricClockLocked; property alias iiConcentricClockScale: adapter.iiConcentricClockScale; property alias iiConcentricClockOpacity: adapter.iiConcentricClockOpacity; property alias iiConcentricClockBg: adapter.iiConcentricClockBg
    property alias iiConcentricClockColor: adapter.iiConcentricClockColor; property alias iiConcentricClockColor2: adapter.iiConcentricClockColor2; property alias iiConcentricClockStyle: adapter.iiConcentricClockStyle; property alias iiConcentricClockRadius: adapter.iiConcentricClockRadius
    property alias iiConcentricClockPad: adapter.iiConcentricClockPad; property alias iiConcentricClockBorder: adapter.iiConcentricClockBorder; property alias iiConcentricClockBorderOpacity: adapter.iiConcentricClockBorderOpacity; property alias iiConcentricClockBackingOpacity: adapter.iiConcentricClockBackingOpacity; property alias iiConcentricClockOpts: adapter.iiConcentricClockOpts
    property alias iiMonthClockEnabled: adapter.iiMonthClockEnabled; property alias iiMonthClockAnchor: adapter.iiMonthClockAnchor; property alias iiMonthClockX: adapter.iiMonthClockX; property alias iiMonthClockY: adapter.iiMonthClockY
    property alias iiMonthClockLocked: adapter.iiMonthClockLocked; property alias iiMonthClockScale: adapter.iiMonthClockScale; property alias iiMonthClockOpacity: adapter.iiMonthClockOpacity; property alias iiMonthClockBg: adapter.iiMonthClockBg
    property alias iiMonthClockColor: adapter.iiMonthClockColor; property alias iiMonthClockColor2: adapter.iiMonthClockColor2; property alias iiMonthClockStyle: adapter.iiMonthClockStyle; property alias iiMonthClockRadius: adapter.iiMonthClockRadius
    property alias iiMonthClockPad: adapter.iiMonthClockPad; property alias iiMonthClockBorder: adapter.iiMonthClockBorder; property alias iiMonthClockBorderOpacity: adapter.iiMonthClockBorderOpacity; property alias iiMonthClockBackingOpacity: adapter.iiMonthClockBackingOpacity; property alias iiMonthClockOpts: adapter.iiMonthClockOpts
    property alias iiScallopDotClockEnabled: adapter.iiScallopDotClockEnabled; property alias iiScallopDotClockAnchor: adapter.iiScallopDotClockAnchor; property alias iiScallopDotClockX: adapter.iiScallopDotClockX; property alias iiScallopDotClockY: adapter.iiScallopDotClockY
    property alias iiScallopDotClockLocked: adapter.iiScallopDotClockLocked; property alias iiScallopDotClockScale: adapter.iiScallopDotClockScale; property alias iiScallopDotClockOpacity: adapter.iiScallopDotClockOpacity; property alias iiScallopDotClockBg: adapter.iiScallopDotClockBg
    property alias iiScallopDotClockColor: adapter.iiScallopDotClockColor; property alias iiScallopDotClockColor2: adapter.iiScallopDotClockColor2; property alias iiScallopDotClockStyle: adapter.iiScallopDotClockStyle; property alias iiScallopDotClockRadius: adapter.iiScallopDotClockRadius
    property alias iiScallopDotClockPad: adapter.iiScallopDotClockPad; property alias iiScallopDotClockBorder: adapter.iiScallopDotClockBorder; property alias iiScallopDotClockBorderOpacity: adapter.iiScallopDotClockBorderOpacity; property alias iiScallopDotClockBackingOpacity: adapter.iiScallopDotClockBackingOpacity; property alias iiScallopDotClockOpts: adapter.iiScallopDotClockOpts
    property alias iiScallopNumberClockEnabled: adapter.iiScallopNumberClockEnabled; property alias iiScallopNumberClockAnchor: adapter.iiScallopNumberClockAnchor; property alias iiScallopNumberClockX: adapter.iiScallopNumberClockX; property alias iiScallopNumberClockY: adapter.iiScallopNumberClockY
    property alias iiScallopNumberClockLocked: adapter.iiScallopNumberClockLocked; property alias iiScallopNumberClockScale: adapter.iiScallopNumberClockScale; property alias iiScallopNumberClockOpacity: adapter.iiScallopNumberClockOpacity; property alias iiScallopNumberClockBg: adapter.iiScallopNumberClockBg
    property alias iiScallopNumberClockColor: adapter.iiScallopNumberClockColor; property alias iiScallopNumberClockColor2: adapter.iiScallopNumberClockColor2; property alias iiScallopNumberClockStyle: adapter.iiScallopNumberClockStyle; property alias iiScallopNumberClockRadius: adapter.iiScallopNumberClockRadius
    property alias iiScallopNumberClockPad: adapter.iiScallopNumberClockPad; property alias iiScallopNumberClockBorder: adapter.iiScallopNumberClockBorder; property alias iiScallopNumberClockBorderOpacity: adapter.iiScallopNumberClockBorderOpacity; property alias iiScallopNumberClockBackingOpacity: adapter.iiScallopNumberClockBackingOpacity; property alias iiScallopNumberClockOpts: adapter.iiScallopNumberClockOpts
    property alias iiCirclePointerClockEnabled: adapter.iiCirclePointerClockEnabled; property alias iiCirclePointerClockAnchor: adapter.iiCirclePointerClockAnchor; property alias iiCirclePointerClockX: adapter.iiCirclePointerClockX; property alias iiCirclePointerClockY: adapter.iiCirclePointerClockY
    property alias iiCirclePointerClockLocked: adapter.iiCirclePointerClockLocked; property alias iiCirclePointerClockScale: adapter.iiCirclePointerClockScale; property alias iiCirclePointerClockOpacity: adapter.iiCirclePointerClockOpacity; property alias iiCirclePointerClockBg: adapter.iiCirclePointerClockBg
    property alias iiCirclePointerClockColor: adapter.iiCirclePointerClockColor; property alias iiCirclePointerClockColor2: adapter.iiCirclePointerClockColor2; property alias iiCirclePointerClockStyle: adapter.iiCirclePointerClockStyle; property alias iiCirclePointerClockRadius: adapter.iiCirclePointerClockRadius
    property alias iiCirclePointerClockPad: adapter.iiCirclePointerClockPad; property alias iiCirclePointerClockBorder: adapter.iiCirclePointerClockBorder; property alias iiCirclePointerClockBorderOpacity: adapter.iiCirclePointerClockBorderOpacity; property alias iiCirclePointerClockBackingOpacity: adapter.iiCirclePointerClockBackingOpacity; property alias iiCirclePointerClockOpts: adapter.iiCirclePointerClockOpts
    property alias iiTripleRingClockEnabled: adapter.iiTripleRingClockEnabled; property alias iiTripleRingClockAnchor: adapter.iiTripleRingClockAnchor; property alias iiTripleRingClockX: adapter.iiTripleRingClockX; property alias iiTripleRingClockY: adapter.iiTripleRingClockY
    property alias iiTripleRingClockLocked: adapter.iiTripleRingClockLocked; property alias iiTripleRingClockScale: adapter.iiTripleRingClockScale; property alias iiTripleRingClockOpacity: adapter.iiTripleRingClockOpacity; property alias iiTripleRingClockBg: adapter.iiTripleRingClockBg
    property alias iiTripleRingClockColor: adapter.iiTripleRingClockColor; property alias iiTripleRingClockColor2: adapter.iiTripleRingClockColor2; property alias iiTripleRingClockStyle: adapter.iiTripleRingClockStyle; property alias iiTripleRingClockRadius: adapter.iiTripleRingClockRadius
    property alias iiTripleRingClockPad: adapter.iiTripleRingClockPad; property alias iiTripleRingClockBorder: adapter.iiTripleRingClockBorder; property alias iiTripleRingClockBorderOpacity: adapter.iiTripleRingClockBorderOpacity; property alias iiTripleRingClockBackingOpacity: adapter.iiTripleRingClockBackingOpacity; property alias iiTripleRingClockOpts: adapter.iiTripleRingClockOpts
    property alias iiGridCardClockEnabled: adapter.iiGridCardClockEnabled; property alias iiGridCardClockAnchor: adapter.iiGridCardClockAnchor; property alias iiGridCardClockX: adapter.iiGridCardClockX; property alias iiGridCardClockY: adapter.iiGridCardClockY
    property alias iiGridCardClockLocked: adapter.iiGridCardClockLocked; property alias iiGridCardClockScale: adapter.iiGridCardClockScale; property alias iiGridCardClockOpacity: adapter.iiGridCardClockOpacity; property alias iiGridCardClockBg: adapter.iiGridCardClockBg
    property alias iiGridCardClockColor: adapter.iiGridCardClockColor; property alias iiGridCardClockColor2: adapter.iiGridCardClockColor2; property alias iiGridCardClockStyle: adapter.iiGridCardClockStyle; property alias iiGridCardClockRadius: adapter.iiGridCardClockRadius
    property alias iiGridCardClockPad: adapter.iiGridCardClockPad; property alias iiGridCardClockBorder: adapter.iiGridCardClockBorder; property alias iiGridCardClockBorderOpacity: adapter.iiGridCardClockBorderOpacity; property alias iiGridCardClockBackingOpacity: adapter.iiGridCardClockBackingOpacity; property alias iiGridCardClockOpts: adapter.iiGridCardClockOpts
    property alias iiClockExpressiveCardEnabled: adapter.iiClockExpressiveCardEnabled; property alias iiClockExpressiveCardAnchor: adapter.iiClockExpressiveCardAnchor; property alias iiClockExpressiveCardX: adapter.iiClockExpressiveCardX; property alias iiClockExpressiveCardY: adapter.iiClockExpressiveCardY
    property alias iiClockExpressiveCardLocked: adapter.iiClockExpressiveCardLocked; property alias iiClockExpressiveCardScale: adapter.iiClockExpressiveCardScale; property alias iiClockExpressiveCardOpacity: adapter.iiClockExpressiveCardOpacity; property alias iiClockExpressiveCardBg: adapter.iiClockExpressiveCardBg
    property alias iiClockExpressiveCardColor: adapter.iiClockExpressiveCardColor; property alias iiClockExpressiveCardColor2: adapter.iiClockExpressiveCardColor2; property alias iiClockExpressiveCardStyle: adapter.iiClockExpressiveCardStyle; property alias iiClockExpressiveCardRadius: adapter.iiClockExpressiveCardRadius
    property alias iiClockExpressiveCardPad: adapter.iiClockExpressiveCardPad; property alias iiClockExpressiveCardBorder: adapter.iiClockExpressiveCardBorder; property alias iiClockExpressiveCardBorderOpacity: adapter.iiClockExpressiveCardBorderOpacity; property alias iiClockExpressiveCardBackingOpacity: adapter.iiClockExpressiveCardBackingOpacity; property alias iiClockExpressiveCardOpts: adapter.iiClockExpressiveCardOpts
    property alias iiCircularMediaEnabled: adapter.iiCircularMediaEnabled; property alias iiCircularMediaAnchor: adapter.iiCircularMediaAnchor; property alias iiCircularMediaX: adapter.iiCircularMediaX; property alias iiCircularMediaY: adapter.iiCircularMediaY
    property alias iiCircularMediaLocked: adapter.iiCircularMediaLocked; property alias iiCircularMediaScale: adapter.iiCircularMediaScale; property alias iiCircularMediaOpacity: adapter.iiCircularMediaOpacity; property alias iiCircularMediaBg: adapter.iiCircularMediaBg
    property alias iiCircularMediaColor: adapter.iiCircularMediaColor; property alias iiCircularMediaColor2: adapter.iiCircularMediaColor2; property alias iiCircularMediaStyle: adapter.iiCircularMediaStyle; property alias iiCircularMediaRadius: adapter.iiCircularMediaRadius
    property alias iiCircularMediaPad: adapter.iiCircularMediaPad; property alias iiCircularMediaBorder: adapter.iiCircularMediaBorder; property alias iiCircularMediaBorderOpacity: adapter.iiCircularMediaBorderOpacity; property alias iiCircularMediaBackingOpacity: adapter.iiCircularMediaBackingOpacity; property alias iiCircularMediaOpts: adapter.iiCircularMediaOpts
    property alias iiMediaCircularEnabled: adapter.iiMediaCircularEnabled; property alias iiMediaCircularAnchor: adapter.iiMediaCircularAnchor; property alias iiMediaCircularX: adapter.iiMediaCircularX; property alias iiMediaCircularY: adapter.iiMediaCircularY
    property alias iiMediaCircularLocked: adapter.iiMediaCircularLocked; property alias iiMediaCircularScale: adapter.iiMediaCircularScale; property alias iiMediaCircularOpacity: adapter.iiMediaCircularOpacity; property alias iiMediaCircularBg: adapter.iiMediaCircularBg
    property alias iiMediaCircularColor: adapter.iiMediaCircularColor; property alias iiMediaCircularColor2: adapter.iiMediaCircularColor2; property alias iiMediaCircularStyle: adapter.iiMediaCircularStyle; property alias iiMediaCircularRadius: adapter.iiMediaCircularRadius
    property alias iiMediaCircularPad: adapter.iiMediaCircularPad; property alias iiMediaCircularBorder: adapter.iiMediaCircularBorder; property alias iiMediaCircularBorderOpacity: adapter.iiMediaCircularBorderOpacity; property alias iiMediaCircularBackingOpacity: adapter.iiMediaCircularBackingOpacity; property alias iiMediaCircularOpts: adapter.iiMediaCircularOpts
    property alias iiMediaExpressiveEnabled: adapter.iiMediaExpressiveEnabled; property alias iiMediaExpressiveAnchor: adapter.iiMediaExpressiveAnchor; property alias iiMediaExpressiveX: adapter.iiMediaExpressiveX; property alias iiMediaExpressiveY: adapter.iiMediaExpressiveY
    property alias iiMediaExpressiveLocked: adapter.iiMediaExpressiveLocked; property alias iiMediaExpressiveScale: adapter.iiMediaExpressiveScale; property alias iiMediaExpressiveOpacity: adapter.iiMediaExpressiveOpacity; property alias iiMediaExpressiveBg: adapter.iiMediaExpressiveBg
    property alias iiMediaExpressiveColor: adapter.iiMediaExpressiveColor; property alias iiMediaExpressiveColor2: adapter.iiMediaExpressiveColor2; property alias iiMediaExpressiveStyle: adapter.iiMediaExpressiveStyle; property alias iiMediaExpressiveRadius: adapter.iiMediaExpressiveRadius
    property alias iiMediaExpressivePad: adapter.iiMediaExpressivePad; property alias iiMediaExpressiveBorder: adapter.iiMediaExpressiveBorder; property alias iiMediaExpressiveBorderOpacity: adapter.iiMediaExpressiveBorderOpacity; property alias iiMediaExpressiveBackingOpacity: adapter.iiMediaExpressiveBackingOpacity; property alias iiMediaExpressiveOpts: adapter.iiMediaExpressiveOpts
    property alias iiMediaAndroidEnabled: adapter.iiMediaAndroidEnabled; property alias iiMediaAndroidAnchor: adapter.iiMediaAndroidAnchor; property alias iiMediaAndroidX: adapter.iiMediaAndroidX; property alias iiMediaAndroidY: adapter.iiMediaAndroidY
    property alias iiMediaAndroidLocked: adapter.iiMediaAndroidLocked; property alias iiMediaAndroidScale: adapter.iiMediaAndroidScale; property alias iiMediaAndroidOpacity: adapter.iiMediaAndroidOpacity; property alias iiMediaAndroidBg: adapter.iiMediaAndroidBg
    property alias iiMediaAndroidColor: adapter.iiMediaAndroidColor; property alias iiMediaAndroidColor2: adapter.iiMediaAndroidColor2; property alias iiMediaAndroidStyle: adapter.iiMediaAndroidStyle; property alias iiMediaAndroidRadius: adapter.iiMediaAndroidRadius
    property alias iiMediaAndroidPad: adapter.iiMediaAndroidPad; property alias iiMediaAndroidBorder: adapter.iiMediaAndroidBorder; property alias iiMediaAndroidBorderOpacity: adapter.iiMediaAndroidBorderOpacity; property alias iiMediaAndroidBackingOpacity: adapter.iiMediaAndroidBackingOpacity; property alias iiMediaAndroidOpts: adapter.iiMediaAndroidOpts
    property alias iiMediaCdEnabled: adapter.iiMediaCdEnabled; property alias iiMediaCdAnchor: adapter.iiMediaCdAnchor; property alias iiMediaCdX: adapter.iiMediaCdX; property alias iiMediaCdY: adapter.iiMediaCdY
    property alias iiMediaCdLocked: adapter.iiMediaCdLocked; property alias iiMediaCdScale: adapter.iiMediaCdScale; property alias iiMediaCdOpacity: adapter.iiMediaCdOpacity; property alias iiMediaCdBg: adapter.iiMediaCdBg
    property alias iiMediaCdColor: adapter.iiMediaCdColor; property alias iiMediaCdColor2: adapter.iiMediaCdColor2; property alias iiMediaCdStyle: adapter.iiMediaCdStyle; property alias iiMediaCdRadius: adapter.iiMediaCdRadius
    property alias iiMediaCdPad: adapter.iiMediaCdPad; property alias iiMediaCdBorder: adapter.iiMediaCdBorder; property alias iiMediaCdBorderOpacity: adapter.iiMediaCdBorderOpacity; property alias iiMediaCdBackingOpacity: adapter.iiMediaCdBackingOpacity; property alias iiMediaCdOpts: adapter.iiMediaCdOpts
    property alias iiNothingRingMediaEnabled: adapter.iiNothingRingMediaEnabled; property alias iiNothingRingMediaAnchor: adapter.iiNothingRingMediaAnchor; property alias iiNothingRingMediaX: adapter.iiNothingRingMediaX; property alias iiNothingRingMediaY: adapter.iiNothingRingMediaY
    property alias iiNothingRingMediaLocked: adapter.iiNothingRingMediaLocked; property alias iiNothingRingMediaScale: adapter.iiNothingRingMediaScale; property alias iiNothingRingMediaOpacity: adapter.iiNothingRingMediaOpacity; property alias iiNothingRingMediaBg: adapter.iiNothingRingMediaBg
    property alias iiNothingRingMediaColor: adapter.iiNothingRingMediaColor; property alias iiNothingRingMediaColor2: adapter.iiNothingRingMediaColor2; property alias iiNothingRingMediaStyle: adapter.iiNothingRingMediaStyle; property alias iiNothingRingMediaRadius: adapter.iiNothingRingMediaRadius
    property alias iiNothingRingMediaPad: adapter.iiNothingRingMediaPad; property alias iiNothingRingMediaBorder: adapter.iiNothingRingMediaBorder; property alias iiNothingRingMediaBorderOpacity: adapter.iiNothingRingMediaBorderOpacity; property alias iiNothingRingMediaBackingOpacity: adapter.iiNothingRingMediaBackingOpacity; property alias iiNothingRingMediaOpts: adapter.iiNothingRingMediaOpts
    property alias iiWeatherDefaultEnabled: adapter.iiWeatherDefaultEnabled; property alias iiWeatherDefaultAnchor: adapter.iiWeatherDefaultAnchor; property alias iiWeatherDefaultX: adapter.iiWeatherDefaultX; property alias iiWeatherDefaultY: adapter.iiWeatherDefaultY
    property alias iiWeatherDefaultLocked: adapter.iiWeatherDefaultLocked; property alias iiWeatherDefaultScale: adapter.iiWeatherDefaultScale; property alias iiWeatherDefaultOpacity: adapter.iiWeatherDefaultOpacity; property alias iiWeatherDefaultBg: adapter.iiWeatherDefaultBg
    property alias iiWeatherDefaultColor: adapter.iiWeatherDefaultColor; property alias iiWeatherDefaultColor2: adapter.iiWeatherDefaultColor2; property alias iiWeatherDefaultStyle: adapter.iiWeatherDefaultStyle; property alias iiWeatherDefaultRadius: adapter.iiWeatherDefaultRadius
    property alias iiWeatherDefaultPad: adapter.iiWeatherDefaultPad; property alias iiWeatherDefaultBorder: adapter.iiWeatherDefaultBorder; property alias iiWeatherDefaultBorderOpacity: adapter.iiWeatherDefaultBorderOpacity; property alias iiWeatherDefaultBackingOpacity: adapter.iiWeatherDefaultBackingOpacity; property alias iiWeatherDefaultOpts: adapter.iiWeatherDefaultOpts
    property alias iiWeatherExpressiveEnabled: adapter.iiWeatherExpressiveEnabled; property alias iiWeatherExpressiveAnchor: adapter.iiWeatherExpressiveAnchor; property alias iiWeatherExpressiveX: adapter.iiWeatherExpressiveX; property alias iiWeatherExpressiveY: adapter.iiWeatherExpressiveY
    property alias iiWeatherExpressiveLocked: adapter.iiWeatherExpressiveLocked; property alias iiWeatherExpressiveScale: adapter.iiWeatherExpressiveScale; property alias iiWeatherExpressiveOpacity: adapter.iiWeatherExpressiveOpacity; property alias iiWeatherExpressiveBg: adapter.iiWeatherExpressiveBg
    property alias iiWeatherExpressiveColor: adapter.iiWeatherExpressiveColor; property alias iiWeatherExpressiveColor2: adapter.iiWeatherExpressiveColor2; property alias iiWeatherExpressiveStyle: adapter.iiWeatherExpressiveStyle; property alias iiWeatherExpressiveRadius: adapter.iiWeatherExpressiveRadius
    property alias iiWeatherExpressivePad: adapter.iiWeatherExpressivePad; property alias iiWeatherExpressiveBorder: adapter.iiWeatherExpressiveBorder; property alias iiWeatherExpressiveBorderOpacity: adapter.iiWeatherExpressiveBorderOpacity; property alias iiWeatherExpressiveBackingOpacity: adapter.iiWeatherExpressiveBackingOpacity; property alias iiWeatherExpressiveOpts: adapter.iiWeatherExpressiveOpts
    property alias iiWeatherForecastEnabled: adapter.iiWeatherForecastEnabled; property alias iiWeatherForecastAnchor: adapter.iiWeatherForecastAnchor; property alias iiWeatherForecastX: adapter.iiWeatherForecastX; property alias iiWeatherForecastY: adapter.iiWeatherForecastY
    property alias iiWeatherForecastLocked: adapter.iiWeatherForecastLocked; property alias iiWeatherForecastScale: adapter.iiWeatherForecastScale; property alias iiWeatherForecastOpacity: adapter.iiWeatherForecastOpacity; property alias iiWeatherForecastBg: adapter.iiWeatherForecastBg
    property alias iiWeatherForecastColor: adapter.iiWeatherForecastColor; property alias iiWeatherForecastColor2: adapter.iiWeatherForecastColor2; property alias iiWeatherForecastStyle: adapter.iiWeatherForecastStyle; property alias iiWeatherForecastRadius: adapter.iiWeatherForecastRadius
    property alias iiWeatherForecastPad: adapter.iiWeatherForecastPad; property alias iiWeatherForecastBorder: adapter.iiWeatherForecastBorder; property alias iiWeatherForecastBorderOpacity: adapter.iiWeatherForecastBorderOpacity; property alias iiWeatherForecastBackingOpacity: adapter.iiWeatherForecastBackingOpacity; property alias iiWeatherForecastOpts: adapter.iiWeatherForecastOpts
    property alias iiWeatherCardEnabled: adapter.iiWeatherCardEnabled; property alias iiWeatherCardAnchor: adapter.iiWeatherCardAnchor; property alias iiWeatherCardX: adapter.iiWeatherCardX; property alias iiWeatherCardY: adapter.iiWeatherCardY
    property alias iiWeatherCardLocked: adapter.iiWeatherCardLocked; property alias iiWeatherCardScale: adapter.iiWeatherCardScale; property alias iiWeatherCardOpacity: adapter.iiWeatherCardOpacity; property alias iiWeatherCardBg: adapter.iiWeatherCardBg
    property alias iiWeatherCardColor: adapter.iiWeatherCardColor; property alias iiWeatherCardColor2: adapter.iiWeatherCardColor2; property alias iiWeatherCardStyle: adapter.iiWeatherCardStyle; property alias iiWeatherCardRadius: adapter.iiWeatherCardRadius
    property alias iiWeatherCardPad: adapter.iiWeatherCardPad; property alias iiWeatherCardBorder: adapter.iiWeatherCardBorder; property alias iiWeatherCardBorderOpacity: adapter.iiWeatherCardBorderOpacity; property alias iiWeatherCardBackingOpacity: adapter.iiWeatherCardBackingOpacity; property alias iiWeatherCardOpts: adapter.iiWeatherCardOpts
    property alias iiWeatherIconEnabled: adapter.iiWeatherIconEnabled; property alias iiWeatherIconAnchor: adapter.iiWeatherIconAnchor; property alias iiWeatherIconX: adapter.iiWeatherIconX; property alias iiWeatherIconY: adapter.iiWeatherIconY
    property alias iiWeatherIconLocked: adapter.iiWeatherIconLocked; property alias iiWeatherIconScale: adapter.iiWeatherIconScale; property alias iiWeatherIconOpacity: adapter.iiWeatherIconOpacity; property alias iiWeatherIconBg: adapter.iiWeatherIconBg
    property alias iiWeatherIconColor: adapter.iiWeatherIconColor; property alias iiWeatherIconColor2: adapter.iiWeatherIconColor2; property alias iiWeatherIconStyle: adapter.iiWeatherIconStyle; property alias iiWeatherIconRadius: adapter.iiWeatherIconRadius
    property alias iiWeatherIconPad: adapter.iiWeatherIconPad; property alias iiWeatherIconBorder: adapter.iiWeatherIconBorder; property alias iiWeatherIconBorderOpacity: adapter.iiWeatherIconBorderOpacity; property alias iiWeatherIconBackingOpacity: adapter.iiWeatherIconBackingOpacity; property alias iiWeatherIconOpts: adapter.iiWeatherIconOpts
    property alias iiWeatherPillEnabled: adapter.iiWeatherPillEnabled; property alias iiWeatherPillAnchor: adapter.iiWeatherPillAnchor; property alias iiWeatherPillX: adapter.iiWeatherPillX; property alias iiWeatherPillY: adapter.iiWeatherPillY
    property alias iiWeatherPillLocked: adapter.iiWeatherPillLocked; property alias iiWeatherPillScale: adapter.iiWeatherPillScale; property alias iiWeatherPillOpacity: adapter.iiWeatherPillOpacity; property alias iiWeatherPillBg: adapter.iiWeatherPillBg
    property alias iiWeatherPillColor: adapter.iiWeatherPillColor; property alias iiWeatherPillColor2: adapter.iiWeatherPillColor2; property alias iiWeatherPillStyle: adapter.iiWeatherPillStyle; property alias iiWeatherPillRadius: adapter.iiWeatherPillRadius
    property alias iiWeatherPillPad: adapter.iiWeatherPillPad; property alias iiWeatherPillBorder: adapter.iiWeatherPillBorder; property alias iiWeatherPillBorderOpacity: adapter.iiWeatherPillBorderOpacity; property alias iiWeatherPillBackingOpacity: adapter.iiWeatherPillBackingOpacity; property alias iiWeatherPillOpts: adapter.iiWeatherPillOpts
    property alias iiWeatherCircleEnabled: adapter.iiWeatherCircleEnabled; property alias iiWeatherCircleAnchor: adapter.iiWeatherCircleAnchor; property alias iiWeatherCircleX: adapter.iiWeatherCircleX; property alias iiWeatherCircleY: adapter.iiWeatherCircleY
    property alias iiWeatherCircleLocked: adapter.iiWeatherCircleLocked; property alias iiWeatherCircleScale: adapter.iiWeatherCircleScale; property alias iiWeatherCircleOpacity: adapter.iiWeatherCircleOpacity; property alias iiWeatherCircleBg: adapter.iiWeatherCircleBg
    property alias iiWeatherCircleColor: adapter.iiWeatherCircleColor; property alias iiWeatherCircleColor2: adapter.iiWeatherCircleColor2; property alias iiWeatherCircleStyle: adapter.iiWeatherCircleStyle; property alias iiWeatherCircleRadius: adapter.iiWeatherCircleRadius
    property alias iiWeatherCirclePad: adapter.iiWeatherCirclePad; property alias iiWeatherCircleBorder: adapter.iiWeatherCircleBorder; property alias iiWeatherCircleBorderOpacity: adapter.iiWeatherCircleBorderOpacity; property alias iiWeatherCircleBackingOpacity: adapter.iiWeatherCircleBackingOpacity; property alias iiWeatherCircleOpts: adapter.iiWeatherCircleOpts
    property alias iiNothingWeatherCircleEnabled: adapter.iiNothingWeatherCircleEnabled; property alias iiNothingWeatherCircleAnchor: adapter.iiNothingWeatherCircleAnchor; property alias iiNothingWeatherCircleX: adapter.iiNothingWeatherCircleX; property alias iiNothingWeatherCircleY: adapter.iiNothingWeatherCircleY
    property alias iiNothingWeatherCircleLocked: adapter.iiNothingWeatherCircleLocked; property alias iiNothingWeatherCircleScale: adapter.iiNothingWeatherCircleScale; property alias iiNothingWeatherCircleOpacity: adapter.iiNothingWeatherCircleOpacity; property alias iiNothingWeatherCircleBg: adapter.iiNothingWeatherCircleBg
    property alias iiNothingWeatherCircleColor: adapter.iiNothingWeatherCircleColor; property alias iiNothingWeatherCircleColor2: adapter.iiNothingWeatherCircleColor2; property alias iiNothingWeatherCircleStyle: adapter.iiNothingWeatherCircleStyle; property alias iiNothingWeatherCircleRadius: adapter.iiNothingWeatherCircleRadius
    property alias iiNothingWeatherCirclePad: adapter.iiNothingWeatherCirclePad; property alias iiNothingWeatherCircleBorder: adapter.iiNothingWeatherCircleBorder; property alias iiNothingWeatherCircleBorderOpacity: adapter.iiNothingWeatherCircleBorderOpacity; property alias iiNothingWeatherCircleBackingOpacity: adapter.iiNothingWeatherCircleBackingOpacity; property alias iiNothingWeatherCircleOpts: adapter.iiNothingWeatherCircleOpts
    property alias iiVolumeMutePillEnabled: adapter.iiVolumeMutePillEnabled; property alias iiVolumeMutePillAnchor: adapter.iiVolumeMutePillAnchor; property alias iiVolumeMutePillX: adapter.iiVolumeMutePillX; property alias iiVolumeMutePillY: adapter.iiVolumeMutePillY
    property alias iiVolumeMutePillLocked: adapter.iiVolumeMutePillLocked; property alias iiVolumeMutePillScale: adapter.iiVolumeMutePillScale; property alias iiVolumeMutePillOpacity: adapter.iiVolumeMutePillOpacity; property alias iiVolumeMutePillBg: adapter.iiVolumeMutePillBg
    property alias iiVolumeMutePillColor: adapter.iiVolumeMutePillColor; property alias iiVolumeMutePillColor2: adapter.iiVolumeMutePillColor2; property alias iiVolumeMutePillStyle: adapter.iiVolumeMutePillStyle; property alias iiVolumeMutePillRadius: adapter.iiVolumeMutePillRadius
    property alias iiVolumeMutePillPad: adapter.iiVolumeMutePillPad; property alias iiVolumeMutePillBorder: adapter.iiVolumeMutePillBorder; property alias iiVolumeMutePillBorderOpacity: adapter.iiVolumeMutePillBorderOpacity; property alias iiVolumeMutePillBackingOpacity: adapter.iiVolumeMutePillBackingOpacity; property alias iiVolumeMutePillOpts: adapter.iiVolumeMutePillOpts
    property alias iiWifiPillEnabled: adapter.iiWifiPillEnabled; property alias iiWifiPillAnchor: adapter.iiWifiPillAnchor; property alias iiWifiPillX: adapter.iiWifiPillX; property alias iiWifiPillY: adapter.iiWifiPillY
    property alias iiWifiPillLocked: adapter.iiWifiPillLocked; property alias iiWifiPillScale: adapter.iiWifiPillScale; property alias iiWifiPillOpacity: adapter.iiWifiPillOpacity; property alias iiWifiPillBg: adapter.iiWifiPillBg
    property alias iiWifiPillColor: adapter.iiWifiPillColor; property alias iiWifiPillColor2: adapter.iiWifiPillColor2; property alias iiWifiPillStyle: adapter.iiWifiPillStyle; property alias iiWifiPillRadius: adapter.iiWifiPillRadius
    property alias iiWifiPillPad: adapter.iiWifiPillPad; property alias iiWifiPillBorder: adapter.iiWifiPillBorder; property alias iiWifiPillBorderOpacity: adapter.iiWifiPillBorderOpacity; property alias iiWifiPillBackingOpacity: adapter.iiWifiPillBackingOpacity; property alias iiWifiPillOpts: adapter.iiWifiPillOpts
    property alias iiBluetoothPillEnabled: adapter.iiBluetoothPillEnabled; property alias iiBluetoothPillAnchor: adapter.iiBluetoothPillAnchor; property alias iiBluetoothPillX: adapter.iiBluetoothPillX; property alias iiBluetoothPillY: adapter.iiBluetoothPillY
    property alias iiBluetoothPillLocked: adapter.iiBluetoothPillLocked; property alias iiBluetoothPillScale: adapter.iiBluetoothPillScale; property alias iiBluetoothPillOpacity: adapter.iiBluetoothPillOpacity; property alias iiBluetoothPillBg: adapter.iiBluetoothPillBg
    property alias iiBluetoothPillColor: adapter.iiBluetoothPillColor; property alias iiBluetoothPillColor2: adapter.iiBluetoothPillColor2; property alias iiBluetoothPillStyle: adapter.iiBluetoothPillStyle; property alias iiBluetoothPillRadius: adapter.iiBluetoothPillRadius
    property alias iiBluetoothPillPad: adapter.iiBluetoothPillPad; property alias iiBluetoothPillBorder: adapter.iiBluetoothPillBorder; property alias iiBluetoothPillBorderOpacity: adapter.iiBluetoothPillBorderOpacity; property alias iiBluetoothPillBackingOpacity: adapter.iiBluetoothPillBackingOpacity; property alias iiBluetoothPillOpts: adapter.iiBluetoothPillOpts
    property alias iiMicPillEnabled: adapter.iiMicPillEnabled; property alias iiMicPillAnchor: adapter.iiMicPillAnchor; property alias iiMicPillX: adapter.iiMicPillX; property alias iiMicPillY: adapter.iiMicPillY
    property alias iiMicPillLocked: adapter.iiMicPillLocked; property alias iiMicPillScale: adapter.iiMicPillScale; property alias iiMicPillOpacity: adapter.iiMicPillOpacity; property alias iiMicPillBg: adapter.iiMicPillBg
    property alias iiMicPillColor: adapter.iiMicPillColor; property alias iiMicPillColor2: adapter.iiMicPillColor2; property alias iiMicPillStyle: adapter.iiMicPillStyle; property alias iiMicPillRadius: adapter.iiMicPillRadius
    property alias iiMicPillPad: adapter.iiMicPillPad; property alias iiMicPillBorder: adapter.iiMicPillBorder; property alias iiMicPillBorderOpacity: adapter.iiMicPillBorderOpacity; property alias iiMicPillBackingOpacity: adapter.iiMicPillBackingOpacity; property alias iiMicPillOpts: adapter.iiMicPillOpts
    property alias iiDarkModePillEnabled: adapter.iiDarkModePillEnabled; property alias iiDarkModePillAnchor: adapter.iiDarkModePillAnchor; property alias iiDarkModePillX: adapter.iiDarkModePillX; property alias iiDarkModePillY: adapter.iiDarkModePillY
    property alias iiDarkModePillLocked: adapter.iiDarkModePillLocked; property alias iiDarkModePillScale: adapter.iiDarkModePillScale; property alias iiDarkModePillOpacity: adapter.iiDarkModePillOpacity; property alias iiDarkModePillBg: adapter.iiDarkModePillBg
    property alias iiDarkModePillColor: adapter.iiDarkModePillColor; property alias iiDarkModePillColor2: adapter.iiDarkModePillColor2; property alias iiDarkModePillStyle: adapter.iiDarkModePillStyle; property alias iiDarkModePillRadius: adapter.iiDarkModePillRadius
    property alias iiDarkModePillPad: adapter.iiDarkModePillPad; property alias iiDarkModePillBorder: adapter.iiDarkModePillBorder; property alias iiDarkModePillBorderOpacity: adapter.iiDarkModePillBorderOpacity; property alias iiDarkModePillBackingOpacity: adapter.iiDarkModePillBackingOpacity; property alias iiDarkModePillOpts: adapter.iiDarkModePillOpts
    property alias iiScreenRecordPillEnabled: adapter.iiScreenRecordPillEnabled; property alias iiScreenRecordPillAnchor: adapter.iiScreenRecordPillAnchor; property alias iiScreenRecordPillX: adapter.iiScreenRecordPillX; property alias iiScreenRecordPillY: adapter.iiScreenRecordPillY
    property alias iiScreenRecordPillLocked: adapter.iiScreenRecordPillLocked; property alias iiScreenRecordPillScale: adapter.iiScreenRecordPillScale; property alias iiScreenRecordPillOpacity: adapter.iiScreenRecordPillOpacity; property alias iiScreenRecordPillBg: adapter.iiScreenRecordPillBg
    property alias iiScreenRecordPillColor: adapter.iiScreenRecordPillColor; property alias iiScreenRecordPillColor2: adapter.iiScreenRecordPillColor2; property alias iiScreenRecordPillStyle: adapter.iiScreenRecordPillStyle; property alias iiScreenRecordPillRadius: adapter.iiScreenRecordPillRadius
    property alias iiScreenRecordPillPad: adapter.iiScreenRecordPillPad; property alias iiScreenRecordPillBorder: adapter.iiScreenRecordPillBorder; property alias iiScreenRecordPillBorderOpacity: adapter.iiScreenRecordPillBorderOpacity; property alias iiScreenRecordPillBackingOpacity: adapter.iiScreenRecordPillBackingOpacity; property alias iiScreenRecordPillOpts: adapter.iiScreenRecordPillOpts
    property alias iiEasyEffectsPillEnabled: adapter.iiEasyEffectsPillEnabled; property alias iiEasyEffectsPillAnchor: adapter.iiEasyEffectsPillAnchor; property alias iiEasyEffectsPillX: adapter.iiEasyEffectsPillX; property alias iiEasyEffectsPillY: adapter.iiEasyEffectsPillY
    property alias iiEasyEffectsPillLocked: adapter.iiEasyEffectsPillLocked; property alias iiEasyEffectsPillScale: adapter.iiEasyEffectsPillScale; property alias iiEasyEffectsPillOpacity: adapter.iiEasyEffectsPillOpacity; property alias iiEasyEffectsPillBg: adapter.iiEasyEffectsPillBg
    property alias iiEasyEffectsPillColor: adapter.iiEasyEffectsPillColor; property alias iiEasyEffectsPillColor2: adapter.iiEasyEffectsPillColor2; property alias iiEasyEffectsPillStyle: adapter.iiEasyEffectsPillStyle; property alias iiEasyEffectsPillRadius: adapter.iiEasyEffectsPillRadius
    property alias iiEasyEffectsPillPad: adapter.iiEasyEffectsPillPad; property alias iiEasyEffectsPillBorder: adapter.iiEasyEffectsPillBorder; property alias iiEasyEffectsPillBorderOpacity: adapter.iiEasyEffectsPillBorderOpacity; property alias iiEasyEffectsPillBackingOpacity: adapter.iiEasyEffectsPillBackingOpacity; property alias iiEasyEffectsPillOpts: adapter.iiEasyEffectsPillOpts
    property alias iiEasyeffectsPresetPortraitEnabled: adapter.iiEasyeffectsPresetPortraitEnabled; property alias iiEasyeffectsPresetPortraitAnchor: adapter.iiEasyeffectsPresetPortraitAnchor; property alias iiEasyeffectsPresetPortraitX: adapter.iiEasyeffectsPresetPortraitX; property alias iiEasyeffectsPresetPortraitY: adapter.iiEasyeffectsPresetPortraitY
    property alias iiEasyeffectsPresetPortraitLocked: adapter.iiEasyeffectsPresetPortraitLocked; property alias iiEasyeffectsPresetPortraitScale: adapter.iiEasyeffectsPresetPortraitScale; property alias iiEasyeffectsPresetPortraitOpacity: adapter.iiEasyeffectsPresetPortraitOpacity; property alias iiEasyeffectsPresetPortraitBg: adapter.iiEasyeffectsPresetPortraitBg
    property alias iiEasyeffectsPresetPortraitColor: adapter.iiEasyeffectsPresetPortraitColor; property alias iiEasyeffectsPresetPortraitColor2: adapter.iiEasyeffectsPresetPortraitColor2; property alias iiEasyeffectsPresetPortraitStyle: adapter.iiEasyeffectsPresetPortraitStyle; property alias iiEasyeffectsPresetPortraitRadius: adapter.iiEasyeffectsPresetPortraitRadius
    property alias iiEasyeffectsPresetPortraitPad: adapter.iiEasyeffectsPresetPortraitPad; property alias iiEasyeffectsPresetPortraitBorder: adapter.iiEasyeffectsPresetPortraitBorder; property alias iiEasyeffectsPresetPortraitBorderOpacity: adapter.iiEasyeffectsPresetPortraitBorderOpacity; property alias iiEasyeffectsPresetPortraitBackingOpacity: adapter.iiEasyeffectsPresetPortraitBackingOpacity; property alias iiEasyeffectsPresetPortraitOpts: adapter.iiEasyeffectsPresetPortraitOpts
    property alias iiEasyeffectsPresetLandscapeEnabled: adapter.iiEasyeffectsPresetLandscapeEnabled; property alias iiEasyeffectsPresetLandscapeAnchor: adapter.iiEasyeffectsPresetLandscapeAnchor; property alias iiEasyeffectsPresetLandscapeX: adapter.iiEasyeffectsPresetLandscapeX; property alias iiEasyeffectsPresetLandscapeY: adapter.iiEasyeffectsPresetLandscapeY
    property alias iiEasyeffectsPresetLandscapeLocked: adapter.iiEasyeffectsPresetLandscapeLocked; property alias iiEasyeffectsPresetLandscapeScale: adapter.iiEasyeffectsPresetLandscapeScale; property alias iiEasyeffectsPresetLandscapeOpacity: adapter.iiEasyeffectsPresetLandscapeOpacity; property alias iiEasyeffectsPresetLandscapeBg: adapter.iiEasyeffectsPresetLandscapeBg
    property alias iiEasyeffectsPresetLandscapeColor: adapter.iiEasyeffectsPresetLandscapeColor; property alias iiEasyeffectsPresetLandscapeColor2: adapter.iiEasyeffectsPresetLandscapeColor2; property alias iiEasyeffectsPresetLandscapeStyle: adapter.iiEasyeffectsPresetLandscapeStyle; property alias iiEasyeffectsPresetLandscapeRadius: adapter.iiEasyeffectsPresetLandscapeRadius
    property alias iiEasyeffectsPresetLandscapePad: adapter.iiEasyeffectsPresetLandscapePad; property alias iiEasyeffectsPresetLandscapeBorder: adapter.iiEasyeffectsPresetLandscapeBorder; property alias iiEasyeffectsPresetLandscapeBorderOpacity: adapter.iiEasyeffectsPresetLandscapeBorderOpacity; property alias iiEasyeffectsPresetLandscapeBackingOpacity: adapter.iiEasyeffectsPresetLandscapeBackingOpacity; property alias iiEasyeffectsPresetLandscapeOpts: adapter.iiEasyeffectsPresetLandscapeOpts
    property alias iiWeatherTypographyEnabled: adapter.iiWeatherTypographyEnabled; property alias iiWeatherTypographyAnchor: adapter.iiWeatherTypographyAnchor; property alias iiWeatherTypographyX: adapter.iiWeatherTypographyX; property alias iiWeatherTypographyY: adapter.iiWeatherTypographyY
    property alias iiWeatherTypographyLocked: adapter.iiWeatherTypographyLocked; property alias iiWeatherTypographyScale: adapter.iiWeatherTypographyScale; property alias iiWeatherTypographyOpacity: adapter.iiWeatherTypographyOpacity; property alias iiWeatherTypographyBg: adapter.iiWeatherTypographyBg
    property alias iiWeatherTypographyColor: adapter.iiWeatherTypographyColor; property alias iiWeatherTypographyColor2: adapter.iiWeatherTypographyColor2; property alias iiWeatherTypographyStyle: adapter.iiWeatherTypographyStyle; property alias iiWeatherTypographyRadius: adapter.iiWeatherTypographyRadius
    property alias iiWeatherTypographyPad: adapter.iiWeatherTypographyPad; property alias iiWeatherTypographyBorder: adapter.iiWeatherTypographyBorder; property alias iiWeatherTypographyBorderOpacity: adapter.iiWeatherTypographyBorderOpacity; property alias iiWeatherTypographyBackingOpacity: adapter.iiWeatherTypographyBackingOpacity; property alias iiWeatherTypographyOpts: adapter.iiWeatherTypographyOpts
    property alias iiWeatherHourlyEnabled: adapter.iiWeatherHourlyEnabled; property alias iiWeatherHourlyAnchor: adapter.iiWeatherHourlyAnchor; property alias iiWeatherHourlyX: adapter.iiWeatherHourlyX; property alias iiWeatherHourlyY: adapter.iiWeatherHourlyY
    property alias iiWeatherHourlyLocked: adapter.iiWeatherHourlyLocked; property alias iiWeatherHourlyScale: adapter.iiWeatherHourlyScale; property alias iiWeatherHourlyOpacity: adapter.iiWeatherHourlyOpacity; property alias iiWeatherHourlyBg: adapter.iiWeatherHourlyBg
    property alias iiWeatherHourlyColor: adapter.iiWeatherHourlyColor; property alias iiWeatherHourlyColor2: adapter.iiWeatherHourlyColor2; property alias iiWeatherHourlyStyle: adapter.iiWeatherHourlyStyle; property alias iiWeatherHourlyRadius: adapter.iiWeatherHourlyRadius
    property alias iiWeatherHourlyPad: adapter.iiWeatherHourlyPad; property alias iiWeatherHourlyBorder: adapter.iiWeatherHourlyBorder; property alias iiWeatherHourlyBorderOpacity: adapter.iiWeatherHourlyBorderOpacity; property alias iiWeatherHourlyBackingOpacity: adapter.iiWeatherHourlyBackingOpacity; property alias iiWeatherHourlyOpts: adapter.iiWeatherHourlyOpts
    property alias iiDateDefaultEnabled: adapter.iiDateDefaultEnabled; property alias iiDateDefaultAnchor: adapter.iiDateDefaultAnchor; property alias iiDateDefaultX: adapter.iiDateDefaultX; property alias iiDateDefaultY: adapter.iiDateDefaultY
    property alias iiDateDefaultLocked: adapter.iiDateDefaultLocked; property alias iiDateDefaultScale: adapter.iiDateDefaultScale; property alias iiDateDefaultOpacity: adapter.iiDateDefaultOpacity; property alias iiDateDefaultBg: adapter.iiDateDefaultBg
    property alias iiDateDefaultColor: adapter.iiDateDefaultColor; property alias iiDateDefaultColor2: adapter.iiDateDefaultColor2; property alias iiDateDefaultStyle: adapter.iiDateDefaultStyle; property alias iiDateDefaultRadius: adapter.iiDateDefaultRadius
    property alias iiDateDefaultPad: adapter.iiDateDefaultPad; property alias iiDateDefaultBorder: adapter.iiDateDefaultBorder; property alias iiDateDefaultBorderOpacity: adapter.iiDateDefaultBorderOpacity; property alias iiDateDefaultBackingOpacity: adapter.iiDateDefaultBackingOpacity; property alias iiDateDefaultOpts: adapter.iiDateDefaultOpts
    property alias iiCalendarMinimalEnabled: adapter.iiCalendarMinimalEnabled; property alias iiCalendarMinimalAnchor: adapter.iiCalendarMinimalAnchor; property alias iiCalendarMinimalX: adapter.iiCalendarMinimalX; property alias iiCalendarMinimalY: adapter.iiCalendarMinimalY
    property alias iiCalendarMinimalLocked: adapter.iiCalendarMinimalLocked; property alias iiCalendarMinimalScale: adapter.iiCalendarMinimalScale; property alias iiCalendarMinimalOpacity: adapter.iiCalendarMinimalOpacity; property alias iiCalendarMinimalBg: adapter.iiCalendarMinimalBg
    property alias iiCalendarMinimalColor: adapter.iiCalendarMinimalColor; property alias iiCalendarMinimalColor2: adapter.iiCalendarMinimalColor2; property alias iiCalendarMinimalStyle: adapter.iiCalendarMinimalStyle; property alias iiCalendarMinimalRadius: adapter.iiCalendarMinimalRadius
    property alias iiCalendarMinimalPad: adapter.iiCalendarMinimalPad; property alias iiCalendarMinimalBorder: adapter.iiCalendarMinimalBorder; property alias iiCalendarMinimalBorderOpacity: adapter.iiCalendarMinimalBorderOpacity; property alias iiCalendarMinimalBackingOpacity: adapter.iiCalendarMinimalBackingOpacity; property alias iiCalendarMinimalOpts: adapter.iiCalendarMinimalOpts
    property alias iiCalendarGridEnabled: adapter.iiCalendarGridEnabled; property alias iiCalendarGridAnchor: adapter.iiCalendarGridAnchor; property alias iiCalendarGridX: adapter.iiCalendarGridX; property alias iiCalendarGridY: adapter.iiCalendarGridY
    property alias iiCalendarGridLocked: adapter.iiCalendarGridLocked; property alias iiCalendarGridScale: adapter.iiCalendarGridScale; property alias iiCalendarGridOpacity: adapter.iiCalendarGridOpacity; property alias iiCalendarGridBg: adapter.iiCalendarGridBg
    property alias iiCalendarGridColor: adapter.iiCalendarGridColor; property alias iiCalendarGridColor2: adapter.iiCalendarGridColor2; property alias iiCalendarGridStyle: adapter.iiCalendarGridStyle; property alias iiCalendarGridRadius: adapter.iiCalendarGridRadius
    property alias iiCalendarGridPad: adapter.iiCalendarGridPad; property alias iiCalendarGridBorder: adapter.iiCalendarGridBorder; property alias iiCalendarGridBorderOpacity: adapter.iiCalendarGridBorderOpacity; property alias iiCalendarGridBackingOpacity: adapter.iiCalendarGridBackingOpacity; property alias iiCalendarGridOpts: adapter.iiCalendarGridOpts
    property alias iiCalendarAgendaEnabled: adapter.iiCalendarAgendaEnabled; property alias iiCalendarAgendaAnchor: adapter.iiCalendarAgendaAnchor; property alias iiCalendarAgendaX: adapter.iiCalendarAgendaX; property alias iiCalendarAgendaY: adapter.iiCalendarAgendaY
    property alias iiCalendarAgendaLocked: adapter.iiCalendarAgendaLocked; property alias iiCalendarAgendaScale: adapter.iiCalendarAgendaScale; property alias iiCalendarAgendaOpacity: adapter.iiCalendarAgendaOpacity; property alias iiCalendarAgendaBg: adapter.iiCalendarAgendaBg
    property alias iiCalendarAgendaColor: adapter.iiCalendarAgendaColor; property alias iiCalendarAgendaColor2: adapter.iiCalendarAgendaColor2; property alias iiCalendarAgendaStyle: adapter.iiCalendarAgendaStyle; property alias iiCalendarAgendaRadius: adapter.iiCalendarAgendaRadius
    property alias iiCalendarAgendaPad: adapter.iiCalendarAgendaPad; property alias iiCalendarAgendaBorder: adapter.iiCalendarAgendaBorder; property alias iiCalendarAgendaBorderOpacity: adapter.iiCalendarAgendaBorderOpacity; property alias iiCalendarAgendaBackingOpacity: adapter.iiCalendarAgendaBackingOpacity; property alias iiCalendarAgendaOpts: adapter.iiCalendarAgendaOpts
    property alias iiCalendarNextEventEnabled: adapter.iiCalendarNextEventEnabled; property alias iiCalendarNextEventAnchor: adapter.iiCalendarNextEventAnchor; property alias iiCalendarNextEventX: adapter.iiCalendarNextEventX; property alias iiCalendarNextEventY: adapter.iiCalendarNextEventY
    property alias iiCalendarNextEventLocked: adapter.iiCalendarNextEventLocked; property alias iiCalendarNextEventScale: adapter.iiCalendarNextEventScale; property alias iiCalendarNextEventOpacity: adapter.iiCalendarNextEventOpacity; property alias iiCalendarNextEventBg: adapter.iiCalendarNextEventBg
    property alias iiCalendarNextEventColor: adapter.iiCalendarNextEventColor; property alias iiCalendarNextEventColor2: adapter.iiCalendarNextEventColor2; property alias iiCalendarNextEventStyle: adapter.iiCalendarNextEventStyle; property alias iiCalendarNextEventRadius: adapter.iiCalendarNextEventRadius
    property alias iiCalendarNextEventPad: adapter.iiCalendarNextEventPad; property alias iiCalendarNextEventBorder: adapter.iiCalendarNextEventBorder; property alias iiCalendarNextEventBorderOpacity: adapter.iiCalendarNextEventBorderOpacity; property alias iiCalendarNextEventBackingOpacity: adapter.iiCalendarNextEventBackingOpacity; property alias iiCalendarNextEventOpts: adapter.iiCalendarNextEventOpts
    property alias iiCalendarPillEnabled: adapter.iiCalendarPillEnabled; property alias iiCalendarPillAnchor: adapter.iiCalendarPillAnchor; property alias iiCalendarPillX: adapter.iiCalendarPillX; property alias iiCalendarPillY: adapter.iiCalendarPillY
    property alias iiCalendarPillLocked: adapter.iiCalendarPillLocked; property alias iiCalendarPillScale: adapter.iiCalendarPillScale; property alias iiCalendarPillOpacity: adapter.iiCalendarPillOpacity; property alias iiCalendarPillBg: adapter.iiCalendarPillBg
    property alias iiCalendarPillColor: adapter.iiCalendarPillColor; property alias iiCalendarPillColor2: adapter.iiCalendarPillColor2; property alias iiCalendarPillStyle: adapter.iiCalendarPillStyle; property alias iiCalendarPillRadius: adapter.iiCalendarPillRadius
    property alias iiCalendarPillPad: adapter.iiCalendarPillPad; property alias iiCalendarPillBorder: adapter.iiCalendarPillBorder; property alias iiCalendarPillBorderOpacity: adapter.iiCalendarPillBorderOpacity; property alias iiCalendarPillBackingOpacity: adapter.iiCalendarPillBackingOpacity; property alias iiCalendarPillOpts: adapter.iiCalendarPillOpts
    property alias iiCalendarUpcoming3daysEnabled: adapter.iiCalendarUpcoming3daysEnabled; property alias iiCalendarUpcoming3daysAnchor: adapter.iiCalendarUpcoming3daysAnchor; property alias iiCalendarUpcoming3daysX: adapter.iiCalendarUpcoming3daysX; property alias iiCalendarUpcoming3daysY: adapter.iiCalendarUpcoming3daysY
    property alias iiCalendarUpcoming3daysLocked: adapter.iiCalendarUpcoming3daysLocked; property alias iiCalendarUpcoming3daysScale: adapter.iiCalendarUpcoming3daysScale; property alias iiCalendarUpcoming3daysOpacity: adapter.iiCalendarUpcoming3daysOpacity; property alias iiCalendarUpcoming3daysBg: adapter.iiCalendarUpcoming3daysBg
    property alias iiCalendarUpcoming3daysColor: adapter.iiCalendarUpcoming3daysColor; property alias iiCalendarUpcoming3daysColor2: adapter.iiCalendarUpcoming3daysColor2; property alias iiCalendarUpcoming3daysStyle: adapter.iiCalendarUpcoming3daysStyle; property alias iiCalendarUpcoming3daysRadius: adapter.iiCalendarUpcoming3daysRadius
    property alias iiCalendarUpcoming3daysPad: adapter.iiCalendarUpcoming3daysPad; property alias iiCalendarUpcoming3daysBorder: adapter.iiCalendarUpcoming3daysBorder; property alias iiCalendarUpcoming3daysBorderOpacity: adapter.iiCalendarUpcoming3daysBorderOpacity; property alias iiCalendarUpcoming3daysBackingOpacity: adapter.iiCalendarUpcoming3daysBackingOpacity; property alias iiCalendarUpcoming3daysOpts: adapter.iiCalendarUpcoming3daysOpts
    property alias iiPhotoEnabled: adapter.iiPhotoEnabled; property alias iiPhotoAnchor: adapter.iiPhotoAnchor; property alias iiPhotoX: adapter.iiPhotoX; property alias iiPhotoY: adapter.iiPhotoY
    property alias iiPhotoLocked: adapter.iiPhotoLocked; property alias iiPhotoScale: adapter.iiPhotoScale; property alias iiPhotoOpacity: adapter.iiPhotoOpacity; property alias iiPhotoBg: adapter.iiPhotoBg
    property alias iiPhotoColor: adapter.iiPhotoColor; property alias iiPhotoColor2: adapter.iiPhotoColor2; property alias iiPhotoStyle: adapter.iiPhotoStyle; property alias iiPhotoRadius: adapter.iiPhotoRadius
    property alias iiPhotoPad: adapter.iiPhotoPad; property alias iiPhotoBorder: adapter.iiPhotoBorder; property alias iiPhotoBorderOpacity: adapter.iiPhotoBorderOpacity; property alias iiPhotoBackingOpacity: adapter.iiPhotoBackingOpacity; property alias iiPhotoOpts: adapter.iiPhotoOpts
    property alias iiPhoto1x1Enabled: adapter.iiPhoto1x1Enabled; property alias iiPhoto1x1Anchor: adapter.iiPhoto1x1Anchor; property alias iiPhoto1x1X: adapter.iiPhoto1x1X; property alias iiPhoto1x1Y: adapter.iiPhoto1x1Y
    property alias iiPhoto1x1Locked: adapter.iiPhoto1x1Locked; property alias iiPhoto1x1Scale: adapter.iiPhoto1x1Scale; property alias iiPhoto1x1Opacity: adapter.iiPhoto1x1Opacity; property alias iiPhoto1x1Bg: adapter.iiPhoto1x1Bg
    property alias iiPhoto1x1Color: adapter.iiPhoto1x1Color; property alias iiPhoto1x1Color2: adapter.iiPhoto1x1Color2; property alias iiPhoto1x1Style: adapter.iiPhoto1x1Style; property alias iiPhoto1x1Radius: adapter.iiPhoto1x1Radius
    property alias iiPhoto1x1Pad: adapter.iiPhoto1x1Pad; property alias iiPhoto1x1Border: adapter.iiPhoto1x1Border; property alias iiPhoto1x1BorderOpacity: adapter.iiPhoto1x1BorderOpacity; property alias iiPhoto1x1BackingOpacity: adapter.iiPhoto1x1BackingOpacity; property alias iiPhoto1x1Opts: adapter.iiPhoto1x1Opts
    property alias iiPhotoWeather2x1Enabled: adapter.iiPhotoWeather2x1Enabled; property alias iiPhotoWeather2x1Anchor: adapter.iiPhotoWeather2x1Anchor; property alias iiPhotoWeather2x1X: adapter.iiPhotoWeather2x1X; property alias iiPhotoWeather2x1Y: adapter.iiPhotoWeather2x1Y
    property alias iiPhotoWeather2x1Locked: adapter.iiPhotoWeather2x1Locked; property alias iiPhotoWeather2x1Scale: adapter.iiPhotoWeather2x1Scale; property alias iiPhotoWeather2x1Opacity: adapter.iiPhotoWeather2x1Opacity; property alias iiPhotoWeather2x1Bg: adapter.iiPhotoWeather2x1Bg
    property alias iiPhotoWeather2x1Color: adapter.iiPhotoWeather2x1Color; property alias iiPhotoWeather2x1Color2: adapter.iiPhotoWeather2x1Color2; property alias iiPhotoWeather2x1Style: adapter.iiPhotoWeather2x1Style; property alias iiPhotoWeather2x1Radius: adapter.iiPhotoWeather2x1Radius
    property alias iiPhotoWeather2x1Pad: adapter.iiPhotoWeather2x1Pad; property alias iiPhotoWeather2x1Border: adapter.iiPhotoWeather2x1Border; property alias iiPhotoWeather2x1BorderOpacity: adapter.iiPhotoWeather2x1BorderOpacity; property alias iiPhotoWeather2x1BackingOpacity: adapter.iiPhotoWeather2x1BackingOpacity; property alias iiPhotoWeather2x1Opts: adapter.iiPhotoWeather2x1Opts
    property alias iiPhotoPill2x1Enabled: adapter.iiPhotoPill2x1Enabled; property alias iiPhotoPill2x1Anchor: adapter.iiPhotoPill2x1Anchor; property alias iiPhotoPill2x1X: adapter.iiPhotoPill2x1X; property alias iiPhotoPill2x1Y: adapter.iiPhotoPill2x1Y
    property alias iiPhotoPill2x1Locked: adapter.iiPhotoPill2x1Locked; property alias iiPhotoPill2x1Scale: adapter.iiPhotoPill2x1Scale; property alias iiPhotoPill2x1Opacity: adapter.iiPhotoPill2x1Opacity; property alias iiPhotoPill2x1Bg: adapter.iiPhotoPill2x1Bg
    property alias iiPhotoPill2x1Color: adapter.iiPhotoPill2x1Color; property alias iiPhotoPill2x1Color2: adapter.iiPhotoPill2x1Color2; property alias iiPhotoPill2x1Style: adapter.iiPhotoPill2x1Style; property alias iiPhotoPill2x1Radius: adapter.iiPhotoPill2x1Radius
    property alias iiPhotoPill2x1Pad: adapter.iiPhotoPill2x1Pad; property alias iiPhotoPill2x1Border: adapter.iiPhotoPill2x1Border; property alias iiPhotoPill2x1BorderOpacity: adapter.iiPhotoPill2x1BorderOpacity; property alias iiPhotoPill2x1BackingOpacity: adapter.iiPhotoPill2x1BackingOpacity; property alias iiPhotoPill2x1Opts: adapter.iiPhotoPill2x1Opts
    property alias iiPhotoMinimalTemp2x1Enabled: adapter.iiPhotoMinimalTemp2x1Enabled; property alias iiPhotoMinimalTemp2x1Anchor: adapter.iiPhotoMinimalTemp2x1Anchor; property alias iiPhotoMinimalTemp2x1X: adapter.iiPhotoMinimalTemp2x1X; property alias iiPhotoMinimalTemp2x1Y: adapter.iiPhotoMinimalTemp2x1Y
    property alias iiPhotoMinimalTemp2x1Locked: adapter.iiPhotoMinimalTemp2x1Locked; property alias iiPhotoMinimalTemp2x1Scale: adapter.iiPhotoMinimalTemp2x1Scale; property alias iiPhotoMinimalTemp2x1Opacity: adapter.iiPhotoMinimalTemp2x1Opacity; property alias iiPhotoMinimalTemp2x1Bg: adapter.iiPhotoMinimalTemp2x1Bg
    property alias iiPhotoMinimalTemp2x1Color: adapter.iiPhotoMinimalTemp2x1Color; property alias iiPhotoMinimalTemp2x1Color2: adapter.iiPhotoMinimalTemp2x1Color2; property alias iiPhotoMinimalTemp2x1Style: adapter.iiPhotoMinimalTemp2x1Style; property alias iiPhotoMinimalTemp2x1Radius: adapter.iiPhotoMinimalTemp2x1Radius
    property alias iiPhotoMinimalTemp2x1Pad: adapter.iiPhotoMinimalTemp2x1Pad; property alias iiPhotoMinimalTemp2x1Border: adapter.iiPhotoMinimalTemp2x1Border; property alias iiPhotoMinimalTemp2x1BorderOpacity: adapter.iiPhotoMinimalTemp2x1BorderOpacity; property alias iiPhotoMinimalTemp2x1BackingOpacity: adapter.iiPhotoMinimalTemp2x1BackingOpacity; property alias iiPhotoMinimalTemp2x1Opts: adapter.iiPhotoMinimalTemp2x1Opts
    property alias iiBluetoothBatteryEnabled: adapter.iiBluetoothBatteryEnabled; property alias iiBluetoothBatteryAnchor: adapter.iiBluetoothBatteryAnchor; property alias iiBluetoothBatteryX: adapter.iiBluetoothBatteryX; property alias iiBluetoothBatteryY: adapter.iiBluetoothBatteryY
    property alias iiBluetoothBatteryLocked: adapter.iiBluetoothBatteryLocked; property alias iiBluetoothBatteryScale: adapter.iiBluetoothBatteryScale; property alias iiBluetoothBatteryOpacity: adapter.iiBluetoothBatteryOpacity; property alias iiBluetoothBatteryBg: adapter.iiBluetoothBatteryBg
    property alias iiBluetoothBatteryColor: adapter.iiBluetoothBatteryColor; property alias iiBluetoothBatteryColor2: adapter.iiBluetoothBatteryColor2; property alias iiBluetoothBatteryStyle: adapter.iiBluetoothBatteryStyle; property alias iiBluetoothBatteryRadius: adapter.iiBluetoothBatteryRadius
    property alias iiBluetoothBatteryPad: adapter.iiBluetoothBatteryPad; property alias iiBluetoothBatteryBorder: adapter.iiBluetoothBatteryBorder; property alias iiBluetoothBatteryBorderOpacity: adapter.iiBluetoothBatteryBorderOpacity; property alias iiBluetoothBatteryBackingOpacity: adapter.iiBluetoothBatteryBackingOpacity; property alias iiBluetoothBatteryOpts: adapter.iiBluetoothBatteryOpts
    property alias iiBluetoothHeadphoneEnabled: adapter.iiBluetoothHeadphoneEnabled; property alias iiBluetoothHeadphoneAnchor: adapter.iiBluetoothHeadphoneAnchor; property alias iiBluetoothHeadphoneX: adapter.iiBluetoothHeadphoneX; property alias iiBluetoothHeadphoneY: adapter.iiBluetoothHeadphoneY
    property alias iiBluetoothHeadphoneLocked: adapter.iiBluetoothHeadphoneLocked; property alias iiBluetoothHeadphoneScale: adapter.iiBluetoothHeadphoneScale; property alias iiBluetoothHeadphoneOpacity: adapter.iiBluetoothHeadphoneOpacity; property alias iiBluetoothHeadphoneBg: adapter.iiBluetoothHeadphoneBg
    property alias iiBluetoothHeadphoneColor: adapter.iiBluetoothHeadphoneColor; property alias iiBluetoothHeadphoneColor2: adapter.iiBluetoothHeadphoneColor2; property alias iiBluetoothHeadphoneStyle: adapter.iiBluetoothHeadphoneStyle; property alias iiBluetoothHeadphoneRadius: adapter.iiBluetoothHeadphoneRadius
    property alias iiBluetoothHeadphonePad: adapter.iiBluetoothHeadphonePad; property alias iiBluetoothHeadphoneBorder: adapter.iiBluetoothHeadphoneBorder; property alias iiBluetoothHeadphoneBorderOpacity: adapter.iiBluetoothHeadphoneBorderOpacity; property alias iiBluetoothHeadphoneBackingOpacity: adapter.iiBluetoothHeadphoneBackingOpacity; property alias iiBluetoothHeadphoneOpts: adapter.iiBluetoothHeadphoneOpts
    property alias iiMobileBatteryEnabled: adapter.iiMobileBatteryEnabled; property alias iiMobileBatteryAnchor: adapter.iiMobileBatteryAnchor; property alias iiMobileBatteryX: adapter.iiMobileBatteryX; property alias iiMobileBatteryY: adapter.iiMobileBatteryY
    property alias iiMobileBatteryLocked: adapter.iiMobileBatteryLocked; property alias iiMobileBatteryScale: adapter.iiMobileBatteryScale; property alias iiMobileBatteryOpacity: adapter.iiMobileBatteryOpacity; property alias iiMobileBatteryBg: adapter.iiMobileBatteryBg
    property alias iiMobileBatteryColor: adapter.iiMobileBatteryColor; property alias iiMobileBatteryColor2: adapter.iiMobileBatteryColor2; property alias iiMobileBatteryStyle: adapter.iiMobileBatteryStyle; property alias iiMobileBatteryRadius: adapter.iiMobileBatteryRadius
    property alias iiMobileBatteryPad: adapter.iiMobileBatteryPad; property alias iiMobileBatteryBorder: adapter.iiMobileBatteryBorder; property alias iiMobileBatteryBorderOpacity: adapter.iiMobileBatteryBorderOpacity; property alias iiMobileBatteryBackingOpacity: adapter.iiMobileBatteryBackingOpacity; property alias iiMobileBatteryOpts: adapter.iiMobileBatteryOpts
    property alias iiBluetoothHeadphoneCookieEnabled: adapter.iiBluetoothHeadphoneCookieEnabled; property alias iiBluetoothHeadphoneCookieAnchor: adapter.iiBluetoothHeadphoneCookieAnchor; property alias iiBluetoothHeadphoneCookieX: adapter.iiBluetoothHeadphoneCookieX; property alias iiBluetoothHeadphoneCookieY: adapter.iiBluetoothHeadphoneCookieY
    property alias iiBluetoothHeadphoneCookieLocked: adapter.iiBluetoothHeadphoneCookieLocked; property alias iiBluetoothHeadphoneCookieScale: adapter.iiBluetoothHeadphoneCookieScale; property alias iiBluetoothHeadphoneCookieOpacity: adapter.iiBluetoothHeadphoneCookieOpacity; property alias iiBluetoothHeadphoneCookieBg: adapter.iiBluetoothHeadphoneCookieBg
    property alias iiBluetoothHeadphoneCookieColor: adapter.iiBluetoothHeadphoneCookieColor; property alias iiBluetoothHeadphoneCookieColor2: adapter.iiBluetoothHeadphoneCookieColor2; property alias iiBluetoothHeadphoneCookieStyle: adapter.iiBluetoothHeadphoneCookieStyle; property alias iiBluetoothHeadphoneCookieRadius: adapter.iiBluetoothHeadphoneCookieRadius
    property alias iiBluetoothHeadphoneCookiePad: adapter.iiBluetoothHeadphoneCookiePad; property alias iiBluetoothHeadphoneCookieBorder: adapter.iiBluetoothHeadphoneCookieBorder; property alias iiBluetoothHeadphoneCookieBorderOpacity: adapter.iiBluetoothHeadphoneCookieBorderOpacity; property alias iiBluetoothHeadphoneCookieBackingOpacity: adapter.iiBluetoothHeadphoneCookieBackingOpacity; property alias iiBluetoothHeadphoneCookieOpts: adapter.iiBluetoothHeadphoneCookieOpts
    property alias iiBluetoothFillCardsEnabled: adapter.iiBluetoothFillCardsEnabled; property alias iiBluetoothFillCardsAnchor: adapter.iiBluetoothFillCardsAnchor; property alias iiBluetoothFillCardsX: adapter.iiBluetoothFillCardsX; property alias iiBluetoothFillCardsY: adapter.iiBluetoothFillCardsY
    property alias iiBluetoothFillCardsLocked: adapter.iiBluetoothFillCardsLocked; property alias iiBluetoothFillCardsScale: adapter.iiBluetoothFillCardsScale; property alias iiBluetoothFillCardsOpacity: adapter.iiBluetoothFillCardsOpacity; property alias iiBluetoothFillCardsBg: adapter.iiBluetoothFillCardsBg
    property alias iiBluetoothFillCardsColor: adapter.iiBluetoothFillCardsColor; property alias iiBluetoothFillCardsColor2: adapter.iiBluetoothFillCardsColor2; property alias iiBluetoothFillCardsStyle: adapter.iiBluetoothFillCardsStyle; property alias iiBluetoothFillCardsRadius: adapter.iiBluetoothFillCardsRadius
    property alias iiBluetoothFillCardsPad: adapter.iiBluetoothFillCardsPad; property alias iiBluetoothFillCardsBorder: adapter.iiBluetoothFillCardsBorder; property alias iiBluetoothFillCardsBorderOpacity: adapter.iiBluetoothFillCardsBorderOpacity; property alias iiBluetoothFillCardsBackingOpacity: adapter.iiBluetoothFillCardsBackingOpacity; property alias iiBluetoothFillCardsOpts: adapter.iiBluetoothFillCardsOpts
    property alias iiPcBatteryBarsEnabled: adapter.iiPcBatteryBarsEnabled; property alias iiPcBatteryBarsAnchor: adapter.iiPcBatteryBarsAnchor; property alias iiPcBatteryBarsX: adapter.iiPcBatteryBarsX; property alias iiPcBatteryBarsY: adapter.iiPcBatteryBarsY
    property alias iiPcBatteryBarsLocked: adapter.iiPcBatteryBarsLocked; property alias iiPcBatteryBarsScale: adapter.iiPcBatteryBarsScale; property alias iiPcBatteryBarsOpacity: adapter.iiPcBatteryBarsOpacity; property alias iiPcBatteryBarsBg: adapter.iiPcBatteryBarsBg
    property alias iiPcBatteryBarsColor: adapter.iiPcBatteryBarsColor; property alias iiPcBatteryBarsColor2: adapter.iiPcBatteryBarsColor2; property alias iiPcBatteryBarsStyle: adapter.iiPcBatteryBarsStyle; property alias iiPcBatteryBarsRadius: adapter.iiPcBatteryBarsRadius
    property alias iiPcBatteryBarsPad: adapter.iiPcBatteryBarsPad; property alias iiPcBatteryBarsBorder: adapter.iiPcBatteryBarsBorder; property alias iiPcBatteryBarsBorderOpacity: adapter.iiPcBatteryBarsBorderOpacity; property alias iiPcBatteryBarsBackingOpacity: adapter.iiPcBatteryBarsBackingOpacity; property alias iiPcBatteryBarsOpts: adapter.iiPcBatteryBarsOpts
    property alias iiPcBatteryCableEnabled: adapter.iiPcBatteryCableEnabled; property alias iiPcBatteryCableAnchor: adapter.iiPcBatteryCableAnchor; property alias iiPcBatteryCableX: adapter.iiPcBatteryCableX; property alias iiPcBatteryCableY: adapter.iiPcBatteryCableY
    property alias iiPcBatteryCableLocked: adapter.iiPcBatteryCableLocked; property alias iiPcBatteryCableScale: adapter.iiPcBatteryCableScale; property alias iiPcBatteryCableOpacity: adapter.iiPcBatteryCableOpacity; property alias iiPcBatteryCableBg: adapter.iiPcBatteryCableBg
    property alias iiPcBatteryCableColor: adapter.iiPcBatteryCableColor; property alias iiPcBatteryCableColor2: adapter.iiPcBatteryCableColor2; property alias iiPcBatteryCableStyle: adapter.iiPcBatteryCableStyle; property alias iiPcBatteryCableRadius: adapter.iiPcBatteryCableRadius
    property alias iiPcBatteryCablePad: adapter.iiPcBatteryCablePad; property alias iiPcBatteryCableBorder: adapter.iiPcBatteryCableBorder; property alias iiPcBatteryCableBorderOpacity: adapter.iiPcBatteryCableBorderOpacity; property alias iiPcBatteryCableBackingOpacity: adapter.iiPcBatteryCableBackingOpacity; property alias iiPcBatteryCableOpts: adapter.iiPcBatteryCableOpts
    property alias iiDevicesBatteryListEnabled: adapter.iiDevicesBatteryListEnabled; property alias iiDevicesBatteryListAnchor: adapter.iiDevicesBatteryListAnchor; property alias iiDevicesBatteryListX: adapter.iiDevicesBatteryListX; property alias iiDevicesBatteryListY: adapter.iiDevicesBatteryListY
    property alias iiDevicesBatteryListLocked: adapter.iiDevicesBatteryListLocked; property alias iiDevicesBatteryListScale: adapter.iiDevicesBatteryListScale; property alias iiDevicesBatteryListOpacity: adapter.iiDevicesBatteryListOpacity; property alias iiDevicesBatteryListBg: adapter.iiDevicesBatteryListBg
    property alias iiDevicesBatteryListColor: adapter.iiDevicesBatteryListColor; property alias iiDevicesBatteryListColor2: adapter.iiDevicesBatteryListColor2; property alias iiDevicesBatteryListStyle: adapter.iiDevicesBatteryListStyle; property alias iiDevicesBatteryListRadius: adapter.iiDevicesBatteryListRadius
    property alias iiDevicesBatteryListPad: adapter.iiDevicesBatteryListPad; property alias iiDevicesBatteryListBorder: adapter.iiDevicesBatteryListBorder; property alias iiDevicesBatteryListBorderOpacity: adapter.iiDevicesBatteryListBorderOpacity; property alias iiDevicesBatteryListBackingOpacity: adapter.iiDevicesBatteryListBackingOpacity; property alias iiDevicesBatteryListOpts: adapter.iiDevicesBatteryListOpts
    property alias iiDevicesBatteryList1x1Enabled: adapter.iiDevicesBatteryList1x1Enabled; property alias iiDevicesBatteryList1x1Anchor: adapter.iiDevicesBatteryList1x1Anchor; property alias iiDevicesBatteryList1x1X: adapter.iiDevicesBatteryList1x1X; property alias iiDevicesBatteryList1x1Y: adapter.iiDevicesBatteryList1x1Y
    property alias iiDevicesBatteryList1x1Locked: adapter.iiDevicesBatteryList1x1Locked; property alias iiDevicesBatteryList1x1Scale: adapter.iiDevicesBatteryList1x1Scale; property alias iiDevicesBatteryList1x1Opacity: adapter.iiDevicesBatteryList1x1Opacity; property alias iiDevicesBatteryList1x1Bg: adapter.iiDevicesBatteryList1x1Bg
    property alias iiDevicesBatteryList1x1Color: adapter.iiDevicesBatteryList1x1Color; property alias iiDevicesBatteryList1x1Color2: adapter.iiDevicesBatteryList1x1Color2; property alias iiDevicesBatteryList1x1Style: adapter.iiDevicesBatteryList1x1Style; property alias iiDevicesBatteryList1x1Radius: adapter.iiDevicesBatteryList1x1Radius
    property alias iiDevicesBatteryList1x1Pad: adapter.iiDevicesBatteryList1x1Pad; property alias iiDevicesBatteryList1x1Border: adapter.iiDevicesBatteryList1x1Border; property alias iiDevicesBatteryList1x1BorderOpacity: adapter.iiDevicesBatteryList1x1BorderOpacity; property alias iiDevicesBatteryList1x1BackingOpacity: adapter.iiDevicesBatteryList1x1BackingOpacity; property alias iiDevicesBatteryList1x1Opts: adapter.iiDevicesBatteryList1x1Opts
    property alias iiBluetoothEarbudsStemEnabled: adapter.iiBluetoothEarbudsStemEnabled; property alias iiBluetoothEarbudsStemAnchor: adapter.iiBluetoothEarbudsStemAnchor; property alias iiBluetoothEarbudsStemX: adapter.iiBluetoothEarbudsStemX; property alias iiBluetoothEarbudsStemY: adapter.iiBluetoothEarbudsStemY
    property alias iiBluetoothEarbudsStemLocked: adapter.iiBluetoothEarbudsStemLocked; property alias iiBluetoothEarbudsStemScale: adapter.iiBluetoothEarbudsStemScale; property alias iiBluetoothEarbudsStemOpacity: adapter.iiBluetoothEarbudsStemOpacity; property alias iiBluetoothEarbudsStemBg: adapter.iiBluetoothEarbudsStemBg
    property alias iiBluetoothEarbudsStemColor: adapter.iiBluetoothEarbudsStemColor; property alias iiBluetoothEarbudsStemColor2: adapter.iiBluetoothEarbudsStemColor2; property alias iiBluetoothEarbudsStemStyle: adapter.iiBluetoothEarbudsStemStyle; property alias iiBluetoothEarbudsStemRadius: adapter.iiBluetoothEarbudsStemRadius
    property alias iiBluetoothEarbudsStemPad: adapter.iiBluetoothEarbudsStemPad; property alias iiBluetoothEarbudsStemBorder: adapter.iiBluetoothEarbudsStemBorder; property alias iiBluetoothEarbudsStemBorderOpacity: adapter.iiBluetoothEarbudsStemBorderOpacity; property alias iiBluetoothEarbudsStemBackingOpacity: adapter.iiBluetoothEarbudsStemBackingOpacity; property alias iiBluetoothEarbudsStemOpts: adapter.iiBluetoothEarbudsStemOpts
    property alias iiEmailInboxEnabled: adapter.iiEmailInboxEnabled; property alias iiEmailInboxAnchor: adapter.iiEmailInboxAnchor; property alias iiEmailInboxX: adapter.iiEmailInboxX; property alias iiEmailInboxY: adapter.iiEmailInboxY
    property alias iiEmailInboxLocked: adapter.iiEmailInboxLocked; property alias iiEmailInboxScale: adapter.iiEmailInboxScale; property alias iiEmailInboxOpacity: adapter.iiEmailInboxOpacity; property alias iiEmailInboxBg: adapter.iiEmailInboxBg
    property alias iiEmailInboxColor: adapter.iiEmailInboxColor; property alias iiEmailInboxColor2: adapter.iiEmailInboxColor2; property alias iiEmailInboxStyle: adapter.iiEmailInboxStyle; property alias iiEmailInboxRadius: adapter.iiEmailInboxRadius
    property alias iiEmailInboxPad: adapter.iiEmailInboxPad; property alias iiEmailInboxBorder: adapter.iiEmailInboxBorder; property alias iiEmailInboxBorderOpacity: adapter.iiEmailInboxBorderOpacity; property alias iiEmailInboxBackingOpacity: adapter.iiEmailInboxBackingOpacity; property alias iiEmailInboxOpts: adapter.iiEmailInboxOpts
    property alias iiEmailInbox2x1Enabled: adapter.iiEmailInbox2x1Enabled; property alias iiEmailInbox2x1Anchor: adapter.iiEmailInbox2x1Anchor; property alias iiEmailInbox2x1X: adapter.iiEmailInbox2x1X; property alias iiEmailInbox2x1Y: adapter.iiEmailInbox2x1Y
    property alias iiEmailInbox2x1Locked: adapter.iiEmailInbox2x1Locked; property alias iiEmailInbox2x1Scale: adapter.iiEmailInbox2x1Scale; property alias iiEmailInbox2x1Opacity: adapter.iiEmailInbox2x1Opacity; property alias iiEmailInbox2x1Bg: adapter.iiEmailInbox2x1Bg
    property alias iiEmailInbox2x1Color: adapter.iiEmailInbox2x1Color; property alias iiEmailInbox2x1Color2: adapter.iiEmailInbox2x1Color2; property alias iiEmailInbox2x1Style: adapter.iiEmailInbox2x1Style; property alias iiEmailInbox2x1Radius: adapter.iiEmailInbox2x1Radius
    property alias iiEmailInbox2x1Pad: adapter.iiEmailInbox2x1Pad; property alias iiEmailInbox2x1Border: adapter.iiEmailInbox2x1Border; property alias iiEmailInbox2x1BorderOpacity: adapter.iiEmailInbox2x1BorderOpacity; property alias iiEmailInbox2x1BackingOpacity: adapter.iiEmailInbox2x1BackingOpacity; property alias iiEmailInbox2x1Opts: adapter.iiEmailInbox2x1Opts
    property alias iiAiChatEnabled: adapter.iiAiChatEnabled; property alias iiAiChatAnchor: adapter.iiAiChatAnchor; property alias iiAiChatX: adapter.iiAiChatX; property alias iiAiChatY: adapter.iiAiChatY
    property alias iiAiChatLocked: adapter.iiAiChatLocked; property alias iiAiChatScale: adapter.iiAiChatScale; property alias iiAiChatOpacity: adapter.iiAiChatOpacity; property alias iiAiChatBg: adapter.iiAiChatBg
    property alias iiAiChatColor: adapter.iiAiChatColor; property alias iiAiChatColor2: adapter.iiAiChatColor2; property alias iiAiChatStyle: adapter.iiAiChatStyle; property alias iiAiChatRadius: adapter.iiAiChatRadius
    property alias iiAiChatPad: adapter.iiAiChatPad; property alias iiAiChatBorder: adapter.iiAiChatBorder; property alias iiAiChatBorderOpacity: adapter.iiAiChatBorderOpacity; property alias iiAiChatBackingOpacity: adapter.iiAiChatBackingOpacity; property alias iiAiChatOpts: adapter.iiAiChatOpts
    property alias iiNotesWidgetEnabled: adapter.iiNotesWidgetEnabled; property alias iiNotesWidgetAnchor: adapter.iiNotesWidgetAnchor; property alias iiNotesWidgetX: adapter.iiNotesWidgetX; property alias iiNotesWidgetY: adapter.iiNotesWidgetY
    property alias iiNotesWidgetLocked: adapter.iiNotesWidgetLocked; property alias iiNotesWidgetScale: adapter.iiNotesWidgetScale; property alias iiNotesWidgetOpacity: adapter.iiNotesWidgetOpacity; property alias iiNotesWidgetBg: adapter.iiNotesWidgetBg
    property alias iiNotesWidgetColor: adapter.iiNotesWidgetColor; property alias iiNotesWidgetColor2: adapter.iiNotesWidgetColor2; property alias iiNotesWidgetStyle: adapter.iiNotesWidgetStyle; property alias iiNotesWidgetRadius: adapter.iiNotesWidgetRadius
    property alias iiNotesWidgetPad: adapter.iiNotesWidgetPad; property alias iiNotesWidgetBorder: adapter.iiNotesWidgetBorder; property alias iiNotesWidgetBorderOpacity: adapter.iiNotesWidgetBorderOpacity; property alias iiNotesWidgetBackingOpacity: adapter.iiNotesWidgetBackingOpacity; property alias iiNotesWidgetOpts: adapter.iiNotesWidgetOpts
    property alias iiNotesWidget2x1Enabled: adapter.iiNotesWidget2x1Enabled; property alias iiNotesWidget2x1Anchor: adapter.iiNotesWidget2x1Anchor; property alias iiNotesWidget2x1X: adapter.iiNotesWidget2x1X; property alias iiNotesWidget2x1Y: adapter.iiNotesWidget2x1Y
    property alias iiNotesWidget2x1Locked: adapter.iiNotesWidget2x1Locked; property alias iiNotesWidget2x1Scale: adapter.iiNotesWidget2x1Scale; property alias iiNotesWidget2x1Opacity: adapter.iiNotesWidget2x1Opacity; property alias iiNotesWidget2x1Bg: adapter.iiNotesWidget2x1Bg
    property alias iiNotesWidget2x1Color: adapter.iiNotesWidget2x1Color; property alias iiNotesWidget2x1Color2: adapter.iiNotesWidget2x1Color2; property alias iiNotesWidget2x1Style: adapter.iiNotesWidget2x1Style; property alias iiNotesWidget2x1Radius: adapter.iiNotesWidget2x1Radius
    property alias iiNotesWidget2x1Pad: adapter.iiNotesWidget2x1Pad; property alias iiNotesWidget2x1Border: adapter.iiNotesWidget2x1Border; property alias iiNotesWidget2x1BorderOpacity: adapter.iiNotesWidget2x1BorderOpacity; property alias iiNotesWidget2x1BackingOpacity: adapter.iiNotesWidget2x1BackingOpacity; property alias iiNotesWidget2x1Opts: adapter.iiNotesWidget2x1Opts
    property alias iiCompactMediaEnabled: adapter.iiCompactMediaEnabled; property alias iiCompactMediaAnchor: adapter.iiCompactMediaAnchor; property alias iiCompactMediaX: adapter.iiCompactMediaX; property alias iiCompactMediaY: adapter.iiCompactMediaY
    property alias iiCompactMediaLocked: adapter.iiCompactMediaLocked; property alias iiCompactMediaScale: adapter.iiCompactMediaScale; property alias iiCompactMediaOpacity: adapter.iiCompactMediaOpacity; property alias iiCompactMediaBg: adapter.iiCompactMediaBg
    property alias iiCompactMediaColor: adapter.iiCompactMediaColor; property alias iiCompactMediaColor2: adapter.iiCompactMediaColor2; property alias iiCompactMediaStyle: adapter.iiCompactMediaStyle; property alias iiCompactMediaRadius: adapter.iiCompactMediaRadius
    property alias iiCompactMediaPad: adapter.iiCompactMediaPad; property alias iiCompactMediaBorder: adapter.iiCompactMediaBorder; property alias iiCompactMediaBorderOpacity: adapter.iiCompactMediaBorderOpacity; property alias iiCompactMediaBackingOpacity: adapter.iiCompactMediaBackingOpacity; property alias iiCompactMediaOpts: adapter.iiCompactMediaOpts
    property alias iiQuickActionsEnabled: adapter.iiQuickActionsEnabled; property alias iiQuickActionsAnchor: adapter.iiQuickActionsAnchor; property alias iiQuickActionsX: adapter.iiQuickActionsX; property alias iiQuickActionsY: adapter.iiQuickActionsY
    property alias iiQuickActionsLocked: adapter.iiQuickActionsLocked; property alias iiQuickActionsScale: adapter.iiQuickActionsScale; property alias iiQuickActionsOpacity: adapter.iiQuickActionsOpacity; property alias iiQuickActionsBg: adapter.iiQuickActionsBg
    property alias iiQuickActionsColor: adapter.iiQuickActionsColor; property alias iiQuickActionsColor2: adapter.iiQuickActionsColor2; property alias iiQuickActionsStyle: adapter.iiQuickActionsStyle; property alias iiQuickActionsRadius: adapter.iiQuickActionsRadius
    property alias iiQuickActionsPad: adapter.iiQuickActionsPad; property alias iiQuickActionsBorder: adapter.iiQuickActionsBorder; property alias iiQuickActionsBorderOpacity: adapter.iiQuickActionsBorderOpacity; property alias iiQuickActionsBackingOpacity: adapter.iiQuickActionsBackingOpacity; property alias iiQuickActionsOpts: adapter.iiQuickActionsOpts
    property alias iiAndroidSearchBarEnabled: adapter.iiAndroidSearchBarEnabled; property alias iiAndroidSearchBarAnchor: adapter.iiAndroidSearchBarAnchor; property alias iiAndroidSearchBarX: adapter.iiAndroidSearchBarX; property alias iiAndroidSearchBarY: adapter.iiAndroidSearchBarY
    property alias iiAndroidSearchBarLocked: adapter.iiAndroidSearchBarLocked; property alias iiAndroidSearchBarScale: adapter.iiAndroidSearchBarScale; property alias iiAndroidSearchBarOpacity: adapter.iiAndroidSearchBarOpacity; property alias iiAndroidSearchBarBg: adapter.iiAndroidSearchBarBg
    property alias iiAndroidSearchBarColor: adapter.iiAndroidSearchBarColor; property alias iiAndroidSearchBarColor2: adapter.iiAndroidSearchBarColor2; property alias iiAndroidSearchBarStyle: adapter.iiAndroidSearchBarStyle; property alias iiAndroidSearchBarRadius: adapter.iiAndroidSearchBarRadius
    property alias iiAndroidSearchBarPad: adapter.iiAndroidSearchBarPad; property alias iiAndroidSearchBarBorder: adapter.iiAndroidSearchBarBorder; property alias iiAndroidSearchBarBorderOpacity: adapter.iiAndroidSearchBarBorderOpacity; property alias iiAndroidSearchBarBackingOpacity: adapter.iiAndroidSearchBarBackingOpacity; property alias iiAndroidSearchBarOpts: adapter.iiAndroidSearchBarOpts
    property alias iiSearchPillEnabled: adapter.iiSearchPillEnabled; property alias iiSearchPillAnchor: adapter.iiSearchPillAnchor; property alias iiSearchPillX: adapter.iiSearchPillX; property alias iiSearchPillY: adapter.iiSearchPillY
    property alias iiSearchPillLocked: adapter.iiSearchPillLocked; property alias iiSearchPillScale: adapter.iiSearchPillScale; property alias iiSearchPillOpacity: adapter.iiSearchPillOpacity; property alias iiSearchPillBg: adapter.iiSearchPillBg
    property alias iiSearchPillColor: adapter.iiSearchPillColor; property alias iiSearchPillColor2: adapter.iiSearchPillColor2; property alias iiSearchPillStyle: adapter.iiSearchPillStyle; property alias iiSearchPillRadius: adapter.iiSearchPillRadius
    property alias iiSearchPillPad: adapter.iiSearchPillPad; property alias iiSearchPillBorder: adapter.iiSearchPillBorder; property alias iiSearchPillBorderOpacity: adapter.iiSearchPillBorderOpacity; property alias iiSearchPillBackingOpacity: adapter.iiSearchPillBackingOpacity; property alias iiSearchPillOpts: adapter.iiSearchPillOpts
    property alias iiQuoteEnabled: adapter.iiQuoteEnabled; property alias iiQuoteAnchor: adapter.iiQuoteAnchor; property alias iiQuoteX: adapter.iiQuoteX; property alias iiQuoteY: adapter.iiQuoteY
    property alias iiQuoteLocked: adapter.iiQuoteLocked; property alias iiQuoteScale: adapter.iiQuoteScale; property alias iiQuoteOpacity: adapter.iiQuoteOpacity; property alias iiQuoteBg: adapter.iiQuoteBg
    property alias iiQuoteColor: adapter.iiQuoteColor; property alias iiQuoteColor2: adapter.iiQuoteColor2; property alias iiQuoteStyle: adapter.iiQuoteStyle; property alias iiQuoteRadius: adapter.iiQuoteRadius
    property alias iiQuotePad: adapter.iiQuotePad; property alias iiQuoteBorder: adapter.iiQuoteBorder; property alias iiQuoteBorderOpacity: adapter.iiQuoteBorderOpacity; property alias iiQuoteBackingOpacity: adapter.iiQuoteBackingOpacity; property alias iiQuoteOpts: adapter.iiQuoteOpts
    property alias iiWaterReminderEnabled: adapter.iiWaterReminderEnabled; property alias iiWaterReminderAnchor: adapter.iiWaterReminderAnchor; property alias iiWaterReminderX: adapter.iiWaterReminderX; property alias iiWaterReminderY: adapter.iiWaterReminderY
    property alias iiWaterReminderLocked: adapter.iiWaterReminderLocked; property alias iiWaterReminderScale: adapter.iiWaterReminderScale; property alias iiWaterReminderOpacity: adapter.iiWaterReminderOpacity; property alias iiWaterReminderBg: adapter.iiWaterReminderBg
    property alias iiWaterReminderColor: adapter.iiWaterReminderColor; property alias iiWaterReminderColor2: adapter.iiWaterReminderColor2; property alias iiWaterReminderStyle: adapter.iiWaterReminderStyle; property alias iiWaterReminderRadius: adapter.iiWaterReminderRadius
    property alias iiWaterReminderPad: adapter.iiWaterReminderPad; property alias iiWaterReminderBorder: adapter.iiWaterReminderBorder; property alias iiWaterReminderBorderOpacity: adapter.iiWaterReminderBorderOpacity; property alias iiWaterReminderBackingOpacity: adapter.iiWaterReminderBackingOpacity; property alias iiWaterReminderOpts: adapter.iiWaterReminderOpts
    property alias iiAtAGlanceEnabled: adapter.iiAtAGlanceEnabled; property alias iiAtAGlanceAnchor: adapter.iiAtAGlanceAnchor; property alias iiAtAGlanceX: adapter.iiAtAGlanceX; property alias iiAtAGlanceY: adapter.iiAtAGlanceY
    property alias iiAtAGlanceLocked: adapter.iiAtAGlanceLocked; property alias iiAtAGlanceScale: adapter.iiAtAGlanceScale; property alias iiAtAGlanceOpacity: adapter.iiAtAGlanceOpacity; property alias iiAtAGlanceBg: adapter.iiAtAGlanceBg
    property alias iiAtAGlanceColor: adapter.iiAtAGlanceColor; property alias iiAtAGlanceColor2: adapter.iiAtAGlanceColor2; property alias iiAtAGlanceStyle: adapter.iiAtAGlanceStyle; property alias iiAtAGlanceRadius: adapter.iiAtAGlanceRadius
    property alias iiAtAGlancePad: adapter.iiAtAGlancePad; property alias iiAtAGlanceBorder: adapter.iiAtAGlanceBorder; property alias iiAtAGlanceBorderOpacity: adapter.iiAtAGlanceBorderOpacity; property alias iiAtAGlanceBackingOpacity: adapter.iiAtAGlanceBackingOpacity; property alias iiAtAGlanceOpts: adapter.iiAtAGlanceOpts
    property alias iiResourceCpuPillEnabled: adapter.iiResourceCpuPillEnabled; property alias iiResourceCpuPillAnchor: adapter.iiResourceCpuPillAnchor; property alias iiResourceCpuPillX: adapter.iiResourceCpuPillX; property alias iiResourceCpuPillY: adapter.iiResourceCpuPillY
    property alias iiResourceCpuPillLocked: adapter.iiResourceCpuPillLocked; property alias iiResourceCpuPillScale: adapter.iiResourceCpuPillScale; property alias iiResourceCpuPillOpacity: adapter.iiResourceCpuPillOpacity; property alias iiResourceCpuPillBg: adapter.iiResourceCpuPillBg
    property alias iiResourceCpuPillColor: adapter.iiResourceCpuPillColor; property alias iiResourceCpuPillColor2: adapter.iiResourceCpuPillColor2; property alias iiResourceCpuPillStyle: adapter.iiResourceCpuPillStyle; property alias iiResourceCpuPillRadius: adapter.iiResourceCpuPillRadius
    property alias iiResourceCpuPillPad: adapter.iiResourceCpuPillPad; property alias iiResourceCpuPillBorder: adapter.iiResourceCpuPillBorder; property alias iiResourceCpuPillBorderOpacity: adapter.iiResourceCpuPillBorderOpacity; property alias iiResourceCpuPillBackingOpacity: adapter.iiResourceCpuPillBackingOpacity; property alias iiResourceCpuPillOpts: adapter.iiResourceCpuPillOpts
    property alias iiResourceRamPillEnabled: adapter.iiResourceRamPillEnabled; property alias iiResourceRamPillAnchor: adapter.iiResourceRamPillAnchor; property alias iiResourceRamPillX: adapter.iiResourceRamPillX; property alias iiResourceRamPillY: adapter.iiResourceRamPillY
    property alias iiResourceRamPillLocked: adapter.iiResourceRamPillLocked; property alias iiResourceRamPillScale: adapter.iiResourceRamPillScale; property alias iiResourceRamPillOpacity: adapter.iiResourceRamPillOpacity; property alias iiResourceRamPillBg: adapter.iiResourceRamPillBg
    property alias iiResourceRamPillColor: adapter.iiResourceRamPillColor; property alias iiResourceRamPillColor2: adapter.iiResourceRamPillColor2; property alias iiResourceRamPillStyle: adapter.iiResourceRamPillStyle; property alias iiResourceRamPillRadius: adapter.iiResourceRamPillRadius
    property alias iiResourceRamPillPad: adapter.iiResourceRamPillPad; property alias iiResourceRamPillBorder: adapter.iiResourceRamPillBorder; property alias iiResourceRamPillBorderOpacity: adapter.iiResourceRamPillBorderOpacity; property alias iiResourceRamPillBackingOpacity: adapter.iiResourceRamPillBackingOpacity; property alias iiResourceRamPillOpts: adapter.iiResourceRamPillOpts
    property alias iiResourceDiskPillEnabled: adapter.iiResourceDiskPillEnabled; property alias iiResourceDiskPillAnchor: adapter.iiResourceDiskPillAnchor; property alias iiResourceDiskPillX: adapter.iiResourceDiskPillX; property alias iiResourceDiskPillY: adapter.iiResourceDiskPillY
    property alias iiResourceDiskPillLocked: adapter.iiResourceDiskPillLocked; property alias iiResourceDiskPillScale: adapter.iiResourceDiskPillScale; property alias iiResourceDiskPillOpacity: adapter.iiResourceDiskPillOpacity; property alias iiResourceDiskPillBg: adapter.iiResourceDiskPillBg
    property alias iiResourceDiskPillColor: adapter.iiResourceDiskPillColor; property alias iiResourceDiskPillColor2: adapter.iiResourceDiskPillColor2; property alias iiResourceDiskPillStyle: adapter.iiResourceDiskPillStyle; property alias iiResourceDiskPillRadius: adapter.iiResourceDiskPillRadius
    property alias iiResourceDiskPillPad: adapter.iiResourceDiskPillPad; property alias iiResourceDiskPillBorder: adapter.iiResourceDiskPillBorder; property alias iiResourceDiskPillBorderOpacity: adapter.iiResourceDiskPillBorderOpacity; property alias iiResourceDiskPillBackingOpacity: adapter.iiResourceDiskPillBackingOpacity; property alias iiResourceDiskPillOpts: adapter.iiResourceDiskPillOpts
    property alias iiResourceFillCardsEnabled: adapter.iiResourceFillCardsEnabled; property alias iiResourceFillCardsAnchor: adapter.iiResourceFillCardsAnchor; property alias iiResourceFillCardsX: adapter.iiResourceFillCardsX; property alias iiResourceFillCardsY: adapter.iiResourceFillCardsY
    property alias iiResourceFillCardsLocked: adapter.iiResourceFillCardsLocked; property alias iiResourceFillCardsScale: adapter.iiResourceFillCardsScale; property alias iiResourceFillCardsOpacity: adapter.iiResourceFillCardsOpacity; property alias iiResourceFillCardsBg: adapter.iiResourceFillCardsBg
    property alias iiResourceFillCardsColor: adapter.iiResourceFillCardsColor; property alias iiResourceFillCardsColor2: adapter.iiResourceFillCardsColor2; property alias iiResourceFillCardsStyle: adapter.iiResourceFillCardsStyle; property alias iiResourceFillCardsRadius: adapter.iiResourceFillCardsRadius
    property alias iiResourceFillCardsPad: adapter.iiResourceFillCardsPad; property alias iiResourceFillCardsBorder: adapter.iiResourceFillCardsBorder; property alias iiResourceFillCardsBorderOpacity: adapter.iiResourceFillCardsBorderOpacity; property alias iiResourceFillCardsBackingOpacity: adapter.iiResourceFillCardsBackingOpacity; property alias iiResourceFillCardsOpts: adapter.iiResourceFillCardsOpts
    property alias iiResourceNothingDiskEnabled: adapter.iiResourceNothingDiskEnabled; property alias iiResourceNothingDiskAnchor: adapter.iiResourceNothingDiskAnchor; property alias iiResourceNothingDiskX: adapter.iiResourceNothingDiskX; property alias iiResourceNothingDiskY: adapter.iiResourceNothingDiskY
    property alias iiResourceNothingDiskLocked: adapter.iiResourceNothingDiskLocked; property alias iiResourceNothingDiskScale: adapter.iiResourceNothingDiskScale; property alias iiResourceNothingDiskOpacity: adapter.iiResourceNothingDiskOpacity; property alias iiResourceNothingDiskBg: adapter.iiResourceNothingDiskBg
    property alias iiResourceNothingDiskColor: adapter.iiResourceNothingDiskColor; property alias iiResourceNothingDiskColor2: adapter.iiResourceNothingDiskColor2; property alias iiResourceNothingDiskStyle: adapter.iiResourceNothingDiskStyle; property alias iiResourceNothingDiskRadius: adapter.iiResourceNothingDiskRadius
    property alias iiResourceNothingDiskPad: adapter.iiResourceNothingDiskPad; property alias iiResourceNothingDiskBorder: adapter.iiResourceNothingDiskBorder; property alias iiResourceNothingDiskBorderOpacity: adapter.iiResourceNothingDiskBorderOpacity; property alias iiResourceNothingDiskBackingOpacity: adapter.iiResourceNothingDiskBackingOpacity; property alias iiResourceNothingDiskOpts: adapter.iiResourceNothingDiskOpts
    property alias iiResourceNothingCpuEnabled: adapter.iiResourceNothingCpuEnabled; property alias iiResourceNothingCpuAnchor: adapter.iiResourceNothingCpuAnchor; property alias iiResourceNothingCpuX: adapter.iiResourceNothingCpuX; property alias iiResourceNothingCpuY: adapter.iiResourceNothingCpuY
    property alias iiResourceNothingCpuLocked: adapter.iiResourceNothingCpuLocked; property alias iiResourceNothingCpuScale: adapter.iiResourceNothingCpuScale; property alias iiResourceNothingCpuOpacity: adapter.iiResourceNothingCpuOpacity; property alias iiResourceNothingCpuBg: adapter.iiResourceNothingCpuBg
    property alias iiResourceNothingCpuColor: adapter.iiResourceNothingCpuColor; property alias iiResourceNothingCpuColor2: adapter.iiResourceNothingCpuColor2; property alias iiResourceNothingCpuStyle: adapter.iiResourceNothingCpuStyle; property alias iiResourceNothingCpuRadius: adapter.iiResourceNothingCpuRadius
    property alias iiResourceNothingCpuPad: adapter.iiResourceNothingCpuPad; property alias iiResourceNothingCpuBorder: adapter.iiResourceNothingCpuBorder; property alias iiResourceNothingCpuBorderOpacity: adapter.iiResourceNothingCpuBorderOpacity; property alias iiResourceNothingCpuBackingOpacity: adapter.iiResourceNothingCpuBackingOpacity; property alias iiResourceNothingCpuOpts: adapter.iiResourceNothingCpuOpts
    property alias iiResourceNothingRamEnabled: adapter.iiResourceNothingRamEnabled; property alias iiResourceNothingRamAnchor: adapter.iiResourceNothingRamAnchor; property alias iiResourceNothingRamX: adapter.iiResourceNothingRamX; property alias iiResourceNothingRamY: adapter.iiResourceNothingRamY
    property alias iiResourceNothingRamLocked: adapter.iiResourceNothingRamLocked; property alias iiResourceNothingRamScale: adapter.iiResourceNothingRamScale; property alias iiResourceNothingRamOpacity: adapter.iiResourceNothingRamOpacity; property alias iiResourceNothingRamBg: adapter.iiResourceNothingRamBg
    property alias iiResourceNothingRamColor: adapter.iiResourceNothingRamColor; property alias iiResourceNothingRamColor2: adapter.iiResourceNothingRamColor2; property alias iiResourceNothingRamStyle: adapter.iiResourceNothingRamStyle; property alias iiResourceNothingRamRadius: adapter.iiResourceNothingRamRadius
    property alias iiResourceNothingRamPad: adapter.iiResourceNothingRamPad; property alias iiResourceNothingRamBorder: adapter.iiResourceNothingRamBorder; property alias iiResourceNothingRamBorderOpacity: adapter.iiResourceNothingRamBorderOpacity; property alias iiResourceNothingRamBackingOpacity: adapter.iiResourceNothingRamBackingOpacity; property alias iiResourceNothingRamOpts: adapter.iiResourceNothingRamOpts

    // brand: the desktop's mark + name, user-overridable from Ryoku Settings ->
    // Shell -> Global. a small cross-cutting identity master (like theme.json).
    // markText is the glyph/short-text seal (default 力); markImage an optional
    // image path that wins over the text; markTint recolours a single-colour
    // image to the accent; name is the wordmark ("Ryoku") shown in chrome copy.
    // Ryoku's own apps (the Hub, ryo* apps) never read this and keep the 力 brand.
    property alias markText:  brandAdapter.markText
    property alias markImage: brandAdapter.markImage
    property alias markTint:  brandAdapter.markTint
    property alias brandName: brandAdapter.name

    // The desktop sets this before opening one of its menus. Once that output
    // owns a fork, the unchanged menu API writes back to that output instead
    // of changing the legacy values followed by every unforked output.
    property string writeMonitor: ""
    property bool writeMonitorForced: false
    // FileView replaces var-valued adapter properties while a watched atomic
    // write reloads. Keep the live forks and item-provided widget keys detached
    // so that reload cannot erase an edit made after the write it is returning.
    property var _monitorState: ({})
    property var _contributedState: ({})
    property int _stateRevision: 0
    property int _flushedStateRevision: 0
    property int _loadingStateRevision: 0

    // Emitted after widgets.json is written, so readers outside the shell
    // (the store's in-use scan) see the new selection.
    signal saved()

    function _plainMap(value) {
        if (!value || typeof value !== "object")
            return {};
        try {
            return JSON.parse(JSON.stringify(value)) || {};
        } catch (e) {
            return {};
        }
    }

    function _finishLoad() {
        if (root._loadingStateRevision === root._stateRevision) {
            root._monitorState = root._plainMap(adapter.monitors);
            root._contributedState = root._plainMap(adapter.contributed);
            root._flushedStateRevision = root._stateRevision;
        } else {
            adapter.monitors = root._plainMap(root._monitorState);
            adapter.contributed = root._plainMap(root._contributedState);
        }
        root._absorbLooseKeys();
        root.ready = true;
    }

    // Item-provided widget keys written at the top level (older files, a rice
    // apply) are not adapter properties, so move them into `contributed`.
    // An older <prefix>Variant becomes <prefix>Size unless a Size is already set.
    function _absorbLooseKeys() {
        let raw = null;
        try {
            raw = JSON.parse(file.text());
        } catch (e) {
            return;
        }
        if (!raw || typeof raw !== "object")
            return;
        const has = (map, key) => Object.prototype.hasOwnProperty.call(map, key);
        const next = Object.assign({}, root._contributedState);
        let found = false;
        for (const key in raw) {
            const renamed = key.endsWith("Variant");
            const target = renamed ? key.slice(0, -7) + "Size" : key;
            if (root._isAdapterKey(target) || WidgetKeys.split(target) === null
                    || has(next, target) || (renamed && has(raw, target)))
                continue;
            next[target] = raw[key];
            found = true;
        }
        if (!found)
            return;
        root._contributedState = next;
        root._stateRevision++;
        settle.restart();
    }

    function selectMonitor(monitor, forced = false) {
        root.writeMonitor = monitor || "";
        root.writeMonitorForced = root.writeMonitor !== "" && forced;
    }

    function _usesMonitorContext() {
        return root.writeMonitor !== ""
            && (root.writeMonitorForced || root.isForked(root.writeMonitor));
    }

    function _monitorMap(monitor) {
        const all = root._monitorState;
        if (!monitor || !all || typeof all !== "object")
            return null;
        const local = all[monitor];
        return local && typeof local === "object" ? local : null;
    }

    function isForked(monitor) {
        return root._monitorMap(monitor) !== null;
    }

    function get(key, monitor) {
        if (!root._isPerMonitorKey(key))
            return root._sharedValue(key);
        const local = root._monitorMap(monitor);
        return local && Object.prototype.hasOwnProperty.call(local, key)
            ? local[key] : root._sharedValue(key);
    }

    function _sharedValue(key) {
        if (root._isAdapterKey(key))
            return adapter[key];
        const stored = root._contributedState[key];
        return stored !== undefined ? stored : WidgetKeys.fallback(key);
    }

    // Depth predates output forks, so an output without its own key keeps
    // reading the old map until its first edit writes beside placement.
    function widgetDepth(id, monitor) {
        const key = id + "Depth";
        const local = root._monitorMap(monitor);
        const legacy = adapter.widgetDepths || {};
        const value = local && Object.prototype.hasOwnProperty.call(local, key)
            ? local[key] : legacy[id];
        return typeof value === "number"
            ? Math.max(0, Math.min(1, value)) : 0.5;
    }
    function setWidgetDepth(monitor, id, value) {
        if (!monitor || !id)
            return;
        root.setFor(monitor, id + "Depth",
            Math.max(0, Math.min(1, value)));
    }

    // Find the widget owning a schema key from its <widget>Enabled property.
    // This keeps hosted rosters extensible: adding a new face to the adapter
    // automatically makes every one of that face's keys fork together. Keys
    // that are not adapter properties belong to item-provided widgets and are
    // split by suffix instead.
    //
    // Every widget binding reads through get(), so this runs hundreds of times
    // per reload; enumerating the adapter's two thousand properties each time
    // stalled the desktop for seconds whenever a widget was added or removed.
    // The schema is fixed once the adapter exists, so the prefixes are listed
    // once and each key's answer is kept. One plain object, mutated in place,
    // so the cache never becomes a binding dependency.
    property var _prefixCache: ({ prefixes: null, adapterKeys: null, byKey: {} })
    function _isAdapterKey(key) {
        const cache = root._prefixCache;
        // Names only: reading every value here would bind the caller to all of them.
        if (cache.adapterKeys === null) {
            const keys = {};
            const names = Object.keys(adapter);
            for (let i = 0; i < names.length; i++)
                keys[names[i]] = true;
            cache.adapterKeys = keys;
        }
        return cache.adapterKeys[key] === true;
    }
    function _widgetPrefix(key) {
        const cache = root._prefixCache;
        const known = cache.byKey[key];
        if (known !== undefined)
            return known;
        let found = "";
        if (root._isAdapterKey(key)) {
            if (cache.prefixes === null)
                cache.prefixes = Object.keys(adapter)
                    .filter(candidate => candidate.endsWith("Enabled"))
                    .map(candidate => candidate.slice(0, -7));
            for (const prefix of cache.prefixes) {
                if (key.startsWith(prefix) && prefix.length > found.length)
                    found = prefix;
            }
        } else {
            const parts = WidgetKeys.split(key);
            found = parts ? parts.prefix : "";
        }
        cache.byKey[key] = found;
        return found;
    }

    function _belongsToWidget(key, prefix) {
        return key.startsWith(prefix);
    }

    function _isPerMonitorKey(key) {
        const prefix = root._widgetPrefix(key);
        if (prefix === "")
            return false;
        return WidgetKeys.perMonitorSuffixes.indexOf(key.slice(prefix.length)) >= 0;
    }

    // Fork one widget from the values this output currently sees. Reassigning
    // the nested map is deliberate: JsonAdapter cannot observe in-place edits.
    function _ensureWidgetFork(all, monitor, key) {
        const current = all[monitor];
        const local = current && typeof current === "object"
            ? Object.assign({}, current) : {};
        const prefix = root._widgetPrefix(key);
        let hasWidget = prefix === "";
        if (prefix !== "") {
            const localKeys = Object.keys(local);
            for (let i = 0; i < localKeys.length; i++) {
                if (root._belongsToWidget(localKeys[i], prefix)
                        && root._isPerMonitorKey(localKeys[i])) {
                    hasWidget = true;
                    break;
                }
            }
        }
        if (!hasWidget && root._isAdapterKey(prefix + "Enabled")) {
            const schemaKeys = Object.keys(adapter);
            for (let i = 0; i < schemaKeys.length; i++) {
                const schemaKey = schemaKeys[i];
                if (schemaKey !== "monitors" && root._belongsToWidget(schemaKey, prefix)
                        && root._isPerMonitorKey(schemaKey)
                        && typeof adapter[schemaKey] !== "function")
                    local[schemaKey] = root.get(schemaKey, monitor);
            }
        } else if (!hasWidget) {
            for (const suffix of WidgetKeys.perMonitorSuffixes) {
                const value = root.get(prefix + suffix, monitor);
                if (value !== undefined)
                    local[prefix + suffix] = value;
            }
        }
        all[monitor] = local;
        return local;
    }

    // The shared value of each key: adapter properties in place, item-provided
    // widget keys through the detached contributed map.
    function _assignShared(values) {
        let next = null;
        for (const key in values) {
            if (root._isAdapterKey(key)) {
                if (adapter[key] !== values[key])
                    adapter[key] = values[key];
            } else if (WidgetKeys.split(key) !== null) {
                next = next || Object.assign({}, root._contributedState);
                next[key] = values[key];
            }
        }
        if (next === null)
            return;
        root._contributedState = next;
        root._stateRevision++;
    }

    function _setManyFor(monitor, values, persist) {
        if (!monitor) {
            root._assignShared(values);
            if (persist)
                settle.restart();
            return;
        }
        const all = root._plainMap(root._monitorState);
        let local = null;
        for (const key in values) {
            local = root._ensureWidgetFork(all, monitor, key);
            local[key] = values[key];
            all[monitor] = local;
        }
        root._monitorState = all;
        root._stateRevision++;
        adapter.monitors = root._plainMap(all);
        if (persist)
            settle.restart();
    }

    function setFor(monitor, key, value) {
        const patch = {};
        patch[key] = value;
        root._setManyFor(monitor, patch, true);
    }

    function setManyFor(monitor, values) {
        root._setManyFor(monitor, values, true);
    }

    function setLiveFor(monitor, key, value) {
        const patch = {};
        patch[key] = value;
        root._setManyFor(monitor, patch, false);
    }

    // Appearance stays global. Only presence, placement, size and lock use the
    // selected desktop's fork; legacy callers otherwise keep their old API.
    function set(key, value) {
        if (root._usesMonitorContext() && root._isPerMonitorKey(key)) {
            root.setFor(root.writeMonitor, key, value);
            return;
        }
        root._setManyFor("", { [key]: value }, true);
    }
    function setMany(values) {
        if (!root._usesMonitorContext()) {
            root._setManyFor("", values, true);
            return;
        }
        const local = {};
        const global = {};
        for (const key in values) {
            if (root._isPerMonitorKey(key))
                local[key] = values[key];
            else
                global[key] = values[key];
        }
        if (Object.keys(local).length > 0)
            root._setManyFor(root.writeMonitor, local, false);
        root._assignShared(global);
        settle.restart();
    }
    function setLive(key, value) {
        if (root._usesMonitorContext() && root._isPerMonitorKey(key)) {
            root.setLiveFor(root.writeMonitor, key, value);
            return;
        }
        root._setManyFor("", { [key]: value }, false);
    }
    function toggle(key) {
        const monitor = root._usesMonitorContext() && root._isPerMonitorKey(key)
            ? root.writeMonitor : "";
        root.set(key, !root.get(key, monitor));
    }
    function setAnchor(prefix, zone) {
        root.set(prefix + "Anchor", zone);
    }
    function setFree(prefix, x, y) {
        const patch = {};
        patch[prefix + "Anchor"] = "free";
        patch[prefix + "X"] = x;
        patch[prefix + "Y"] = y;
        root.setMany(patch);
    }

    Timer {
        id: settle
        interval: 400
        onTriggered: {
            adapter.monitors = root._plainMap(root._monitorState);
            adapter.contributed = root._plainMap(root._contributedState);
            root._flushedStateRevision = root._stateRevision;
            file.writeAdapter();
            root.saved();
        }
    }

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/widgets.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: {
            if (settle.running || root._stateRevision !== root._flushedStateRevision)
                return;
            root._loadingStateRevision = root._stateRevision;
            reload();
        }
        onLoaded: root._finishLoad()
        onLoadFailed: root.ready = true

        JsonAdapter {
            id: adapter
            property var monitors: ({})
            property var contributed: ({})
            property bool clockEnabled: true
            property string clockDesign: "digital"
            property bool clock24h: true
            property bool clockSeconds: false
            property string clockAccent: "palette"
            property real clockScale: 1.0
            property string clockAnchor: "top-left"
            property int clockX: 72
            property int clockY: 64
            property bool clockLocked: false
            property real clockOpacity: 1.0
            property string clockBg: "none"
            property int clockRadius: 26
            property string clockColor: ""
            property string clockColor2: ""
            property bool clockGradient: false
            property var widgetDepths: ({})
            property bool dateShow: true
            property string dateDesign: "inline"
            property string widgetFont: ""
            property bool calendarEnabled: true
            property string calendarStyle: "glass"
            property int calendarWeeks: 6
            property bool calendarWeekNumbers: true
            property string calendarHolidayRegion: ""
            property real calendarScale: 1.0
            property string calendarAnchor: "bottom-right"
            property int calendarX: 80
            property int calendarY: 80
            property bool calendarLocked: false
            property real calendarOpacity: 1.0
            property string calendarColor: ""
            property string calendarColor2: ""
            property bool calendarGradient: false
            property bool musicEnabled: false
            property string musicStyle: "cover"
            property bool musicLyrics: true
            property string musicViz: "bars"
            property real musicScale: 1.0
            property string musicAnchor: "bottom-left"
            property int musicX: 80
            property int musicY: 80
            property bool musicLocked: false
            property real musicOpacity: 1.0
            property string musicApp: ""
            property string musicShape: "wide"
            property string musicVideo: "canvas"
            property string musicVideoFile: ""
            property string musicColor: ""
            property string musicColor2: ""
            property bool musicGradient: false
            property bool aioEnabled: false
            property string aioStyle: "wide"
            property real aioScale: 1.0
            property string aioAnchor: "top-right"
            property int aioX: 80
            property int aioY: 80
            property bool aioLocked: false
            property real aioOpacity: 1.0
            property string aioColor: ""
            property string aioColor2: ""
            property bool aioGradient: false
            property bool statsEnabled: false
            property real statsScale: 1.0
            property string statsAnchor: "bottom-right"
            property int statsX: 80
            property int statsY: 80
            property bool statsLocked: false
            property real statsOpacity: 1.0
            property string statsColor: ""
            property string statsColor2: ""
            property bool statsGradient: false
            property bool weatherEnabled: false
            property string weatherDesign: "compact"
            property real weatherScale: 1.0
            property string weatherAnchor: "top-right"
            property int weatherX: 80
            property int weatherY: 80
            property bool weatherLocked: false
            property real weatherOpacity: 1.0
            property string weatherColor: ""
            property string weatherColor2: ""
            property bool weatherGradient: false
            property bool notesEnabled: false
            property real notesScale: 1.0
            property string notesAnchor: "right"
            property int notesX: 80
            property int notesY: 80
            property bool notesLocked: false
            property real notesOpacity: 1.0
            property int notesWidth: 260
            property int notesHeight: 180
            property string notesColor: ""
            property string notesColor2: ""
            property bool notesGradient: false
            property bool dayprogressEnabled: false
            property string dayprogressStyle: "ring"
            property bool dayprogressShowDate: true
            property real dayprogressScale: 1.0
            property string dayprogressAnchor: "left"
            property int dayprogressX: 80
            property int dayprogressY: 260
            property bool dayprogressLocked: false
            property real dayprogressOpacity: 1.0
            property string dayprogressColor: ""
            property string dayprogressColor2: ""
            property bool dayprogressGradient: false
            property bool shapeEnabled: false
            property string shapeKind: "dot"
            property bool shapeOutline: false
            property real shapeScale: 1.0
            property string shapeAnchor: "bottom-left"
            property int shapeX: 80
            property int shapeY: 240
            property bool shapeLocked: false
            property real shapeOpacity: 1.0
            property string shapeColor: ""
            property string shapeColor2: ""
            property bool shapeGradient: false
            // ii desktop widgets: placement, scale, opacity and backing per face.
            property bool iiClockCookieEnabled: false;; property string iiClockCookieAnchor: "free";; property int iiClockCookieX: 120;; property int iiClockCookieY: 120
            property bool iiClockCookieLocked: false;; property real iiClockCookieScale: 1.0;; property real iiClockCookieOpacity: 1.0;; property string iiClockCookieBg: "none"
            property string iiClockCookieColor: "";; property string iiClockCookieColor2: "";; property string iiClockCookieStyle: "ii";; property int iiClockCookieRadius: -1
            property real iiClockCookiePad: -1;; property real iiClockCookieBorder: -1;; property real iiClockCookieBorderOpacity: -1;; property real iiClockCookieBackingOpacity: -1;; property string iiClockCookieOpts: ""
            property bool iiClockDigitalEnabled: false;; property string iiClockDigitalAnchor: "free";; property int iiClockDigitalX: 144;; property int iiClockDigitalY: 157
            property bool iiClockDigitalLocked: false;; property real iiClockDigitalScale: 1.0;; property real iiClockDigitalOpacity: 1.0;; property string iiClockDigitalBg: "none"
            property string iiClockDigitalColor: "";; property string iiClockDigitalColor2: "";; property string iiClockDigitalStyle: "ii";; property int iiClockDigitalRadius: -1
            property real iiClockDigitalPad: -1;; property real iiClockDigitalBorder: -1;; property real iiClockDigitalBorderOpacity: -1;; property real iiClockDigitalBackingOpacity: -1;; property string iiClockDigitalOpts: ""
            property bool iiClockNagasakiEnabled: false;; property string iiClockNagasakiAnchor: "free";; property int iiClockNagasakiX: 168;; property int iiClockNagasakiY: 194
            property bool iiClockNagasakiLocked: false;; property real iiClockNagasakiScale: 1.0;; property real iiClockNagasakiOpacity: 1.0;; property string iiClockNagasakiBg: "none"
            property string iiClockNagasakiColor: "";; property string iiClockNagasakiColor2: "";; property string iiClockNagasakiStyle: "ii";; property int iiClockNagasakiRadius: -1
            property real iiClockNagasakiPad: -1;; property real iiClockNagasakiBorder: -1;; property real iiClockNagasakiBorderOpacity: -1;; property real iiClockNagasakiBackingOpacity: -1;; property string iiClockNagasakiOpts: ""
            property bool iiClockFlexEnabled: false;; property string iiClockFlexAnchor: "free";; property int iiClockFlexX: 192;; property int iiClockFlexY: 231
            property bool iiClockFlexLocked: false;; property real iiClockFlexScale: 1.0;; property real iiClockFlexOpacity: 1.0;; property string iiClockFlexBg: "none"
            property string iiClockFlexColor: "";; property string iiClockFlexColor2: "";; property string iiClockFlexStyle: "ii";; property int iiClockFlexRadius: -1
            property real iiClockFlexPad: -1;; property real iiClockFlexBorder: -1;; property real iiClockFlexBorderOpacity: -1;; property real iiClockFlexBackingOpacity: -1;; property string iiClockFlexOpts: ""
            property bool iiClockHoriEnabled: false;; property string iiClockHoriAnchor: "free";; property int iiClockHoriX: 216;; property int iiClockHoriY: 268
            property bool iiClockHoriLocked: false;; property real iiClockHoriScale: 1.0;; property real iiClockHoriOpacity: 1.0;; property string iiClockHoriBg: "none"
            property string iiClockHoriColor: "";; property string iiClockHoriColor2: "";; property string iiClockHoriStyle: "ii";; property int iiClockHoriRadius: -1
            property real iiClockHoriPad: -1;; property real iiClockHoriBorder: -1;; property real iiClockHoriBorderOpacity: -1;; property real iiClockHoriBackingOpacity: -1;; property string iiClockHoriOpts: ""
            property bool iiClockIosEnabled: false;; property string iiClockIosAnchor: "free";; property int iiClockIosX: 240;; property int iiClockIosY: 305
            property bool iiClockIosLocked: false;; property real iiClockIosScale: 1.0;; property real iiClockIosOpacity: 1.0;; property string iiClockIosBg: "none"
            property string iiClockIosColor: "";; property string iiClockIosColor2: "";; property string iiClockIosStyle: "ii";; property int iiClockIosRadius: -1
            property real iiClockIosPad: -1;; property real iiClockIosBorder: -1;; property real iiClockIosBorderOpacity: -1;; property real iiClockIosBackingOpacity: -1;; property string iiClockIosOpts: ""
            property bool iiClockNothingEnabled: false;; property string iiClockNothingAnchor: "free";; property int iiClockNothingX: 264;; property int iiClockNothingY: 342
            property bool iiClockNothingLocked: false;; property real iiClockNothingScale: 1.0;; property real iiClockNothingOpacity: 1.0;; property string iiClockNothingBg: "none"
            property string iiClockNothingColor: "";; property string iiClockNothingColor2: "";; property string iiClockNothingStyle: "ii";; property int iiClockNothingRadius: -1
            property real iiClockNothingPad: -1;; property real iiClockNothingBorder: -1;; property real iiClockNothingBorderOpacity: -1;; property real iiClockNothingBackingOpacity: -1;; property string iiClockNothingOpts: ""
            property bool iiNothingWheelClockEnabled: false; property string iiNothingWheelClockAnchor: "free"; property int iiNothingWheelClockX: 288; property int iiNothingWheelClockY: 379
            property bool iiNothingWheelClockLocked: false; property real iiNothingWheelClockScale: 1.0; property real iiNothingWheelClockOpacity: 1.0; property string iiNothingWheelClockBg: "none"
            property string iiNothingWheelClockColor: ""; property string iiNothingWheelClockColor2: ""; property string iiNothingWheelClockStyle: "ii"; property int iiNothingWheelClockRadius: -1
            property real iiNothingWheelClockPad: -1; property real iiNothingWheelClockBorder: -1; property real iiNothingWheelClockBorderOpacity: -1; property real iiNothingWheelClockBackingOpacity: -1; property string iiNothingWheelClockOpts: ""
            property bool iiNagasakiTextEnabled: false; property string iiNagasakiTextAnchor: "free"; property int iiNagasakiTextX: 312; property int iiNagasakiTextY: 416
            property bool iiNagasakiTextLocked: false; property real iiNagasakiTextScale: 1.0; property real iiNagasakiTextOpacity: 1.0; property string iiNagasakiTextBg: "none"
            property string iiNagasakiTextColor: ""; property string iiNagasakiTextColor2: ""; property string iiNagasakiTextStyle: "ii"; property int iiNagasakiTextRadius: -1
            property real iiNagasakiTextPad: -1; property real iiNagasakiTextBorder: -1; property real iiNagasakiTextBorderOpacity: -1; property real iiNagasakiTextBackingOpacity: -1; property string iiNagasakiTextOpts: ""
            property bool iiClockWordEnabled: false;; property string iiClockWordAnchor: "free";; property int iiClockWordX: 336;; property int iiClockWordY: 453
            property bool iiClockWordLocked: false;; property real iiClockWordScale: 1.0;; property real iiClockWordOpacity: 1.0;; property string iiClockWordBg: "none"
            property string iiClockWordColor: "";; property string iiClockWordColor2: "";; property string iiClockWordStyle: "ii";; property int iiClockWordRadius: -1
            property real iiClockWordPad: -1;; property real iiClockWordBorder: -1;; property real iiClockWordBorderOpacity: -1;; property real iiClockWordBackingOpacity: -1;; property string iiClockWordOpts: ""
            property bool iiClockDialEnabled: false;; property string iiClockDialAnchor: "free";; property int iiClockDialX: 360;; property int iiClockDialY: 490
            property bool iiClockDialLocked: false;; property real iiClockDialScale: 1.0;; property real iiClockDialOpacity: 1.0;; property string iiClockDialBg: "none"
            property string iiClockDialColor: "";; property string iiClockDialColor2: "";; property string iiClockDialStyle: "ii";; property int iiClockDialRadius: -1
            property real iiClockDialPad: -1;; property real iiClockDialBorder: -1;; property real iiClockDialBorderOpacity: -1;; property real iiClockDialBackingOpacity: -1;; property string iiClockDialOpts: ""
            property bool iiClockWearosEnabled: false;; property string iiClockWearosAnchor: "free";; property int iiClockWearosX: 384;; property int iiClockWearosY: 527
            property bool iiClockWearosLocked: false;; property real iiClockWearosScale: 1.0;; property real iiClockWearosOpacity: 1.0;; property string iiClockWearosBg: "none"
            property string iiClockWearosColor: "";; property string iiClockWearosColor2: "";; property string iiClockWearosStyle: "ii";; property int iiClockWearosRadius: -1
            property real iiClockWearosPad: -1;; property real iiClockWearosBorder: -1;; property real iiClockWearosBorderOpacity: -1;; property real iiClockWearosBackingOpacity: -1;; property string iiClockWearosOpts: ""
            property bool iiWearosArcClockEnabled: false; property string iiWearosArcClockAnchor: "free"; property int iiWearosArcClockX: 408; property int iiWearosArcClockY: 144
            property bool iiWearosArcClockLocked: false; property real iiWearosArcClockScale: 1.0; property real iiWearosArcClockOpacity: 1.0; property string iiWearosArcClockBg: "none"
            property string iiWearosArcClockColor: ""; property string iiWearosArcClockColor2: ""; property string iiWearosArcClockStyle: "ii"; property int iiWearosArcClockRadius: -1
            property real iiWearosArcClockPad: -1; property real iiWearosArcClockBorder: -1; property real iiWearosArcClockBorderOpacity: -1; property real iiWearosArcClockBackingOpacity: -1; property string iiWearosArcClockOpts: ""
            property bool iiConcentricClockEnabled: false; property string iiConcentricClockAnchor: "free"; property int iiConcentricClockX: 432; property int iiConcentricClockY: 181
            property bool iiConcentricClockLocked: false; property real iiConcentricClockScale: 1.0; property real iiConcentricClockOpacity: 1.0; property string iiConcentricClockBg: "none"
            property string iiConcentricClockColor: ""; property string iiConcentricClockColor2: ""; property string iiConcentricClockStyle: "ii"; property int iiConcentricClockRadius: -1
            property real iiConcentricClockPad: -1; property real iiConcentricClockBorder: -1; property real iiConcentricClockBorderOpacity: -1; property real iiConcentricClockBackingOpacity: -1; property string iiConcentricClockOpts: ""
            property bool iiMonthClockEnabled: false; property string iiMonthClockAnchor: "free"; property int iiMonthClockX: 456; property int iiMonthClockY: 218
            property bool iiMonthClockLocked: false; property real iiMonthClockScale: 1.0; property real iiMonthClockOpacity: 1.0; property string iiMonthClockBg: "none"
            property string iiMonthClockColor: ""; property string iiMonthClockColor2: ""; property string iiMonthClockStyle: "ii"; property int iiMonthClockRadius: -1
            property real iiMonthClockPad: -1; property real iiMonthClockBorder: -1; property real iiMonthClockBorderOpacity: -1; property real iiMonthClockBackingOpacity: -1; property string iiMonthClockOpts: ""
            property bool iiScallopDotClockEnabled: false; property string iiScallopDotClockAnchor: "free"; property int iiScallopDotClockX: 480; property int iiScallopDotClockY: 255
            property bool iiScallopDotClockLocked: false; property real iiScallopDotClockScale: 1.0; property real iiScallopDotClockOpacity: 1.0; property string iiScallopDotClockBg: "none"
            property string iiScallopDotClockColor: ""; property string iiScallopDotClockColor2: ""; property string iiScallopDotClockStyle: "ii"; property int iiScallopDotClockRadius: -1
            property real iiScallopDotClockPad: -1; property real iiScallopDotClockBorder: -1; property real iiScallopDotClockBorderOpacity: -1; property real iiScallopDotClockBackingOpacity: -1; property string iiScallopDotClockOpts: ""
            property bool iiScallopNumberClockEnabled: false; property string iiScallopNumberClockAnchor: "free"; property int iiScallopNumberClockX: 504; property int iiScallopNumberClockY: 292
            property bool iiScallopNumberClockLocked: false; property real iiScallopNumberClockScale: 1.0; property real iiScallopNumberClockOpacity: 1.0; property string iiScallopNumberClockBg: "none"
            property string iiScallopNumberClockColor: ""; property string iiScallopNumberClockColor2: ""; property string iiScallopNumberClockStyle: "ii"; property int iiScallopNumberClockRadius: -1
            property real iiScallopNumberClockPad: -1; property real iiScallopNumberClockBorder: -1; property real iiScallopNumberClockBorderOpacity: -1; property real iiScallopNumberClockBackingOpacity: -1; property string iiScallopNumberClockOpts: ""
            property bool iiCirclePointerClockEnabled: false; property string iiCirclePointerClockAnchor: "free"; property int iiCirclePointerClockX: 528; property int iiCirclePointerClockY: 329
            property bool iiCirclePointerClockLocked: false; property real iiCirclePointerClockScale: 1.0; property real iiCirclePointerClockOpacity: 1.0; property string iiCirclePointerClockBg: "none"
            property string iiCirclePointerClockColor: ""; property string iiCirclePointerClockColor2: ""; property string iiCirclePointerClockStyle: "ii"; property int iiCirclePointerClockRadius: -1
            property real iiCirclePointerClockPad: -1; property real iiCirclePointerClockBorder: -1; property real iiCirclePointerClockBorderOpacity: -1; property real iiCirclePointerClockBackingOpacity: -1; property string iiCirclePointerClockOpts: ""
            property bool iiTripleRingClockEnabled: false; property string iiTripleRingClockAnchor: "free"; property int iiTripleRingClockX: 552; property int iiTripleRingClockY: 366
            property bool iiTripleRingClockLocked: false; property real iiTripleRingClockScale: 1.0; property real iiTripleRingClockOpacity: 1.0; property string iiTripleRingClockBg: "none"
            property string iiTripleRingClockColor: ""; property string iiTripleRingClockColor2: ""; property string iiTripleRingClockStyle: "ii"; property int iiTripleRingClockRadius: -1
            property real iiTripleRingClockPad: -1; property real iiTripleRingClockBorder: -1; property real iiTripleRingClockBorderOpacity: -1; property real iiTripleRingClockBackingOpacity: -1; property string iiTripleRingClockOpts: ""
            property bool iiGridCardClockEnabled: false; property string iiGridCardClockAnchor: "free"; property int iiGridCardClockX: 576; property int iiGridCardClockY: 403
            property bool iiGridCardClockLocked: false; property real iiGridCardClockScale: 1.0; property real iiGridCardClockOpacity: 1.0; property string iiGridCardClockBg: "none"
            property string iiGridCardClockColor: ""; property string iiGridCardClockColor2: ""; property string iiGridCardClockStyle: "ii"; property int iiGridCardClockRadius: -1
            property real iiGridCardClockPad: -1; property real iiGridCardClockBorder: -1; property real iiGridCardClockBorderOpacity: -1; property real iiGridCardClockBackingOpacity: -1; property string iiGridCardClockOpts: ""
            property bool iiClockExpressiveCardEnabled: false;; property string iiClockExpressiveCardAnchor: "free";; property int iiClockExpressiveCardX: 120;; property int iiClockExpressiveCardY: 440
            property bool iiClockExpressiveCardLocked: false;; property real iiClockExpressiveCardScale: 1.0;; property real iiClockExpressiveCardOpacity: 1.0;; property string iiClockExpressiveCardBg: "none"
            property string iiClockExpressiveCardColor: "";; property string iiClockExpressiveCardColor2: "";; property string iiClockExpressiveCardStyle: "ii";; property int iiClockExpressiveCardRadius: -1
            property real iiClockExpressiveCardPad: -1;; property real iiClockExpressiveCardBorder: -1;; property real iiClockExpressiveCardBorderOpacity: -1;; property real iiClockExpressiveCardBackingOpacity: -1;; property string iiClockExpressiveCardOpts: ""
            property bool iiCircularMediaEnabled: false; property string iiCircularMediaAnchor: "free"; property int iiCircularMediaX: 144; property int iiCircularMediaY: 477
            property bool iiCircularMediaLocked: false; property real iiCircularMediaScale: 1.0; property real iiCircularMediaOpacity: 1.0; property string iiCircularMediaBg: "none"
            property string iiCircularMediaColor: ""; property string iiCircularMediaColor2: ""; property string iiCircularMediaStyle: "ii"; property int iiCircularMediaRadius: -1
            property real iiCircularMediaPad: -1; property real iiCircularMediaBorder: -1; property real iiCircularMediaBorderOpacity: -1; property real iiCircularMediaBackingOpacity: -1; property string iiCircularMediaOpts: ""
            property bool iiMediaCircularEnabled: false; property string iiMediaCircularAnchor: "free"; property int iiMediaCircularX: 168; property int iiMediaCircularY: 514
            property bool iiMediaCircularLocked: false; property real iiMediaCircularScale: 1.0; property real iiMediaCircularOpacity: 1.0; property string iiMediaCircularBg: "none"
            property string iiMediaCircularColor: ""; property string iiMediaCircularColor2: ""; property string iiMediaCircularStyle: "ii"; property int iiMediaCircularRadius: -1
            property real iiMediaCircularPad: -1; property real iiMediaCircularBorder: -1; property real iiMediaCircularBorderOpacity: -1; property real iiMediaCircularBackingOpacity: -1; property string iiMediaCircularOpts: ""
            property bool iiMediaExpressiveEnabled: false; property string iiMediaExpressiveAnchor: "free"; property int iiMediaExpressiveX: 192; property int iiMediaExpressiveY: 131
            property bool iiMediaExpressiveLocked: false; property real iiMediaExpressiveScale: 1.0; property real iiMediaExpressiveOpacity: 1.0; property string iiMediaExpressiveBg: "none"
            property string iiMediaExpressiveColor: ""; property string iiMediaExpressiveColor2: ""; property string iiMediaExpressiveStyle: "ii"; property int iiMediaExpressiveRadius: -1
            property real iiMediaExpressivePad: -1; property real iiMediaExpressiveBorder: -1; property real iiMediaExpressiveBorderOpacity: -1; property real iiMediaExpressiveBackingOpacity: -1; property string iiMediaExpressiveOpts: ""
            property bool iiMediaAndroidEnabled: false; property string iiMediaAndroidAnchor: "free"; property int iiMediaAndroidX: 216; property int iiMediaAndroidY: 168
            property bool iiMediaAndroidLocked: false; property real iiMediaAndroidScale: 1.0; property real iiMediaAndroidOpacity: 1.0; property string iiMediaAndroidBg: "none"
            property string iiMediaAndroidColor: ""; property string iiMediaAndroidColor2: ""; property string iiMediaAndroidStyle: "ii"; property int iiMediaAndroidRadius: -1
            property real iiMediaAndroidPad: -1; property real iiMediaAndroidBorder: -1; property real iiMediaAndroidBorderOpacity: -1; property real iiMediaAndroidBackingOpacity: -1; property string iiMediaAndroidOpts: ""
            property bool iiMediaCdEnabled: false; property string iiMediaCdAnchor: "free"; property int iiMediaCdX: 240; property int iiMediaCdY: 205
            property bool iiMediaCdLocked: false; property real iiMediaCdScale: 1.0; property real iiMediaCdOpacity: 1.0; property string iiMediaCdBg: "none"
            property string iiMediaCdColor: ""; property string iiMediaCdColor2: ""; property string iiMediaCdStyle: "ii"; property int iiMediaCdRadius: -1
            property real iiMediaCdPad: -1; property real iiMediaCdBorder: -1; property real iiMediaCdBorderOpacity: -1; property real iiMediaCdBackingOpacity: -1; property string iiMediaCdOpts: ""
            property bool iiNothingRingMediaEnabled: false; property string iiNothingRingMediaAnchor: "free"; property int iiNothingRingMediaX: 264; property int iiNothingRingMediaY: 242
            property bool iiNothingRingMediaLocked: false; property real iiNothingRingMediaScale: 1.0; property real iiNothingRingMediaOpacity: 1.0; property string iiNothingRingMediaBg: "none"
            property string iiNothingRingMediaColor: ""; property string iiNothingRingMediaColor2: ""; property string iiNothingRingMediaStyle: "ii"; property int iiNothingRingMediaRadius: -1
            property real iiNothingRingMediaPad: -1; property real iiNothingRingMediaBorder: -1; property real iiNothingRingMediaBorderOpacity: -1; property real iiNothingRingMediaBackingOpacity: -1; property string iiNothingRingMediaOpts: ""
            property bool iiWeatherDefaultEnabled: false; property string iiWeatherDefaultAnchor: "free"; property int iiWeatherDefaultX: 288; property int iiWeatherDefaultY: 279
            property bool iiWeatherDefaultLocked: false; property real iiWeatherDefaultScale: 1.0; property real iiWeatherDefaultOpacity: 1.0; property string iiWeatherDefaultBg: "none"
            property string iiWeatherDefaultColor: ""; property string iiWeatherDefaultColor2: ""; property string iiWeatherDefaultStyle: "ii"; property int iiWeatherDefaultRadius: -1
            property real iiWeatherDefaultPad: -1; property real iiWeatherDefaultBorder: -1; property real iiWeatherDefaultBorderOpacity: -1; property real iiWeatherDefaultBackingOpacity: -1; property string iiWeatherDefaultOpts: ""
            property bool iiWeatherExpressiveEnabled: false; property string iiWeatherExpressiveAnchor: "free"; property int iiWeatherExpressiveX: 312; property int iiWeatherExpressiveY: 316
            property bool iiWeatherExpressiveLocked: false; property real iiWeatherExpressiveScale: 1.0; property real iiWeatherExpressiveOpacity: 1.0; property string iiWeatherExpressiveBg: "none"
            property string iiWeatherExpressiveColor: ""; property string iiWeatherExpressiveColor2: ""; property string iiWeatherExpressiveStyle: "ii"; property int iiWeatherExpressiveRadius: -1
            property real iiWeatherExpressivePad: -1; property real iiWeatherExpressiveBorder: -1; property real iiWeatherExpressiveBorderOpacity: -1; property real iiWeatherExpressiveBackingOpacity: -1; property string iiWeatherExpressiveOpts: ""
            property bool iiWeatherForecastEnabled: false; property string iiWeatherForecastAnchor: "free"; property int iiWeatherForecastX: 336; property int iiWeatherForecastY: 353
            property bool iiWeatherForecastLocked: false; property real iiWeatherForecastScale: 1.0; property real iiWeatherForecastOpacity: 1.0; property string iiWeatherForecastBg: "none"
            property string iiWeatherForecastColor: ""; property string iiWeatherForecastColor2: ""; property string iiWeatherForecastStyle: "ii"; property int iiWeatherForecastRadius: -1
            property real iiWeatherForecastPad: -1; property real iiWeatherForecastBorder: -1; property real iiWeatherForecastBorderOpacity: -1; property real iiWeatherForecastBackingOpacity: -1; property string iiWeatherForecastOpts: ""
            property bool iiWeatherCardEnabled: false; property string iiWeatherCardAnchor: "free"; property int iiWeatherCardX: 360; property int iiWeatherCardY: 390
            property bool iiWeatherCardLocked: false; property real iiWeatherCardScale: 1.0; property real iiWeatherCardOpacity: 1.0; property string iiWeatherCardBg: "none"
            property string iiWeatherCardColor: ""; property string iiWeatherCardColor2: ""; property string iiWeatherCardStyle: "ii"; property int iiWeatherCardRadius: -1
            property real iiWeatherCardPad: -1; property real iiWeatherCardBorder: -1; property real iiWeatherCardBorderOpacity: -1; property real iiWeatherCardBackingOpacity: -1; property string iiWeatherCardOpts: ""
            property bool iiWeatherIconEnabled: false; property string iiWeatherIconAnchor: "free"; property int iiWeatherIconX: 384; property int iiWeatherIconY: 427
            property bool iiWeatherIconLocked: false; property real iiWeatherIconScale: 1.0; property real iiWeatherIconOpacity: 1.0; property string iiWeatherIconBg: "none"
            property string iiWeatherIconColor: ""; property string iiWeatherIconColor2: ""; property string iiWeatherIconStyle: "ii"; property int iiWeatherIconRadius: -1
            property real iiWeatherIconPad: -1; property real iiWeatherIconBorder: -1; property real iiWeatherIconBorderOpacity: -1; property real iiWeatherIconBackingOpacity: -1; property string iiWeatherIconOpts: ""
            property bool iiWeatherPillEnabled: false; property string iiWeatherPillAnchor: "free"; property int iiWeatherPillX: 408; property int iiWeatherPillY: 464
            property bool iiWeatherPillLocked: false; property real iiWeatherPillScale: 1.0; property real iiWeatherPillOpacity: 1.0; property string iiWeatherPillBg: "none"
            property string iiWeatherPillColor: ""; property string iiWeatherPillColor2: ""; property string iiWeatherPillStyle: "ii"; property int iiWeatherPillRadius: -1
            property real iiWeatherPillPad: -1; property real iiWeatherPillBorder: -1; property real iiWeatherPillBorderOpacity: -1; property real iiWeatherPillBackingOpacity: -1; property string iiWeatherPillOpts: ""
            property bool iiWeatherCircleEnabled: false; property string iiWeatherCircleAnchor: "free"; property int iiWeatherCircleX: 432; property int iiWeatherCircleY: 501
            property bool iiWeatherCircleLocked: false; property real iiWeatherCircleScale: 1.0; property real iiWeatherCircleOpacity: 1.0; property string iiWeatherCircleBg: "none"
            property string iiWeatherCircleColor: ""; property string iiWeatherCircleColor2: ""; property string iiWeatherCircleStyle: "ii"; property int iiWeatherCircleRadius: -1
            property real iiWeatherCirclePad: -1; property real iiWeatherCircleBorder: -1; property real iiWeatherCircleBorderOpacity: -1; property real iiWeatherCircleBackingOpacity: -1; property string iiWeatherCircleOpts: ""
            property bool iiNothingWeatherCircleEnabled: false; property string iiNothingWeatherCircleAnchor: "free"; property int iiNothingWeatherCircleX: 456; property int iiNothingWeatherCircleY: 538
            property bool iiNothingWeatherCircleLocked: false; property real iiNothingWeatherCircleScale: 1.0; property real iiNothingWeatherCircleOpacity: 1.0; property string iiNothingWeatherCircleBg: "none"
            property string iiNothingWeatherCircleColor: ""; property string iiNothingWeatherCircleColor2: ""; property string iiNothingWeatherCircleStyle: "ii"; property int iiNothingWeatherCircleRadius: -1
            property real iiNothingWeatherCirclePad: -1; property real iiNothingWeatherCircleBorder: -1; property real iiNothingWeatherCircleBorderOpacity: -1; property real iiNothingWeatherCircleBackingOpacity: -1; property string iiNothingWeatherCircleOpts: ""
            property bool iiVolumeMutePillEnabled: false; property string iiVolumeMutePillAnchor: "free"; property int iiVolumeMutePillX: 480; property int iiVolumeMutePillY: 155
            property bool iiVolumeMutePillLocked: false; property real iiVolumeMutePillScale: 1.0; property real iiVolumeMutePillOpacity: 1.0; property string iiVolumeMutePillBg: "none"
            property string iiVolumeMutePillColor: ""; property string iiVolumeMutePillColor2: ""; property string iiVolumeMutePillStyle: "ii"; property int iiVolumeMutePillRadius: -1
            property real iiVolumeMutePillPad: -1; property real iiVolumeMutePillBorder: -1; property real iiVolumeMutePillBorderOpacity: -1; property real iiVolumeMutePillBackingOpacity: -1; property string iiVolumeMutePillOpts: ""
            property bool iiWifiPillEnabled: false; property string iiWifiPillAnchor: "free"; property int iiWifiPillX: 504; property int iiWifiPillY: 192
            property bool iiWifiPillLocked: false; property real iiWifiPillScale: 1.0; property real iiWifiPillOpacity: 1.0; property string iiWifiPillBg: "none"
            property string iiWifiPillColor: ""; property string iiWifiPillColor2: ""; property string iiWifiPillStyle: "ii"; property int iiWifiPillRadius: -1
            property real iiWifiPillPad: -1; property real iiWifiPillBorder: -1; property real iiWifiPillBorderOpacity: -1; property real iiWifiPillBackingOpacity: -1; property string iiWifiPillOpts: ""
            property bool iiBluetoothPillEnabled: false; property string iiBluetoothPillAnchor: "free"; property int iiBluetoothPillX: 528; property int iiBluetoothPillY: 229
            property bool iiBluetoothPillLocked: false; property real iiBluetoothPillScale: 1.0; property real iiBluetoothPillOpacity: 1.0; property string iiBluetoothPillBg: "none"
            property string iiBluetoothPillColor: ""; property string iiBluetoothPillColor2: ""; property string iiBluetoothPillStyle: "ii"; property int iiBluetoothPillRadius: -1
            property real iiBluetoothPillPad: -1; property real iiBluetoothPillBorder: -1; property real iiBluetoothPillBorderOpacity: -1; property real iiBluetoothPillBackingOpacity: -1; property string iiBluetoothPillOpts: ""
            property bool iiMicPillEnabled: false; property string iiMicPillAnchor: "free"; property int iiMicPillX: 552; property int iiMicPillY: 266
            property bool iiMicPillLocked: false; property real iiMicPillScale: 1.0; property real iiMicPillOpacity: 1.0; property string iiMicPillBg: "none"
            property string iiMicPillColor: ""; property string iiMicPillColor2: ""; property string iiMicPillStyle: "ii"; property int iiMicPillRadius: -1
            property real iiMicPillPad: -1; property real iiMicPillBorder: -1; property real iiMicPillBorderOpacity: -1; property real iiMicPillBackingOpacity: -1; property string iiMicPillOpts: ""
            property bool iiDarkModePillEnabled: false; property string iiDarkModePillAnchor: "free"; property int iiDarkModePillX: 576; property int iiDarkModePillY: 303
            property bool iiDarkModePillLocked: false; property real iiDarkModePillScale: 1.0; property real iiDarkModePillOpacity: 1.0; property string iiDarkModePillBg: "none"
            property string iiDarkModePillColor: ""; property string iiDarkModePillColor2: ""; property string iiDarkModePillStyle: "ii"; property int iiDarkModePillRadius: -1
            property real iiDarkModePillPad: -1; property real iiDarkModePillBorder: -1; property real iiDarkModePillBorderOpacity: -1; property real iiDarkModePillBackingOpacity: -1; property string iiDarkModePillOpts: ""
            property bool iiScreenRecordPillEnabled: false; property string iiScreenRecordPillAnchor: "free"; property int iiScreenRecordPillX: 120; property int iiScreenRecordPillY: 340
            property bool iiScreenRecordPillLocked: false; property real iiScreenRecordPillScale: 1.0; property real iiScreenRecordPillOpacity: 1.0; property string iiScreenRecordPillBg: "none"
            property string iiScreenRecordPillColor: ""; property string iiScreenRecordPillColor2: ""; property string iiScreenRecordPillStyle: "ii"; property int iiScreenRecordPillRadius: -1
            property real iiScreenRecordPillPad: -1; property real iiScreenRecordPillBorder: -1; property real iiScreenRecordPillBorderOpacity: -1; property real iiScreenRecordPillBackingOpacity: -1; property string iiScreenRecordPillOpts: ""
            property bool iiEasyEffectsPillEnabled: false; property string iiEasyEffectsPillAnchor: "free"; property int iiEasyEffectsPillX: 144; property int iiEasyEffectsPillY: 377
            property bool iiEasyEffectsPillLocked: false; property real iiEasyEffectsPillScale: 1.0; property real iiEasyEffectsPillOpacity: 1.0; property string iiEasyEffectsPillBg: "none"
            property string iiEasyEffectsPillColor: ""; property string iiEasyEffectsPillColor2: ""; property string iiEasyEffectsPillStyle: "ii"; property int iiEasyEffectsPillRadius: -1
            property real iiEasyEffectsPillPad: -1; property real iiEasyEffectsPillBorder: -1; property real iiEasyEffectsPillBorderOpacity: -1; property real iiEasyEffectsPillBackingOpacity: -1; property string iiEasyEffectsPillOpts: ""
            property bool iiEasyeffectsPresetPortraitEnabled: false; property string iiEasyeffectsPresetPortraitAnchor: "free"; property int iiEasyeffectsPresetPortraitX: 168; property int iiEasyeffectsPresetPortraitY: 414
            property bool iiEasyeffectsPresetPortraitLocked: false; property real iiEasyeffectsPresetPortraitScale: 1.0; property real iiEasyeffectsPresetPortraitOpacity: 1.0; property string iiEasyeffectsPresetPortraitBg: "none"
            property string iiEasyeffectsPresetPortraitColor: ""; property string iiEasyeffectsPresetPortraitColor2: ""; property string iiEasyeffectsPresetPortraitStyle: "ii"; property int iiEasyeffectsPresetPortraitRadius: -1
            property real iiEasyeffectsPresetPortraitPad: -1; property real iiEasyeffectsPresetPortraitBorder: -1; property real iiEasyeffectsPresetPortraitBorderOpacity: -1; property real iiEasyeffectsPresetPortraitBackingOpacity: -1; property string iiEasyeffectsPresetPortraitOpts: ""
            property bool iiEasyeffectsPresetLandscapeEnabled: false; property string iiEasyeffectsPresetLandscapeAnchor: "free"; property int iiEasyeffectsPresetLandscapeX: 192; property int iiEasyeffectsPresetLandscapeY: 451
            property bool iiEasyeffectsPresetLandscapeLocked: false; property real iiEasyeffectsPresetLandscapeScale: 1.0; property real iiEasyeffectsPresetLandscapeOpacity: 1.0; property string iiEasyeffectsPresetLandscapeBg: "none"
            property string iiEasyeffectsPresetLandscapeColor: ""; property string iiEasyeffectsPresetLandscapeColor2: ""; property string iiEasyeffectsPresetLandscapeStyle: "ii"; property int iiEasyeffectsPresetLandscapeRadius: -1
            property real iiEasyeffectsPresetLandscapePad: -1; property real iiEasyeffectsPresetLandscapeBorder: -1; property real iiEasyeffectsPresetLandscapeBorderOpacity: -1; property real iiEasyeffectsPresetLandscapeBackingOpacity: -1; property string iiEasyeffectsPresetLandscapeOpts: ""
            property bool iiWeatherTypographyEnabled: false; property string iiWeatherTypographyAnchor: "free"; property int iiWeatherTypographyX: 216; property int iiWeatherTypographyY: 488
            property bool iiWeatherTypographyLocked: false; property real iiWeatherTypographyScale: 1.0; property real iiWeatherTypographyOpacity: 1.0; property string iiWeatherTypographyBg: "none"
            property string iiWeatherTypographyColor: ""; property string iiWeatherTypographyColor2: ""; property string iiWeatherTypographyStyle: "ii"; property int iiWeatherTypographyRadius: -1
            property real iiWeatherTypographyPad: -1; property real iiWeatherTypographyBorder: -1; property real iiWeatherTypographyBorderOpacity: -1; property real iiWeatherTypographyBackingOpacity: -1; property string iiWeatherTypographyOpts: ""
            property bool iiWeatherHourlyEnabled: false; property string iiWeatherHourlyAnchor: "free"; property int iiWeatherHourlyX: 240; property int iiWeatherHourlyY: 525
            property bool iiWeatherHourlyLocked: false; property real iiWeatherHourlyScale: 1.0; property real iiWeatherHourlyOpacity: 1.0; property string iiWeatherHourlyBg: "none"
            property string iiWeatherHourlyColor: ""; property string iiWeatherHourlyColor2: ""; property string iiWeatherHourlyStyle: "ii"; property int iiWeatherHourlyRadius: -1
            property real iiWeatherHourlyPad: -1; property real iiWeatherHourlyBorder: -1; property real iiWeatherHourlyBorderOpacity: -1; property real iiWeatherHourlyBackingOpacity: -1; property string iiWeatherHourlyOpts: ""
            property bool iiDateDefaultEnabled: false; property string iiDateDefaultAnchor: "free"; property int iiDateDefaultX: 264; property int iiDateDefaultY: 142
            property bool iiDateDefaultLocked: false; property real iiDateDefaultScale: 1.0; property real iiDateDefaultOpacity: 1.0; property string iiDateDefaultBg: "none"
            property string iiDateDefaultColor: ""; property string iiDateDefaultColor2: ""; property string iiDateDefaultStyle: "ii"; property int iiDateDefaultRadius: -1
            property real iiDateDefaultPad: -1; property real iiDateDefaultBorder: -1; property real iiDateDefaultBorderOpacity: -1; property real iiDateDefaultBackingOpacity: -1; property string iiDateDefaultOpts: ""
            property bool iiCalendarMinimalEnabled: false; property string iiCalendarMinimalAnchor: "free"; property int iiCalendarMinimalX: 288; property int iiCalendarMinimalY: 179
            property bool iiCalendarMinimalLocked: false; property real iiCalendarMinimalScale: 1.0; property real iiCalendarMinimalOpacity: 1.0; property string iiCalendarMinimalBg: "none"
            property string iiCalendarMinimalColor: ""; property string iiCalendarMinimalColor2: ""; property string iiCalendarMinimalStyle: "ii"; property int iiCalendarMinimalRadius: -1
            property real iiCalendarMinimalPad: -1; property real iiCalendarMinimalBorder: -1; property real iiCalendarMinimalBorderOpacity: -1; property real iiCalendarMinimalBackingOpacity: -1; property string iiCalendarMinimalOpts: ""
            property bool iiCalendarGridEnabled: false; property string iiCalendarGridAnchor: "free"; property int iiCalendarGridX: 312; property int iiCalendarGridY: 216
            property bool iiCalendarGridLocked: false; property real iiCalendarGridScale: 1.0; property real iiCalendarGridOpacity: 1.0; property string iiCalendarGridBg: "none"
            property string iiCalendarGridColor: ""; property string iiCalendarGridColor2: ""; property string iiCalendarGridStyle: "ii"; property int iiCalendarGridRadius: -1
            property real iiCalendarGridPad: -1; property real iiCalendarGridBorder: -1; property real iiCalendarGridBorderOpacity: -1; property real iiCalendarGridBackingOpacity: -1; property string iiCalendarGridOpts: ""
            property bool iiCalendarAgendaEnabled: false; property string iiCalendarAgendaAnchor: "free"; property int iiCalendarAgendaX: 336; property int iiCalendarAgendaY: 253
            property bool iiCalendarAgendaLocked: false; property real iiCalendarAgendaScale: 1.0; property real iiCalendarAgendaOpacity: 1.0; property string iiCalendarAgendaBg: "none"
            property string iiCalendarAgendaColor: ""; property string iiCalendarAgendaColor2: ""; property string iiCalendarAgendaStyle: "ii"; property int iiCalendarAgendaRadius: -1
            property real iiCalendarAgendaPad: -1; property real iiCalendarAgendaBorder: -1; property real iiCalendarAgendaBorderOpacity: -1; property real iiCalendarAgendaBackingOpacity: -1; property string iiCalendarAgendaOpts: ""
            property bool iiCalendarNextEventEnabled: false; property string iiCalendarNextEventAnchor: "free"; property int iiCalendarNextEventX: 360; property int iiCalendarNextEventY: 290
            property bool iiCalendarNextEventLocked: false; property real iiCalendarNextEventScale: 1.0; property real iiCalendarNextEventOpacity: 1.0; property string iiCalendarNextEventBg: "none"
            property string iiCalendarNextEventColor: ""; property string iiCalendarNextEventColor2: ""; property string iiCalendarNextEventStyle: "ii"; property int iiCalendarNextEventRadius: -1
            property real iiCalendarNextEventPad: -1; property real iiCalendarNextEventBorder: -1; property real iiCalendarNextEventBorderOpacity: -1; property real iiCalendarNextEventBackingOpacity: -1; property string iiCalendarNextEventOpts: ""
            property bool iiCalendarPillEnabled: false; property string iiCalendarPillAnchor: "free"; property int iiCalendarPillX: 384; property int iiCalendarPillY: 327
            property bool iiCalendarPillLocked: false; property real iiCalendarPillScale: 1.0; property real iiCalendarPillOpacity: 1.0; property string iiCalendarPillBg: "none"
            property string iiCalendarPillColor: ""; property string iiCalendarPillColor2: ""; property string iiCalendarPillStyle: "ii"; property int iiCalendarPillRadius: -1
            property real iiCalendarPillPad: -1; property real iiCalendarPillBorder: -1; property real iiCalendarPillBorderOpacity: -1; property real iiCalendarPillBackingOpacity: -1; property string iiCalendarPillOpts: ""
            property bool iiCalendarUpcoming3daysEnabled: false; property string iiCalendarUpcoming3daysAnchor: "free"; property int iiCalendarUpcoming3daysX: 408; property int iiCalendarUpcoming3daysY: 364
            property bool iiCalendarUpcoming3daysLocked: false; property real iiCalendarUpcoming3daysScale: 1.0; property real iiCalendarUpcoming3daysOpacity: 1.0; property string iiCalendarUpcoming3daysBg: "none"
            property string iiCalendarUpcoming3daysColor: ""; property string iiCalendarUpcoming3daysColor2: ""; property string iiCalendarUpcoming3daysStyle: "ii"; property int iiCalendarUpcoming3daysRadius: -1
            property real iiCalendarUpcoming3daysPad: -1; property real iiCalendarUpcoming3daysBorder: -1; property real iiCalendarUpcoming3daysBorderOpacity: -1; property real iiCalendarUpcoming3daysBackingOpacity: -1; property string iiCalendarUpcoming3daysOpts: ""
            property bool iiPhotoEnabled: false; property string iiPhotoAnchor: "free"; property int iiPhotoX: 432; property int iiPhotoY: 401
            property bool iiPhotoLocked: false; property real iiPhotoScale: 1.0; property real iiPhotoOpacity: 1.0; property string iiPhotoBg: "none"
            property string iiPhotoColor: ""; property string iiPhotoColor2: ""; property string iiPhotoStyle: "ii"; property int iiPhotoRadius: -1
            property real iiPhotoPad: -1; property real iiPhotoBorder: -1; property real iiPhotoBorderOpacity: -1; property real iiPhotoBackingOpacity: -1; property string iiPhotoOpts: ""
            property bool iiPhoto1x1Enabled: false; property string iiPhoto1x1Anchor: "free"; property int iiPhoto1x1X: 456; property int iiPhoto1x1Y: 438
            property bool iiPhoto1x1Locked: false; property real iiPhoto1x1Scale: 1.0; property real iiPhoto1x1Opacity: 1.0; property string iiPhoto1x1Bg: "none"
            property string iiPhoto1x1Color: ""; property string iiPhoto1x1Color2: ""; property string iiPhoto1x1Style: "ii"; property int iiPhoto1x1Radius: -1
            property real iiPhoto1x1Pad: -1; property real iiPhoto1x1Border: -1; property real iiPhoto1x1BorderOpacity: -1; property real iiPhoto1x1BackingOpacity: -1; property string iiPhoto1x1Opts: ""
            property bool iiPhotoWeather2x1Enabled: false; property string iiPhotoWeather2x1Anchor: "free"; property int iiPhotoWeather2x1X: 480; property int iiPhotoWeather2x1Y: 475
            property bool iiPhotoWeather2x1Locked: false; property real iiPhotoWeather2x1Scale: 1.0; property real iiPhotoWeather2x1Opacity: 1.0; property string iiPhotoWeather2x1Bg: "none"
            property string iiPhotoWeather2x1Color: ""; property string iiPhotoWeather2x1Color2: ""; property string iiPhotoWeather2x1Style: "ii"; property int iiPhotoWeather2x1Radius: -1
            property real iiPhotoWeather2x1Pad: -1; property real iiPhotoWeather2x1Border: -1; property real iiPhotoWeather2x1BorderOpacity: -1; property real iiPhotoWeather2x1BackingOpacity: -1; property string iiPhotoWeather2x1Opts: ""
            property bool iiPhotoPill2x1Enabled: false; property string iiPhotoPill2x1Anchor: "free"; property int iiPhotoPill2x1X: 504; property int iiPhotoPill2x1Y: 512
            property bool iiPhotoPill2x1Locked: false; property real iiPhotoPill2x1Scale: 1.0; property real iiPhotoPill2x1Opacity: 1.0; property string iiPhotoPill2x1Bg: "none"
            property string iiPhotoPill2x1Color: ""; property string iiPhotoPill2x1Color2: ""; property string iiPhotoPill2x1Style: "ii"; property int iiPhotoPill2x1Radius: -1
            property real iiPhotoPill2x1Pad: -1; property real iiPhotoPill2x1Border: -1; property real iiPhotoPill2x1BorderOpacity: -1; property real iiPhotoPill2x1BackingOpacity: -1; property string iiPhotoPill2x1Opts: ""
            property bool iiPhotoMinimalTemp2x1Enabled: false; property string iiPhotoMinimalTemp2x1Anchor: "free"; property int iiPhotoMinimalTemp2x1X: 528; property int iiPhotoMinimalTemp2x1Y: 129
            property bool iiPhotoMinimalTemp2x1Locked: false; property real iiPhotoMinimalTemp2x1Scale: 1.0; property real iiPhotoMinimalTemp2x1Opacity: 1.0; property string iiPhotoMinimalTemp2x1Bg: "none"
            property string iiPhotoMinimalTemp2x1Color: ""; property string iiPhotoMinimalTemp2x1Color2: ""; property string iiPhotoMinimalTemp2x1Style: "ii"; property int iiPhotoMinimalTemp2x1Radius: -1
            property real iiPhotoMinimalTemp2x1Pad: -1; property real iiPhotoMinimalTemp2x1Border: -1; property real iiPhotoMinimalTemp2x1BorderOpacity: -1; property real iiPhotoMinimalTemp2x1BackingOpacity: -1; property string iiPhotoMinimalTemp2x1Opts: ""
            property bool iiBluetoothBatteryEnabled: false; property string iiBluetoothBatteryAnchor: "free"; property int iiBluetoothBatteryX: 552; property int iiBluetoothBatteryY: 166
            property bool iiBluetoothBatteryLocked: false; property real iiBluetoothBatteryScale: 1.0; property real iiBluetoothBatteryOpacity: 1.0; property string iiBluetoothBatteryBg: "none"
            property string iiBluetoothBatteryColor: ""; property string iiBluetoothBatteryColor2: ""; property string iiBluetoothBatteryStyle: "ii"; property int iiBluetoothBatteryRadius: -1
            property real iiBluetoothBatteryPad: -1; property real iiBluetoothBatteryBorder: -1; property real iiBluetoothBatteryBorderOpacity: -1; property real iiBluetoothBatteryBackingOpacity: -1; property string iiBluetoothBatteryOpts: ""
            property bool iiBluetoothHeadphoneEnabled: false; property string iiBluetoothHeadphoneAnchor: "free"; property int iiBluetoothHeadphoneX: 576; property int iiBluetoothHeadphoneY: 203
            property bool iiBluetoothHeadphoneLocked: false; property real iiBluetoothHeadphoneScale: 1.0; property real iiBluetoothHeadphoneOpacity: 1.0; property string iiBluetoothHeadphoneBg: "none"
            property string iiBluetoothHeadphoneColor: ""; property string iiBluetoothHeadphoneColor2: ""; property string iiBluetoothHeadphoneStyle: "ii"; property int iiBluetoothHeadphoneRadius: -1
            property real iiBluetoothHeadphonePad: -1; property real iiBluetoothHeadphoneBorder: -1; property real iiBluetoothHeadphoneBorderOpacity: -1; property real iiBluetoothHeadphoneBackingOpacity: -1; property string iiBluetoothHeadphoneOpts: ""
            property bool iiMobileBatteryEnabled: false; property string iiMobileBatteryAnchor: "free"; property int iiMobileBatteryX: 120; property int iiMobileBatteryY: 240
            property bool iiMobileBatteryLocked: false; property real iiMobileBatteryScale: 1.0; property real iiMobileBatteryOpacity: 1.0; property string iiMobileBatteryBg: "none"
            property string iiMobileBatteryColor: ""; property string iiMobileBatteryColor2: ""; property string iiMobileBatteryStyle: "ii"; property int iiMobileBatteryRadius: -1
            property real iiMobileBatteryPad: -1; property real iiMobileBatteryBorder: -1; property real iiMobileBatteryBorderOpacity: -1; property real iiMobileBatteryBackingOpacity: -1; property string iiMobileBatteryOpts: ""
            property bool iiBluetoothHeadphoneCookieEnabled: false; property string iiBluetoothHeadphoneCookieAnchor: "free"; property int iiBluetoothHeadphoneCookieX: 144; property int iiBluetoothHeadphoneCookieY: 277
            property bool iiBluetoothHeadphoneCookieLocked: false; property real iiBluetoothHeadphoneCookieScale: 1.0; property real iiBluetoothHeadphoneCookieOpacity: 1.0; property string iiBluetoothHeadphoneCookieBg: "none"
            property string iiBluetoothHeadphoneCookieColor: ""; property string iiBluetoothHeadphoneCookieColor2: ""; property string iiBluetoothHeadphoneCookieStyle: "ii"; property int iiBluetoothHeadphoneCookieRadius: -1
            property real iiBluetoothHeadphoneCookiePad: -1; property real iiBluetoothHeadphoneCookieBorder: -1; property real iiBluetoothHeadphoneCookieBorderOpacity: -1; property real iiBluetoothHeadphoneCookieBackingOpacity: -1; property string iiBluetoothHeadphoneCookieOpts: ""
            property bool iiBluetoothFillCardsEnabled: false; property string iiBluetoothFillCardsAnchor: "free"; property int iiBluetoothFillCardsX: 168; property int iiBluetoothFillCardsY: 314
            property bool iiBluetoothFillCardsLocked: false; property real iiBluetoothFillCardsScale: 1.0; property real iiBluetoothFillCardsOpacity: 1.0; property string iiBluetoothFillCardsBg: "none"
            property string iiBluetoothFillCardsColor: ""; property string iiBluetoothFillCardsColor2: ""; property string iiBluetoothFillCardsStyle: "ii"; property int iiBluetoothFillCardsRadius: -1
            property real iiBluetoothFillCardsPad: -1; property real iiBluetoothFillCardsBorder: -1; property real iiBluetoothFillCardsBorderOpacity: -1; property real iiBluetoothFillCardsBackingOpacity: -1; property string iiBluetoothFillCardsOpts: ""
            property bool iiPcBatteryBarsEnabled: false; property string iiPcBatteryBarsAnchor: "free"; property int iiPcBatteryBarsX: 192; property int iiPcBatteryBarsY: 351
            property bool iiPcBatteryBarsLocked: false; property real iiPcBatteryBarsScale: 1.0; property real iiPcBatteryBarsOpacity: 1.0; property string iiPcBatteryBarsBg: "none"
            property string iiPcBatteryBarsColor: ""; property string iiPcBatteryBarsColor2: ""; property string iiPcBatteryBarsStyle: "ii"; property int iiPcBatteryBarsRadius: -1
            property real iiPcBatteryBarsPad: -1; property real iiPcBatteryBarsBorder: -1; property real iiPcBatteryBarsBorderOpacity: -1; property real iiPcBatteryBarsBackingOpacity: -1; property string iiPcBatteryBarsOpts: ""
            property bool iiPcBatteryCableEnabled: false; property string iiPcBatteryCableAnchor: "free"; property int iiPcBatteryCableX: 216; property int iiPcBatteryCableY: 388
            property bool iiPcBatteryCableLocked: false; property real iiPcBatteryCableScale: 1.0; property real iiPcBatteryCableOpacity: 1.0; property string iiPcBatteryCableBg: "none"
            property string iiPcBatteryCableColor: ""; property string iiPcBatteryCableColor2: ""; property string iiPcBatteryCableStyle: "ii"; property int iiPcBatteryCableRadius: -1
            property real iiPcBatteryCablePad: -1; property real iiPcBatteryCableBorder: -1; property real iiPcBatteryCableBorderOpacity: -1; property real iiPcBatteryCableBackingOpacity: -1; property string iiPcBatteryCableOpts: ""
            property bool iiDevicesBatteryListEnabled: false; property string iiDevicesBatteryListAnchor: "free"; property int iiDevicesBatteryListX: 240; property int iiDevicesBatteryListY: 425
            property bool iiDevicesBatteryListLocked: false; property real iiDevicesBatteryListScale: 1.0; property real iiDevicesBatteryListOpacity: 1.0; property string iiDevicesBatteryListBg: "none"
            property string iiDevicesBatteryListColor: ""; property string iiDevicesBatteryListColor2: ""; property string iiDevicesBatteryListStyle: "ii"; property int iiDevicesBatteryListRadius: -1
            property real iiDevicesBatteryListPad: -1; property real iiDevicesBatteryListBorder: -1; property real iiDevicesBatteryListBorderOpacity: -1; property real iiDevicesBatteryListBackingOpacity: -1; property string iiDevicesBatteryListOpts: ""
            property bool iiDevicesBatteryList1x1Enabled: false; property string iiDevicesBatteryList1x1Anchor: "free"; property int iiDevicesBatteryList1x1X: 264; property int iiDevicesBatteryList1x1Y: 462
            property bool iiDevicesBatteryList1x1Locked: false; property real iiDevicesBatteryList1x1Scale: 1.0; property real iiDevicesBatteryList1x1Opacity: 1.0; property string iiDevicesBatteryList1x1Bg: "none"
            property string iiDevicesBatteryList1x1Color: ""; property string iiDevicesBatteryList1x1Color2: ""; property string iiDevicesBatteryList1x1Style: "ii"; property int iiDevicesBatteryList1x1Radius: -1
            property real iiDevicesBatteryList1x1Pad: -1; property real iiDevicesBatteryList1x1Border: -1; property real iiDevicesBatteryList1x1BorderOpacity: -1; property real iiDevicesBatteryList1x1BackingOpacity: -1; property string iiDevicesBatteryList1x1Opts: ""
            property bool iiBluetoothEarbudsStemEnabled: false; property string iiBluetoothEarbudsStemAnchor: "free"; property int iiBluetoothEarbudsStemX: 288; property int iiBluetoothEarbudsStemY: 499
            property bool iiBluetoothEarbudsStemLocked: false; property real iiBluetoothEarbudsStemScale: 1.0; property real iiBluetoothEarbudsStemOpacity: 1.0; property string iiBluetoothEarbudsStemBg: "none"
            property string iiBluetoothEarbudsStemColor: ""; property string iiBluetoothEarbudsStemColor2: ""; property string iiBluetoothEarbudsStemStyle: "ii"; property int iiBluetoothEarbudsStemRadius: -1
            property real iiBluetoothEarbudsStemPad: -1; property real iiBluetoothEarbudsStemBorder: -1; property real iiBluetoothEarbudsStemBorderOpacity: -1; property real iiBluetoothEarbudsStemBackingOpacity: -1; property string iiBluetoothEarbudsStemOpts: ""
            property bool iiEmailInboxEnabled: false; property string iiEmailInboxAnchor: "free"; property int iiEmailInboxX: 312; property int iiEmailInboxY: 536
            property bool iiEmailInboxLocked: false; property real iiEmailInboxScale: 1.0; property real iiEmailInboxOpacity: 1.0; property string iiEmailInboxBg: "none"
            property string iiEmailInboxColor: ""; property string iiEmailInboxColor2: ""; property string iiEmailInboxStyle: "ii"; property int iiEmailInboxRadius: -1
            property real iiEmailInboxPad: -1; property real iiEmailInboxBorder: -1; property real iiEmailInboxBorderOpacity: -1; property real iiEmailInboxBackingOpacity: -1; property string iiEmailInboxOpts: ""
            property bool iiEmailInbox2x1Enabled: false; property string iiEmailInbox2x1Anchor: "free"; property int iiEmailInbox2x1X: 336; property int iiEmailInbox2x1Y: 153
            property bool iiEmailInbox2x1Locked: false; property real iiEmailInbox2x1Scale: 1.0; property real iiEmailInbox2x1Opacity: 1.0; property string iiEmailInbox2x1Bg: "none"
            property string iiEmailInbox2x1Color: ""; property string iiEmailInbox2x1Color2: ""; property string iiEmailInbox2x1Style: "ii"; property int iiEmailInbox2x1Radius: -1
            property real iiEmailInbox2x1Pad: -1; property real iiEmailInbox2x1Border: -1; property real iiEmailInbox2x1BorderOpacity: -1; property real iiEmailInbox2x1BackingOpacity: -1; property string iiEmailInbox2x1Opts: ""
            property bool iiAiChatEnabled: false; property string iiAiChatAnchor: "free"; property int iiAiChatX: 360; property int iiAiChatY: 190
            property bool iiAiChatLocked: false; property real iiAiChatScale: 1.0; property real iiAiChatOpacity: 1.0; property string iiAiChatBg: "none"
            property string iiAiChatColor: ""; property string iiAiChatColor2: ""; property string iiAiChatStyle: "ii"; property int iiAiChatRadius: -1
            property real iiAiChatPad: -1; property real iiAiChatBorder: -1; property real iiAiChatBorderOpacity: -1; property real iiAiChatBackingOpacity: -1; property string iiAiChatOpts: ""
            property bool iiNotesWidgetEnabled: false; property string iiNotesWidgetAnchor: "free"; property int iiNotesWidgetX: 384; property int iiNotesWidgetY: 227
            property bool iiNotesWidgetLocked: false; property real iiNotesWidgetScale: 1.0; property real iiNotesWidgetOpacity: 1.0; property string iiNotesWidgetBg: "none"
            property string iiNotesWidgetColor: ""; property string iiNotesWidgetColor2: ""; property string iiNotesWidgetStyle: "ii"; property int iiNotesWidgetRadius: -1
            property real iiNotesWidgetPad: -1; property real iiNotesWidgetBorder: -1; property real iiNotesWidgetBorderOpacity: -1; property real iiNotesWidgetBackingOpacity: -1; property string iiNotesWidgetOpts: ""
            property bool iiNotesWidget2x1Enabled: false; property string iiNotesWidget2x1Anchor: "free"; property int iiNotesWidget2x1X: 408; property int iiNotesWidget2x1Y: 264
            property bool iiNotesWidget2x1Locked: false; property real iiNotesWidget2x1Scale: 1.0; property real iiNotesWidget2x1Opacity: 1.0; property string iiNotesWidget2x1Bg: "none"
            property string iiNotesWidget2x1Color: ""; property string iiNotesWidget2x1Color2: ""; property string iiNotesWidget2x1Style: "ii"; property int iiNotesWidget2x1Radius: -1
            property real iiNotesWidget2x1Pad: -1; property real iiNotesWidget2x1Border: -1; property real iiNotesWidget2x1BorderOpacity: -1; property real iiNotesWidget2x1BackingOpacity: -1; property string iiNotesWidget2x1Opts: ""
            property bool iiCompactMediaEnabled: false; property string iiCompactMediaAnchor: "free"; property int iiCompactMediaX: 432; property int iiCompactMediaY: 301
            property bool iiCompactMediaLocked: false; property real iiCompactMediaScale: 1.0; property real iiCompactMediaOpacity: 1.0; property string iiCompactMediaBg: "none"
            property string iiCompactMediaColor: ""; property string iiCompactMediaColor2: ""; property string iiCompactMediaStyle: "ii"; property int iiCompactMediaRadius: -1
            property real iiCompactMediaPad: -1; property real iiCompactMediaBorder: -1; property real iiCompactMediaBorderOpacity: -1; property real iiCompactMediaBackingOpacity: -1; property string iiCompactMediaOpts: ""
            property bool iiQuickActionsEnabled: false; property string iiQuickActionsAnchor: "free"; property int iiQuickActionsX: 456; property int iiQuickActionsY: 338
            property bool iiQuickActionsLocked: false; property real iiQuickActionsScale: 1.0; property real iiQuickActionsOpacity: 1.0; property string iiQuickActionsBg: "none"
            property string iiQuickActionsColor: ""; property string iiQuickActionsColor2: ""; property string iiQuickActionsStyle: "ii"; property int iiQuickActionsRadius: -1
            property real iiQuickActionsPad: -1; property real iiQuickActionsBorder: -1; property real iiQuickActionsBorderOpacity: -1; property real iiQuickActionsBackingOpacity: -1; property string iiQuickActionsOpts: ""
            property bool iiAndroidSearchBarEnabled: false; property string iiAndroidSearchBarAnchor: "free"; property int iiAndroidSearchBarX: 480; property int iiAndroidSearchBarY: 375
            property bool iiAndroidSearchBarLocked: false; property real iiAndroidSearchBarScale: 1.0; property real iiAndroidSearchBarOpacity: 1.0; property string iiAndroidSearchBarBg: "none"
            property string iiAndroidSearchBarColor: ""; property string iiAndroidSearchBarColor2: ""; property string iiAndroidSearchBarStyle: "ii"; property int iiAndroidSearchBarRadius: -1
            property real iiAndroidSearchBarPad: -1; property real iiAndroidSearchBarBorder: -1; property real iiAndroidSearchBarBorderOpacity: -1; property real iiAndroidSearchBarBackingOpacity: -1; property string iiAndroidSearchBarOpts: ""
            property bool iiSearchPillEnabled: false; property string iiSearchPillAnchor: "free"; property int iiSearchPillX: 504; property int iiSearchPillY: 412
            property bool iiSearchPillLocked: false; property real iiSearchPillScale: 1.0; property real iiSearchPillOpacity: 1.0; property string iiSearchPillBg: "none"
            property string iiSearchPillColor: ""; property string iiSearchPillColor2: ""; property string iiSearchPillStyle: "ii"; property int iiSearchPillRadius: -1
            property real iiSearchPillPad: -1; property real iiSearchPillBorder: -1; property real iiSearchPillBorderOpacity: -1; property real iiSearchPillBackingOpacity: -1; property string iiSearchPillOpts: ""
            property bool iiQuoteEnabled: false; property string iiQuoteAnchor: "free"; property int iiQuoteX: 528; property int iiQuoteY: 449
            property bool iiQuoteLocked: false; property real iiQuoteScale: 1.0; property real iiQuoteOpacity: 1.0; property string iiQuoteBg: "none"
            property string iiQuoteColor: ""; property string iiQuoteColor2: ""; property string iiQuoteStyle: "ii"; property int iiQuoteRadius: -1
            property real iiQuotePad: -1; property real iiQuoteBorder: -1; property real iiQuoteBorderOpacity: -1; property real iiQuoteBackingOpacity: -1; property string iiQuoteOpts: ""
            property bool iiWaterReminderEnabled: false; property string iiWaterReminderAnchor: "free"; property int iiWaterReminderX: 552; property int iiWaterReminderY: 486
            property bool iiWaterReminderLocked: false; property real iiWaterReminderScale: 1.0; property real iiWaterReminderOpacity: 1.0; property string iiWaterReminderBg: "none"
            property string iiWaterReminderColor: ""; property string iiWaterReminderColor2: ""; property string iiWaterReminderStyle: "ii"; property int iiWaterReminderRadius: -1
            property real iiWaterReminderPad: -1; property real iiWaterReminderBorder: -1; property real iiWaterReminderBorderOpacity: -1; property real iiWaterReminderBackingOpacity: -1; property string iiWaterReminderOpts: ""
            property bool iiAtAGlanceEnabled: false; property string iiAtAGlanceAnchor: "free"; property int iiAtAGlanceX: 576; property int iiAtAGlanceY: 523
            property bool iiAtAGlanceLocked: false; property real iiAtAGlanceScale: 1.0; property real iiAtAGlanceOpacity: 1.0; property string iiAtAGlanceBg: "none"
            property string iiAtAGlanceColor: ""; property string iiAtAGlanceColor2: ""; property string iiAtAGlanceStyle: "ii"; property int iiAtAGlanceRadius: -1
            property real iiAtAGlancePad: -1; property real iiAtAGlanceBorder: -1; property real iiAtAGlanceBorderOpacity: -1; property real iiAtAGlanceBackingOpacity: -1; property string iiAtAGlanceOpts: ""
            property bool iiResourceCpuPillEnabled: false; property string iiResourceCpuPillAnchor: "free"; property int iiResourceCpuPillX: 120; property int iiResourceCpuPillY: 140
            property bool iiResourceCpuPillLocked: false; property real iiResourceCpuPillScale: 1.0; property real iiResourceCpuPillOpacity: 1.0; property string iiResourceCpuPillBg: "none"
            property string iiResourceCpuPillColor: ""; property string iiResourceCpuPillColor2: ""; property string iiResourceCpuPillStyle: "ii"; property int iiResourceCpuPillRadius: -1
            property real iiResourceCpuPillPad: -1; property real iiResourceCpuPillBorder: -1; property real iiResourceCpuPillBorderOpacity: -1; property real iiResourceCpuPillBackingOpacity: -1; property string iiResourceCpuPillOpts: ""
            property bool iiResourceRamPillEnabled: false; property string iiResourceRamPillAnchor: "free"; property int iiResourceRamPillX: 144; property int iiResourceRamPillY: 177
            property bool iiResourceRamPillLocked: false; property real iiResourceRamPillScale: 1.0; property real iiResourceRamPillOpacity: 1.0; property string iiResourceRamPillBg: "none"
            property string iiResourceRamPillColor: ""; property string iiResourceRamPillColor2: ""; property string iiResourceRamPillStyle: "ii"; property int iiResourceRamPillRadius: -1
            property real iiResourceRamPillPad: -1; property real iiResourceRamPillBorder: -1; property real iiResourceRamPillBorderOpacity: -1; property real iiResourceRamPillBackingOpacity: -1; property string iiResourceRamPillOpts: ""
            property bool iiResourceDiskPillEnabled: false; property string iiResourceDiskPillAnchor: "free"; property int iiResourceDiskPillX: 168; property int iiResourceDiskPillY: 214
            property bool iiResourceDiskPillLocked: false; property real iiResourceDiskPillScale: 1.0; property real iiResourceDiskPillOpacity: 1.0; property string iiResourceDiskPillBg: "none"
            property string iiResourceDiskPillColor: ""; property string iiResourceDiskPillColor2: ""; property string iiResourceDiskPillStyle: "ii"; property int iiResourceDiskPillRadius: -1
            property real iiResourceDiskPillPad: -1; property real iiResourceDiskPillBorder: -1; property real iiResourceDiskPillBorderOpacity: -1; property real iiResourceDiskPillBackingOpacity: -1; property string iiResourceDiskPillOpts: ""
            property bool iiResourceFillCardsEnabled: false; property string iiResourceFillCardsAnchor: "free"; property int iiResourceFillCardsX: 192; property int iiResourceFillCardsY: 251
            property bool iiResourceFillCardsLocked: false; property real iiResourceFillCardsScale: 1.0; property real iiResourceFillCardsOpacity: 1.0; property string iiResourceFillCardsBg: "none"
            property string iiResourceFillCardsColor: ""; property string iiResourceFillCardsColor2: ""; property string iiResourceFillCardsStyle: "ii"; property int iiResourceFillCardsRadius: -1
            property real iiResourceFillCardsPad: -1; property real iiResourceFillCardsBorder: -1; property real iiResourceFillCardsBorderOpacity: -1; property real iiResourceFillCardsBackingOpacity: -1; property string iiResourceFillCardsOpts: ""
            property bool iiResourceNothingDiskEnabled: false; property string iiResourceNothingDiskAnchor: "free"; property int iiResourceNothingDiskX: 216; property int iiResourceNothingDiskY: 288
            property bool iiResourceNothingDiskLocked: false; property real iiResourceNothingDiskScale: 1.0; property real iiResourceNothingDiskOpacity: 1.0; property string iiResourceNothingDiskBg: "none"
            property string iiResourceNothingDiskColor: ""; property string iiResourceNothingDiskColor2: ""; property string iiResourceNothingDiskStyle: "ii"; property int iiResourceNothingDiskRadius: -1
            property real iiResourceNothingDiskPad: -1; property real iiResourceNothingDiskBorder: -1; property real iiResourceNothingDiskBorderOpacity: -1; property real iiResourceNothingDiskBackingOpacity: -1; property string iiResourceNothingDiskOpts: ""
            property bool iiResourceNothingCpuEnabled: false; property string iiResourceNothingCpuAnchor: "free"; property int iiResourceNothingCpuX: 240; property int iiResourceNothingCpuY: 325
            property bool iiResourceNothingCpuLocked: false; property real iiResourceNothingCpuScale: 1.0; property real iiResourceNothingCpuOpacity: 1.0; property string iiResourceNothingCpuBg: "none"
            property string iiResourceNothingCpuColor: ""; property string iiResourceNothingCpuColor2: ""; property string iiResourceNothingCpuStyle: "ii"; property int iiResourceNothingCpuRadius: -1
            property real iiResourceNothingCpuPad: -1; property real iiResourceNothingCpuBorder: -1; property real iiResourceNothingCpuBorderOpacity: -1; property real iiResourceNothingCpuBackingOpacity: -1; property string iiResourceNothingCpuOpts: ""
            property bool iiResourceNothingRamEnabled: false; property string iiResourceNothingRamAnchor: "free"; property int iiResourceNothingRamX: 264; property int iiResourceNothingRamY: 362
            property bool iiResourceNothingRamLocked: false; property real iiResourceNothingRamScale: 1.0; property real iiResourceNothingRamOpacity: 1.0; property string iiResourceNothingRamBg: "none"
            property string iiResourceNothingRamColor: ""; property string iiResourceNothingRamColor2: ""; property string iiResourceNothingRamStyle: "ii"; property int iiResourceNothingRamRadius: -1
            property real iiResourceNothingRamPad: -1; property real iiResourceNothingRamBorder: -1; property real iiResourceNothingRamBorderOpacity: -1; property real iiResourceNothingRamBackingOpacity: -1; property string iiResourceNothingRamOpts: ""
        }
    }

    // brand identity master (mark + name), the cross-cutting identity shared with
    // doctor, the Hub editor and the rest of the shell. the always-on
    // pill seeds it; these defaults cover its absence, so no seed is written here.
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()
        JsonAdapter {
            id: brandAdapter
            property string markText: "力"
            property string markImage: ""
            property bool markTint: true
            property string name: "Ryoku"
        }
    }

    Component.onCompleted: if (!file.text()) file.writeAdapter();
}
