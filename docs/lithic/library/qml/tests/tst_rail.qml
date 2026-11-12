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

    function named(item, name) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (kids[i].Accessible.name === name && kids[i].clicked !== undefined)
                return kids[i];
            const inner = named(kids[i], name);
            if (inner)
                return inner;
        }
        return null;
    }

    function railOf(item) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (kids[i].destinationTriggered !== undefined && kids[i].currentIndex !== undefined)
                return kids[i];
            const inner = railOf(kids[i]);
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
            for (let step = 0; step < 10; ++step) {
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

        function test_railFollowsTheFrameAfterAPick() {
            const rail = host.railOf(frame);
            const desktop = host.named(rail, "Desktop");
            verify(desktop !== null, "the rail has a Desktop destination");
            desktop.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Space);
            compare(frame.currentDestination, 1, "the pick reaches the frame");
            frame.currentDestination = 2;
            compare(rail.currentIndex, 2, "navigation after a pick still moves the rail");
            mouseClick(host.named(rail, "Appearance"));
            compare(frame.currentDestination, 0);
            frame.currentDestination = 1;
            compare(rail.currentIndex, 1, "navigation after a pointer pick moves the rail too");
        }
    }
}
