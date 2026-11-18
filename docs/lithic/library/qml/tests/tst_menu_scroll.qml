import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 600
    height: 480
    readonly property var languages: Array.from({ length: 400 }, (_, index) => "Language " + index)

    Picker {
        id: picker
        x: 20
        y: 210
        width: 260
        options: host.languages
        currentIndex: 350
    }

    Picker {
        id: preview
        x: 300
        y: 20
        width: 240
        options: ["One", "Two", "Three"]
        currentIndex: 1
        previewOpen: true
        overlayParent: host
    }

    TestCase {
        name: "MenuScroll"
        when: windowShown

        function popup() {
            return Array.from(picker.data).find(child => child.lithicPopoverMarker === true);
        }

        function menu() {
            const pop = popup();
            return pop._findMenu(pop.contentItem);
        }

        function visibleChoice() {
            const list = menu().contentItem.children.find(child => child.positionViewAtIndex !== undefined);
            const item = list.currentItem;
            verify(item !== null);
            verify(item.y >= list.contentY - 1);
            verify(item.y + item.height <= list.contentY + list.height + 1);
        }

        function cleanup() {
            picker.close();
            tryCompare(picker, "opened", false);
            picker.options = host.languages;
            picker.y = 210;
            picker.currentIndex = 350;
        }

        function test_longMenuShrinksBesideItsField() {
            picker.open();
            tryCompare(picker, "opened", true);
            const pop = popup();
            const margin = LithicTokens.size.popoverAnchorGap;
            verify(pop.y >= margin);
            verify(pop.y + pop.height <= host.Window.window.height - margin,
                   "bottom " + (pop.y + pop.height) + ", window " + host.Window.window.height);
            verify(pop.y + pop.height <= picker.y - margin
                   || pop.y >= picker.y + picker.height + margin);
            compare(menu().focusedIndex, 350);
            const list = menu().contentItem.children.find(child => child.positionViewAtIndex !== undefined);
            verify(list.contentItem.children.length < 100, "only visible rows are instantiated");
            visibleChoice();
            keyClick(Qt.Key_End);
            compare(menu().focusedIndex, 399);
            visibleChoice();
            keyClick(Qt.Key_Home);
            compare(menu().focusedIndex, 0);
            visibleChoice();
            keyClick(Qt.Key_PageDown);
            verify(menu().focusedIndex > 0);
            visibleChoice();
            keyClick(Qt.Key_PageUp);
            compare(menu().focusedIndex, 0);
            visibleChoice();
        }

        function test_mixedScriptsShareThePrimaryBaseline() {
            picker.y = 20;
            picker.options = ["English", "Arabic · العربية", "Persian · فارسی",
                              "Hebrew · עברית", "Hindi · हिन्दी", "Japanese · 日本語"];
            picker.currentIndex = 0;
            picker.open();
            tryCompare(picker, "opened", true);
            const list = menu().contentItem.children.find(child => child.positionViewAtIndex !== undefined);
            list.forceLayout();
            let baseline = -1;
            for (let index = 0; index < picker.options.length; ++index) {
                const item = list.itemAtIndex(index);
                verify(item !== null);
                const label = item.contentItem.children.find(child => child.text === picker.options[index]);
                verify(label !== undefined);
                const current = label.y + label.baselineOffset;
                if (baseline < 0)
                    baseline = current;
                else
                    fuzzyCompare(current, baseline, 0.01);
            }
        }

        function test_embeddedPickerKeepsItsMenuBesideTheField() {
            const pop = Array.from(preview.data).find(child => child.lithicPopoverMarker === true);
            verify(pop.previewEmbedded);
            tryVerify(() => pop.placementPosition.y >= preview.y + preview.height
                            + LithicTokens.size.popoverAnchorGap);
            const menu = pop._findMenu(pop.contentItem);
            compare(menu.height, menu.implicitHeight);
            verify(menu.height > 0);
            verify(pop.placementPosition.y + pop.height <= host.height - LithicTokens.size.popoverAnchorGap);
            verify(!menu.activeFocus, "a design preview must not take focus");
        }
    }
}
