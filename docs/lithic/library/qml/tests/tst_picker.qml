import QtQuick
import QtTest
import Lithic

Item {
    width: 600
    height: 400

    Picker {
        id: picker
        x: 20
        y: 20
        width: 240
        options: ["Left", "Middle", "Right"]
        currentIndex: 0
    }

    SignalSpy {
        id: activations
        target: picker
        signalName: "activated"
    }

    TestCase {
        name: "PickerKeys"
        when: windowShown

        function init() {
            if (picker.opened)
                picker.close();
            tryCompare(picker, "opened", false);
            activations.clear();
        }

        function test_arrowsAndReturnChoose() {
            picker.currentIndex = 0;
            picker.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(picker, "opened", true);
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            compare(activations.count, 1);
            compare(picker.currentIndex, 2);
            tryCompare(picker, "opened", false);
        }

        function test_startsOnTheCurrentChoice() {
            picker.currentIndex = 1;
            picker.forceActiveFocus();
            keyClick(Qt.Key_Space);
            tryCompare(picker, "opened", true);
            keyClick(Qt.Key_Return);
            compare(activations.count, 1);
            compare(picker.currentIndex, 1);
        }

        function test_escapeKeepsTheValue() {
            picker.currentIndex = 1;
            picker.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(picker, "opened", true);
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Escape);
            tryCompare(picker, "opened", false);
            compare(activations.count, 0);
            compare(picker.currentIndex, 1);
        }

        function test_homeEndAndEnter() {
            picker.currentIndex = 1;
            picker.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(picker, "opened", true);
            keyClick(Qt.Key_End);
            keyClick(Qt.Key_Enter);
            compare(picker.currentIndex, 2);
            tryCompare(picker, "opened", false);
            tryVerify(() => picker.activeFocus, 1000);
            keyClick(Qt.Key_Down);
            tryCompare(picker, "opened", true);
            keyClick(Qt.Key_Home);
            keyClick(Qt.Key_Return);
            compare(picker.currentIndex, 0);
            compare(activations.count, 2);
        }

        function test_keyboardMarkFollowsTheKeys() {
            picker.currentIndex = 1;
            picker.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(picker, "opened", true);
            verify(picker.Window.activeFocusItem.visualFocus, "opened from the keys, the current item shows the mark");
            keyClick(Qt.Key_Escape);
            tryCompare(picker, "opened", false);
            tryVerify(() => picker.activeFocus, 1000);

            mouseClick(picker);
            tryCompare(picker, "opened", true);
            verify(!picker.Window.activeFocusItem.visualFocus, "opened with the pointer, no keyboard mark");
            keyClick(Qt.Key_Down);
            verify(picker.Window.activeFocusItem.visualFocus, "the first key move shows the mark");
            keyClick(Qt.Key_Escape);
            tryCompare(picker, "opened", false);
        }
    }
}
