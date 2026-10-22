pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import M3Shapes
import Caelestia.Config
import Caelestia.Services
import caelestia.services

Item {
    id: root

    readonly property list<int> shapePool: [
        MaterialShape.Circle,
        MaterialShape.Cookie4Sided,
        MaterialShape.Cookie6Sided,
        MaterialShape.Cookie7Sided,
        MaterialShape.Cookie9Sided,
        MaterialShape.Cookie12Sided,
        MaterialShape.Sunny,
        MaterialShape.VerySunny,
        MaterialShape.SoftBurst,
        MaterialShape.Gem,
        MaterialShape.Diamond,
        MaterialShape.Clover4Leaf,
        MaterialShape.Clover8Leaf
    ]
    readonly property list<string> colorPool: [
        Colours.palette.m3primary,
        Colours.palette.m3secondary,
        Colours.palette.m3tertiary
    ]
    readonly property bool isPlaying: Players.active?.isPlaying ?? false
    readonly property real speedMultiplier: Config.general.mediaGifSpeedAdjustment > 0
        ? (Config.general.mediaGifSpeedAdjustment / 300)
        : 1.0
    readonly property real currentBpm: Math.max(50.0, Math.min(220.0, Audio.beatTracker?.bpm > 20 ? Audio.beatTracker.bpm : 120.0))

    property bool active: true
    property color activeColor: Colours.palette.m3primary
    property bool hasAudio: false

    property real rawBass: 0.0
    property real rawEnergy: 0.0
    property real peakBass: 0.2

    property real smoothBass: 0.0
    property real smoothEnergy: 0.0

    property real breathPhase: 0.0
    property real baseRotation: 0.0
    property real beatPulse: 0.0
    property real lastMorphTime: 999.0

    function lerpColor(c1: color, c2: color, t: real): color {
        const f = Math.max(0.0, Math.min(1.0, t));
        const a = c1.a !== undefined ? (c1.a + (c2.a - c1.a) * f) : 1.0;
        return Qt.rgba(
            c1.r + (c2.r - c1.r) * f,
            c1.g + (c2.g - c1.g) * f,
            c1.b + (c2.b - c1.b) * f,
            a
        );
    }

    function morphRandomShape(): void {
        let nextShape = root.shapePool[Math.floor(Math.random() * root.shapePool.length)];
        if (nextShape === materialShape.shape && root.shapePool.length > 1) {
            nextShape = root.shapePool[(Math.floor(Math.random() * (root.shapePool.length - 1)) + 1) % root.shapePool.length];
        }
        materialShape.shape = nextShape;
        root.lastMorphTime = 0.0;
        root.activeColor = root.colorPool[Math.floor(Math.random() * root.colorPool.length)];
    }

    function processAudio(): void {
        if (!root.active || !root.visible)
            return;

        const vals = Audio.cava?.values;
        if (!vals || vals.length === 0) {
            root.rawBass = 0.0;
            root.rawEnergy = 0.0;
            root.hasAudio = false;
            return;
        }

        let bassSum = 0.0;
        const bassCount = Math.min(vals.length, 4);
        for (let i = 0; i < bassCount; ++i)
            bassSum += vals[i];
        const bassAvg = bassSum / Math.max(1, bassCount);

        let energySum = 0.0;
        for (let i = 0; i < vals.length; ++i)
            energySum += vals[i];
        const energyAvg = energySum / Math.max(1, vals.length);

        root.peakBass = Math.max(0.2, Math.max(bassAvg * 1.1, root.peakBass * 0.996));
        const normBass = Math.min(1.0, bassAvg / Math.max(0.05, root.peakBass));
        const normEnergy = Math.min(1.0, energyAvg * 3.0);

        const soundActive = bassAvg > 0.015 || energyAvg > 0.01;
        root.hasAudio = soundActive || root.isPlaying;

        root.rawBass = soundActive ? normBass : 0.0;
        root.rawEnergy = soundActive ? normEnergy : 0.0;

        const isTooLoudBass = (normBass > 0.72) || (normBass > root.smoothBass + 0.28 && normBass > 0.55);

        if (isTooLoudBass) {
            root.beatPulse = 1.0;
            if (root.lastMorphTime > 320.0)
                root.morphRandomShape();
        }
    }

    Loader {
        active: root.visible
        sourceComponent: ServiceRef {
            service: Audio.cava
        }
    }

    Loader {
        active: root.visible
        sourceComponent: ServiceRef {
            service: Audio.beatTracker
        }
    }

    FrameAnimation {
        id: frameAnim

        running: root.visible && root.active

        onTriggered: {
            const dt = Math.min(frameTime, 0.05);
            root.lastMorphTime += dt * 1000.0;
            const active = root.hasAudio && (root.isPlaying || root.rawBass > 0.04);

            const targetBass = active ? root.rawBass : 0.0;
            const targetEnergy = active ? root.rawEnergy : 0.0;

            const attackRate = 20.0;
            const decayRate = active ? 4.0 : 6.0;
            const bassRate = targetBass > root.smoothBass ? attackRate : decayRate;
            const energyRate = targetEnergy > root.smoothEnergy ? attackRate : decayRate;
            root.smoothBass += (targetBass - root.smoothBass) * Math.min(1.0, bassRate * dt);
            root.smoothEnergy += (targetEnergy - root.smoothEnergy) * Math.min(1.0, energyRate * dt);

            const breathSpeed = active
                ? ((root.currentBpm / 120.0) * (0.6 + root.smoothEnergy * 2.0))
                : 0.4;
            root.breathPhase = (root.breathPhase + dt * breathSpeed * 2.0 * Math.PI) % (2.0 * Math.PI);

            root.beatPulse = Math.max(0.0, root.beatPulse - dt * 4.5);

            if (active) {
                const tempoSpin = (root.currentBpm / 60.0) * 30.0;
                const bassSpin = root.smoothBass * 70.0;
                root.baseRotation = (root.baseRotation + (tempoSpin + bassSpin) * dt) % 360.0;
            } else {
                root.baseRotation = (root.baseRotation + 8.0 * dt) % 360.0;
            }

            if (!active && root.lastMorphTime > 2500.0 && materialShape.shape !== MaterialShape.Circle) {
                materialShape.shape = MaterialShape.Circle;
                root.lastMorphTime = 0.0;
            }
        }
    }

    Connections {
        function onValuesChanged(): void {
            root.processAudio();
        }

        target: Audio.cava
    }

    Connections {
        function onBeat(bpm: real): void {
            if (!root.active || !root.visible)
                return;
            root.beatPulse = Math.max(root.beatPulse, 0.6);
        }

        target: Audio.beatTracker
    }

    MaterialShape {
        id: materialShape

        anchors.centerIn: parent
        implicitSize: Math.min(parent.width, parent.height) * 0.82
        shape: MaterialShape.Circle
        animationDuration: 320
        rotation: root.baseRotation
        scale: {
            const breath = 1.0 + Math.sin(root.breathPhase) * 0.035;
            const bassSwell = root.smoothBass * 0.16;
            const bassPunch = root.beatPulse * 0.20;
            return Math.max(0.7, Math.min(1.45, breath + bassSwell + bassPunch));
        }
        color: {
            const idleColor = root.activeColor;
            const maxRed = Colours.palette.m3error;
            const bassFactor = Math.max(0.0, Math.min(1.0, root.rawBass));
            return root.lerpColor(idleColor, maxRed, bassFactor);
        }
    }
}
