import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 900
    height: 700

    Plaque {
        id: plaque
        x: 20
        y: 20
        text: "Copy"
    }

    Plaque {
        id: clearPlaque
        x: 200
        y: 20
        text: "Clear history"
        onClicked: confirm.open()
    }

    Tablet {
        id: tablet
        x: 20
        y: 80
        width: 300
        title: "Studio"
    }

    Confirm {
        id: confirm
        title: "Remove all rules?"
        text: "Every rule goes."
        actionText: "Remove all"
    }

    Confirm {
        id: named
        title: "Erase the files?"
        text: "Type the name to erase."
        actionText: "Erase"
        requiredName: "alpha"
    }

    Dialog {
        id: chooser
        title: "Keyboard layout"
        parent: host
        actions: [{ text: "Cancel", kind: "quiet", closeOnTrigger: true }]
        Column {
            width: parent.width
            Picker {
                id: innerPicker
                width: 200
                options: ["One", "Two"]
                currentIndex: 0
            }
            Search {
                id: innerSearch
                width: 300
                focus: true
            }
        }
    }

    Dialog {
        id: growing
        title: "Removing the profile"
        parent: host
        actions: [{ text: "Close", kind: "quiet", closeOnTrigger: true }]
        Item {
            id: growingBody
            width: 10
            implicitHeight: 40
        }
    }

    SignalSpy { id: plaqueClicks; target: plaque; signalName: "clicked" }
    SignalSpy { id: tabletClicks; target: tablet; signalName: "clicked" }
    SignalSpy { id: tabletOpens; target: tablet; signalName: "primaryAction" }
    SignalSpy { id: cancels; target: confirm; signalName: "cancelled" }
    SignalSpy { id: confirms; target: confirm; signalName: "confirmed" }

    TestCase {
        name: "DialogKeys"
        when: windowShown

        function test_returnPressesAPlaque() {
            plaque.busy = false;
            plaque.forceActiveFocus();
            plaqueClicks.clear();
            keyClick(Qt.Key_Return);
            keyClick(Qt.Key_Enter);
            compare(plaqueClicks.count, 2);
            plaque.busy = true;
            keyClick(Qt.Key_Return);
            keyClick(Qt.Key_Space);
            compare(plaqueClicks.count, 2, "a busy plaque swallows the keys");
            plaque.busy = false;
        }

        function test_returnOpensATablet() {
            tablet.forceActiveFocus();
            tabletClicks.clear();
            tabletOpens.clear();
            keyClick(Qt.Key_Return);
            compare(tabletClicks.count, 1);
            compare(tabletOpens.count, 1);
            keyClick(Qt.Key_Space);
            compare(tabletClicks.count, 2, "Space selects");
            compare(tabletOpens.count, 1, "Space does not open");
        }

        function test_confirmStartsOnCancel() {
            cancels.clear();
            confirms.clear();
            confirm.open();
            tryCompare(confirm, "opened", true);
            keyClick(Qt.Key_Return);
            tryCompare(confirm, "opened", false);
            compare(cancels.count, 1, "Return on a fresh confirm cancels");
            compare(confirms.count, 0);

            confirm.open();
            tryCompare(confirm, "opened", true);
            keyClick(Qt.Key_Tab);
            keyClick(Qt.Key_Return);
            tryCompare(confirm, "opened", false);
            compare(confirms.count, 1, "Tab then Return confirms");
        }

        function test_reopenedConfirmStartsOnCancelAgain() {
            confirms.clear();
            confirm.open();
            tryCompare(confirm, "opened", true);
            keyClick(Qt.Key_Tab);
            keyClick(Qt.Key_Return);
            tryCompare(confirm, "opened", false);
            compare(confirms.count, 1, "Tab then Return confirms");
            cancels.clear();
            confirms.clear();
            confirm.open();
            tryCompare(confirm, "opened", true);
            keyClick(Qt.Key_Return);
            tryCompare(confirm, "opened", false);
            compare(confirms.count, 0, "Return on a reopened confirm must not confirm");
            compare(cancels.count, 1, "a reopened confirm starts on Cancel");
        }

        function test_namedConfirmStartsInTheField() {
            named.open();
            tryCompare(named, "opened", true);
            keyClick(Qt.Key_A);
            keyClick(Qt.Key_L);
            compare(named.inputText, "al");
            keyClick(Qt.Key_Escape);
            tryCompare(named, "opened", false);
        }

        function test_dialogKeepsItsOwnFocusAndEscapeCloses() {
            chooser.open();
            tryCompare(chooser, "opened", true);
            verify(innerSearch.activeFocus, "a dialog's own focused field keeps focus");
            keyClick(Qt.Key_X);
            compare(innerSearch.text, "x");
            keyClick(Qt.Key_Escape);
            compare(innerSearch.text, "", "the first Escape clears the search");
            verify(chooser.opened, "and leaves the dialog open");
            keyClick(Qt.Key_Escape);
            tryCompare(chooser, "opened", false);
        }

        function test_escapeAfterAPickerChoiceClosesTheDialog() {
            chooser.open();
            tryCompare(chooser, "opened", true);
            innerPicker.forceActiveFocus();
            keyClick(Qt.Key_Down);
            tryCompare(innerPicker, "opened", true);
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            tryCompare(innerPicker, "opened", false);
            compare(innerPicker.currentIndex, 1);
            tryVerify(() => innerPicker.activeFocus, 1000, "focus returns to the picker once the list has closed");
            keyClick(Qt.Key_Escape);
            tryCompare(chooser, "opened", false);
        }

        function test_dialogTopHoldsWhileTheBodyGrows() {
            growingBody.implicitHeight = 40;
            growing.open();
            tryCompare(growing, "opened", true);
            const top = growing.y;
            const small = growing.height;
            growingBody.implicitHeight = 120;
            tryVerify(() => growing.height >= small + 80, 1000, "the body grew");
            compare(growing.y, top, "a Working line or progress bar appearing does not move the controls");
            growingBody.implicitHeight = host.height;
            tryVerify(() => growing.height > host.height, 1000, "the body outgrew the room");
            verify(growing.y >= LithicTokens.size.dialogMargin, "a dialog taller than the room keeps its top margin");
            verify(growing.y < top, "and rises only when the bottom margin needs it");
            growing.close();
            tryCompare(growing, "opened", false);
            growingBody.implicitHeight = 40;
            growing.open();
            tryCompare(growing, "opened", true);
            compare(growing.y, top, "the next open centres again");
            growing.close();
            tryCompare(growing, "opened", false);
        }

        function test_keyboardOpenMarksTheFirstControl() {
            clearPlaque.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Return);
            tryCompare(confirm, "opened", true);
            verify(host.Window.activeFocusItem.visualFocus, "a Confirm opened from the keys shows the mark on Cancel");
            keyClick(Qt.Key_Escape);
            tryCompare(confirm, "opened", false);
            tryVerify(() => clearPlaque.activeFocus && clearPlaque.visualFocus, 1000,
                      "focus comes back to the opener with the mark after a keyboard open");
            // the pointer case starts from a control without keyboard focus; a click on a control that
            // already has it keeps its focus reason in Qt
            host.forceActiveFocus();
            // the closing dialog swallows pointer presses until its exit ends
            wait(300);
            mouseClick(clearPlaque);
            tryCompare(confirm, "opened", true);
            verify(!host.Window.activeFocusItem.visualFocus, "a pointer open shows no keyboard mark");
            keyClick(Qt.Key_Escape);
            tryCompare(confirm, "opened", false);
            wait(300);
            verify(!host.Window.activeFocusItem.visualFocus, "after a pointer open focus comes back without the mark");
        }
    }
}
