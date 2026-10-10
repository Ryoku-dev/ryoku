import QtQuick
import Quickshell
import Quickshell.Widgets
import "../../shared/Singletons"
import "metrics.js" as MainMetrics
import "../../shared/lib/weather.js" as Wx
import "../../shared" as Shared
import "." as MainVariant
import Ryoku.Ui.Singletons

// Zero-query home card. A solar-arc scene: the hero clock and greeting read over
// a filled wave horizon that traces the real day. The wave behind the marker
// glows the phase colour (how far through this phase we are), the stretch ahead
// stays faint (what is left of it), and a sun by day or a carved crescent moon by
// night rides the ridge at the true position. Day runs from the IP-located
// sunrise to sunset, night wraps midnight to the next sunrise, both fetched
// through the same Open-Meteo call as the weather; until that resolves the marker
// falls back to a plain clock. It is the same fill-is-elapsed grammar as the
// NowPlaying seekbar below it, so the resting card and the playing card read as
// one family. The sky follows the palette when Match wallpaper is on (warm day,
// cool night), or the fixed gold/blue when it is off; a launcher setting can pin
// the line to one colour or hide it.
// Surface is the recessed cardBot with a hairline border and a top sheen so it
// sits in the window; corner radius steps one inside the window so the nested
// corners read concentric. Right column carries the weather glance when resolved
// (glyph + temperature, condition and city, mixed-case date at the base) and
// falls back to a clean date-only readout while Weather is fetching so the column
// is never dead space. The wave drifts and the colon breathes only while the
// launcher is shown, so an idle palette costs nothing.
Item {
    id: root

    property real s: 1
    implicitHeight: 106 * s

    readonly property var now: clock.date
    readonly property string hh: TimeFormat.is24h ? Qt.formatTime(now, "HH") : String((now.getHours() % 12) || 12)
    readonly property string mm: Qt.formatTime(now, "mm")
    readonly property string date: Qt.locale("en_US").toString(now, "dddd, MMM d")
    readonly property string greeting: {
        var h = now.getHours();
        return h < 5 ? I18n.tr("Good night") : h < 12 ? I18n.tr("Good morning") : h < 18 ? I18n.tr("Good afternoon") : I18n.tr("Good evening");
    }
    // current second of the local day, the input to the solar arc.
    readonly property int nowSec: now.getHours() * 3600 + now.getMinutes() * 60
    // real solar arc from the IP-located sunrise/sunset: day sweeps sunrise..sunset,
    // night wraps midnight, so the marker crosses its own phase 0..1. Null (polar
    // day/night or feed not in yet) falls back to a plain clock and a 6..20
    // daylight guess so the marker is always sensible.
    readonly property var sun: Weather.available ? Wx.sunFrac(nowSec, Weather.sunrise, Weather.sunset) : null
    readonly property real dayFrac: sun ? sun.frac : nowSec / 86400
    readonly property bool isDay: sun ? sun.isDay : (now.getHours() >= 6 && now.getHours() < 20)
    // fixed sky colour for the phase: golden sun by day, cool moonlight by night.
    readonly property color phaseColor: isDay ? Theme.sunGold : Theme.moonGlow
    // The horizon line and marker: fixed mode uses the stored colour, auto the
    // palette sky. The disc is its bright counterpart. phaseColor above stays the
    // sky tint for the breathing colon, which is the clock, not the horizon.
    readonly property color horizonTint: LauncherConfig.horizonMode === "fixed"
        ? LauncherConfig.horizonColor : (isDay ? Theme.sunGold : Theme.moonGlow)
    readonly property color horizonDisc: LauncherConfig.horizonMode === "fixed"
        ? LauncherConfig.horizonColor : (isDay ? Theme.sunGold : Theme.moonDisc)
    readonly property bool wxReady: Weather.available && LauncherConfig.showWeather
    readonly property real cardRadius: Math.max(0, LauncherConfig.radius - 4)

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
    }

    MainVariant.Squircle {
        anchors.fill: parent
        radius: root.cardRadius
        power: 4
        color: Theme.drawer
        borderColor: Theme.border
        borderWidth: 1

        // Backmost layer: the launcher's hero art, dimmed so it reads as
        // atmosphere. Image, strength, and focal spot come from the Hub's App
        // Launcher page (launcher.json); empty falls back to the shipped
        // hands-of-creation art. Sized to cover the card, shifted to the saved spot.
        ClippingRectangle {
            anchors.fill: parent
            radius: root.cardRadius * root.s
            color: "transparent"
            Image {
                id: hero
                readonly property real ir: hero.implicitHeight > 0 ? hero.implicitWidth / hero.implicitHeight : 1
                readonly property real fr: parent.height > 0 ? parent.width / parent.height : 1
                width: hero.ir > hero.fr ? parent.height * hero.ir : parent.width
                height: hero.ir > hero.fr ? parent.height : parent.width / hero.ir
                x: (parent.width - width) * LauncherConfig.heroPosX
                y: (parent.height - height) * LauncherConfig.heroPosY
                source: LauncherConfig.heroImage !== ""
                    ? LauncherConfig.heroImage
                    : Qt.resolvedUrl("../../shared/art/hands-adam.png")
                opacity: LauncherConfig.heroStrength
                asynchronous: true
                smooth: true
            }
        }

        // Lit top edge: a hairline of palette sheen inset past the rounded
        // corners so the recessed panel catches light from above, the cue the
        // NowPlaying card gets from its blurred art bleed.
        Rectangle {
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.topMargin: 1
            anchors.leftMargin: root.cardRadius * root.s
            anchors.rightMargin: root.cardRadius * root.s
            height: 1
            color: Theme.sheen
        }

        // The solar wave: a filled horizon spanning the card, clipped to the
        // rounded corners so it never spills. Painted behind the text.
        ClippingRectangle {
            id: waveClip
            visible: LauncherConfig.horizonMode !== "off"
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 42 * root.s
            radius: root.cardRadius * root.s
            color: "transparent"

            Canvas {
                id: wave
                anchors.fill: parent
                property real phase: 0
                readonly property real frac: root.dayFrac
                // the horizon tint: the stored colour in fixed mode, else the
                // palette sky (part 1).
                readonly property color tint: root.horizonTint

                onFracChanged: requestPaint()
                onTintChanged: requestPaint()
                onVisibleChanged: if (visible) requestPaint()

                // Canvas gradients want rgba() strings, not QML color objects
                // (a color serializes to #aarrggbb and corrupts the stop).
                function rgba(c, a) {
                    return "rgba(" + Math.round(c.r * 255) + "," + Math.round(c.g * 255)
                        + "," + Math.round(c.b * 255) + "," + a + ")";
                }

                onPaint: {
                    var ctx = getContext("2d");
                    ctx.reset();
                    var w = width, h = height, s = root.s;
                    var base = h * 0.66, amp = 5 * s, steps = 72;
                    function ridge(x) {
                        var t = x / w;
                        return base + amp * Math.sin(t * Math.PI * 3 + wave.phase)
                            + amp * 0.4 * Math.sin(t * Math.PI * 7 - wave.phase * 0.7);
                    }
                    var nodeX = Math.max(2 * s, Math.min(w - 2 * s, w * wave.frac));

                    function fillRegion(x0, x1, style) {
                        ctx.beginPath();
                        ctx.moveTo(x0, h);
                        for (var i = 0; i <= steps; i++) {
                            var x = x0 + (x1 - x0) * i / steps;
                            ctx.lineTo(x, ridge(x));
                        }
                        ctx.lineTo(x1, h);
                        ctx.closePath();
                        ctx.fillStyle = style;
                        ctx.fill();
                    }
                    function strokeRidge(x0, x1, style) {
                        ctx.beginPath();
                        for (var i = 0; i <= steps; i++) {
                            var x = x0 + (x1 - x0) * i / steps;
                            var y = ridge(x);
                            if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
                        }
                        ctx.strokeStyle = style;
                        ctx.stroke();
                    }

                    // elapsed fill (phase tint) then remaining fill (ghost), both
                    // fading up from the baseline so the ridge reads as a horizon.
                    var gA = ctx.createLinearGradient(0, base - amp * 2, 0, h);
                    gA.addColorStop(0, wave.rgba(wave.tint, 0.0));
                    gA.addColorStop(0.5, wave.rgba(wave.tint, 0.15));
                    gA.addColorStop(1, wave.rgba(wave.tint, 0.36));
                    fillRegion(0, nodeX, gA);

                    var gF = ctx.createLinearGradient(0, base - amp * 2, 0, h);
                    gF.addColorStop(0, wave.rgba(Theme.providerOther, 0.0));
                    gF.addColorStop(1, wave.rgba(Theme.providerOther, 0.22));
                    fillRegion(nodeX, w, gF);

                    ctx.lineWidth = 2 * s;
                    ctx.lineCap = "round";
                    strokeRidge(0, nodeX, wave.rgba(wave.tint, 0.9));
                    strokeRidge(nodeX, w, wave.rgba(Theme.faint, 0.5));

                    // sun / moon node: a soft glow halo in the phase tint, a filled
                    // disc, and a carved crescent by night.
                    var sy = ridge(nodeX);
                    var glow = ctx.createRadialGradient(nodeX, sy, 0, nodeX, sy, 15 * s);
                    glow.addColorStop(0, wave.rgba(wave.tint, 0.5));
                    glow.addColorStop(1, wave.rgba(wave.tint, 0.0));
                    ctx.fillStyle = glow;
                    ctx.beginPath();
                    ctx.arc(nodeX, sy, 15 * s, 0, 2 * Math.PI);
                    ctx.fill();

                    var r = 5.5 * s;
                    ctx.fillStyle = wave.rgba(root.horizonDisc, root.isDay ? 1 : 0.98);
                    ctx.beginPath();
                    ctx.arc(nodeX, sy, r, 0, 2 * Math.PI);
                    ctx.fill();
                    if (!root.isDay) {
                        ctx.fillStyle = wave.rgba(Theme.drawer, 1);
                        ctx.beginPath();
                        ctx.arc(nodeX + r * 0.6, sy - r * 0.4, r, 0, 2 * Math.PI);
                        ctx.fill();
                    }
                }

                // Gentle drift, only while the palette is shown, so an idle
                // launcher triggers no repaints.
                FrameAnimation {
                    running: root.visible && LauncherConfig.horizonMode !== "off"
                    onTriggered: {
                        wave.phase = Date.now() / 900;
                        wave.requestPaint();
                    }
                }
            }
        }

        // left: greeting eyebrow over the hero clock.
        Column {
            anchors.left: parent.left
            anchors.leftMargin: MainMetrics.padOuter * root.s
            anchors.top: parent.top
            anchors.topMargin: 16 * root.s
            spacing: 4 * root.s

            Text {
                visible: LauncherConfig.showGreeting
                text: root.greeting
                color: Theme.subtle
                font.family: Theme.font
                font.pixelSize: Metrics.fontEyebrow * root.s
                font.weight: Font.DemiBold
                font.capitalization: Font.AllUppercase
                font.letterSpacing: 1.6 * root.s
            }
            Row {
                spacing: 0
                Text {
                    text: root.hh
                    color: Theme.bright
                    font.family: Theme.mono
                    font.pixelSize: 34 * root.s
                    font.weight: Font.Medium
                    font.features: { "tnum": 1 }
                }
                Text {
                    text: ":"
                    color: root.phaseColor
                    font.family: Theme.mono
                    font.pixelSize: 34 * root.s
                    font.weight: Font.Medium
                    // Breathing colon in the phase tint, the shared clock-face
                    // heartbeat tied to the sky rather than the wallust accent.
                    // Gated on visibility so a hidden palette really is idle.
                    SequentialAnimation on opacity {
                        loops: Animation.Infinite
                        running: root.visible
                        NumberAnimation { from: 1; to: 0.3; duration: 620; easing.type: Easing.InOutSine }
                        NumberAnimation { from: 0.3; to: 1; duration: 620; easing.type: Easing.InOutSine }
                    }
                }
                Text {
                    text: root.mm + (TimeFormat.is24h ? "" : " " + root.now.toLocaleTimeString(TimeFormat.locale, "AP"))
                    color: Theme.bright
                    font.family: Theme.mono
                    font.pixelSize: 34 * root.s
                    font.weight: Font.Medium
                    font.features: { "tnum": 1 }
                }
            }
        }

        // right: weather glance when resolved, date-only fallback while it is
        // still fetching so the column never reads as dead space.
        Column {
            anchors.right: parent.right
            anchors.rightMargin: MainMetrics.padOuter * root.s
            anchors.top: parent.top
            anchors.topMargin: 18 * root.s
            spacing: 3 * root.s

            // headline: icon + temperature, matched in weight to the left clock.
            Row {
                anchors.right: parent.right
                spacing: 6 * root.s
                visible: root.wxReady

                Shared.WeatherGlyph {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 22 * root.s
                    height: 22 * root.s
                    name: Weather.glyph
                    color: Theme.cream
                    stroke: 1.7
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: Weather.temp
                    color: Theme.bright
                    font.family: Theme.font
                    font.pixelSize: 22 * root.s
                    font.weight: Font.Medium
                    font.features: { "tnum": 1 }
                }
            }

            // fallback headline while weather has not resolved yet, in the same
            // slot as the temperature so the layout does not jump on arrival.
            Text {
                anchors.right: parent.right
                visible: !root.wxReady
                text: root.date
                color: Theme.bright
                font.family: Theme.font
                font.pixelSize: 22 * root.s
                font.weight: Font.Medium
            }

            Text {
                anchors.right: parent.right
                visible: root.wxReady
                text: Weather.condition + (Weather.city.length ? "  \u00b7  " + Weather.city : "")
                color: Theme.subtle
                font.family: Theme.font
                font.pixelSize: Metrics.fontSubtitle * root.s
            }

            Text {
                anchors.right: parent.right
                visible: root.wxReady
                text: root.date
                color: Theme.faint
                font.family: Theme.font
                font.pixelSize: Metrics.fontEyebrow * root.s
            }
        }
    }
}
