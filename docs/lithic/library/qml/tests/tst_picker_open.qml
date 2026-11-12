import QtQuick
import QtTest
import Lithic

// What a Picker does on its very first open, each case on a picker that has
// never opened before: where the list lands, which keys open it, what Escape
// closes, what a screen reader hears, and the same inside a dialog.
Item {
    id: host
    width: 700
    height: 600
    property int hostEscapes: 0
    Keys.onEscapePressed: hostEscapes++

    Picker { id: firstPlace; x: 20; y: 20; width: 240; label: "Refresh rate"; options: ["60 Hz", "120 Hz", "165 Hz"]; currentIndex: 2 }
    Picker { id: byReturn; x: 300; y: 20; width: 200; options: ["Off", "On"]; currentIndex: 0 }
    Picker { id: byAltDown; x: 300; y: 100; width: 200; options: ["Off", "On"]; currentIndex: 0 }
    Picker { id: byUp; x: 300; y: 180; width: 200; options: ["Off", "On"]; currentIndex: 0 }
    Picker { id: escaping; x: 20; y: 180; width: 200; options: ["Off", "On", "Auto"]; currentIndex: 1 }
    Picker { id: named; x: 20; y: 260; width: 220; label: "Scale"; options: ["100%", { label: "125%", glyph: "monitor" }, "150%"]; currentIndex: 0 }

    Dialog {
        id: dialog
        title: "Display"
        parent: host
        actions: [{ text: "Cancel", kind: "quiet", closeOnTrigger: true }]
        Picker { id: inDialog; width: 200; options: ["One", "Two", "Three"]; currentIndex: 0 }
    }

    SignalSpy { id: dialogChoices; target: inDialog; signalName: "activated" }

    TestCase {
        name: "PickerFirstOpen"
        when: windowShown

        // a closing dialog or list still catches a press until its exit ends
        function init() {
            tryVerify(() => !dialog.visible, 2000);
            wait(250);
        }

        // the menu entry drawing text, wherever the overlay put it
        function entry(text) {
            let found = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !found; i++) {
                    const child = item.children[i];
                    if (child.text === text && child.triggered !== undefined && child.visible)
                        found = child;
                    else
                        walk(child);
                }
            })(host.Window.window.contentItem);
            return found;
        }

        function test_firstOpenLandsWhereEveryLaterOpenDoes() {
            const samples = [];
            mouseClick(firstPlace);
            // follow the list through the enter transition, frame by frame
            for (let i = 0; i < 30 && !(firstPlace.opened && samples.length > 3 && samples[samples.length - 1].settled); i++) {
                const item = entry("60 Hz");
                if (item) {
                    const p = item.mapToItem(host, 0, 0);
                    samples.push({ x: p.x, y: p.y, w: item.width, settled: firstPlace.opened });
                }
                wait(16);
            }
            tryCompare(firstPlace, "opened", true);
            wait(400);
            const first = entry("60 Hz").mapToItem(host, 0, 0);
            const firstWidth = entry("60 Hz").width;
            verify(samples.length > 0, "the list was drawn while it entered");
            for (const s of samples) {
                compare(s.x, first.x, "the list never moves sideways while it enters");
                compare(s.w, firstWidth, "the list never changes width while it enters");
                verify(Math.abs(s.y - first.y) <= LithicTokens.size.popoverEnterTravel,
                       "the list only slides the enter travel, from " + s.y + " to " + first.y);
            }
            const field = firstPlace.mapToItem(host, 0, firstPlace.height);
            compare(first.y >= field.y, true, "the list opens below the field");
            keyClick(Qt.Key_Escape);
            tryCompare(firstPlace, "opened", false);
            tryVerify(() => firstPlace.activeFocus, 1000);
            wait(300);
            mouseClick(firstPlace);
            tryCompare(firstPlace, "opened", true);
            wait(400);
            const second = entry("60 Hz").mapToItem(host, 0, 0);
            compare(second.x, first.x);
            compare(second.y, first.y);
            compare(entry("60 Hz").width, firstWidth);
            keyClick(Qt.Key_Escape);
            tryCompare(firstPlace, "opened", false);
        }

        function test_returnOpensFromTheKeys() {
            byReturn.forceActiveFocus();
            keyClick(Qt.Key_Return);
            tryCompare(byReturn, "opened", true);
            verify(host.Window.window.activeFocusItem.visualFocus, "a key open marks the current choice");
            compare(host.Window.window.activeFocusItem.text, "Off");
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            tryCompare(byReturn, "opened", false);
            compare(byReturn.currentIndex, 1);
        }

        function test_altDownOpensAndUpDoesNot() {
            byUp.forceActiveFocus();
            keyClick(Qt.Key_Up);
            wait(50);
            compare(byUp.opened, false, "Up on a closed picker leaves it closed");
            byAltDown.forceActiveFocus();
            keyClick(Qt.Key_Down, Qt.AltModifier);
            tryCompare(byAltDown, "opened", true);
            keyClick(Qt.Key_Escape);
            tryCompare(byAltDown, "opened", false);
        }

        function test_escapeClosesOnlyTheList() {
            host.hostEscapes = 0;
            escaping.forceActiveFocus();
            keyClick(Qt.Key_Space);
            tryCompare(escaping, "opened", true);
            keyClick(Qt.Key_Escape);
            tryCompare(escaping, "opened", false);
            compare(host.hostEscapes, 0, "Escape on the open list closes only the list");
            tryVerify(() => escaping.activeFocus, 1000);
            keyClick(Qt.Key_Escape);
            compare(host.hostEscapes, 1, "Escape on the closed picker goes on to the host");
        }

        function test_namesAreReadOut() {
            compare(named.Accessible.name, "Scale");
            compare(named.Accessible.description, "100%");
            named.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(named, "opened", true);
            for (const label of ["100%", "125%", "150%"]) {
                const item = entry(label);
                verify(item !== null, label + " is in the list");
                compare(item.Accessible.name, label);
            }
            compare(entry("100%").Accessible.checked, true, "the current choice reads as checked");
            keyClick(Qt.Key_Escape);
            tryCompare(named, "opened", false);
        }

        function test_firstOpenInsideADialog() {
            dialog.open();
            tryCompare(dialog, "opened", true);
            mouseClick(inDialog);
            tryCompare(inDialog, "opened", true);
            verify(!host.Window.window.activeFocusItem.visualFocus, "a pointer open shows no keyboard mark");
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            tryCompare(inDialog, "opened", false);
            compare(inDialog.currentIndex, 1, "Down then Return chose the second item");
            compare(dialogChoices.count, 1);
            verify(dialog.opened, "choosing leaves the dialog open");
            tryVerify(() => inDialog.activeFocus, 1000);
            keyClick(Qt.Key_Down);
            tryCompare(inDialog, "opened", true);
            keyClick(Qt.Key_Escape);
            tryCompare(inDialog, "opened", false);
            verify(dialog.opened, "Escape on the open list leaves the dialog open");
            tryVerify(() => inDialog.activeFocus, 1000, "focus returns to the picker");
            keyClick(Qt.Key_Escape);
            tryCompare(dialog, "opened", false);
        }
    }
}
