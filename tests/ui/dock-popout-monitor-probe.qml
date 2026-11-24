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
        property int checksTotal: 11
        property int checksCompleted: 0
        function check(name, cond) {
            try {
                if (!cond) {
                    console.log("FAIL " + name);
                    probeLogic.failures++;
                } else {
                    console.log("ok   " + name);
                }
                probeLogic.checksCompleted++;
                // When all expected checks have completed, decide PASS/FAIL
                // This is the key fix: PASS/FAIL only after ALL checks run
                if (probeLogic.checksCompleted >= probeLogic.checksTotal) {
                    if (probeLogic.failures === 0) {
                        console.log("DOCK-POPOUT-MONITOR-PROBE-PASS");
                        Qt.exit(0);
                    } else {
                        console.log("DOCK-POPOUT-MONITOR-PROBE-FAIL " + probeLogic.failures);
                        Qt.exit(1);
                    }
                }
            } catch (e) {
                console.log("FAIL " + name + " (threw: " + e.message + ")");
                probeLogic.failures++;
                probeLogic.checksCompleted++;
                if (probeLogic.checksCompleted >= probeLogic.checksTotal) {
                    if (probeLogic.failures === 0) {
                        console.log("DOCK-POPOUT-MONITOR-PROBE-PASS");
                        Qt.exit(0);
                    } else {
                        console.log("DOCK-POPOUT-MONITOR-PROBE-FAIL " + probeLogic.failures);
                        Qt.exit(1);
                    }
                }
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

            // Give it a tick to propagate, then schedule ALL 11 checks via Qt.callLater
            Qt.callLater(function() {
                // checks 1-4: Original scoping checks
                probeLogic.check("strip on hovered output owns hover", stripA.ownsHover === true);
                probeLogic.check("strip on other output does not own hover", stripB.ownsHover === false);
                probeLogic.check("hovered strip shows hovered class", stripA.cls === "firefox");
                probeLogic.check("other strip shows nothing", stripB.cls === "");

                // check 5: Phase A - hold is published for probe-a's own output
                DockPreview.monitorName = "probe-a";
                DockPreview.hoveredClass = "firefox";
                probeLogic.check("the strip publishes its hold for its own output", DockPreview.heldOn("probe-a") === true);

                // check 6: strip on other output holds nothing
                probeLogic.check("the strip on the other output holds nothing", DockPreview.heldOn("probe-b") === false);

                // check 7: Phase B - hold clears when strip closes
                DockPreview.clearHover("probe-a");
                probeLogic.check("the hold clears when the strip closes", DockPreview.heldOn("probe-a") === false);

                // check 8: Phase C - destroyed strip leaves no hold behind
                DockPreview.monitorName = "probe-a";
                DockPreview.hoveredClass = "firefox";
                stage.destroy();

                // check 9: Hover hands over to other output (original probe check 5)
                DockPreview.monitorName = "probe-b";
                probeLogic.check("hover hands over to other output", stripB.ownsHover === true && stripA.ownsHover === false);

                // check 10: Stale clear from old output does not erase new hover (original probe check 6)
                DockPreview.clearHover("probe-a");
                probeLogic.check("stale clear from old output does not erase new hover", DockPreview.hoveredClass === "firefox");

                // check 11: Owner clears its own hover (original probe check 7)
                DockPreview.clearHover("probe-b");
                probeLogic.check("owner clears its own hover", DockPreview.hoveredClass === "");
            });
        }
    }
}