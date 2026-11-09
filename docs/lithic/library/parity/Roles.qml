import QtQuick
import Lithic

Item {
    readonly property var roleNames: ["ground", "stone1", "stone2", "stone3", "stone4", "vein", "fg", "fg2", "fg3", "sel", "onSel", "onSel2", "gold", "onGold", "goldInk", "jade", "lapis", "amber", "jasper", "scrim"]

    function hex(c) {
        var byte = function(v) {
            var s = Math.round(v * 255).toString(16);
            return s.length < 2 ? "0" + s : s;
        };
        return "#" + byte(c.r) + byte(c.g) + byte(c.b) + byte(c.a);
    }

    function report() {
        var out = { source: Lithic.source };
        for (var i = 0; i < roleNames.length; ++i)
            out[roleNames[i]] = hex(Lithic[roleNames[i]]);
        console.log("ROLES " + JSON.stringify(out));
        Qt.quit();
    }

    Connections {
        target: Lithic
        function onPaletteReadyChanged() {
            if (Lithic.paletteReady)
                report();
        }
    }

    Component.onCompleted: if (Lithic.paletteReady) report()
}
