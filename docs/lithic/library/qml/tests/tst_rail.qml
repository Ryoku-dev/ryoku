import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 1280
    height: 800

    AppFrame {
        id: frame
        anchors.fill: parent
        destinations: [{ label: "Appearance", glyph: "palette" }, { label: "Desktop", glyph: "dashboard" },
                       { label: "Updates", glyph: "system_update" }]
        showNew: false
        footIndex: 2
        compactBar: false
        Item { anchors.fill: parent }
    }

    function ringOf(item) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (kids[i].keyboardFocusVisible !== undefined && kids[i].outset !== undefined)
                return kids[i];
            const inner = ringOf(kids[i]);
            if (inner)
                return inner;
        }
        return null;
    }

    TestCase {
        name: "RailFocus"
        when: windowShown

        function test_tabShowsTheRingOnEveryDestination() {
            const seen = [];
            for (let step = 0; step < 6; ++step) {
                keyClick(Qt.Key_Tab);
                const focused = host.Window.activeFocusItem;
                if (!focused || !focused.Accessible.name)
                    continue;
                const ring = host.ringOf(focused.background) || host.ringOf(focused);
                verify(ring !== null, focused.Accessible.name + " has a focus ring");
                verify(ring.visible, "Tab shows the ring on " + focused.Accessible.name);
                seen.push(focused.Accessible.name);
            }
            verify(seen.indexOf("Appearance") >= 0 && seen.indexOf("Updates") >= 0,
                   "Tab reached the destinations: " + seen.join(", "));
        }
    }
}
