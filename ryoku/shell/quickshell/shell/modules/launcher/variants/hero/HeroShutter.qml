import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../shared/Singletons"
import "../../shared/lib/launcherstate.js" as LauncherState
import "../../shared/lib/weather.js" as WeatherMath
import "../../shared" as Shared
import "." as HeroVariant

Item {
    id: root

    property real s: 1
    property bool shown: false
    property bool compressed: false
    property bool shelfOpen: false
    property bool windowRailActive: false
    property bool windowRailFocused: false
    property bool resultNavigationActive: false
    property string activeMode: "rest"
    property alias query: field.text
    readonly property string preeditText: field.preeditText
    readonly property bool inputFocused: field.activeFocus

    signal moved(int delta)
    signal accepted()
    signal dismissed()
    signal helpRequested()
    signal modeRequested(string mode)
    signal keyPressed(var event)
    signal compositionStarted()

    implicitHeight: (compressed ? 126 : 250) * s
    clip: true

    readonly property var now: clock.date
    readonly property int nowSeconds: now.getHours() * 3600 + now.getMinutes() * 60
    readonly property var sunState: Weather.available
        ? WeatherMath.sunFrac(nowSeconds, Weather.sunrise, Weather.sunset) : null
    readonly property real solarFraction: sunState ? sunState.frac : nowSeconds / 86400
    readonly property bool daylight: sunState ? sunState.isDay
        : (now.getHours() >= 6 && now.getHours() < 20)
    // Fixed mode paints the line and marker in the stored colour; auto follows
    // the palette sky (part 1). The disc is the bright counterpart of the line.
    readonly property color solarColor: LauncherConfig.horizonMode === "fixed"
        ? LauncherConfig.horizonColor : (daylight ? Theme.sunGold : Theme.moonGlow)
    readonly property color discColor: LauncherConfig.horizonMode === "fixed"
        ? LauncherConfig.horizonColor : (daylight ? Theme.sunGold : Theme.moonDisc)
    readonly property string greeting: {
        var hour = now.getHours();
        return hour < 5 ? I18n.tr("GOOD NIGHT") : hour < 12 ? I18n.tr("GOOD MORNING")
            : hour < 18 ? I18n.tr("GOOD AFTERNOON") : I18n.tr("GOOD EVENING");
    }
    readonly property string dateText: Qt.locale("en_US").toString(now, "ddd, MMM d")

    // The hero art and its text were authored light-on-dark: the edge vignette
    // and the raised-text emboss are black so light glyphs separate from the
    // wallpaper. On a light palette that scrim must flip white, or the dark ink
    // muddies into its own shadow. `shade` is the channel both share.
    readonly property real shade: Tokens.light ? 1 : 0

    function focusField() {
        field.forceActiveFocus();
    }

    function cancelPreedit() {
        Qt.inputMethod.reset();
        field.forceActiveFocus();
    }

    function handlePreeditChange(text) {
        if (String(text || "").length === 0)
            return;
        field.forceActiveFocus();
        compositionStarted();
    }

    onPreeditTextChanged: handlePreeditChange(preeditText)

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
    }

    HeroCrop {
        anchors.fill: parent
        source: LauncherConfig.heroImage
        fallbackSource: Qt.resolvedUrl("../../shared/art/hands-adam.png")
        focalX: LauncherConfig.heroPosX
        focalY: LauncherConfig.heroPosY
        strength: LauncherConfig.heroStrength
    }

    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            GradientStop { position: 0; color: Qt.rgba(root.shade, root.shade, root.shade, 0.52) }
            GradientStop { position: 0.34; color: Qt.rgba(root.shade, root.shade, root.shade, 0.06) }
            GradientStop { position: 0.68; color: Qt.rgba(root.shade, root.shade, root.shade, 0.10) }
            GradientStop { position: 1; color: Qt.rgba(root.shade, root.shade, root.shade, 0.64) }
        }
    }

    Canvas {
        id: horizon
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: (root.compressed ? 24 : 36) * root.s
        visible: LauncherConfig.horizonMode !== "off"
        property real phase: 0
        readonly property color tint: root.solarColor

        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onPhaseChanged: requestPaint()
        onTintChanged: requestPaint()
        onVisibleChanged: if (visible) requestPaint()

        function rgba(color, alpha) {
            return "rgba(" + Math.round(color.r * 255) + ","
                + Math.round(color.g * 255) + ","
                + Math.round(color.b * 255) + "," + alpha + ")";
        }

        onPaint: {
            var context = getContext("2d");
            context.reset();
            var w = width;
            var h = height;
            if (w <= 0 || h <= 0)
                return;
            var base = h * 0.64;
            var amplitude = 2.5 * root.s;
            var markerX = Math.max(3 * root.s,
                Math.min(w - 3 * root.s, w * root.solarFraction));
            function ridge(x) {
                return base + amplitude * Math.sin(x / w * Math.PI * 3 + horizon.phase)
                    + amplitude * 0.35 * Math.sin(x / w * Math.PI * 7 - horizon.phase * 0.6);
            }
            function stroke(from, to, color) {
                context.beginPath();
                for (var i = 0; i <= 64; i++) {
                    var x = from + (to - from) * i / 64;
                    if (i === 0) context.moveTo(x, ridge(x));
                    else context.lineTo(x, ridge(x));
                }
                context.strokeStyle = color;
                context.lineWidth = Math.max(1, root.s);
                context.stroke();
            }
            stroke(0, markerX, horizon.rgba(horizon.tint, 0.78));
            stroke(markerX, w, horizon.rgba(Theme.bright, 0.18));

            var markerY = ridge(markerX);
            var glow = context.createRadialGradient(
                markerX, markerY, 0, markerX, markerY, 12 * root.s);
            glow.addColorStop(0, horizon.rgba(horizon.tint, 0.42));
            glow.addColorStop(1, horizon.rgba(horizon.tint, 0));
            context.fillStyle = glow;
            context.beginPath();
            context.arc(markerX, markerY, 12 * root.s, 0, Math.PI * 2);
            context.fill();

            context.fillStyle = horizon.rgba(root.discColor, 0.98);
            context.beginPath();
            context.arc(markerX, markerY, 4.5 * root.s, 0, Math.PI * 2);
            context.fill();
            if (!root.daylight) {
                context.fillStyle = horizon.rgba(Theme.drawer, 0.92);
                context.beginPath();
                context.arc(markerX + 2.8 * root.s, markerY - 1.8 * root.s,
                    4.5 * root.s, 0, Math.PI * 2);
                context.fill();
            }
        }

        FrameAnimation {
            running: root.shown && !Motion.reduce && LauncherConfig.horizonMode !== "off"
            onTriggered: {
                horizon.phase = Date.now() / 1800;
                horizon.requestPaint();
            }
        }
    }

    Column {
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.leftMargin: 16 * root.s
        anchors.topMargin: 12 * root.s
        spacing: 1 * root.s

        Text {
            visible: LauncherConfig.showGreeting && !root.compressed
            text: root.greeting
            color: Theme.bright
            style: Text.Raised
            styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.82)
            font.family: Theme.mono
            font.pixelSize: 9 * root.s
            font.weight: Font.DemiBold
            font.letterSpacing: 1.5 * root.s
        }

        Text {
            text: TimeFormat.format(root.now)
            color: Theme.bright
            style: Text.Raised
            styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.86)
            font.family: Theme.font
            font.pixelSize: (root.compressed ? 17 : 28) * root.s
            font.weight: Font.Light
            font.features: ({ "tnum": 1 })
        }
    }

    Column {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.rightMargin: 16 * root.s
        anchors.topMargin: 12 * root.s
        spacing: 1 * root.s

        Row {
            anchors.right: parent.right
            spacing: 5 * root.s
            visible: LauncherConfig.showWeather && Weather.available

            Shared.WeatherGlyph {
                anchors.verticalCenter: parent.verticalCenter
                width: (root.compressed ? 14 : 18) * root.s
                height: width
                name: Weather.glyph
                color: Theme.bright
                stroke: 1.5
            }

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Weather.temp
                color: Theme.bright
                style: Text.Raised
                styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.86)
                font.family: Theme.font
                font.pixelSize: (root.compressed ? 14 : 19) * root.s
                font.weight: Font.Medium
                font.features: ({ "tnum": 1 })
            }
        }

        Text {
            anchors.right: parent.right
            visible: LauncherConfig.showWeather && Weather.available && !root.compressed
            text: Weather.condition
            color: Theme.subtle
            style: Text.Raised
            styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.86)
            font.family: Theme.font
            font.pixelSize: 10 * root.s
        }

        Text {
            anchors.right: parent.right
            text: root.dateText
            color: Theme.bright
            style: Text.Raised
            styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.86)
            font.family: Theme.mono
            font.pixelSize: 9 * root.s
            font.letterSpacing: 0.6 * root.s
        }
    }

    Item {
        id: queryLine
        anchors.horizontalCenter: parent.horizontalCenter
        y: (root.compressed ? 39 : 105) * root.s
        width: Math.min(parent.width - 48 * root.s, 430 * root.s)
        height: 30 * root.s

        Item {
            id: searchGlyph
            anchors.left: parent.left
            anchors.verticalCenter: field.verticalCenter
            width: 16 * root.s
            height: 16 * root.s

            Rectangle {
                anchors.left: parent.left
                anchors.top: parent.top
                width: 9 * root.s
                height: 9 * root.s
                radius: width / 2
                color: "transparent"
                border.width: Math.max(1, 1.4 * root.s)
                border.color: Theme.bright
            }
            Rectangle {
                width: 7 * root.s
                height: Math.max(1, 1.4 * root.s)
                color: Theme.bright
                x: 8 * root.s
                y: 10 * root.s
                rotation: 45
                transformOrigin: Item.Left
            }
        }

        TextInput {
            id: field
            anchors.left: searchGlyph.right
            anchors.leftMargin: 10 * root.s
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 26 * root.s
            color: Theme.bright
            selectionColor: Theme.modeActive
            selectedTextColor: Theme.onModeActive
            font.family: Theme.mono
            font.pixelSize: 13 * root.s
            font.weight: Font.Medium
            font.letterSpacing: query.length === 0 ? 1.2 * root.s : 0
            selectByMouse: true
            clip: true
            Accessible.role: Accessible.EditableText
            Accessible.name: I18n.tr("Launcher search")
            Accessible.description: I18n.tr("Type to search or use the mode controls")
            Accessible.editable: true
            Accessible.searchEdit: true
            Accessible.focusable: true
            Accessible.focused: activeFocus
            Keys.onPressed: event => {
                var composition = LauncherState.compositionKeyDisposition(
                    field.preeditText.length > 0,
                    event.key === Qt.Key_Escape);
                if (composition === "input") {
                    return;
                } else if (composition === "cancel") {
                    root.dismissed();
                    event.accepted = true;
                } else if ((root.shelfOpen && (event.key === Qt.Key_Tab
                        || event.key === Qt.Key_Backtab
                        || event.key === Qt.Key_Left
                        || event.key === Qt.Key_Right
                        || event.key === Qt.Key_Up
                        || event.key === Qt.Key_Down))
                        || (root.resultNavigationActive
                            && (event.key === Qt.Key_Left
                                || event.key === Qt.Key_Right
                                || event.key === Qt.Key_Up
                                || event.key === Qt.Key_Down))) {
                    root.keyPressed(event);
                    event.accepted = true;
                } else if (event.key === Qt.Key_Up || event.key === Qt.Key_Down) {
                    root.moved(event.key === Qt.Key_Up ? -1 : 1);
                    event.accepted = true;
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                    root.accepted();
                    event.accepted = true;
                } else if (event.key === Qt.Key_Escape) {
                    root.dismissed();
                    event.accepted = true;
                } else if (event.key === Qt.Key_F1) {
                    root.helpRequested();
                    event.accepted = true;
                } else if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab
                        || ((event.modifiers & Qt.ControlModifier)
                            && (event.key === Qt.Key_K
                                || (event.key === Qt.Key_A
                                    && root.activeMode === "rest"
                                    && field.text.length === 0)))) {
                    root.keyPressed(event);
                    event.accepted = true;
                }
            }
        }

        Text {
            anchors.left: field.left
            anchors.verticalCenter: field.verticalCenter
            visible: field.text.length === 0 && field.preeditText.length === 0
            text: I18n.tr("TYPE TO SEARCH")
            color: Theme.faint
            style: Text.Raised
            styleColor: Qt.rgba(root.shade, root.shade, root.shade, 0.86)
            font.family: Theme.mono
            font.pixelSize: 11 * root.s
            font.weight: Font.Medium
            font.letterSpacing: 1.3 * root.s
        }

        Rectangle {
            anchors.left: field.left
            anchors.right: field.right
            anchors.bottom: parent.bottom
            height: Math.max(1, root.s)
            color: field.activeFocus ? Theme.bright : Theme.faint
        }
    }

    Row {
        id: modeKeys
        anchors.horizontalCenter: parent.horizontalCenter
        y: (root.compressed ? 83 : 154) * root.s
        spacing: 7 * root.s

        HeroVariant.ModeKey {
            s: root.s
            label: I18n.tr("ALL")
            active: root.activeMode === "all"
            onActivated: root.modeRequested("all")
        }
        HeroVariant.ModeKey {
            s: root.s
            label: I18n.tr("IMG")
            active: root.activeMode === "image"
            onActivated: root.modeRequested("image")
        }
        HeroVariant.ModeKey {
            s: root.s
            label: I18n.tr("FILE")
            active: root.activeMode === "file"
            onActivated: root.modeRequested("file")
        }
        HeroVariant.ModeKey {
            s: root.s
            label: I18n.tr("REC")
            active: root.activeMode === "recent"
            onActivated: root.modeRequested("recent")
        }
    }
}
