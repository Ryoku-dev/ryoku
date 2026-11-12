import QtQuick
import QtTest
import Lithic

// When the keys closed a ListEditor form or removed its last item, focus comes
// back with the keyboard mark; an open Disclosure never cuts a focus ring at
// its edge.
Item {
    id: host
    width: 700
    height: 700

    property var items: [{ title: "Quiet" }]

    ListEditor {
        id: list
        width: 500
        model: host.items
        addText: "Add a preset"
        editor: Component {
            TextField {
                property var item
                readonly property bool valid: text.length > 0
                function value() { return { title: text } }
                text: item && item.title ? item.title : ""
            }
        }
        onAdded: item => host.items = host.items.concat([item])
        onEdited: (index, item) => {
            const next = host.items.slice();
            next[index] = item;
            host.items = next;
        }
        onRemoved: index => {
            const next = host.items.slice();
            next.splice(index, 1);
            // the host removes after a confirm, a moment later
            Qt.callLater(() => host.items = next);
        }
    }

    Disclosure {
        id: fold
        y: 400
        width: 300
        text: "More"
        open: true
        Plaque { id: inner; text: "Reset"; previewState: "focus" }
    }

    TestCase {
        name: "ListEditorMark"
        when: windowShown

        function init() {
            list.cancel();
            host.items = [{ title: "Quiet" }];
            wait(50);
        }

        function focused() {
            return host.Window.window.activeFocusItem;
        }

        function addPlaque() {
            let found = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !found; i++) {
                    const child = item.children[i];
                    if (child.text === "Add a preset" && child.clicked !== undefined)
                        found = child;
                    else
                        walk(child);
                }
            })(list);
            return found;
        }

        function test_escapeOnAnAddFormMarksTheAddPlaque() {
            addPlaque().forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Space);
            compare(list.editingIndex, 1);
            keyClick(Qt.Key_Escape);
            compare(list.editingIndex, -1);
            tryVerify(() => focused() === addPlaque(), 1000);
            verify(addPlaque().visualFocus, "Escape hands focus back with the mark");
        }

        function test_escapeAfterAPointerOpenStillMarks() {
            mouseClick(addPlaque());
            compare(list.editingIndex, 1);
            wait(50);
            keyClick(Qt.Key_Escape);
            wait(300);
            tryVerify(() => focused() === addPlaque(), 1000);
            verify(addPlaque().visualFocus, "Escape is a key, so the mark shows");
        }

        function test_doneOnARenameMarksTheEditPlaque() {
            list.openEditor(0);
            wait(50);
            keyClick(Qt.Key_A, Qt.ControlModifier);
            for (const ch of "Calm")
                keyClick(ch);
            keyClick(Qt.Key_Return);
            compare(host.items[0].title, "Calm");
            tryVerify(() => focused() && focused().glyph !== undefined && focused().visualFocus, 1000,
                      "Return hands focus back to the row with the mark");
        }

        function test_removingTheLastItemFromTheKeysMarksTheAddPlaque() {
            let remove = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !remove; i++) {
                    const child = item.children[i];
                    if (child.glyph === "delete" && child.clicked !== undefined)
                        remove = child;
                    else
                        walk(child);
                }
            })(list);
            verify(remove !== null, "the row has a remove plaque");
            remove.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Space);
            tryCompare(host.items, "length", 0);
            tryVerify(() => focused() === addPlaque(), 1000);
            verify(addPlaque().visualFocus, "removing from the keys hands focus back with the mark");
        }

        function test_anOpenDisclosureKeepsItsRingsWhole() {
            tryVerify(() => fold.contentItem.children[1].height === fold.contentItem.children[1].implicitHeight, 2000);
            verify(!fold.contentItem.children[1].clip, "an open fold does not clip, so the ring outside it shows");
        }
    }
}
