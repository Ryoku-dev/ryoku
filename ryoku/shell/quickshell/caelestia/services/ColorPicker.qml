pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import caelestia.utils

Item {
    id: root

    Process {
        id: dbusProcess

        command: [
            "dbus-send",
            "--session",
            "--print-reply",
            "--dest=org.kde.KWin",
            "/ColorPicker",
            "org.kde.kwin.ColorPicker.pick"
        ]
        
        stdout: StdioCollector {
            id: outCollector

            onStreamFinished: {
                let text = outCollector.text;
                let match = text.match(/uint32\s+(\d+)/);
                if (match) {
                    let decimalColor = parseInt(match[1], 10);
                    if (decimalColor === 0) return;
                    let hex = (decimalColor & 0x00FFFFFF).toString(16).padStart(6, '0');
                    let colorCode = "#" + hex.toUpperCase();
                    
                    Quickshell.execDetached(["bash", "-c", `echo -n '${colorCode}' | wl-copy`]);
                    
                    Quickshell.execDetached(["notify-send", "Color Picker", `Color ${colorCode} copied to clipboard!`]);
                }
            }
        }
    }

    function pickColor() {
        if (!dbusProcess.running) {
            dbusProcess.running = true;
        }
    }
}
