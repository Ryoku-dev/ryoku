import QtQuick
import QtTest
import Lithic

// Return in an editable Stepper inside a form (a ListEditor panel's editor): the value is
// taken and the form hears Return, as it does for a single-line TextField
Item {
    id: form
    width: 500
    height: 200
    property int returns: 0
    property int taken: -1
    Keys.onPressed: event => {
        if (event.key !== Qt.Key_Return && event.key !== Qt.Key_Enter)
            return;
        form.taken = pause.value;
        form.returns++;
        event.accepted = true;
    }

    Stepper {
        id: pause
        x: 10
        y: 10
        from: 10
        to: 60000
        stepSize: 10
        value: 100
        editable: true
    }

    TestCase {
        name: "StepperInForm"
        when: windowShown

        function init() {
            pause.value = 100;
            form.returns = 0;
            form.taken = -1;
        }

        function test_typedReturnReachesTheFormWithTheValue() {
            pause.forceActiveFocus();
            keyClick("2");
            keyClick("5");
            keyClick("0");
            keyClick(Qt.Key_Return);
            compare(pause.value, 250, "the typed value is taken");
            compare(form.returns, 1, "one Return in the typed field also reaches the form");
            compare(form.taken, 250, "the form sees the typed value when it hears Return");
        }

        function test_untypedReturnReachesTheForm() {
            pause.forceActiveFocus();
            keyClick(Qt.Key_Up);
            keyClick(Qt.Key_Return);
            compare(form.returns, 1);
            compare(form.taken, 110);
        }
    }
}
