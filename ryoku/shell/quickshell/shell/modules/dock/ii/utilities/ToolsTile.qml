import QtQuick
import QtQuick.Layouts
import Ryoku.Ui.Singletons
import stage.modules.common.widgets
import stage.services
import shell.modules.phone
import shell.services as ShellServices
import "../clock"
import "UtilityTools.js" as UtilityTools
import ".."

/**
 * Tools: the bar's utility buttons on the dock — screenshot, screen record,
 * color picker, keyboard, wallpaper, mic, dark mode, power profile, phone
 * mirror; Settings picks which. Square: the toolbox on a shape, a click opens
 * them all in the panel; while recording it turns into the stop button.
 * Wide: the tools themselves, as many as fit, the rest behind "more".
 */
UtilityTile {
    id: tile

    readonly property var cfg: Config.options?.dock?.utilities?.tools ?? null
    readonly property var shownIds: UtilityTools.shown(tile.cfg?.shown ?? [])
    readonly property bool recording: ShellServices.Recorder.active
    readonly property bool recordPaused: ShellServices.Recorder.paused

    // Each shown tool with its live look. Pause joins the recorder while it
    // runs, as on the bar.
    readonly property var tools: {
        const list = [];
        for (const id of tile.shownIds) {
            if (id === "liveDraw" && !LiveDraw.enabled)
                continue;
            list.push(tile.look(id));
            if (id === "screenRecord" && tile.recording)
                list.push(tile.look("recordPause"));
        }
        return list;
    }

    function look(id) {
        const base = UtilityTools.find(id);
        const tool = { id: id, symbol: base?.symbol ?? "", title: Translation.tr(base?.title ?? ""), active: false };
        switch (id) {
        case "screenRecord":
            if (tile.recording) {
                tool.symbol = "stop";
                tool.title = Translation.tr("Stop recording");
                tool.active = true;
            }
            break;
        case "recordPause":
            tool.symbol = tile.recordPaused ? "play_arrow" : "pause";
            tool.title = tile.recordPaused ? Translation.tr("Resume recording") : Translation.tr("Pause recording");
            break;
        case "liveDraw":
            tool.active = LiveDraw.trayOpen;
            break;
        case "keyboard":
            tool.active = GlobalStates.oskOpen;
            break;
        case "wallpaper":
            break;
        case "mic": {
            const muted = ShellServices.OsdFeed.micMuted;
            tool.symbol = muted ? "mic_off" : "mic";
            tool.title = muted ? Translation.tr("Unmute microphone") : Translation.tr("Mute microphone");
            tool.active = !muted;
            break;
        }
        case "darkMode":
            tool.symbol = Appearance.m3colors.darkmode ? "light_mode" : "dark_mode";
            tool.title = Appearance.m3colors.darkmode ? Translation.tr("Light mode") : Translation.tr("Dark mode");
            tool.active = Appearance.m3colors.darkmode;
            break;
        case "performance":
            tool.symbol = PowerProfiles.profile === PowerProfile.PowerSaver ? "energy_savings_leaf"
                : PowerProfiles.profile === PowerProfile.Performance ? "local_fire_department" : "airwave";
            tool.title = PowerProfiles.profile === PowerProfile.PowerSaver ? Translation.tr("Power saver")
                : PowerProfiles.profile === PowerProfile.Performance ? Translation.tr("Performance") : Translation.tr("Balanced");
            tool.active = PowerProfiles.profile !== PowerProfile.Balanced;
            break;
        case "phoneMirror":
            tool.symbol = GlobalStates.phoneMirrorRunning ? "screen_share" : "smartphone";
            tool.active = GlobalStates.phoneMirrorRunning;
            break;
        }
        return tool;
    }

    // Tools that grab the screen wait for the panel to leave and the click
    // to be released, or they capture the dock's own popup.
    readonly property var grabsScreen: ["screenSnip", "screenRecord", "colorPicker"]
    property string pending: ""
    Timer {
        id: grabDelay
        interval: 220
        onTriggered: {
            const id = tile.pending;
            tile.pending = "";
            tile.execute(id, false);
        }
    }

    function run(id, alt) {
        tile.host?.closePanel();
        if (tile.grabsScreen.includes(id) && !(id === "screenRecord" && tile.recording)) {
            tile.pending = alt && id === "screenRecord" ? "screenRecordRegion" : id;
            grabDelay.restart();
            return;
        }
        tile.execute(id, alt);
    }

    function execute(id, alt) {
        switch (id) {
        case "screenSnip":
            Spawn.run(["ryoku-cmd-ryoshot"]);
            break;
        case "screenRecord":
            if (tile.recording)
                ShellServices.Recorder.stop();
            else
                ShellServices.Recorder.start(["--fullscreen"]);
            break;
        case "screenRecordRegion":
            ShellServices.Recorder.start(["--region"]);
            break;
        case "recordPause":
            ShellServices.Recorder.togglePause();
            break;
        case "colorPicker":
            Spawn.run(["ryoku-cmd-color-picker"]);
            break;
        case "liveDraw":
            LiveDraw.toggle();
            break;
        case "keyboard":
            GlobalStates.oskOpen = !GlobalStates.oskOpen;
            break;
        case "wallpaper":
            Spawn.run(["ryogami", "wallpaper", "ui"]);
            break;
        case "mic":
            Spawn.run(["wpctl", "set-mute", "@DEFAULT_SOURCE@", "toggle"]);
            break;
        case "darkMode":
            Appearance.toggleDarkMode();
            break;
        case "performance":
            if (PowerProfiles.hasPerformanceProfile) {
                PowerProfiles.profile = PowerProfiles.profile === PowerProfile.PowerSaver ? PowerProfile.Balanced
                    : PowerProfiles.profile === PowerProfile.Balanced ? PowerProfile.Performance : PowerProfile.PowerSaver;
            } else {
                PowerProfiles.profile = PowerProfiles.profile === PowerProfile.Balanced ? PowerProfile.PowerSaver : PowerProfile.Balanced;
            }
            break;
        case "phoneMirror":
            PhoneScrcpyService.openMirrorWindow();
            break;
        }
    }

    function activate() {
        // While recording the square face is the stop button.
        if (!tile.wide && tile.recording) {
            tile.run("screenRecord", false);
            return true;
        }
        return false;
    }

    surfaceColor: ClockStyle.colSurfaceHigh
    contentColor: ClockStyle.colOnSurface

    tooltipText: tile.recording ? Translation.tr("Recording · click to stop")
        : tile.tools.length === 0 ? Translation.tr("No tools chosen")
        : tile.tools.map(tool => tool.title).join(" · ")
    panelSubtitle: tile.tools.length === 1 ? Translation.tr("1 tool") : Translation.tr("%1 tools").arg(tile.tools.length)

    // ── Square ──────────────────────────────────────────────────────────
    TileBadge {
        anchors.centerIn: parent
        visible: !tile.wide
        width: Math.round(tile.side * 0.8)
        height: width
        renderScale: tile.renderScale
        shape: tile.recording ? MaterialShape.Shape.Square : MaterialShape.Shape.Cookie7Sided
        color: tile.recording ? ClockStyle.colError : ClockStyle.colSecondaryContainer
        colSymbol: tile.recording ? ClockStyle.colOnError : ClockStyle.colOnSecondaryContainer
        text: tile.recording ? "stop" : "handyman"
        fill: 1
        iconScale: 0.46
    }

    // ── Wide: the tools, as many as fit ─────────────────────────────────
    readonly property real buttonSize: Math.round(tile.badgeSize * 0.8)
    readonly property real buttonGap: Math.max(2, Math.round(tile.pad * 0.6))
    readonly property int fits: Math.max(1, Math.floor((tile.width - tile.pad * 2 + tile.buttonGap) / (tile.buttonSize + tile.buttonGap)))
    readonly property bool overflows: tile.tools.length > tile.fits
    readonly property var wideTools: tile.overflows ? tile.tools.slice(0, tile.fits - 1) : tile.tools

    Row {
        anchors.centerIn: parent
        visible: tile.wide && tile.tools.length > 0
        spacing: tile.buttonGap

        Repeater {
            model: tile.wideTools
            delegate: TileButton {
                required property var modelData
                width: tile.buttonSize
                height: tile.buttonSize
                symbol: modelData.symbol
                active: modelData.active
                filled: modelData.active
                colFilled: modelData.id === "screenRecord" ? ClockStyle.colError : ClockStyle.colPrimary
                colOnFilled: modelData.id === "screenRecord" ? ClockStyle.colOnError : ClockStyle.colOnPrimary
                colContent: tile.contentColor
                tip: modelData.title
                onClicked: tile.run(modelData.id, false)
            }
        }
        TileButton {
            visible: tile.overflows
            width: tile.buttonSize
            height: tile.buttonSize
            symbol: "more_horiz"
            colContent: tile.contentColor
            tip: Translation.tr("All tools")
            onClicked: tile.host?.togglePanel()
        }
    }

    TileCaption {
        anchors.centerIn: parent
        visible: tile.wide && tile.tools.length === 0
        text: Translation.tr("No tools chosen")
        color: tile.captionColor
        font.pixelSize: tile.captionSize
    }
}
