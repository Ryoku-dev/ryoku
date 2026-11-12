import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 700
    height: 400

    Row {
        spacing: 16
        DesignTile {
            id: narrow
            width: 190
            title: "Glass dock"
            status: "Update available"
            actionText: "Update"
        }
        DesignTile {
            id: narrowShort
            width: 190
            title: "Glass dock"
            status: "Ready"
            actionText: "Update"
        }
        DesignTile {
            id: wide
            width: 420
            title: "Glass dock"
            status: "Update available"
            actionText: "Update"
        }
    }

    TestCase {
        name: "DesignTileStatus"
        when: windowShown

        function textItem(item, text) {
            if (item.text === text && item.lineCount !== undefined)
                return item;
            for (let index = 0; index < item.children.length; ++index) {
                const found = textItem(item.children[index], text);
                if (found)
                    return found;
            }
            return null;
        }

        function test_narrowTileShowsItsWholeStatus() {
            const status = textItem(narrow, "Update available");
            verify(!status.truncated, "a narrow tile keeps its whole status, on a second line if it must");
            compare(narrow.implicitHeight, narrowShort.implicitHeight, "wrapping keeps the tile's height, so a shelf's rows stay even");
            compare(textItem(wide, "Update available").lineCount, 1);
        }
    }
}
