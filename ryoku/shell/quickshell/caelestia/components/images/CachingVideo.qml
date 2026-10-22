import QtQuick
import QtMultimedia
import Quickshell
import Quickshell.Widgets
import Caelestia.Config
import caelestia.services

Item {
    id: root

    property string path
    property var screen
    property bool isFirstInstance: false

    property alias playing: mediaPlayer.playing
    property alias fillMode: videoOutput.fillMode
    property alias playbackState: mediaPlayer.playbackState

    function checkPauseState() {
        if (!root.screen)
            return;

        if (GlobalConfig.background.videoWallpaperPaused) {
            if (mediaPlayer.playing)
                mediaPlayer.pause();
            return;
        }

        const pauseOnAllDisplays = GlobalConfig.background.videoWallpaperPauseOnAllDisplays;
        const pauseOnFullscreen = GlobalConfig.background.videoWallpaperPauseOnFullscreen;
        const pauseOnTiled = GlobalConfig.background.videoWallpaperPauseOnTiled;

        let shouldPause = false;

        try {
const wins = Kwin.windowList || [];
// KWin serialises fullscreen as a boolean (true/false), not
// the integer levels Hyprland uses (0/1/2). Use === true so
// the check works for both truthy booleans and int > 0.
if (pauseOnAllDisplays) {
    for (let i = 0; i < wins.length; i++) {
        if (pauseOnFullscreen && wins[i].fullscreen === true)
            shouldPause = true;
        if (pauseOnTiled && !wins[i].floating && !wins[i].fullscreen)
            shouldPause = true;
    }
} else {
    const screenName = root.screen ? root.screen.name : "";
    const activeOut = Kwin.activeOutputName || "";
    if (activeOut === screenName || screenName === "") {
        for (let i = 0; i < wins.length; i++) {
            if (pauseOnFullscreen && wins[i].fullscreen === true)
                shouldPause = true;
            if (pauseOnTiled && !wins[i].floating && !wins[i].fullscreen)
                shouldPause = true;
        }
    }
}
        
        } catch (e) {
        }

        if (shouldPause && mediaPlayer.playing) {
            mediaPlayer.pause();
        } else if (!shouldPause && !mediaPlayer.playing && root.path) {
            mediaPlayer.play();
        }
    }

    function checkMuteState() {
        const muteOnMedia = GlobalConfig.background.videoWallpaperMuteOnMedia;
        const soundEnabled = GlobalConfig.background.videoWallpaperSoundEnabled;
        const isPlaying = Players.active?.isPlaying ?? false;

        if (audioLoader.item) {
            audioLoader.item.muted = !root.isFirstInstance || !soundEnabled || (muteOnMedia && isPlaying);
        }
    }

    Component.onCompleted: {
        isFirstInstance = (VideoWallpaperPlayer.firstInstance === null);
        VideoWallpaperPlayer.firstInstance = root;
        Qt.callLater(checkPauseState);
        Qt.callLater(checkMuteState);
    }

    Component.onDestruction: {
        if (VideoWallpaperPlayer.firstInstance === root) {
            VideoWallpaperPlayer.firstInstance = null;
        }
    }

    onPathChanged: {
        mediaPlayer.source = path || "";
        if (path)
            mediaPlayer.play();
    }

    Loader {
        id: audioLoader

        active: GlobalConfig.background.videoWallpaperSoundEnabled

        sourceComponent: AudioOutput {
            id: audioOutputImpl

            muted: !root.isFirstInstance || (GlobalConfig.background.videoWallpaperMuteOnMedia && (Players.active?.isPlaying ?? false))

            Component.onCompleted: {
                mediaPlayer.audioOutput = audioOutputImpl;
            }

            Component.onDestruction: {
                if (mediaPlayer.audioOutput === audioOutputImpl)
                    mediaPlayer.audioOutput = null;
            }
        }
    }

    MediaPlayer {
        id: mediaPlayer

        source: path || ""
        videoOutput: videoOutput
        loops: MediaPlayer.Infinite
        autoPlay: true
        // audioOutput is wired dynamically by the Loader above so that the
        // PipeWire backend is only initialised when the user has enabled sound.

        onErrorOccurred: function(error, errorString) {
            if (mediaPlayer.audioOutput !== null) {
                console.warn("[CachingVideo] MediaPlayer error (audio?), retrying video-only:", errorString);
                mediaPlayer.audioOutput = null;
                if (root.path)
                    Qt.callLater(() => mediaPlayer.play());
            }
        }
    }

    VideoOutput {
        id: videoOutput

        anchors.fill: parent
        fillMode: VideoOutput.PreserveAspectCrop

        Component.onDestruction: {
            mediaPlayer.stop();
        }
    }

    Timer {
        id: mediaCheckTimer

        interval: 500
        running: GlobalConfig.background.videoWallpaperMuteOnMedia && audioLoader.active
        repeat: true

        onTriggered: checkMuteState()
    }

    Timer {
        id: checkTimer

        interval: 100
        running: true
        repeat: true

        onTriggered: {
            checkPauseState();
            checkMuteState();
        }
    }

    Connections {
        function onVideoWallpaperPausedChanged() {
            checkPauseState();
        }

        function onVideoWallpaperPauseOnAllDisplaysChanged() {
            checkPauseState();
        }

        function onVideoWallpaperPauseOnFullscreenChanged() {
            checkPauseState();
        }

        function onVideoWallpaperPauseOnTiledChanged() {
            checkPauseState();
        }

        function onVideoWallpaperMuteOnMediaChanged() {
            checkMuteState();
        }

        function onVideoWallpaperSoundEnabledChanged() {
            checkMuteState();
        }

        target: GlobalConfig.background
    }
}
