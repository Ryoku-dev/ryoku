import QtQuick

QtObject {
    required property string screenName
    property bool ready: false
    property string path: ""
    property int revision: 0
    property string fit: "Cover"
    property var transition: null
    property string depth: ""
    property int depthRev: 0
    property string videoPath: ""
    property bool live: false
    // The in-shell clip's audio: muted by default, full volume (0-100). The
    // daemon fills them for a video frame; a still leaves the defaults.
    property bool mute: true
    property int volume: 100
    // The daemon's pause rules freeze the clip in place rather than dropping back to the still.
    property bool paused: false
    // An uncommitted wall.preview paint: the palette bridge and other readers
    // must not treat this entry as the committed wallpaper.
    property bool preview: false
    // A scene renderer under the desktop surface wants hover positions.
    property bool pointer: false

    function apply(line: string): bool {
        try {
            const state = JSON.parse(line);
            const entry = (state.outputs && state.outputs[screenName]) || state.default;
            if (!entry)
                return false;
            ready = true;
            fit = entry.fit || "Cover";
            transition = entry.transition || null;
            path = entry.path || "";
            depth = entry.depth || "";
            depthRev = entry.depthRev || 0;
            revision = entry.revision || 0;
            videoPath = entry.videoPath || "";
            live = entry.live === true;
            mute = entry.mute !== false;
            volume = (typeof entry.volume === "number") ? entry.volume : 100;
            paused = entry.paused === true;
            preview = entry.preview === true;
            pointer = entry.pointer === true;
            return true;
        } catch (error) {
            return false;
        }
    }
}
