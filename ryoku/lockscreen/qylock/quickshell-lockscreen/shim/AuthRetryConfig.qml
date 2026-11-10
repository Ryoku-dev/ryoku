import QtQuick
import Quickshell.Io

QtObject {
    id: root

    required property string path
    property bool enabled: false

    function apply(value, readable) {
        enabled = readable && value.trim() === "on"
    }

    property var reader: Process {
        running: true
        command: ["bash", "-c", "[[ -r $1 && -f $1 ]] && cat -- \"$1\"", "qylock-auth-retry", root.path]
        stdout: StdioCollector {
            onStreamFinished: root.apply(text, true)
        }
        onExited: (code) => {
            if (code !== 0)
                root.apply("", false)
        }
    }
}
