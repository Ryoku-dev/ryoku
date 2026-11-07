import QtQuick
import Quickshell
import Quickshell.Io

// Wiring probe for the Hub's real FileView + JsonAdapter shell contract,
// writing frame-bar and preserved-sidebar fields into a disposable copy.
ShellRoot {
    id: root
    property string cfgDir: Quickshell.env("RYOKU_TEST_CFG")
    property bool loaded: false
    property var draft: ({})

    readonly property var shellKeys: [
        "language","frameRadius","roundness","frameBorder","frameEnabled",
        "frameSmoothing","frameOpacity","shadowStrength","shadowSize",
        "surfaceColor","osdRadius","osdOpacity","frameBars","barStyle","fontFamily","fontScale","weatherLocation","weatherUnit",
        "sidebarLeftPanes","sidebarRightPanes","sidebarWidth"
    ]

    function adopt() {
        var d = {};
        for (var i = 0; i < shellKeys.length; i++) d[shellKeys[i]] = shellA[shellKeys[i]];
        draft = d; loaded = true;
        console.log("ADOPTED frameBorder=" + d.frameBorder + " frameBars=" + JSON.stringify(d.frameBars));
    }
    function edit(k, v) {
        var d = {}; for (var x in draft) d[x] = draft[x];
        d[k] = v; draft = d;
        console.log("EDIT " + k + "=" + JSON.stringify(v));
    }
    function flush() {
        for (var i = 0; i < shellKeys.length; i++) {
            var k = shellKeys[i];
            if (draft[k] !== undefined) shellA[k] = draft[k];
        }
        cfgShell.writeAdapter();
        console.log("FLUSHED");
    }

    FileView {
        id: cfgShell
        path: root.cfgDir + "/shell.json"
        watchChanges: true
        onFileChanged: reload()
        onLoaded: if (!root.loaded) root.adopt()
        JsonAdapter {
            id: shellA
            property string language: "Auto"
            property real frameRadius: 9
            property real roundness: 10
            property real frameBorder: 59
            property bool frameEnabled: true
            property real frameSmoothing: 8
            property real frameOpacity: 1
            property real shadowStrength: 0.63
            property real shadowSize: 12
            property string surfaceColor: "#0f1115"
            property real osdRadius: 28
            property real osdOpacity: 1
            property string fontFamily: "Space Grotesk"
            property real fontScale: 1.3
            property string weatherLocation: ""
            property string weatherUnit: "auto"
            property var sidebarLeftPanes: ["stash"]
            property var sidebarRightPanes: ["notifications","calendar","media","weather","recording"]
            property real sidebarWidth: 340
            property var frameBars: ({})
            property string barStyle: "orpheus"
        }
    }

    // Drive the probe headlessly: adopt, edit a real, frame bars and set, flush, quit.
    Timer {
        interval: 900; running: true
        onTriggered: {
            if (!root.loaded) { console.log("PROBE-FAIL not loaded"); Qt.quit(); return }
            root.edit("frameBorder", 88);                              // real
            root.edit("frameBars", {
                "version": 1,
                "rails": { "top": { "enabled": true, "size": 38, "reveal": true,
                                      "start": ["tray"], "center": ["clock"], "end": [] },
                           "left": { "enabled": true, "size": 52, "reveal": true,
                                      "top": [], "center": ["dock"], "bottom": [] },
                           "bottom": { "enabled": false, "size": 32, "reveal": true,
                                        "start": [], "center": [], "end": [] },
                           "right": { "enabled": false, "size": 48, "reveal": true,
                                       "top": [], "center": [], "bottom": [] } },
                "menus": {}, "dock": { "pinned": [] }
            });
            root.edit("barStyle", "kairos");
            root.edit("sidebarRightPanes", ["notifications","calendar"]); // set
            root.flush();
            quit.start();
        }
    }
    Timer { id: quit; interval: 700; onTriggered: Qt.quit() }
}
