import QtQuick
import QtTest
import Lithic

// After a choose, by keyboard or pointer, Ctrl+Z reaches whoever holds the
// PathField; typing edits the path; Escape drops an edit, then passes on; the
// choose plaque is named for what it does, never by a placeholder path.
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

    PathField {
        id: field
        x: 20
        y: 20
        width: 360
        placeholder: "/home/ops/Pictures/Screenshots"
        path: "/home/ops/Pictures"
        // stands in for the portal dialog: the pick lands at once
        onChooseRequested: path = "/home/ops/Shots"
    }

    PathField { id: labelled; x: 20; y: 120; width: 360; label: "Recordings" }
    PathField { id: fileField; x: 20; y: 220; width: 360; kind: "file"; placeholder: "/home/ops/notes.txt" }

    SignalSpy { id: edits; target: field; signalName: "edited" }

    TestCase {
        name: "PathFieldKeys"
        when: windowShown

        function init() {
            field.path = "/home/ops/Pictures";
            field.forceActiveFocus(Qt.TabFocusReason);
            host.undos = 0;
            host.escapes = 0;
            edits.clear();
        }

        function focused() {
            return host.Window.window.activeFocusItem;
        }

        function chooseOf(pathField) {
            let found = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !found; i++) {
                    const child = item.children[i];
                    if (child.glyph === "folder_open")
                        found = child;
                    else
                        walk(child);
                }
            })(pathField);
            return found;
        }

        function pathText() {
            return field.contentItem.children[0];
        }

        function type(text) {
            for (const ch of text)
                keyClick(ch);
        }

        function test_keyboardChooseThenUndoReachesTheHost() {
            keyClick(Qt.Key_Tab);
            verify(focused() === chooseOf(field), "Tab goes from the field to its choose plaque");
            keyClick(Qt.Key_Space);
            compare(field.path, "/home/ops/Shots");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1, "Ctrl+Z after a choose reaches the app's undo");
        }

        function test_pointerChooseThenUndoReachesTheHost() {
            mouseClick(chooseOf(field));
            compare(field.path, "/home/ops/Shots");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1);
        }

        function test_restingFieldPassesUndoOn() {
            verify(focused() === field);
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 1, "with nothing typed, Ctrl+Z goes on");
        }

        function test_typingOpensThePathAndUndoStaysThere() {
            type("/t");
            verify(pathText().activeFocus, "typing moves into the path");
            compare(pathText().text, "/t");
            keyClick(Qt.Key_Z, Qt.ControlModifier);
            compare(host.undos, 0, "while editing, Ctrl+Z undoes the typing");
        }

        function test_returnReportsTheEditOnce() {
            type("/srv/shots");
            keyClick(Qt.Key_Return);
            compare(edits.count, 1);
            compare(edits.signalArguments[0][0], "/srv/shots");
            verify(focused() === field, "Return hands focus back to the field");
        }

        function test_escapeDropsAnEditThenPassesOn() {
            type("/x");
            keyClick(Qt.Key_Escape);
            compare(pathText().text, "/home/ops/Pictures");
            compare(edits.count, 0, "a dropped edit is never reported");
            compare(host.escapes, 0);
            verify(focused() === field);
            keyClick(Qt.Key_Escape);
            compare(host.escapes, 1, "with nothing typed, Escape goes on");
        }

        function test_choosePlaqueIsNamedForWhatItDoes() {
            compare(chooseOf(field).Accessible.name, "Choose a folder", "never the placeholder path");
            compare(chooseOf(labelled).Accessible.name, "Choose Recordings");
            compare(chooseOf(fileField).Accessible.name, "Choose a file");
        }
    }
}
