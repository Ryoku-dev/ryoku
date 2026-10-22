pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// The desktop's shortcuts as the Hub lists them (`ryoku-hub keybinds`): the
// shared catalogue with the user's rebinds applied, for whichever compositor runs.
Singleton {
    id: root

    property var keybinds: []
    property bool initialized: false

    signal loaded

    function loadKeybinds(): void {
        if (!reader.running)
            reader.running = true;
    }

    function query(searchText: string): var {
        if (!root.initialized)
            root.loadKeybinds();
        const q = searchText.trim().toLowerCase();
        if (q === "")
            return root.keybinds;
        return root.keybinds.filter(k => k.bind.toLowerCase().includes(q) || k.description.toLowerCase().includes(q));
    }

    Process {
        id: reader
        command: ["ryoku-hub", "keybinds"]
        stdout: StdioCollector {
            onStreamFinished: {
                const out = [];
                try {
                    for (const cat of JSON.parse(text).categories || [])
                        for (const b of cat.binds || [])
                            if (b.chord)
                                out.push({ bind: b.chord, description: b.hint ? `${b.label}: ${b.hint}` : b.label, action: b.id });
                } catch (e) {
                    console.warn("keybinds: could not read ryoku-hub keybinds:", e);
                }
                root.keybinds = out;
                root.initialized = true;
                root.loaded();
            }
        }
    }
}
