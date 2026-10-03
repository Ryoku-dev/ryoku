import QtQuick
import "../"

// Per-wave facade over the shared WaveTick: registers with the tick while
// `running` and republishes the shared phase, so a caller binds
// `wavePhase: waveClock.phase` exactly as it used to bind the property of its
// own infinite NumberAnimation.
QtObject {
    id: clock

    property bool running: false
    property int duration: 3400

    readonly property real phase: clock.running ? WaveTick.phase(clock.duration) : 0.0

    function _sync(): void {
        const i = WaveTick.clients.indexOf(clock);
        if (clock.running && i < 0)
            WaveTick.clients = WaveTick.clients.concat([clock]);
        else if (!clock.running && i >= 0) {
            const next = WaveTick.clients.slice();
            next.splice(i, 1);
            WaveTick.clients = next;
        }
    }

    onRunningChanged: _sync()
    Component.onDestruction: _sync()
}
