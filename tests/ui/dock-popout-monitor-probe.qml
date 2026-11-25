import QtQuick
import Quickshell
import Ryoku.Blobs
import shell.services
import "modules/bar/popouts" as BarPopouts

// Probe: the DockPreviewPopout on each monitor must only show the strip when
// that monitor's dock owns the hover. The DockPreview singleton now carries
// `monitorName` so only the popout on that output opens; the other stays closed.
// The popouts are built at runtime instead of being declared inline: destroy()
// is only supported on objects the engine created from a Component, and the
// probe has to destroy a strip to prove its hold dies with it.
ShellRoot {
    id: root

    readonly property real span: 800
    readonly property real tall: 600

    BlobGroup { id: probeGroup }

    // The popouts under test, created by the root timer.
    property var stripA: null
    property var stripB: null
    property var cardA: null
    property var cardB: null

    Item {
        id: stage
        width: root.span
        height: root.tall
    }

    Component {
        id: stripComponent
        BarPopouts.DockPreviewPopout {
            group: probeGroup
            frameThickness: 16
        }
    }

    Component {
        id: cardComponent
        BarPopouts.MusicHoverPopout {
            group: probeGroup
            frameThickness: 16
        }
    }

    QtObject {
        id: probeLogic
        property int failures: 0
        property int checksTotal: 16
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

    // Wait helper - at ShellRoot level, not inside stage
    Timer {
        id: waitTimer
        property var then: null
        interval: 0
        repeat: false
        onTriggered: {
            const next = waitTimer.then;
            waitTimer.then = null;
            if (next) next();
        }
    }
    function wait(ms, then) {
        waitTimer.interval = ms;
        waitTimer.then = then;
        waitTimer.restart();
    }

    Timer {
        id: rootTimer
        interval: 200
        running: true
        repeat: false
        onTriggered: {
            // Build the popouts under test, one per monitor.
            root.stripA = stripComponent.createObject(stage, { monitorName: "probe-a" });
            root.stripB = stripComponent.createObject(stage, { monitorName: "probe-b" });
            root.cardA = cardComponent.createObject(stage, { monitorName: "probe-a" });
            root.cardB = cardComponent.createObject(stage, { monitorName: "probe-b" });

            // Arm the hover as the owner of "probe-a"
            DockPreview.monitorName = "probe-a";
            DockPreview.hoveredClass = "firefox";

            // Scoping checks 1-4 (the existing four)
            probeLogic.check("strip on hovered output owns hover", stripA.ownsHover === true);
            probeLogic.check("strip on other output does not own hover", stripB.ownsHover === false);
            probeLogic.check("hovered strip shows hovered class", stripA.cls === "firefox");
            probeLogic.check("other strip shows nothing", stripB.cls === "");

            // Scoping checks 5-7: hover handover and clears
            // 5. DockPreview.monitorName = "probe-b"; then stripB.ownsHover === true && stripA.ownsHover === false
            DockPreview.monitorName = "probe-b";
            probeLogic.check("hover hands over to other output", stripB.ownsHover === true && stripA.ownsHover === false);

            // 6. DockPreview.clearHover("probe-a"); then DockPreview.hoveredClass === "firefox"
            DockPreview.clearHover("probe-a");
            probeLogic.check("stale clear from old output does not erase new hover", DockPreview.hoveredClass === "firefox");

            // 7. DockPreview.clearHover("probe-b"); then DockPreview.hoveredClass === ""
            DockPreview.clearHover("probe-b");
            probeLogic.check("owner clears its own hover", DockPreview.hoveredClass === "");

            // MUSIC SCOPING CHECKS 8-11
            // 8. Set monitorName to "probe-a", hover true, cardA owns the hover
            // The binding re-evaluates synchronously on the imperative assignment below;
            // check 8 would read a stale value if the assignment had not just occurred.
            MusicPreview.monitorName = "probe-a";
            MusicPreview.hovered = true;
            probeLogic.check("card on hovered output owns hover", cardA.ownsHover === true && cardA.musicHovered === true);
            // 9. cardB on other output does not own hover
            probeLogic.check("card on other output does not own hover", cardB.ownsHover === false && cardB.musicHovered === false);
            // 10. card's hold is per output
            MusicPreview.holds.set("probe-a", true);
            probeLogic.check("card's hold is per output", MusicPreview.holds.has("probe-a") === true && MusicPreview.holds.has("probe-b") === false);
            // 11. card's hold clears
            MusicPreview.holds.set("probe-a", false);
            probeLogic.check("card's hold clears", MusicPreview.holds.has("probe-a") === false);
            // Reset music state so it does not leak into the hold phases below
            MusicPreview.hovered = false;

            // PHASE A: re-arm the hover as the owner of "probe-a"
            DockPreview.monitorName = "probe-a";
            DockPreview.hoveredClass = "firefox";
            // Wait 1400 ms for the strip to open and publish its hold
            wait(1400, function() {
                // check: the strip publishes its hold for its own output
                probeLogic.check("the strip publishes its hold for its own output", DockPreview.holds.has("probe-a") === true);
                // check: the strip on the other output holds nothing
                probeLogic.check("the strip on the other output holds nothing", DockPreview.holds.has("probe-b") === false);

                // PHASE B: clear the hover AS THE OWNER ("probe-a")
                // Per the brief: after the strip's close grace (1200 ms)
                DockPreview.clearHover("probe-a");
                // Wait 1400 ms (covers the close grace)
                wait(1400, function() {
                    // check: the hold clears when the strip closes
                    probeLogic.check("the hold clears when the strip closes", DockPreview.holds.has("probe-a") === false);

                    // PHASE C: re-arm the hover as the owner of "probe-a"
                    DockPreview.monitorName = "probe-a";
                    DockPreview.hoveredClass = "firefox";
                    // Wait 2000 ms for the hold to be published
                    wait(2000, function() {
                        // check: the strip publishes its hold again
                        probeLogic.check("the strip publishes its hold again", DockPreview.holds.has("probe-a") === true);
                        // Destroy the strips and the cards, then wait for destruction to propagate
                        stripA.destroy();
                        stripB.destroy();
                        cardA.destroy();
                        cardB.destroy();
                        // wait 2000 ms for destruction propagation
                        wait(2000, function() {
                            // check: a destroyed strip leaves no hold behind
                            probeLogic.check("a destroyed strip leaves no hold behind", DockPreview.holds.has("probe-a") === false);
                        });
                    });
                });
            });
        }
    }
}
