import QtQuick
import QtTest
import Lithic

// one Tab stop per stepper: Tab from a stepper goes to the next control, editable or not
Item {
    id: host
    width: 500
    height: 300

    Stepper { id: a; x: 10; y: 10; from: 0; to: 10; value: 4 }
    Stepper { id: b; x: 10; y: 80; from: 0; to: 10; value: 4 }
    Stepper { id: c; x: 10; y: 150; from: 0; to: 10; value: 4; editable: true }
    Stepper { id: d; x: 10; y: 220; from: 0; to: 10; value: 4; editable: true }

    TestCase {
        name: "StepperTabStops"
        when: windowShown

        function holder(item) {
            for (var p = host.Window.activeFocusItem; p; p = p.parent)
                if (p === item)
                    return true;
            return false;
        }

        function test_tabLeavesANonEditableStepperInOneStep() {
            a.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Tab);
            verify(holder(b), "one Tab from a non-editable stepper reaches the next one, not a second stop inside the first");
        }

        function test_tabLeavesAnEditableStepperInOneStep() {
            c.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Tab);
            verify(holder(d), "one Tab from an editable stepper reaches the next one");
        }
    }
}
