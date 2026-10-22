pragma Singleton

import QtQuick
import QtMultimedia
import Quickshell
import Quickshell.Io
import Quickshell.Services.Pipewire
import Caelestia
import Caelestia.Config
import Caelestia.Services

Singleton {
    id: root

    property bool showInactiveDevices: false
    property string previousSinkName: ""
    property string previousSourceName: ""

    property list<PwNode> sinks: []
    property list<PwNode> sources: []
    property list<PwNode> streams: []

    property list<PwNode> appStreams: []

    readonly property string selfAppName: "caelestia-shell"

    readonly property PwNode sink: Pipewire.defaultAudioSink
    readonly property PwNode source: Pipewire.defaultAudioSource

    readonly property bool muted: !!sink?.audio?.muted
    readonly property real volume: sink?.audio?.volume ?? 0

    readonly property bool sourceMuted: !!source?.audio?.muted
    readonly property real sourceVolume: source?.audio?.volume ?? 0

    property var cava: null
    readonly property alias beatTracker: beatTracker

    property var _sfxCache: ({})

    function setVolume(newVolume: real): void {
        if (sink?.ready && sink?.audio) {
            sink.audio.muted = false;
            sink.audio.volume = Math.max(0, Math.min(GlobalConfig.services.maxVolume, newVolume));
        }
    }

    function incrementVolume(amount: real): void {
        setVolume(volume + (amount || GlobalConfig.services.audioIncrement));
    }

    function decrementVolume(amount: real): void {
        setVolume(volume - (amount || GlobalConfig.services.audioIncrement));
    }

    function setSourceVolume(newVolume: real): void {
        if (source?.ready && source?.audio) {
            source.audio.muted = false;
            source.audio.volume = Math.max(0, Math.min(GlobalConfig.services.maxVolume, newVolume));
        }
    }

    function incrementSourceVolume(amount: real): void {
        setSourceVolume(sourceVolume + (amount || GlobalConfig.services.audioIncrement));
    }

    function decrementSourceVolume(amount: real): void {
        setSourceVolume(sourceVolume - (amount || GlobalConfig.services.audioIncrement));
    }

    function setAudioSink(newSink: PwNode): void {
        Pipewire.preferredDefaultAudioSink = newSink;
    }

    function setAudioSource(newSource: PwNode): void {
        Pipewire.preferredDefaultAudioSource = newSource;
    }

    function getNodeDisplayName(node: PwNode): string {
        return node?.properties?.["node.nick"] || node?.description || node?.name || qsTr("Unknown Device");
    }

    function cycleNextAudioOutput(): void {
        if (sinks.length === 0)
            return;

        const currentIndex = sinks.findIndex(s => s === sink);
        const nextIndex = (currentIndex + 1) % sinks.length;
        setAudioSink(sinks[nextIndex]);
    }

    function setStreamVolume(stream: PwNode, newVolume: real): void {
        if (stream?.ready && stream?.audio) {
            stream.audio.muted = false;
            stream.audio.volume = Math.max(0, Math.min(GlobalConfig.services.maxVolume, newVolume));
        }
    }

    function setStreamMuted(stream: PwNode, muted: bool): void {
        if (stream?.ready && stream?.audio) {
            stream.audio.muted = muted;
        }
    }

    function getStreamVolume(stream: PwNode): real {
        return stream?.audio?.volume ?? 0;
    }

    function getStreamMuted(stream: PwNode): bool {
        return !!stream?.audio?.muted;
    }

    function getStreamName(stream: PwNode): string {
        if (!stream)
            return qsTr("Unknown");
        return stream.properties["application.name"] || stream.description || stream.name || qsTr("Unknown Application");
    }

    // App-level controls operate on every stream sharing the same app name, so a
    // single "caelestia-shell" row adjusts all of its streams together.
    function getAppVolume(stream: PwNode): real {
        if (!stream)
            return 0;

        const name = getStreamName(stream);
        let volume = 0;
        for (const s of root.streams) {
            if (getStreamName(s) === name && s?.audio)
                volume = Math.max(volume, s.audio.volume ?? 0);
        }
        return volume;
    }

    function getAppMuted(stream: PwNode): bool {
        if (!stream)
            return true;

        const name = getStreamName(stream);
        let hasStream = false;
        let allMuted = true;
        for (const s of root.streams) {
            if (getStreamName(s) !== name || !s?.audio)
                continue;
            hasStream = true;
            if (!s.audio.muted)
                allMuted = false;
        }
        return hasStream && allMuted;
    }

    function setAppVolume(stream: PwNode, newVolume: real): void {
        const name = getStreamName(stream);
        const clamped = Math.max(0, Math.min(GlobalConfig.services.maxVolume, newVolume));
        for (const s of root.streams) {
            if (getStreamName(s) === name && s?.ready && s?.audio) {
                s.audio.muted = false;
                s.audio.volume = clamped;
            }
        }
    }

    function setAppMuted(stream: PwNode, muted: bool): void {
        const name = getStreamName(stream);
        for (const s of root.streams) {
            if (getStreamName(s) === name && s?.ready && s?.audio)
                s.audio.muted = muted;
        }
    }

    Component {
        id: sfxComponent

        SoundEffect {}
    }

    function playSoundSource(sourcePath: string, enabled: bool, volume: real): void {
        if (!GlobalConfig.audio.sounds.enabled || !enabled)
            return;

        let sfx = root._sfxCache[sourcePath];
        if (!sfx) {
            sfx = sfxComponent.createObject(root, { source: sourcePath, volume: volume });
            root._sfxCache[sourcePath] = sfx;
        } else {
            sfx.volume = volume;
        }
        sfx.play();
    }

    function playNotification(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/notifications/" + GlobalConfig.audio.sounds.notificationSound), true, GlobalConfig.audio.sounds.notificationVolume);
    }

    function playCameraClick(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/camera_click.wav"), GlobalConfig.audio.sounds.cameraClick, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playChargingStarted(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/ChargingStarted.wav"), GlobalConfig.audio.sounds.chargingStarted, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playEffectTick(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/Effect_Tick.wav"), GlobalConfig.audio.sounds.effectTick, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playLock(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/Lock.wav"), GlobalConfig.audio.sounds.lock, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playUnlock(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/Unlock.wav"), GlobalConfig.audio.sounds.unlock, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playLowBattery(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/LowBattery.wav"), GlobalConfig.audio.sounds.lowBattery, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playVideoRecord(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/VideoRecord.wav"), GlobalConfig.audio.sounds.screenRecord, GlobalConfig.audio.sounds.sfxVolume);
    }

    function playVideoStop(): void {
        playSoundSource(Qt.resolvedUrl("../assets/sounds/VideoStop.wav"), GlobalConfig.audio.sounds.screenRecord, GlobalConfig.audio.sounds.sfxVolume);
    }

    function refreshNodes(): void {
        const newStreams = [];
        const newAppStreams = [];
        const seenApps = new Set();
        const seenSinks = new Map();
        const seenSources = new Map();

        for (const node of Pipewire.nodes.values) {
            if (!node.isStream) {
                if (node.isSink) {
                    if (true) {
                        if (!seenSinks.has(node.name)) {
                            seenSinks.set(node.name, node);
                        } else {
                            const existing = seenSinks.get(node.name);
                            if (node === Pipewire.defaultAudioSink || (existing !== Pipewire.defaultAudioSink && node.id > existing.id)) {
                                seenSinks.set(node.name, node);
                            }
                        }
                    }
                }
                else if (node.audio) {
                    if (true) {
                        if (!seenSources.has(node.name)) {
                            seenSources.set(node.name, node);
                        } else {
                            const existing = seenSources.get(node.name);
                            if (node === Pipewire.defaultAudioSource || (existing !== Pipewire.defaultAudioSource && node.id > existing.id)) {
                                seenSources.set(node.name, node);
                            }
                        }
                    }
                }
            } else if (node.audio) {
                newStreams.push(node);

                const name = getStreamName(node);
                if (name === root.selfAppName)
                    continue;
                if (!seenApps.has(name)) {
                    seenApps.add(name);
                    newAppStreams.push(node);
                }
            }
        }

        root.appStreams = newAppStreams;
        root.sinks = [...seenSinks.values()];
        root.sources = [...seenSources.values()];
        root.streams = newStreams;
    }

    onSinkChanged: {
        if (!sink?.ready)
            return;

        const newSinkName = root.getNodeDisplayName(sink);

        if (previousSinkName && previousSinkName !== newSinkName && GlobalConfig.utilities.toasts.audioOutputChanged)
            Toaster.toast(qsTr("Audio output changed"), qsTr("Now using: %1").arg(newSinkName), "volume_up");

        previousSinkName = newSinkName;
    }

    onSourceChanged: {
        if (!source?.ready)
            return;

        const newSourceName = root.getNodeDisplayName(source);

        if (previousSourceName && previousSourceName !== newSourceName && GlobalConfig.utilities.toasts.audioInputChanged)
            Toaster.toast(qsTr("Audio input changed"), qsTr("Now using: %1").arg(newSourceName), "mic");

        previousSourceName = newSourceName;
    }

    Component.onCompleted: {
        refreshNodes();
        previousSinkName = root.getNodeDisplayName(sink);
        previousSourceName = root.getNodeDisplayName(source);

        try {
            root.cava = Qt.createQmlObject(
                'import Caelestia.Config\nimport Caelestia.Services\nCavaProvider { bars: GlobalConfig.services.visualiserBars }',
                root, "CavaProviderDynamic");
        } catch (e) {
            console.warn("Caelestia: CavaProvider unavailable, visualiser disabled:", e);
        }
    }

    Connections {
        function onValuesChanged(): void {
            root.refreshNodes();
        }

        target: Pipewire.nodes
    }

    PwObjectTracker {
        objects: [root.sink, root.source, ...root.sinks, ...root.sources, ...root.streams].filter(n => n)
    }

    BeatTracker {
        id: beatTracker
    }

    IpcHandler {
        function cycleOutput(): void {
            root.cycleNextAudioOutput();
        }

        target: "audio"
    }
}
