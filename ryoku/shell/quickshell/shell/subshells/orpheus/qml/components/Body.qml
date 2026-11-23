pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Blobs
import shell.services
import "../kit" as K
import "../Metrics.js" as M

Item {
    id: body

    required property var prefs
    property string span: "islands"
    property var leftDrop: ({ x: 0, w: 0 })
    property var voiceDrop: ({ x: 0, w: 0 })
    property var rightDrop: ({ x: 0, w: 0 })
    property real waveX: 0
    property real waveW: 0
    property var bubbleLeft: ({ x: 0, w: 0, visible: false, key: "" })
    // drawn in every span, `y` docks it under the top edge while the bar is hidden
    property var recordBubble: ({ x: 0, w: 0, y: 0 })
    property string cardKey: ""
    property real originX: voiceDrop.x
    property real originW: voiceDrop.w
    property real cardW: 0
    property real cardH: 0
    // the width ceiling of the page being shown (the launcher is wider)
    property real cardMaxW: M.cardMaxW
    // the page is the launcher: its lower edge can carry the music
    property bool launcherCard: false
    // the launcher asks for the music edge (open, playing, ambient motion allowed)
    property bool musicEdge: false
    property bool waveActive: true

    property real cardProgress: 0
    property real heldOriginX: originX
    property real heldOriginW: Math.max(12, originW)
    property real cardAnchor: originX + originW / 2

    // Square trades the liquid fillets for crisp corners and a block wave. The
    // corner and smoothing follow the desktop design (Look): Lithic cuts its
    // corners, Material swells soft and round, Adwaita rounds moderately, Fluent
    // keeps small corners. The blob field reads the corner as a cut or a radius.
    readonly property bool square: prefs.form === "square"
    readonly property real radius: square ? K.Look.cornerAt(K.Look.cutSm, 200, 40, K.Style.scale)
        : K.Look.cornerAt(K.Look.cutXl, 400, K.Style.barH, K.Style.scale)
    readonly property real smoothing: square ? K.Look.squareSmoothing : K.Look.liquidSmoothing
    // A page that would stop just short of a screen edge snaps flush to it, so it
    // either melts into the edge or stands clear of it, never half-bridging.
    readonly property real edgeSnap: smoothing * 2
    readonly property real restCardW: Math.max(12, heldOriginW)
    readonly property real restCardX: Math.max(0, Math.min(
        Math.max(0, width - restCardW), cardAnchor - restCardW / 2))
    readonly property real cardTargetW: cardW > 0
        ? Math.min(cardMaxW, width, cardW) : restCardW
    readonly property real cardNaturalX: Math.max(0, Math.min(
        Math.max(0, width - cardTargetW), cardAnchor - cardTargetW / 2))
    readonly property real cardTargetX: cardNaturalX < edgeSnap ? 0
        : width - cardNaturalX - cardTargetW < edgeSnap
            ? Math.max(0, width - cardTargetW) : cardNaturalX
    // The card buds out of the drop it grows from. Its visible face sits just
    // below the bar lip; the blob carries a neck up into the band so the
    // smooth-min welds card and drop into one liquid edge (the frame popout's
    // neck). On close the neck buries one smoothing deeper so the fillet drains
    // flush before the shape drops out at zero size, no shrinking nub left.
    readonly property real cardY: K.Style.barH
    readonly property bool cardActive: cardKey !== "" || cardProgress > 0.001
    readonly property real cardX: restCardX
        + (cardTargetX - restCardX) * cardProgress
    readonly property real cardCurW: cardActive
        ? restCardW + (cardTargetW - restCardW) * cardProgress : 0
    readonly property real cardCurH: cardActive ? cardH * cardProgress : 0
    readonly property real cardNeck: K.Style.barH + smoothing
    readonly property real cardBurial: (1 - cardProgress) * smoothing
    readonly property real cardBlobY: cardY - cardNeck + cardBurial
    readonly property real cardBlobH: cardCurH + cardNeck - cardBurial
    // The card content rides this matrix so it stretches with the blob while
    // the page travels, the same liquid deform the frame popout applies.
    readonly property alias cardDeform: cardShape.deformMatrix

    // The body fades to the chosen opacity at rest and firms up while a page is
    // open, so a page is always read on solid paper. Shapes overlap where they
    // blend, so translucency is applied to the composed body, not per shape.
    opacity: prefs.opacity + (1 - prefs.opacity) * cardProgress
    layer.enabled: prefs.opacity < 0.999

    function shown(drop) {
        return drop && Number(drop.w) > 0.001;
    }

    function flattenFor(x, w) {
        if (!body.cardActive || w <= 0)
            return 0;
        return body.cardX < x + w && body.cardX + body.cardCurW > x
            ? body.cardProgress : 0;
    }

    // A shape touching a screen edge keeps a square corner there; the edge
    // shoulder turns that corner into a fillet down the edge.
    function edgeRadius(x, w, side) {
        if (side === "left")
            return x < 1 ? 0 : body.radius;
        return x + w > body.width - 1 ? 0 : body.radius;
    }

    function syncOrigin() {
        if (body.cardKey === "")
            return;
        body.heldOriginX = body.originX;
        body.heldOriginW = Math.max(12, body.originW);
        body.cardAnchor = body.originX + body.originW / 2;
    }

    // The music wave is passive eye-candy: it runs only while the desktop has
    // headroom (Balanced or Performance, never Power Saver) and motion is
    // allowed. Micro-animations (the card weld, hover, press) are not gated here.
    function syncAudioClaim() {
        AudioBars.setActive(body, body.waveActive && body.prefs.gain > 0
            && K.Look.ambientMotion);
    }

    // The launcher's edge claims the music under its own owner, so the bar's
    // claim and this one never release each other.
    readonly property var edgeOwner: ({ owner: "orpheus-launcher-edge" })
    readonly property bool edgeRunning: musicEdge && launcherCard
        && prefs.gain > 0 && K.Look.ambientMotion

    function syncEdgeClaim() {
        AudioBars.setActive(body.edgeOwner, body.edgeRunning);
    }

    onCardKeyChanged: if (cardKey !== "") syncOrigin()
    onOriginXChanged: syncOrigin()
    onOriginWChanged: syncOrigin()
    onWaveActiveChanged: syncAudioClaim()
    onEdgeRunningChanged: syncEdgeClaim()
    Component.onCompleted: {
        syncAudioClaim();
        syncEdgeClaim();
    }
    Component.onDestruction: {
        AudioBars.setActive(body, false);
        AudioBars.setActive(body.edgeOwner, false);
    }

    Connections {
        target: body.prefs
        function onGainChanged() { body.syncAudioClaim(); }
    }

    Connections {
        target: K.Look
        function onAmbientMotionChanged() { body.syncAudioClaim(); }
    }

    Behavior on cardAnchor {
        enabled: !K.Style.still
        NumberAnimation {
            duration: K.Style.spatial
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Style.spatialCurve
        }
    }

    states: State {
        name: "open"
        when: body.cardKey !== ""
        PropertyChanges { body.cardProgress: 1 }
    }

    transitions: [
        Transition {
            to: "open"
            enabled: !K.Style.still
            NumberAnimation {
                property: "cardProgress"
                duration: K.Style.enter
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Style.enterCurve
            }
        },
        Transition {
            from: "open"
            enabled: !K.Style.still
            NumberAnimation {
                property: "cardProgress"
                duration: K.Style.exit
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Style.exitCurve
            }
        }
    ]

    BlobGroup {
        id: liquid
        color: K.Style.base
        smoothing: body.smoothing
        cutCorners: K.Look.lithic
        borderWidth: body.prefs.rim ? 1 : 0
        borderColor: K.Style.rim
        waveColor: K.Style.trace
        shadowStrength: 0
        shadowSize: 0
    }

    // Every shape in the group is unioned by the shader, visible or not, so a
    // hidden shape must also have zero size.

    // Off-screen walls along the side edges: anything that reaches an edge
    // melts into it with a fillet, the way every shape melts into the top.
    BlobRect {
        group: liquid
        x: -M.shoulder - 2
        y: -M.shoulder - 2
        width: M.shoulder
        height: body.height + M.shoulder + 2
        radius: 0
        sinks: false
        deformScale: 0
    }

    BlobRect {
        group: liquid
        x: body.width + 2
        y: -M.shoulder - 2
        width: M.shoulder
        height: body.height + M.shoulder + 2
        radius: 0
        sinks: false
        deformScale: 0
    }

    Shoulder { drop: body.leftDrop }
    Shoulder { drop: ({ x: body.waveX, w: body.waveW }) }
    Shoulder { drop: body.rightDrop }
    Shoulder { drop: body.span === "islands" ? body.bubbleLeft : ({ x: 0, w: 0 }) }
    Shoulder {
        drop: body.recordBubble
        dock: Number(body.recordBubble.y) || 0
    }

    // The side drops never blend with the voice: where a fused body covers
    // them, blending would square the shared corners and drip at the seams.
    Drop {
        id: leftShape
        group: liquid
        exclude: [wave]
        visible: body.shown(body.leftDrop)
        x: body.leftDrop.x
        y: 0
        width: body.shown(body.leftDrop) ? body.leftDrop.w : 0
        height: K.Style.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.edgeRadius(body.leftDrop.x, body.leftDrop.w, "left")
        bottomRightRadius: body.radius
        Behavior on bottomLeftRadius { enabled: !K.Style.still; NumberAnimation { duration: K.Style.ms(300) } }
    }

    Drop {
        id: rightShape
        group: liquid
        exclude: [wave]
        visible: body.shown(body.rightDrop)
        x: body.rightDrop.x
        y: 0
        width: body.shown(body.rightDrop) ? body.rightDrop.w : 0
        height: K.Style.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.radius
        bottomRightRadius: body.edgeRadius(body.rightDrop.x, body.rightDrop.w, "right")
        Behavior on bottomRightRadius { enabled: !K.Style.still; NumberAnimation { duration: K.Style.ms(300) } }
    }

    BlobRect {
        group: liquid
        visible: body.span === "islands" && body.bubbleLeft.w > 0.5
        x: body.bubbleLeft.x
        y: 0
        width: body.span === "islands" && body.bubbleLeft.w > 0.5 ? body.bubbleLeft.w : 0
        height: K.Style.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.radius
        bottomRightRadius: body.radius
        sinks: false
        deformScale: 0
    }

    BlobRect {
        group: liquid
        visible: body.recordBubble.w > 0.5
        x: body.recordBubble.x
        y: Number(body.recordBubble.y) || 0
        width: body.recordBubble.w > 0.5 ? body.recordBubble.w : 0
        height: K.Style.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.radius
        bottomRightRadius: body.radius
        sinks: false
        deformScale: 0
    }

    // The voice, and in the fused and full spans the whole band.
    Drop {
        id: wave
        group: liquid
        exclude: [leftShape, rightShape]
        visible: true
        x: body.waveX
        y: 0
        width: body.waveW
        height: K.Style.barH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.edgeRadius(body.waveX, body.waveW, "left")
        bottomRightRadius: body.edgeRadius(body.waveX, body.waveW, "right")
        Behavior on bottomLeftRadius { enabled: !K.Style.still; NumberAnimation { duration: K.Style.ms(300) } }
        Behavior on bottomRightRadius { enabled: !K.Style.still; NumberAnimation { duration: K.Style.ms(300) } }
    }

    // The card page, welded into the drop it grows from. It lives in the bar's
    // own blob field (one window) so the neck smooth-mins into the band and the
    // page reads as the bar swelling open, not a panel laid on top. The top
    // corners are flush where the neck meets the band; the bottom corners round
    // (or melt into a screen edge). Velocity deform stretches it liquidly while
    // it travels between anchors. It is the group's fourth wave: flat for every
    // card, and for the launcher its lower edge ripples outward with the music.
    BlobWave {
        id: cardShape
        group: liquid
        visible: body.cardActive
        x: body.cardX
        y: body.cardBlobY
        width: body.cardCurW
        height: body.cardBlobH
        topLeftRadius: 0
        topRightRadius: 0
        bottomLeftRadius: body.edgeRadius(body.cardX, body.cardCurW, "left")
        bottomRightRadius: body.edgeRadius(body.cardX, body.cardCurW, "right")
        stiffness: 180
        damping: 16
        deformScale: body.cardActive ? 0.000015 : 0
        sinks: false
        levels: body.edgeRunning ? AudioBars.levels : []
        gain: body.prefs.gain
        amplitude: body.launcherCard ? M.launcherWave * K.Style.scale : 0
        points: Math.round(Math.min(64, Math.max(12,
            width / (12 * K.Style.scale))))
        stepped: body.square
        blockPitch: Math.max(14 * K.Style.scale, Math.min(26 * K.Style.scale,
            (width - 2 * K.Style.barPad) / 10))
        mirror: body.prefs.mirror
        edgeInset: body.square ? K.Style.barPad
            : Math.max(K.Style.barPad, width * 0.16)
        waveStiffness: body.prefs.waveStiffness
        waveDamping: body.prefs.waveDamping
    }

    component Drop: BlobWave {
        levels: body.waveActive && body.prefs.gain > 0 ? AudioBars.levels : []
        gain: body.prefs.gain
        points: Math.round(Math.min(64, Math.max(12,
            width / (12 * K.Style.scale))))
        stepped: body.square
        blockPitch: Math.max(14 * K.Style.scale, Math.min(26 * K.Style.scale,
            (width - 2 * K.Style.barPad) / 10))
        amplitude: body.prefs.depth * (1 - body.flattenFor(x, width))
        mirror: body.prefs.mirror
        edgeInset: body.square ? K.Style.barPad
            : Math.max(K.Style.barPad, width * 0.16)
        waveStiffness: body.prefs.waveStiffness
        waveDamping: body.prefs.waveDamping
        sinks: false
        deformScale: 0
    }

    component Shoulder: BlobRect {
        required property var drop
        property real dock: 0
        readonly property bool active: body.shown(drop)
        group: liquid
        visible: active
        x: Number(drop.x) - M.shoulderReach
        y: dock - M.shoulder - 2
        width: active ? Number(drop.w) + M.shoulderReach * 2 : 0
        height: M.shoulder
        radius: 0
        sinks: false
        deformScale: 0
    }
}
