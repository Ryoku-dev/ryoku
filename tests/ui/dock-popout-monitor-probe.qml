import QtQuick
import Quickshell
import Ryoku.Blobs
import shell.services
import "modules/bar/popouts" as BarPopouts

// Probe: the DockPreviewPopout on each monitor must only show the strip when
// that monitor's dock owns the hover. The DockPreview singleton now carries
// `monitorName` so only the popout on that output opens; the other stays closed.
ShellRoot {
    id: root

    readonly property real span: 800
    readonly property real tall: 600

    BlobGroup { id: probeGroup }

    Item {
        id: stage
        width: root.span
        height: root.tall

        BarPopouts.DockPreviewPopout {
            id: stripA
            monitorName: "probe-a"
            group: probeGroup
            frameThickness: 16
        }

        BarPopouts.DockPreviewPopout {
            id: stripB
            monitorName: "probe-b"
            group: probeGroup
            frameThickness: 16
        }
    }

    QtObject {
        id: probeLogic
        property int failures: 0
        function check(name, cond) {
            try {
                if (!cond) {
                    console.log("FAIL " + name);
                    probeLogic.failures++;
                } else {
                    console.log("ok   " + name);
                }
            } catch (e) {
                console.log("FAIL " + name + " (threw: " + e.message + ")");
                probeLogic.failures++;
            }
        }
    }

    Timer {
        interval: 200
        running: true
        repeat: false
        onTriggered: {
            // 1. DockPreview.monitorName = "probe-a"; DockPreview.hoveredClass = "firefox";
            //    then stripA.ownsHover === true ("the strip on the hovered output owns the hover")
            DockPreview.monitorName = "probe-a";
            DockPreview.hoveredClass = "firefox";

            // Give it a tick to propagate
            Qt.callLater(function() {
                probeLogic.check("strip on hovered output owns hover", stripA.ownsHover === true);
                probeLogic.check("strip on other output does not own hover", stripB.ownsHover === false);
                probeLogic.check("hovered strip shows hovered class", stripA.cls === "firefox");
                probeLogic.check("other strip shows nothing", stripB.cls === "");

                // 5. DockPreview.monitorName = "probe-b"; then stripB.ownsHover === true && stripA.ownsHover === false
                DockPreview.monitorName = "probe-b";
                Qt.callLater(function() {
                    probeLogic.check("hover hands over to other output", stripB.ownsHover === true && stripA.ownsHover === false);

                    // 6. DockPreview.clearHover("probe-a"); then DockPreview.hoveredClass === "firefox"
                    DockPreview.clearHover("probe-a");
                    Qt.callLater(function() {
                        probeLogic.check("stale clear from old output does not erase new hover", DockPreview.hoveredClass === "firefox");

                        // 7. DockPreview.clearHover("probe-b"); then DockPreview.hoveredClass === ""
                        DockPreview.clearHover("probe-b");
                        Qt.callLater(function() {
                            probeLogic.check("owner clears its own hover", DockPreview.hoveredClass === "");

                            if (probeLogic.failures === 0) {
                                console.log("DOCK-POPOUT-MONITOR-PROBE-PASS");
                                Qt.exit(0);
                            } else {
                                console.log("DOCK-POPOUT-MONITOR-PROBE-FAIL " + probeLogic.failures);
                                Qt.exit(1);
                            }
                        });
                    });
                });
            });
        }
    }
}
