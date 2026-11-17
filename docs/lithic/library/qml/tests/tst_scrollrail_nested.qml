import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 400
    height: 400

    Flickable {
        id: page
        anchors.fill: parent
        contentWidth: width
        contentHeight: 2000
        boundsBehavior: Flickable.StopAtBounds

        Flickable {
            id: inner
            y: 100
            width: 300
            height: 200
            contentWidth: width
            contentHeight: 2000
            boundsBehavior: Flickable.StopAtBounds
            ScrollRail { id: rail; flickable: inner; anchors.top: parent.top; anchors.bottom: parent.bottom; anchors.right: parent.right }
        }
    }

    TestCase {
        name: "ScrollRailInsideAFlickable"
        when: windowShown

        function test_dragKeepsTheRailInsteadOfScrollingThePage() {
            page.contentY = 300;
            const start = rail.mapToItem(host, rail.width - 1, rail._thumbY + 4);
            mousePress(host, start.x, start.y, Qt.LeftButton);
            let y = start.y;
            for (let i = 0; i < 12; ++i) {
                y += 6;
                mouseMove(host, start.x, y, 10, Qt.LeftButton);
            }
            mouseRelease(host, start.x, y, Qt.LeftButton);
            compare(page.contentY, 300, "the page stays put under a rail drag");
            verify(inner.contentY > 300, "the rail kept the whole drag: " + inner.contentY);
        }
    }
}
