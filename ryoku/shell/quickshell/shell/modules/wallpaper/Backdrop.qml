pragma ComponentBehavior: Bound

import QtQuick
import QtMultimedia
import "Singletons"

// A full-bleed wallpaper surface that REVEALS each new image over the current one
// through a GPU shader mask (reveal.frag.qsb). The daemon publishes, per revision,
// a transition {kind, angle, waveAmp, originX, originY, bezier, edgeSoftness,
// durationMs}; the shader reveals newTex over oldTex along that mask with the
// preset's cubic-bezier timing and feathered edge, over the shared 2.2 s. This
// restores the wallpaper-daemon transition set in-shell. fade is a plain eased
// crossfade; init and the live still-frame (a null transition) also just crossfade.
//
// Live clips play in-shell (QtMultimedia): the daemon publishes the clip's still
// as `url` and the clip itself as `videoUrl`; the still reveals like any image,
// then the reveal shader yields to the video surface once frames present.
//
// Two image buffers ping-pong: the just-shown image becomes the base of the next
// reveal. A revision arriving mid-reveal commits the in-flight reveal instantly
// (the incoming image becomes the committed base with no seam left behind), then
// the next reveal grows from that image, so rapid Super+Shift+W never flashes.
Item {
    id: view
    // Yield to the ryogami-live C player: when the frame says live, the whole
    // painter hides so the C player's layer below shows through.
    visible: !view.live

    // The full file url the surface paints (path + cache-busting revision), "" until
    // the first frame; the window's paper colour shows through until then and in the
    // letterbox margins of a Contain / ScaleDown fit.
    property string url: ""

    // content_fit -> Image.fillMode. ScaleDown is Contain that never upscales, which
    // QML has no fillMode for, so a picture smaller than the surface is padded and a
    // larger one is fit.
    property string fit: "Cover"

    // The reveal preset for the current revision (null -> plain crossfade). Set by
    // the shell root from the topic frame just before `url`, so onUrlChanged reads
    // the matching transition.
    property var transition: null

    // The monitor scale, so the decode cap below matches physical pixels.
    property real dpr: 1

    // The live clip ("" for a still).
    property string videoUrl: ""
    // The ryogami-live yield flag: the painter hides while the C player owns
    // the layer; false for the in-shell engine, which plays inside this surface.
    property bool live: false

    // The in-shell clip's audio the player binds: muted by default, volume
    // 0-100 (scaled to AudioOutput's 0..1). A live wallpaper is no longer
    // forced silent; the daemon threads the picker's mute/volume here.
    property bool videoMuted: true
    property int videoVolume: 100

    // Decoding is capped at the surface resolution: an 8K source costs a
    // screen-sized texture instead of a full-resolution decode, which lagged
    // every switch and overran the image allocation cap on very large files.
    readonly property int decodeW: Math.max(1, Math.ceil(width * dpr))
    readonly property int decodeH: Math.max(1, Math.ceil(height * dpr))

    // Which buffer currently holds the committed (front) image; the other is the
    // load target for the incoming image.
    property bool aFront: true
    property bool rendered: false
    readonly property bool decoded: imgA.status === Image.Ready || imgB.status === Image.Ready

    // The player is presenting frames: the reveal shader yields to the video
    // surface, exactly where the old ryogami-live READY flip hid the still.
    // Read this through the player itself from any notify handler: reading the
    // binding from its own notify handler (onVideoActiveChanged) looped the
    // MediaPlayer's playbackState notify and froze the wallpaper.
    readonly property bool videoActive: player.playbackState === MediaPlayer.PlayingState && player.hasVideo

    // The video owns the surface once it has presented a frame; the reveal
    // hides only behind this, never a transient playbackState dip.
    property bool videoOn: false

    // Hop the videoOn assignment ~80 ms after the first frame so vout only
    // becomes visible with a frame already in its surface (a too-early flip
    // paints black for one frame). yieldScheduled keeps onVideoFrameChanged
    // (every frame) from re-arming the timer each time.
    property bool yieldScheduled: false
    Timer {
        id: yieldDelay
        interval: 80
        onTriggered: {
            view.yieldScheduled = false;
            if (player.playing && view.videoUrl !== "")
                view.videoOn = true;
        }
    }

    function fillModeFor(img) {
        switch (view.fit) {
        case "Contain":
            return Image.PreserveAspectFit;
        case "Fill":
            return Image.Stretch;
        case "Center":
            return Image.Pad; // 1:1, centred, no scale
        case "Tile":
            return Image.Tile; // repeat the source across the surface
        case "ScaleDown":
            return (img.sourceSize.width <= view.width && img.sourceSize.height <= view.height) ? Image.Pad : Image.PreserveAspectFit;
        default:
            return Image.PreserveAspectCrop; // Cover
        }
    }

    // VideoOutput has no Pad and no Tile; Contain and Center both map to a
    // letterboxed fit (a clip cannot repeat, so Tile covers, and Center's
    // "no scale" is unrepresentable for a decoder that fills its output).
    // Fill stretches, everything else covers. ScaleDown is Contain here (a
    // small clip plays fit, a large one fits).
    function videoFill() {
        switch (view.fit) {
        case "Contain":
        case "ScaleDown":
        case "Center":
            return VideoOutput.PreserveAspectFit;
        case "Fill":
            return VideoOutput.Stretch;
        default:
            return VideoOutput.PreserveAspectCrop;
        }
    }

    function kindCode(k) {
        switch (k) {
        case "fade":
            return 0;
        case "wipe":
            return 1;
        case "wave":
            return 2;
        case "center":
            return 3;
        case "grow":
            return 4;
        case "any":
            return 5;
        case "outer":
            return 6;
        case "pixelate":
            return 7;
        case "dissolve":
            return 8;
        case "ripple":
            return 9;
        case "shatter":
            return 10;
        case "glitch":
            return 11;
        case "crt":
            return 12;
        case "stripes":
            return 13;
        case "melt":
            return 14;
        case "peel":
            return 15;
        }
        return 0;
    }

    onUrlChanged: {
        view.rendered = false;
        view.beginReveal();
    }
    Component.onCompleted: view.beginReveal()

    // A new clip loads on the url change; a cleared videoUrl stops the player
    // and hands the surface back to the reveal shader (the still under it). A
    // video swap to the same clip still restarts: the daemon re-publishes a
    // fresh revision rather than sending the identical url twice. The clip is
    // buffered here but NOT played: the transition runs first, and the video
    // starts only once the reveal's picture is committed (startVideo).
    onVideoUrlChanged: {
        if (view.videoUrl === "") {
            player.stop();
            view.videoOn = false;
            view.yieldScheduled = false;
            yieldDelay.stop();
            return;
        }
        view.videoOn = false;
        view.yieldScheduled = false;
        yieldDelay.stop();
        player.source = view.videoUrl;
    }

    // Watchdog: a clip whose still never decoded must still play, and the yield is
    // re-attempted until the player owns the surface. Playback starts
    // asynchronously, so the single attempt at the reveal's end lands before the
    // player is playing; this keeps trying until videoOn flips, then stops.
    Timer {
        id: startWatch
        interval: 400
        repeat: true
        running: view.videoUrl !== "" && !view.videoOn
        onTriggered: {
            view.startVideo();
            view.maybeYield();
        }
    }

    // Begin playback at the still's moment (the daemon extracts the frame at
    // second 1); the seek runs only once the media is loaded and long enough.
    function startVideo() {
        if (view.videoUrl === "" || player.playbackState === MediaPlayer.PlayingState)
            return;
        const loaded = player.mediaStatus === MediaPlayer.LoadedMedia
            || player.mediaStatus === MediaPlayer.BufferedMedia;
        if (loaded && player.duration > 1000)
            player.position = 1000;
        player.play();
    }

    // The video takes the surface once the reveal is done and frames present.
    // MediaPlayer exposes playbackStateChanged (there is no videoFrameChanged
    // signal, which is why the yield used to be attempted once and never again),
    // so the yield is driven from the state it actually reports.
    Connections {
        target: player
        function onPlaybackStateChanged() { view.maybeYield() }
        function onErrorChanged() {
            if (player.error !== MediaPlayer.NoError && view.videoUrl !== "") {
                player.position = 0;
                player.play();
            }
        }
    }

    function maybeYield() {
        if (view.videoUrl === "" || revealAnim.running)
            return;
        const playing = player.playing;
        const back = view.aFront ? imgB : imgA;
        if (!playing) {
            if (back.source === view.url && back.status === Image.Loading)
                return;
            view.startVideo();
            return;
        }
        if (back.source === view.url && back.status === Image.Loading)
            return;
        if (view.yieldScheduled)
            return;
        view.yieldScheduled = true;
        yieldDelay.restart();
    }

    // Load `url` into the back buffer and, once decoded, start the reveal. A reveal
    // already running is committed instantly first, so the new reveal starts from a
    // whole image rather than a frozen half-mask.
    function beginReveal() {
        if (view.url === "")
            return;
        if (revealAnim.running)
            view.commitInstant();
        const back = view.aFront ? imgB : imgA;
        if (back.source == view.url) {
            if (back.status === Image.Ready) {
                renderTimer.restart();
                view.startReveal();
            }
            return;
        }
        back.source = view.url;
    }

    // Commit the current front/back pair: the back (incoming) image becomes the new
    // committed front, and the reveal front resets so the shader shows that image
    // whole. Used both when a reveal finishes and, via commitInstant, when one is
    // interrupted.
    function commit() {
        view.aFront = !view.aFront;
        reveal.progress = 0;
        // Drop the outgoing image: it was only needed for the reveal, and held
        // a second full decode of the wallpaper for the rest of the session.
        const stale = view.aFront ? imgB : imgA;
        stale.source = "";
    }

    function commitInstant() {
        revealAnim.stop(); // an explicit stop emits no `finished`, so commit runs once
        view.commit();
    }

    // Configure the shader + animation for the current transition and run it. A null
    // transition (init / live still-frame) or reduce-motion collapses to a plain
    // eased crossfade (instant when motion is reduced). A skwd catalog transition
    // (t.shader set) loads its own ported fragment shader and eases inside it, so
    // progress runs linear over the configured duration.
    function startReveal() {
        const t = view.transition;
        if (Motion.reduce || !t) {
            reveal.fragmentShader = "reveal.frag.qsb";
            reveal.kind = 0;
            reveal.angle = 0;
            reveal.waveAmp = 0;
            reveal.originX = 0.5;
            reveal.originY = 0.5;
            reveal.edgeSoftness = 0.002;
            reveal.seed = 0;
            revealAnim.easing.type = Easing.InOutQuad;
            revealAnim.duration = Motion.reduce ? 0 : Motion.wallpaperFade;
            revealAnim.restart();
            return;
        }
        if (t.shader) {
            reveal.fragmentShader = "skwd/" + t.shader + ".frag.qsb";
            reveal.seed = t.seed !== undefined ? t.seed : Math.random();
            revealAnim.easing.type = Easing.Linear;
            revealAnim.duration = t.durationMs > 0 ? t.durationMs : 600;
            revealAnim.restart();
            return;
        }
        reveal.fragmentShader = "reveal.frag.qsb";
        reveal.kind = view.kindCode(t.kind);
        reveal.angle = t.angle !== undefined ? t.angle : 0;
        reveal.waveAmp = t.waveAmp !== undefined ? t.waveAmp : 0;
        reveal.originX = t.originX !== undefined ? t.originX : 0.5;
        reveal.originY = t.originY !== undefined ? t.originY : 0.5;
        reveal.edgeSoftness = t.edgeSoftness !== undefined ? t.edgeSoftness : 0.002;
        // Fresh per switch so the noise kinds (dissolve/shatter/glitch/melt) never
        // replay one pattern; fall back to a local roll if the daemon sent none.
        reveal.seed = t.seed !== undefined ? t.seed : Math.random();
        const b = t.bezier;
        if (b && b.length === 4) {
            revealAnim.easing.type = Easing.Bezier;
            revealAnim.easing.bezierCurve = [b[0], b[1], b[2], b[3], 1, 1];
        } else {
            revealAnim.easing.type = Easing.InOutCubic;
        }
        revealAnim.duration = t.durationMs > 0 ? t.durationMs : 2200;
        revealAnim.restart();
    }

    Timer {
        id: renderTimer
        interval: 16
        onTriggered: view.rendered = true
    }

    // The two ping-pong image buffers. Each renders (with its fill mode) only into
    // its ShaderEffectSource; the shader composites the sources, so the raw Images
    // are hidden. A buffer fires startReveal once its incoming image is decoded, but
    // only while it is the back buffer (imgA is back when !aFront, imgB when aFront).
    Image {
        id: imgA
        anchors.fill: parent
        cache: false
        asynchronous: true
        sourceSize.width: view.decodeW
        sourceSize.height: view.decodeH
        fillMode: view.fillModeFor(imgA)
        onStatusChanged: {
            if (status === Image.Ready) {
                srcA.scheduleUpdate();
                if (source === view.url)
                    renderTimer.restart();
            }
            if (status === Image.Ready && source == view.url && !view.aFront)
                view.startReveal();
            else if (status === Image.Error)
                view.maybeYield();
        }
    }
    Image {
        id: imgB
        anchors.fill: parent
        cache: false
        asynchronous: true
        sourceSize.width: view.decodeW
        sourceSize.height: view.decodeH
        fillMode: view.fillModeFor(imgB)
        onStatusChanged: {
            if (status === Image.Ready) {
                srcB.scheduleUpdate();
                if (source === view.url)
                    renderTimer.restart();
            }
            if (status === Image.Ready && source == view.url && view.aFront)
                view.startReveal();
            else if (status === Image.Error)
                view.maybeYield();
        }
    }

    // live:false + an explicit scheduleUpdate() on load: re-capture only when the
    // image content actually changes, not every frame forever on a static wallpaper.
    ShaderEffectSource {
        id: srcA
        sourceItem: imgA
        hideSource: true
        live: false
    }
    ShaderEffectSource {
        id: srcB
        sourceItem: imgB
        hideSource: true
        live: false
    }

    // The reveal: mix(oldTex, newTex, mask) over the shared duration. oldTex is the
    // committed front buffer, newTex the incoming back buffer; they swap on commit,
    // so at rest (progress 0) the shader simply shows the committed image. It
    // yields to the video surfaces only while the clip owns the slot (videoOn).
    ShaderEffect {
        id: reveal
        anchors.fill: parent
        visible: !view.videoOn

        property variant oldTex: view.aFront ? srcA : srcB
        property variant newTex: view.aFront ? srcB : srcA
        property real progress: 0
        property int kind: 0
        property real angle: 0
        property real waveAmp: 0
        property real originX: 0.5
        property real originY: 0.5
        property real edgeSoftness: 0.002
        property real seed: 0
        property vector2d res: Qt.vector2d(width, height)

        fragmentShader: "reveal.frag.qsb"

        NumberAnimation {
            id: revealAnim
            target: reveal
            property: "progress"
            from: 0
            to: 1
            onFinished: {
                view.commit();
                view.startVideo();
                view.maybeYield();
            }
        }
    }

    MediaPlayer {
        id: player
        loops: MediaPlayer.Infinite
        videoOutput: vout
        audioOutput: AudioOutput {
            muted: view.videoMuted
            volume: view.videoVolume / 100
        }
    }
    VideoOutput {
        id: vout
        anchors.fill: parent
        fillMode: view.videoFill()
        visible: view.videoOn
    }

}
