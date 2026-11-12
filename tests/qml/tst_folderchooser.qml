import QtQuick
import QtTest
import "../../ryoku/hub/quickshell/components"

// Settings' folder chooser takes the folder it shows. Run from the repository root:
//   QT_QPA_PLATFORM=offscreen "$(qmake6 -query QT_HOST_BINS)/qmltestrunner" -import docs/lithic/library/qml -import tests/qml/stubs -input tests/qml/tst_folderchooser.qml
// tests/ has subfolders (perf-budget, qml, ui); tests/ui has none.
Item {
    id: root
    width: 900
    height: 800

    readonly property string testsDir: decodeURIComponent(String(Qt.resolvedUrl("..")).replace(/^file:\/\//, "").replace(/\/$/, ""))

    FolderChooser {
        id: chooser
    }

    SignalSpy {
        id: accepts
        target: chooser
        signalName: "accepted"
    }

    SignalSpy {
        id: rejects
        target: chooser
        signalName: "rejected"
    }

    TestCase {
        name: "FolderChooser"
        when: windowShown

        function init() {
            chooser.selectedFolder = "";
            accepts.clear();
            rejects.clear();
        }

        function cleanup() {
            if (chooser.showing)
                chooser.close();
            tryCompare(chooser, "showing", false);
        }

        function localOf(url) {
            return decodeURIComponent(String(url).replace(/^file:\/\//, ""));
        }

        function list() {
            return findChild(chooser.windowOverlay, "folderChooserList");
        }

        function findText(item, text) {
            if (!item)
                return null;
            if (item.text === text && item.visible)
                return item;
            for (let i = 0; i < item.children.length; ++i) {
                const found = findText(item.children[i], text);
                if (found)
                    return found;
            }
            return null;
        }

        function button(text) {
            const found = findText(chooser.windowOverlay, text);
            verify(found !== null, "the dialog shows " + text);
            return found;
        }

        // opens and waits until the folder has listed and the list holds the keys
        function openAt(path, hasFolders) {
            chooser.currentFolder = "file://" + path;
            chooser.open();
            tryCompare(chooser, "showing", true);
            const rows = list();
            verify(rows !== null, "the dialog lists folders");
            if (hasFolders)
                tryVerify(() => rows.listView.count > 0 && rows.currentIndex === 0, 3000, "the folder lists its subfolders");
            else
                tryVerify(() => findText(chooser.windowOverlay, "No folders inside") !== null, 3000, "an empty folder says so");
            tryVerify(() => rows.activeFocus, 3000, "the list has the keys");
            return rows;
        }

        function test_use_takes_a_folder_with_subfolders() {
            openAt(root.testsDir, true);
            const use = button("Use this folder");
            verify(use.enabled);
            mouseClick(use);
            compare(accepts.count, 1);
            compare(localOf(chooser.selectedFolder), root.testsDir, "not its first subfolder");
            tryCompare(chooser, "showing", false);
        }

        function test_use_takes_a_folder_without_subfolders() {
            openAt(root.testsDir + "/ui", false);
            const use = button("Use this folder");
            verify(use.enabled, "an empty folder can be chosen");
            mouseClick(use);
            compare(accepts.count, 1);
            compare(localOf(chooser.selectedFolder), root.testsDir + "/ui");
        }

        function focusedName() {
            const item = chooser.windowOverlay.Window.activeFocusItem;
            return item ? String(item.Accessible.name) : "";
        }

        function test_keys_choose_the_shown_folder() {
            openAt(root.testsDir + "/ui", false);
            for (let i = 0; i < 6 && focusedName() !== "Use this folder"; ++i)
                keyClick(Qt.Key_Tab);
            compare(focusedName(), "Use this folder", "Tab reaches the action");
            keyClick(Qt.Key_Return);
            compare(accepts.count, 1);
            compare(localOf(chooser.selectedFolder), root.testsDir + "/ui");
        }

        function test_entering_a_subfolder_then_use_takes_it() {
            const rows = openAt(root.testsDir, true);
            keyClick(Qt.Key_End);
            const last = rows.listView.count - 1;
            compare(rows.currentIndex, last, "End selects the last folder");
            keyClick(Qt.Key_Home);
            compare(rows.currentIndex, 0, "Home selects the first folder");
            keyClick(Qt.Key_Down);
            compare(rows.currentIndex, 1, "Down moves the selection");
            keyClick(Qt.Key_Up);
            keyClick(Qt.Key_Down);
            const name = rows.listView.currentItem.fileName;
            keyClick(Qt.Key_Return);
            compare(chooser.shownPath, root.testsDir + "/" + name, "Return enters the folder");
            mouseClick(button("Use this folder"));
            compare(localOf(chooser.selectedFolder), root.testsDir + "/" + name);
        }

        function test_space_and_a_click_enter_a_folder() {
            const rows = openAt(root.testsDir, true);
            const name = rows.listView.currentItem.fileName;
            keyClick(Qt.Key_Space);
            compare(chooser.shownPath, root.testsDir + "/" + name, "Space enters the folder");
            chooser.close();
            tryCompare(chooser, "showing", false);
            const again = openAt(root.testsDir, true);
            const second = again.listView.itemAtIndex(1);
            mouseClick(second);
            compare(chooser.shownPath, root.testsDir + "/" + second.fileName, "a click enters the folder");
        }

        function test_up_then_use_takes_the_parent() {
            const rows = openAt(root.testsDir + "/ui", false);
            const up = findChild(chooser.windowOverlay, "folderChooserUp");
            compare(up.Accessible.name, "Up to tests");
            mouseClick(up);
            compare(chooser.shownPath, root.testsDir);
            tryVerify(() => rows.listView.currentItem !== null && rows.listView.currentItem.fileName === "ui", 3000, "the folder left stays selected");
            mouseClick(button("Use this folder"));
            compare(localOf(chooser.selectedFolder), root.testsDir);
        }

        function test_up_is_off_at_the_top() {
            openAt("/", true);
            verify(!findChild(chooser.windowOverlay, "folderChooserUp").enabled);
        }

        function test_a_missing_start_opens_at_the_folder_that_exists() {
            openAt(root.testsDir + "/ui/missing/deeper", false);
            compare(chooser.shownPath, root.testsDir + "/ui");
            mouseClick(button("Use this folder"));
            compare(localOf(chooser.selectedFolder), root.testsDir + "/ui");
        }

        function test_escape_and_cancel_leave_the_value() {
            chooser.selectedFolder = "file:///kept";
            openAt(root.testsDir, true);
            keyClick(Qt.Key_Escape);
            tryCompare(chooser, "showing", false);
            compare(rejects.count, 1);
            openAt(root.testsDir, true);
            mouseClick(button("Cancel"));
            tryCompare(chooser, "showing", false);
            compare(rejects.count, 2);
            compare(accepts.count, 0);
            compare(String(chooser.selectedFolder), "file:///kept");
        }

        function test_every_open_starts_at_the_current_folder() {
            openAt(root.testsDir, true);
            keyClick(Qt.Key_Return);
            verify(chooser.shownPath !== root.testsDir);
            keyClick(Qt.Key_Escape);
            tryCompare(chooser, "showing", false);
            openAt(root.testsDir, true);
            compare(chooser.shownPath, root.testsDir);
        }
    }
}
