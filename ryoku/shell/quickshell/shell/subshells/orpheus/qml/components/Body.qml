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
    property var bubbleRight: ({ x: 0, w: 0, visible: false, key: "" })
    property string cardKey: ""
    property real originX: voiceDrop.x
    property real originW: voiceDrop.w
    property real cardW: 0
    property real cardH: 0
    property bool waveActive: true

    property real cardProgress: 0
    property real heldOriginX: originX
    property real heldOriginW: Math.max(12, originW)
    property real cardAnchor: originX + originW / 2

    // Square trades the liquid fillets for crisp corners and a block wave.
    readonly property bool square: prefs.form === "square"
    readonly property real radius: square ? K.Style.squareRadius : K.Style.liquidRadius
    readonly property real smoothing: square ? M.squareSmoothing : M.smoothing
    // A page that would stop just short of a screen edge snaps flush to it, so it
    // either melts into the edge or stands clear of it, never half-bridging.
    readonly property real edgeSnap: smoothing * 2
    readonly property real restCardW: Math.max(12, heldOriginW)
    readonly property real restCardX: Math.max(0, Math.min(
        Math.max(0, width - restCardW), cardAnchor - restCardW / 2))
    readonly property real cardTargetW: cardW > 0
        ? Math.min(M.cardMaxW, width, cardW) : restCardW
    readonly property real cardNaturalX: Math.max(0, Math.min(
        Math.max(0, width - cardTargetW), cardAnchor - cardTargetW / 2))
    readonly property real cardTargetX: cardNaturalX < edgeSnap ? 0
        : width - cardNaturalX - cardTargetW < edgeSnap
            ? Math.max(0, width - cardTargetW) : cardNaturalX
    readonly property real cardY: K.Style.barH - M.weld
    readonly property bool cardActive: cardKey !== "" || cardProgress > 0.001
    readonly property real cardX: restCardX
        + (cardTargetX - restCardX) * cardProgress
    readonly property real cardCurW: cardActive
        ? restCardW + (cardTargetW - restCardW) * cardProgress : 0
    readonly property real cardCurH: cardActive
        ? M.weld + cardH * cardProgress : 0
    readonly property bool leftOverhang: cardX < heldOriginX - 0.5
    readonly property bool rightOverhang: cardX + cardCurW
        > heldOriginX + heldOriginW + 0.5

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

    function syncAudioClaim() {
        AudioBars.setActive(body, body.waveActive && body.prefs.gain > 0);
    }

    onCardKeyChanged: if (cardKey !== "") syncOrigin()
    onOriginXChanged: syncOrigin()
    onOriginWChanged: syncOrigin()
    onWaveActiveChanged: syncAudioClaim()
    Component.onCompleted: syncAudioClaim()
    Component.onDestruction: AudioBars.setActive(body, false)

    Connections {
        target: body.prefs
        function onGainChanged() { body.syncAudioClaim(); }
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
                duration: K.Style.spatial
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Style.enterCurve
            }
        },
        Transition {
            from: "open"
            enabled: !K.Style.still
            NumberAnimation {
                property: "cardProgress"
                duration: K.Style.ms(300)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Style.exitCurve
            }
        }
    ]

    BlobGroup {
        id: liquid
        color: K.Style.base
        smoothing: body.smoothing
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
    Shoulder { drop: body.span === "islands" ? body.bubbleRight : ({ x: 0, w: 0 }) }

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
        visible: body.span === "islands" && body.bubbleRight.w > 0.5
        x: body.bubbleRight.x
        y: 0
        width: body.span === "islands" && body.bubbleRight.w > 0.5 ? body.bubbleRight.w : 0
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

    BlobRect {
        group: liquid
        visible: body.cardActive
        x: body.cardX
        y: body.cardY
        width: body.cardCurW
        height: body.cardCurH
        topLeftRadius: body.leftOverhang ? body.radius : 0
        topRightRadius: body.rightOverhang ? body.radius : 0
        bottomLeftRadius: body.edgeRadius(body.cardX, body.cardCurW, "left")
        bottomRightRadius: body.edgeRadius(body.cardX, body.cardCurW, "right")
        stiffness: 180
        damping: 16
        deformScale: 0
        sinks: false
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
        readonly property bool active: body.shown(drop)
        group: liquid
        visible: active
        x: Number(drop.x) - M.shoulderReach
        y: -M.shoulder - 2
        width: active ? Number(drop.w) + M.shoulderReach * 2 : 0
        height: M.shoulder
        radius: 0
        sinks: false
        deformScale: 0
    }
}
