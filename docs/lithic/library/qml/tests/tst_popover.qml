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

    property int chosen: -1

    TestCase {
        name: "Popover"
        when: windowShown

        function cleanup() {
            edgePopover.close();
            menuPopover.close();
            tryVerify(() => !edgePopover.visible && !menuPopover.visible);
            host.chosen = -1;
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
    }
}
