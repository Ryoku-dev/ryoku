import QtQuick
import Quickshell
import "modules/desktop" as Desk
import "modules/stage/Singletons" as Stage

// widget-depth-lift-probe: the per-widget `Depth` row (built-in menu, plugin
// tile menu, and the visualiser's own menu scope) lifts the named widget above
// Stage cut-outs and persists the
// choice.
// Loads the real shell components against the mirrored shell tree, the way
// center-popout-probe does. Commit nothing.
ShellRoot {
    id: root

    property int failures: 0
    function check(name, ok) {
        console.log((ok ? "ok  " : "FAIL ") + name);
        if (!ok)
            failures++;
    }

    // Walk the tree for the first descendant exposing `property` === `value`.
    function findByProp(item, property, value) {
        if (!item)
            return null;
        for (var i = 0; i < item.children.length; i++) {
            var c = item.children[i];
            if (c[property] !== undefined && c[property] === value)
                return c;
            var r = findByProp(c, property, value);
            if (r)
                return r;
        }
        return null;
    }
    // Segments carry arrays, so match on shape instead of identity.
    function findDepthSeg(item) {
        if (!item)
            return null;
        for (var i = 0; i < item.children.length; i++) {
            var c = item.children[i];
            if (c.options && c.options.length === 2 && c.options[0] === "Behind"
                && c.current !== undefined)
                return c;
            var r = findDepthSeg(c);
            if (r)
                return r;
        }
        return null;
    }

    readonly property string wall: "/tmp/ryoku-probe-wall.png"
    readonly property string other: "/tmp/ryoku-probe-plain.png"
    readonly property string spatialWall: "/tmp/ryoku-probe-spatial.png"

    // Feed StageBackend the frame the daemon would publish for this wallpaper:
    // depth effect, one subject layer behind the widgets.
    function armStage() {
        var walls = {};
        walls[wall] = {
            effect: "depth", subject: "/tmp/ryoku-probe-sub.png",
            background: "", rev: 1,
            layers: [{ out: "/tmp/ryoku-probe-sub.png", label: "Subject",
                       enabled: true, front: false, depth: 0.5 }]
        };
        walls[spatialWall] = {
            effect: "spatial", subject: "", background: "",
            depth: "/tmp/ryoku-probe-depth.png", rev: 1, layers: []
        };
        Stage.StageBackend.apply(JSON.stringify({
            current: wall, job: null, last: null, walls: walls
        }));
    }

    Item {
        id: stage
        width: 1600
        height: 900

        Desk.WidgetMenu { id: widgetMenu }
        Desk.PluginWidgetMenu { id: pluginMenu }
    }

    function run() {
        armStage();
        check("backend active for the cut wall",
            Stage.StageBackend.isActiveFor(wall) === true);
        check("backend recognizes a cut mode",
            Stage.StageBackend.isCutModeFor(wall) === true);
        check("backend inactive for a plain wall",
            Stage.StageBackend.isActiveFor(other) === false);

        Stage.Config.setFront("depth", "clock", true);
        check("isFront after lift",
            Stage.Config.isFront("depth", "clock") === true);
        Stage.Config.setFront("depth", "clock", true);
        check("lift is idempotent",
            Stage.Config.frontList("depth").length === 1);
        Stage.Config.setFront("parallax", "calendar", true);
        check("parallax list stays separate",
            Stage.Config.isFront("parallax", "clock") === false
            && Stage.Config.isFront("depth", "calendar") === false);

        // The widget menu builds its Depth row only while the wall cuts; the
        // row toggles its own scope and the card stays open.
        widgetMenu.openFor("clock", 10, 10, wall);
        var row = findByProp(widgetMenu, "label", "Depth");
        check("widget menu builds a Depth row", row !== null);
        check("row visible while stage on", row && row.visible === true);
        check("lifted row reads In front", row && row.value === "In front");
        row.triggered();
        check("trigger unlifts",
            Stage.Config.isFront("depth", "clock") === false
                && widgetMenu.lifted === false);
        check("row value follows", row.value === "Behind");
        row.triggered();
        check("trigger re-lifts",
            Stage.Config.isFront("depth", "clock") === true
                && widgetMenu.lifted === true);
        widgetMenu.wall = other;
        check("row hides without a cut", widgetMenu.stageActive === false
            && row.visible === false);
        widgetMenu.wall = spatialWall;
        check("row hides for Spatial", widgetMenu.stageActive === false
            && row.visible === false);

        // The plugin tile menu keys the lift off the tile id.
        pluginMenu.openFor("probe.tile", false, 10, 10,
            { name: "Probe Tile" }, { desktopWidget: { x: 10, y: 10 } }, wall);
        var prow = findByProp(pluginMenu, "label", "Depth");
        check("plugin menu builds a Depth row", prow !== null);
        check("plugin row gated on its wall", prow && prow.visible === true);
        prow.triggered();
        check("plugin trigger lifts the tile id",
            Stage.Config.isFront("depth", "probe.tile") === true);

        // The visualiser wears the same row: its scope opens the built-in
        // menu shape with the visualiser's id, and the lift keys off that id.
        widgetMenu.openFor("visualizer", 10, 10, wall);
        var vrow = findByProp(widgetMenu, "label", "Depth");
        check("visualiser menu builds a Depth row", vrow !== null);
        check("visualiser row gated on its wall", vrow && vrow.visible === true);
        vrow.triggered();
        check("visualiser trigger lifts the look",
            Stage.Config.isFront("depth", "visualizer") === true);
        vrow.triggered();
        check("second trigger drops it back",
            Stage.Config.isFront("depth", "visualizer") === false);

        // Final state: clock + tile lifted, visualizer behind. The coalesced
        // write is asynchronous, so the harness checks the file after exit.
        console.log(failures === 0 ? "PROBE-CHECKS-PASS" : "PROBE-CHECKS-FAIL " + failures);
        finish.restart();
    }

    function checkSavedFront() {
        check("Depth lifts are restored after restart",
            Stage.Config.isFront("depth", "clock")
            && Stage.Config.isFront("depth", "probe.tile")
            && !Stage.Config.isFront("depth", "visualizer"));
        check("Parallax keeps its own list after restart",
            Stage.Config.isFront("parallax", "calendar")
            && !Stage.Config.isFront("parallax", "clock")
            && !Stage.Config.isFront("depth", "calendar"));

        Stage.Config.setFront("depth", "clock", false);
        check("unlifting a restored widget keeps the other lifts",
            !Stage.Config.isFront("depth", "clock")
            && Stage.Config.isFront("depth", "probe.tile")
            && Stage.Config.isFront("parallax", "calendar"));
        finish.restart();
    }
    Timer {
        id: finish
        interval: 800
        onTriggered: {
            console.log(failures === 0
                ? "WIDGET-DEPTH-LIFT-PROBE-PASS"
                : "WIDGET-DEPTH-LIFT-PROBE-FAIL " + failures);
            Qt.exit(failures === 0 ? 0 : 1);
        }
    }

    // Let the watched FileViews (stage.json seed, visualizer.json) settle
    // before the probe starts mutating, so the first setFront lands on top of
    // the loaded adapter rather than racing its load.
    Timer {
        interval: 500
        running: true
        onTriggered: {
            if (Quickshell.env("RYOKU_PROBE_RESTART") === "1")
                root.checkSavedFront();
            else
                root.run();
        }
    }
}
