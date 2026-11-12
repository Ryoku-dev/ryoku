import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 500
    height: 300

    Stepper {
        id: port
        x: 10
        y: 10
        from: 1
        to: 65535
        value: 5555
        editable: true
    }

    Stepper {
        id: minutes
        x: 10
        y: 80
        from: 0
        to: 1425
        stepSize: 15
        value: 60
        editable: true
    }

    Stepper {
        id: fixed
        x: 10
        y: 150
        from: 0
        to: 10
        value: 4
    }

    SignalSpy { id: portModified; target: port; signalName: "valueModified" }

    TestCase {
        name: "StepperTyping"
        when: windowShown

        function init() {
            host.forceActiveFocus();
            port.value = 5555;
            minutes.value = 60;
            portModified.clear();
        }

        function typeText(text) {
            for (const ch of text)
                keyClick(ch);
        }

        function test_typedValueCommitsOnReturn() {
            port.forceActiveFocus();
            typeText("37421");
            keyClick(Qt.Key_Return);
            compare(port.value, 37421, "an editable stepper takes a typed value");
            compare(portModified.count, 1);
            compare(port.displayText, "37421", "a port reads without a thousands separator");
        }

        function test_typedValueCommitsWhenFocusLeaves() {
            port.forceActiveFocus();
            typeText("8080");
            minutes.forceActiveFocus();
            compare(port.value, 8080);
        }

        function test_typedValueIsClampedAndStepped() {
            port.forceActiveFocus();
            typeText("99999");
            keyClick(Qt.Key_Return);
            compare(port.value, 65535, "above the bound clamps to it");
            minutes.forceActiveFocus();
            typeText("100");
            keyClick(Qt.Key_Return);
            compare(minutes.value, 105, "a typed value snaps to the step");
        }

        function test_escapeDropsAnEditThenPassesOn() {
            port.forceActiveFocus();
            typeText("12");
            keyClick(Qt.Key_Escape);
            compare(port.value, 5555);
            compare(portModified.count, 0);
            keyClick(Qt.Key_Return);
            compare(port.value, 5555, "the dropped edit is not committed later");
        }

        function test_arrowsStillStepAndTheTextFollows() {
            port.forceActiveFocus();
            keyClick(Qt.Key_Up);
            compare(port.value, 5556);
            keyClick(Qt.Key_Return);
            compare(port.value, 5556, "Return after a step keeps the stepped value");
        }

        function test_nonEditableStepperIgnoresTyping() {
            fixed.forceActiveFocus();
            typeText("9");
            keyClick(Qt.Key_Return);
            compare(fixed.value, 4);
            keyClick(Qt.Key_Up);
            compare(fixed.value, 5);
        }
    }
}
