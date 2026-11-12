import QtQuick
import QtTest
import Lithic

// After a pick, or with nothing typed, the ColorField holds focus itself so
// Ctrl+Z reaches whoever holds it; typing edits the hex; Return commits a valid
// hex and refuses an invalid one; Escape drops an edit, then passes on.
Item {
    id: host
    width: 700
    height: 500
    property int undos: 0
    property int escapes: 0
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Z && (event.modifiers & Qt.ControlModifier))
            undos++
        else if (event.key === Qt.Key_Escape)
            escapes++
    }

    ColorField {
        id: field
        x: 20
        y: 20
        width: 300
        color: "#ce5e32"
        swatches: [{ label: "Accent", color: "#e0b030" }, { label: "Jade", color: "#7ab8a0" }]
    }

    SignalSpy { id: picked; target: field; signalName: "picked" }

    TestCase {
        name: "ColorFieldKeys"
        when: windowShown

        function init() {
            if (field.opened)
                field.close();
            tryCompare(field, "opened", false);
            wait(250);
            field.color = "#ce5e32";
            field.forceActiveFocus(Qt.TabFocusReason);
            host.undos = 0;
            host.escapes = 0;
            picked.clear();
        }

        function focused() {
            return host.Window.window.activeFocusItem;
        }

        function hexField() {
            return field.contentItem.children.find(child => child.errorText !== undefined);
        }

        function type(text) {
            for (const ch of text)
                keyClick(ch);
        }

        function pickJadeFromTheKeys() {
            keyClick(Qt.Key_Tab);
            keyClick(Qt.Key_Space);
            tryCompare(field, "opened", true);
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            tryCompare(field, "opened", false);
            compare(field.hex, "#7ab8a0");
            tryVerify(() => focused() === field, 1000, "after a pick the field itself holds focus");
        }

        function test_keyboardPickThenUndoReachesTheHost() {
            pickJadeFromTheKeys();
            verify(field.visualFocus, "a pick from the keys keeps the keyboard mark");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1, "Ctrl+Z after a pick reaches the app's undo");
        }

        function test_pointerPickThenUndoReachesTheHost() {
            mouseClick(field, field.width - 20, field.height / 2);
            tryCompare(field, "opened", true);
            wait(200);
            const jade = focused().nextItemInFocusChain(true);
            mouseClick(jade);
            tryCompare(field, "opened", false);
            compare(field.hex, "#7ab8a0");
            tryVerify(() => focused() === field, 1000);
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1);
        }

        function test_typingOpensTheHexAndUndoStaysThere() {
            type("#1");
            verify(hexField().activeFocus, "typing moves into the hex field");
            compare(hexField().text, "#1", "the first character starts a new value");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 0, "while editing, Ctrl+Z undoes the typing");
        }

        function test_returnCommitsAValidHex() {
            pickJadeFromTheKeys();
            type("#123456");
            keyClick(Qt.Key_Return);
            compare(field.hex, "#123456");
            compare(picked.count, 2, "the pick and the typed value");
            compare(field.opened, false, "Return never opens the colours");
            compare(hexField().errorText, "");
            verify(focused() === field, "Return hands focus back to the field");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1);
        }

        function test_returnRefusesAnInvalidHex() {
            pickJadeFromTheKeys();
            type("zz");
            keyClick(Qt.Key_Return);
            compare(field.opened, false, "Return never opens the colours");
            compare(field.hex, "#7ab8a0", "the stored value stays");
            compare(picked.count, 1);
            verify(hexField().errorText.length > 0, "the field says the hex is not a colour");
            verify(hexField().activeFocus, "focus stays in the field to fix it");
            keyClick(Qt.Key_Escape);
            compare(hexField().text, "#7ab8a0");
            compare(hexField().errorText, "", "Escape drops the edit and its error");
        }

        function test_escapeDropsAnEditThenPassesOn() {
            type("#12");
            keyClick(Qt.Key_Escape);
            compare(hexField().text, "#ce5e32", "Escape drops the edit");
            compare(field.hex, "#ce5e32");
            compare(host.escapes, 0, "Escape on an edit stays with the field");
            verify(focused() === field, "focus goes back to the field");
            keyClick(Qt.Key_Escape);
            compare(host.escapes, 1, "with nothing typed, Escape goes on");
        }

        function test_clickingIntoTheHexEdits() {
            mouseClick(hexField(), 30, hexField().height / 2);
            verify(hexField().activeFocus, "a click into the hex edits it");
            keyClick(Qt.Key_A, Qt.ControlModifier);
            type("#abcdef");
            keyClick(Qt.Key_Return);
            compare(field.hex, "#abcdef");
        }
    }
}
