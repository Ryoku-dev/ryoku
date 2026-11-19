import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons

QtObject {
    id: root

    property bool enabled: false
    readonly property bool ready: state.bytes.length > state.index
    property string error: ""
    property var state: ({ bytes: [], index: 0 })
    signal poolReady()

    function options(base) {
        const result = Object.assign({}, base ?? {});
        result._randomState = root.state;
        root.state = ({ bytes: [], index: 0 });
        return result;
    }

    function replenishSoon() {
        if (enabled && root.state.bytes.length - root.state.index < 16384 && !randomProcess.running)
            randomProcess.running = true;
    }
    onEnabledChanged: if (enabled) replenishSoon()

    property Process randomProcess: Process {
        id: randomProcess
        running: false
        command: ["od", "-An", "-v", "-t", "u1", "-N", "32768", "/dev/urandom"]
        stdout: StdioCollector {
            id: randomOutput
            onStreamFinished: {
                const tokens = text.trim().split(/\s+/);
                const bytes = [];
                for (const token of tokens) {
                    const value = Number(token);
                    if (!Number.isInteger(value) || value < 0 || value > 255) {
                        root.error = I18n.tr("Secure random source returned invalid data");
                        return;
                    }
                    bytes.push(value);
                }
                if (bytes.length !== 32768) {
                    root.error = I18n.tr("Secure random source returned incomplete data");
                    return;
                }
                root.state = ({ bytes: bytes, index: 0 });
                root.error = "";
                root.poolReady();
            }
        }
        onExited: (exitCode, exitStatus) => {
            if (exitCode !== 0 && !root.ready)
                root.error = I18n.tr("Secure random source is unavailable");
        }
    }
}
