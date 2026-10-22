pragma Singleton

import ".."
import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.services
import caelestia.utils

Searcher {
    id: root

    signal loaded()

    function transformSearch(search: string): string {
        return search.slice(`${GlobalConfig.launcher.actionPrefix}animations `.length);
    }

    Process {
        id: getAnimationsProc

        running: false
        command: ["sh", "-c", "ls -1 ~/.config/caelestia/animations/*.lua || true"]
        stdout: StdioCollector {
            onStreamFinished: {
                let lines = text.trim().split("\n").filter(l => l.length > 0);
                
                const result = [];
                
                if (lines.length > 0) {
                    result.push({
                        name: "Default (None)",
                        path: "default"
                    });
                }

                for (let file of lines) {
                    let parts = file.split("/");
                    let filename = parts[parts.length - 1];
                    let name = filename.replace(".lua", "");
                    
                    name = name.charAt(0).toUpperCase() + name.slice(1);
                    
                    result.push({
                        name: name,
                        path: file
                    });
                }
                anims.model = result;
                root.loaded();
            }
        }
    }

    list: anims.instances
    useFuzzy: true

    Variants {
        id: anims
        
        QtObject {
            id: animItem
            required property var modelData
            
            readonly property string name: modelData.name
            readonly property string path: modelData.path
            
            function onClicked(list: var): void {
                if (list && list.visibilities) {
                    list.visibilities.launcher = false;
                }

                console.warn("Animations: animation switching is not supported on KDE");
            }
        }
    }
}
