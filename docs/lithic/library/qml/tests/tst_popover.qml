import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 600
    height: 400

    Plaque {
        id: edgeAnchor
        x: host.width - width - 2
        y: host.height - height - 2
        text: "Details"
    }

    Popover {
        id: edgePopover
        parent: host
        anchorItem: edgeAnchor
        opener: edgeAnchor

        Rectangle {
            implicitWidth: 240
            implicitHeight: 120
            color: "transparent"
        }
    }

    Plaque {
        id: menuOpener
        x: 20
        y: 20
        text: "Actions"
        onClicked: menuPopover.open()
    }

    Popover {
        id: menuPopover
        parent: host
        anchorItem: menuOpener
        opener: menuOpener

        Menu {
            id: menu
            items: [{ label: "Copy" }, { label: "Rename" }, { label: "Remove" }]
            onTriggered: index => {
                host.chosen = index
                menuPopover.close()
            }
        }
    }

    Plaque {
        id: focusOpener
        x: 220
        y: 20
        text: "More"
        onClicked: focusPopover.open()
    }

    Popover {
        id: focusPopover
        parent: host
        anchorItem: focusOpener
        opener: focusOpener

        Menu {
            focus: true
            items: [{ label: "Pin" }, { label: "Hide" }]
        }
    }

    property int chosen: -1

    TestCase {
        name: "Popover"
        when: windowShown

        function cleanup() {
            edgePopover.close();
            menuPopover.close();
            focusPopover.close();
            tryVerify(() => !edgePopover.visible && !menuPopover.visible && !focusPopover.visible);
            host.chosen = -1;
            // closing hands focus back to an opener; each test starts with nothing focused
            host.forceActiveFocus();
        }

        function test_popoverKeepsAMarginFromTheWindowEdges() {
            edgePopover.open();
            tryVerify(() => edgePopover.opened);
            const margin = LithicTokens.size.popoverAnchorGap;
            verify(edgePopover.x >= margin, "left edge kept " + edgePopover.x);
            verify(edgePopover.y >= margin, "top edge kept " + edgePopover.y);
            verify(edgePopover.x + edgePopover.width <= host.width - margin,
                   "right edge kept: popover ends at " + (edgePopover.x + edgePopover.width));
            verify(edgePopover.y + edgePopover.height <= host.height - margin,
                   "bottom edge kept: popover ends at " + (edgePopover.y + edgePopover.height));
        }

        function test_menuInPopoverTakesTheKeysAfterAPointerOpen() {
            mouseClick(menuOpener);
            tryVerify(() => menuPopover.opened);
            const focused = host.Window.window.activeFocusItem;
            verify(focused && focused.text === "Copy", "the first item holds focus, got " + (focused ? focused.text : null));
            verify(!focused.visualFocus, "a pointer open shows no keyboard ring");
            keyClick(Qt.Key_Down);
            keyClick(Qt.Key_Return);
            compare(host.chosen, 1, "Down then Return chooses the second item");
        }

        function test_menuInPopoverShowsTheRingAfterAKeyOpen() {
            menuOpener.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Return);
            tryVerify(() => menuPopover.opened);
            const focused = host.Window.window.activeFocusItem;
            verify(focused && focused.text === "Copy", "the first item holds focus, got " + (focused ? focused.text : null));
            verify(focused.visualFocus, "a key open shows the keyboard ring on the first item");
        }

        function test_aMenuThatTakesFocusItselfMarksOnlyAfterAKeyOpen() {
            mouseClick(focusOpener);
            tryVerify(() => focusPopover.opened);
            let focused = host.Window.window.activeFocusItem;
            verify(focused && focused.text === "Pin", "the first item holds focus, got " + (focused ? focused.text : null));
            verify(!focused.visualFocus, "a pointer open shows no keyboard ring");
            focusPopover.close();
            tryVerify(() => !focusPopover.visible);
            host.forceActiveFocus();
            focusOpener.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Return);
            tryVerify(() => focusPopover.opened);
            focused = host.Window.window.activeFocusItem;
            verify(focused && focused.text === "Pin", "the first item holds focus after a key open");
            verify(focused.visualFocus, "a key open shows the keyboard ring");
        }
    }
}
