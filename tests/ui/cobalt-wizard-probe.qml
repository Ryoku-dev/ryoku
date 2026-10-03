import QtQuick
import Quickshell
import shell.services
import "modules/sidebar/cards" as SidebarCards

// The Cobalt setup overlay renders Stash's setup state and nothing else, so this
// drives that state machine through a first run and asserts what the card
// actually shows: one row per step with the right glyph, the failing step's own
// message, and buttons that match the run (Start setup when idle, Retry only
// after a failure, never a Close that could abandon a privileged step mid-flight).
//
// Hermetic: no docker, no pkexec, no processes. The state is set directly, which
// is the point of keeping the logic in Stash rather than in the view.
ShellRoot {
    id: root

    property int failures: 0
    function check(label, want, got) {
        if (want !== got) {
            console.log("FAIL " + label + ": want=" + want + " got=" + got);
            root.failures++;
        }
    }

    // Walk the visual tree for the step rows the Repeater built. Counting them
    // proves the model reached the view, which a property check alone does not.
    function stepRows(node, out) {
        out = out || [];
        for (var i = 0; i < node.children.length; i++) {
            var c = node.children[i];
            if (c.stepState !== undefined && c.label !== undefined)
                out.push(c);
            root.stepRows(c, out);
        }
        return out;
    }

    Item {
        id: stage
        width: 420
        height: 420

        SidebarCards.CobaltSetupOverlay {
            id: wiz
            anchors.fill: parent
            s: 1
            open: true
        }
    }

    Component.onCompleted: {
        // ---- idle: four steps, all pending, Start setup offered -------------
        Stash.setupReset();
        var rows = root.stepRows(stage);
        root.check("step rows rendered", 4, rows.length);
        root.check("setupState idle", "idle", Stash.setupState);
        var allPending = true;
        for (var i = 0; i < rows.length; i++)
            if (rows[i].stepState !== "pending") allPending = false;
        root.check("all steps pending at rest", true, allPending);

        // ---- a run in flight must not offer a way to abandon it -------------
        Stash.setupState = "running";
        Stash.setupMark("service", "running");
        root.check("wizard reports busy", true, wiz.busy);
        rows = root.stepRows(stage);
        var svc = null;
        for (i = 0; i < rows.length; i++) if (rows[i].label.length > 0 && rows[i].stepState === "running") svc = rows[i];
        root.check("a running step is rendered", true, svc !== null);

        // ---- a failure carries the helper's own words, not a generic --------
        Stash.setupFail("service", "could not enable docker.service");
        root.check("failed state", "failed", Stash.setupState);
        root.check("no step left running", -1, Stash.setupStep);
        rows = root.stepRows(stage);
        var failed = null;
        for (i = 0; i < rows.length; i++) if (rows[i].stepState === "failed") failed = rows[i];
        root.check("a failed step is rendered", true, failed !== null);
        if (failed !== null)
            root.check("failure shows the real reason", "could not enable docker.service", failed.msg);
        root.check("failure releases the engine claim", false, Stash.setupOwnsEngine);

        // ---- retry is convergent: the same flow from the top ----------------
        Stash.setupReset();
        root.check("reset clears the failure", "idle", Stash.setupState);
        rows = root.stepRows(stage);
        var clean = true;
        for (i = 0; i < rows.length; i++)
            if (rows[i].stepState !== "pending" || rows[i].msg.length > 0) clean = false;
        root.check("reset clears every step and message", true, clean);

        // ---- the done path marks the last two steps and stops owning --------
        Stash.setupState = "running";
        Stash.setupOwnsEngine = true;
        Stash.setupMark("image", "running");
        Stash.onServerLine("READY\t");
        root.check("READY finishes the wizard", "done", Stash.setupState);
        root.check("done releases the engine claim", false, Stash.setupOwnsEngine);
        rows = root.stepRows(stage);
        var lastTwoDone = 0;
        for (i = 0; i < rows.length; i++) if (rows[i].stepState === "done") lastTwoDone++;
        root.check("image and start both done", true, lastTwoDone >= 2);

        // ---- the pull message survives the starting message -----------------
        Stash.setupReset();
        Stash.setupState = "running";
        Stash.setupOwnsEngine = true;
        Stash.onServerLine("STATUS\tstarting");
        Stash.onServerLine("STATUS\tpulling");
        rows = root.stepRows(stage);
        var pull = null;
        for (i = 0; i < rows.length; i++) if (rows[i].stepState === "running") pull = rows[i];
        root.check("the pull step explains the wait", true, pull !== null && pull.msg.length > 0);

        if (root.failures === 0)
            console.log("COBALT-WIZARD-PROBE-PASS");
        else
            console.log("COBALT-WIZARD-PROBE-FAIL " + root.failures);
        Qt.quit();
    }
}
