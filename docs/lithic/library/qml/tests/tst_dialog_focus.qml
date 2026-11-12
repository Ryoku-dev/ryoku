import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 900
    height: 700

    Dialog {
        id: stepper
        property int step: 0
        title: "Tools for hacker"
        parent: host
        actions: stepper.step === 0 ? [{ text: "Apply", kind: "gold", closeOnTrigger: false }] : [{ text: "Done", kind: "gold" }]
        onActionTriggered: index => { if (stepper.step === 0) stepper.step = 1 }
        Column {
            width: parent.width
            Plaque {
                id: retry
                visible: stepper.step === 0
                kind: "quiet"
                text: "Retry"
            }
        }
    }

    Plaque {
        id: pageAdd
        x: 20
        y: 20
        text: "Add a fingerprint"
        onClicked: enroll.open()
    }

    Dialog {
        id: enroll
        property bool done: false
        title: "Scan your finger"
        parent: host
        actions: enroll.done ? [{ text: "Done", kind: "gold" }]
                             : [{ text: "Stop", kind: "quiet", closeOnTrigger: false }]
        Label {
            text: "Touch the sensor"
        }
    }

    SignalSpy { id: retries; target: retry; signalName: "clicked" }
    SignalSpy { id: closes; target: stepper; signalName: "closed" }
    SignalSpy { id: adds; target: pageAdd; signalName: "clicked" }
    SignalSpy { id: enrollCloses; target: enroll; signalName: "closed" }

    TestCase {
        name: "DialogFocusAfterChange"
        when: windowShown

        function init() {
            retries.clear()
            closes.clear()
            stepper.step = 0
            stepper.open()
            tryCompare(stepper, "opened", true)
        }

        function cleanup() {
            stepper.close()
            tryCompare(stepper, "visible", false)
        }

        // the page's case: Retry pressed, the job succeeds, Retry hides; Return must not press it again
        function test_hiddenPlaqueIgnoresReturn() {
            retry.forceActiveFocus(Qt.TabFocusReason)
            stepper.step = 1
            compare(retry.visible, false)
            keyClick(Qt.Key_Return)
            compare(retries.count, 0, "a hidden plaque acted on Return")
        }

        // the step changed the actions; Return should reach the new first control (Done), as it does on open
        function test_returnReachesNewAction() {
            retry.forceActiveFocus(Qt.TabFocusReason)
            stepper.step = 1
            wait(50)
            keyClick(Qt.Key_Return)
            tryCompare(closes, "count", 1, 1000, "Return reached nothing; focus is on " + host.Window.activeFocusItem)
        }
    }

    TestCase {
        name: "DialogFocusStaysInside"
        when: windowShown

        // the focused action is rebuilt away while the dialog is open (enrolment finished):
        // focus must stay in the dialog, so Return reaches Done, never the page underneath
        function test_focusStaysInsideWhenTheFocusedActionGoes() {
            enroll.done = false
            pageAdd.forceActiveFocus(Qt.TabFocusReason)
            adds.clear()
            enrollCloses.clear()
            keyClick(Qt.Key_Return)
            tryCompare(enroll, "opened", true)
            compare(adds.count, 1)
            enroll.done = true
            wait(100)
            verify(host.Window.activeFocusItem.visualFocus, "the new first action carries the keyboard mark")
            keyClick(Qt.Key_Return)
            tryCompare(enrollCloses, "count", 1, 1000, "Return reached " + host.Window.activeFocusItem + ", not Done")
            compare(adds.count, 1, "the page's plaque under the dialog was pressed")
        }
    }
}
