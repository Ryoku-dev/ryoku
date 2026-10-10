pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons
import QtQuick.Shapes
import shell.services
import shell.services as Svc
import "../Singletons"
import "../../../services/lib/weather.js" as Wx

// The all-in-one desktop face: one card that fuses the weather cluster, the day
// and the clock with a live cava spectrum, in two fixed layouts. The design is
// drawn at its native pixel box (wide 708x454, tall 714x885) with the preview's
// absolute coordinates verbatim, then the whole box is scaled by `s`, so s=1 is
// pixel-identical to the reference. Ink is fixed bright white by design; only the
// data is live: the clock rides Now.date, the weather cluster reads the Weather
// singleton (Material Symbols glyph via Wx.symbolFor, hi/lo off daily[0].hi/lo),
// and the spectrum bars are the shared cava feed (AudioBars), owner-refcounted so
// a hidden or disabled widget claims no analyser.
Item {
    id: root

    property real underL: 0                 // wallpaper L* under this slot; feeds inkOn2
    // WidgetSlot pins a hex here to paint this widget's ink; "" follows the wallpaper.
    property string inkColorA: ""
    property string style: "wide"           // wide | tall
    property real s: 1                       // scale (Config.aioScale)
    property bool active: true               // visible/enabled

    readonly property bool tall: root.style === "tall"

    implicitWidth: box.width * root.s
    implicitHeight: box.height * root.s

    // The design box, drawn at its native pixel size and scaled as one. A Loader
    // keeps only the active layout alive, so the other layout's spectrum never
    // claims cava. The Loader carries the design size so the item is laid out at
    // exactly the reference box, whatever its own implicit size resolves to.
    Loader {
        id: box
        width: root.tall ? 714 : 708
        height: root.tall ? 885 : 454
        active: true
        sourceComponent: root.tall ? tallCard : wideCard
        transform: Scale { xScale: root.s; yScale: root.s }
    }

    // ── WIDE (708 x 454) ────────────────────────────────────────────────────
    Component {
        id: wideCard
        Item {
            id: w
            implicitWidth: 708
            implicitHeight: 454
            readonly property color ink: Theme.inkOn2(root.underL, root.inkColorA)
            readonly property color dim: Theme.inkDimOn2(root.underL, root.inkColorA)
            readonly property string sym: "Material Symbols Rounded"

            // today's hi/lo, probed off the daemon frame; hidden if absent.
            readonly property var day0: (Weather.daily && Weather.daily.length > 0) ? Weather.daily[0] : null
            readonly property bool hasHiLo: w.day0 !== null && w.day0.hi !== undefined && w.day0.lo !== undefined

            // the letter-spaced date line: MONTH DD YYYY, each glyph split apart.
            readonly property string dateLine: {
                var parts = [Now.date.toLocaleDateString(Svc.Config.formatLoc, "MMMM").toUpperCase(),
                             Qt.formatDate(Now.date, "dd"),
                             Qt.formatDate(Now.date, "yyyy")];
                var out = [];
                for (var i = 0; i < parts.length; i++)
                    out.push(parts[i].split("").join(" "));
                return out.join("   ");
            }


            // ---- diagonal divider ----
            Shape {
                anchors.fill: parent
                ShapePath {
                    strokeColor: w.ink; strokeWidth: 2; fillColor: "transparent"
                    startX: 360; startY: 20
                    PathLine { x: 314; y: 150 }
                }
            }

            // ---- weather cluster (top-left) ----
            Text {
                id: moon
                x: 44; y: 40
                text: Weather.current ? Wx.symbolFor(Weather.current.code, Weather.current.isDay) : "cloud"
                color: w.ink; font.family: w.sym; font.pixelSize: 46
            }
            Text {
                id: temp
                x: 150; y: 34
                text: Weather.available ? (Weather.tempNow + "\u00b0") : "\u2014\u00b0"
                color: w.ink
                font.family: "Inter Display"; font.weight: Font.DemiBold; font.pixelSize: 52
            }
            Row {
                x: 150; y: 96; spacing: 14
                visible: w.hasHiLo
                Row { spacing: 2
                    Text { text: "\u2191"; color: "#e0564b"; font.pixelSize: 22; font.family: "Inter Display"; font.weight: Font.Bold }
                    Text { text: w.hasHiLo ? w.day0.hi + "\u00b0" : ""; color: w.dim; font.pixelSize: 22; font.family: "Inter Display"; font.weight: Font.Medium }
                }
                Row { spacing: 2
                    Text { text: "\u2193"; color: "#4a90d9"; font.pixelSize: 22; font.family: "Inter Display"; font.weight: Font.Bold }
                    Text { text: w.hasHiLo ? w.day0.lo + "\u00b0" : ""; color: w.dim; font.pixelSize: 22; font.family: "Inter Display"; font.weight: Font.Medium }
                }
            }
            Text {
                x: 150; y: 128
                text: Weather.condition; color: w.dim
                font.family: "Inter Display"; font.pixelSize: 24; font.weight: Font.Medium
            }

            // ---- big weekday ----
            Text {
                x: 34; y: 150
                text: Now.date.toLocaleDateString(Svc.Config.formatLoc, "ddd").toUpperCase(); color: w.ink
                font.family: "Inter Display"; font.weight: Font.Black
                font.pixelSize: 200; font.letterSpacing: -6
            }

            // ---- time card (top-right): a true parallelogram leaning '/' with
            // the divider, filled white, holding an upright dark clock. Corners
            // are the preview's 0.28*height horizontal shear resolved to points. ----
            Item {
                id: card
                x: 392; y: 74; width: 296; height: 74
                Shape {
                    anchors.fill: parent
                    ShapePath {
                        strokeColor: "transparent"; strokeWidth: 0; fillColor: "#ffffff"
                        startX: 20.72; startY: 0
                        PathLine { x: 316.72; y: 0 }
                        PathLine { x: 296; y: 74 }
                        PathLine { x: 0; y: 74 }
                        PathLine { x: 20.72; y: 0 }
                    }
                }
                Text {
                    anchors.centerIn: parent
                    text: TimeFormat.format(Now.date); color: "#1a1c1f"
                    font.family: "Inter Display"; font.weight: Font.Black; font.pixelSize: 46; font.letterSpacing: 1
                }
            }
            Text {
                x: 392; y: 168
                text: w.dateLine; color: w.dim
                font.family: "Inter Display"; font.weight: Font.Medium; font.pixelSize: 17; font.letterSpacing: 3
            }

            // ---- audio spectrum (bottom-right): the shared cava feed mapped
            // across 34 bars, sharing a fixed baseline at the row's bottom. ----
            Item {
                id: vizW
                x: 330; y: 300
                width: 302; height: 120
                readonly property bool wanted: root.active && root.visible
                onWantedChanged: AudioBars.setActive(vizW, vizW.wanted)
                Component.onCompleted: AudioBars.setActive(vizW, vizW.wanted)
                Component.onDestruction: AudioBars.setActive(vizW, false)

                Row {
                    anchors.fill: parent
                    spacing: 4
                    Repeater {
                        model: 34
                        Rectangle {
                            id: barW
                            required property int index
                            readonly property int band: Math.round(barW.index * (AudioBars.bars - 1) / 33)
                            readonly property real lvl: AudioBars.active ? (AudioBars.levels[barW.band] || 0) : 0
                            width: 5; radius: 2.5; color: Theme.accentOn2(root.underL, root.inkColorA)
                            anchors.bottom: parent.bottom
                            height: Math.max(6, Math.min(120, barW.lvl * 120))
                            Behavior on height { NumberAnimation { duration: 90; easing.type: Easing.OutSine } }
                        }
                    }
                }
            }
        }
    }

    // ── TALL (714 x 885) ──────────────────────────────────────────────────────
    Component {
        id: tallCard
        Item {
            id: t
            implicitWidth: 714
            implicitHeight: 885


            // ---- waveform (top): the cava feed across 74 slivers, grown from
            // the mid-line both ways. ----
            Item {
                id: vizT
                anchors.fill: parent
                readonly property bool wanted: root.active && root.visible
                onWantedChanged: AudioBars.setActive(vizT, vizT.wanted)
                Component.onCompleted: AudioBars.setActive(vizT, vizT.wanted)
                Component.onDestruction: AudioBars.setActive(vizT, false)

                Row {
                    x: 92; y: 92; spacing: 3
                    Repeater {
                        model: 74
                        Item {
                            id: barT
                            required property int index
                            readonly property int band: Math.round(barT.index * (AudioBars.bars - 1) / 73)
                            readonly property real lvl: AudioBars.active ? (AudioBars.levels[barT.band] || 0) : 0
                            width: 3; height: 70
                            Rectangle {
                                width: 2; radius: 1; color: Theme.accentOn2(root.underL, root.inkColorA); opacity: 0.85
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.verticalCenter: parent.verticalCenter
                                height: Math.max(2, Math.min(56, barT.lvl * 56))
                                Behavior on height { NumberAnimation { duration: 90; easing.type: Easing.OutSine } }
                            }
                        }
                    }
                }
            }

            // ---- huge day number ----
            Text {
                id: day
                anchors.horizontalCenter: parent.horizontalCenter
                y: 150
                text: Qt.formatDate(Now.date, "dd")
                font.family: "Inter Display"; font.weight: Font.Black
                font.pixelSize: 620; font.letterSpacing: -30
                color: Theme.accentOn2(root.underL, root.inkColorA); opacity: 0.55
            }

            // ---- month overlaid ----
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                y: 430
                text: Now.date.toLocaleDateString(Svc.Config.formatLoc, "MMMM").toUpperCase()
                font.family: "Inter Display"; font.weight: Font.Bold
                font.pixelSize: 92; font.letterSpacing: 6
                color: Theme.inkOn2(root.underL, root.inkColorA); opacity: 0.92
            }

            // ---- time + underline ----
            Text {
                id: tm
                anchors.horizontalCenter: parent.horizontalCenter
                y: 560
                text: TimeFormat.format(Now.date)
                color: Theme.inkOn2(root.underL, root.inkColorA)
                font.family: "Inter Display"; font.weight: Font.Medium
                font.pixelSize: 30; font.letterSpacing: 4
            }
            Rectangle {
                anchors.horizontalCenter: parent.horizontalCenter
                y: 606; width: 120; height: 2; color: Theme.inkDimOn2(root.underL, root.inkColorA)
            }

            // ---- vertical weekday ----
            Text {
                text: Now.date.toLocaleDateString(Svc.Config.formatLoc, "dddd").toUpperCase()
                color: Theme.inkDimOn2(root.underL, root.inkColorA)
                font.family: "Inter Display"; font.weight: Font.Medium
                font.pixelSize: 22; font.letterSpacing: 8
                transformOrigin: Item.Center
                rotation: -90
                x: 150 - width / 2 + height / 2
                y: 430
            }

            // ---- faint stars ----
            Repeater {
                model: [[600, 250], [640, 300], [90, 640], [610, 690], [120, 200]]
                Rectangle {
                    required property var modelData
                    x: modelData[0]; y: modelData[1]
                    width: 3; height: 3; radius: 1.5; color: Theme.inkDimOn2(root.underL, root.inkColorA); opacity: 0.5
                }
            }
        }
    }
}
