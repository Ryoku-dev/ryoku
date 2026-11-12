import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 800
    height: 400

    Placard {
        id: placard
        parent: host
        message: "Wallpaper changed"
        actionText: "Undo"
    }

    TestCase {
        name: "PlacardHold"
        when: windowShown

        function cleanup() {
            placard.close();
            Lithic.reduceMotion = false;
            mouseMove(host, 5, 5);
            host.forceActiveFocus();
        }

        function action() {
            const found = [];
            (function walk(item) {
                for (let i = 0; i < item.children.length; i++) {
                    const child = item.children[i];
                    if (child.text === "Undo" && child.clicked !== undefined)
                        found.push(child);
                    walk(child);
                }
            })(placard.contentItem);
            verify(found.length === 1, "the placard has one action");
            return found[0];
        }

        function test_hoverHoldsTheLifeAndLeavingResumesIt() {
            placard.open();
            tryVerify(() => placard.opened);
            wait(1000);
            mouseMove(placard.contentItem, 40, placard.height / 2);
            wait(100);
            const blade = placard.remaining;
            wait(5500);
            verify(placard.opened, "a hovered placard outlives its 5 s life");
            compare(placard.remaining, blade, "the drain blade stands still while held");
            mouseMove(host, 5, 5);
                        wait(500);
            verify(placard.opened, "leaving resumes the life where it stood rather than ending it");
            verify(placard.remaining < blade, "the blade drains again once the pointer leaves");
            tryVerify(() => !placard.opened, 5000, "the resumed life runs out");
        }

        function test_focusHoldsTheLifeWithReducedMotion() {
            Lithic.reduceMotion = true;
            placard.open();
            tryVerify(() => placard.opened);
            wait(1000);
            action().forceActiveFocus();
            verify(action().activeFocus, "Undo holds focus");
            wait(5500);
            verify(placard.opened, "a placard whose Undo holds focus outlives its life");
            host.forceActiveFocus();
                        wait(500);
            verify(placard.opened, "leaving focus resumes the life rather than ending it");
            tryVerify(() => !placard.opened, 5000, "the resumed life runs out");
        }
    }
}
