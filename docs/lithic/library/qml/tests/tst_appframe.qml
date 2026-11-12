import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 1280
    height: 800

    AppFrame {
        id: frame
        anchors.fill: parent
        destinations: [{ label: "Appearance", glyph: "palette" }, { label: "Updates", glyph: "system_update" }]
        showNew: false
        compactBar: false
        showList: false
        Item {
            id: work
            anchors.fill: parent
        }
        listData: [
            Item {
                id: list
                anchors.fill: parent
            }
        ]
    }

    // the stone under the work content: the pane that holds its host
    function paneOf(item) {
        for (let p = item.parent; p; p = p.parent) {
            if (p.fill !== undefined && p.cutTopLeft !== undefined)
                return p;
        }
        return null;
    }

    TestCase {
        name: "AppFrame"
        when: windowShown

        function init() {
            frame.workFlush = false;
            frame.showList = false;
        }

        function test_work_content_keeps_its_inset_on_stone() {
            const at = work.mapToItem(frame, 0, 0);
            compare(at.x, LithicTokens.size.railExpanded + LithicTokens.size.paneGap + LithicTokens.size.workContentInset);
            compare(at.y, LithicTokens.size.paneTop + LithicTokens.size.paneDecorStrip + LithicTokens.size.workContentInset);
            verify(Qt.colorEqual(paneOf(work).fill, Lithic.stone2));
        }

        function test_flush_work_content_lies_on_the_ground_edge_to_edge() {
            frame.workFlush = true;
            const at = work.mapToItem(frame, 0, 0);
            compare(at.x, LithicTokens.size.railExpanded + LithicTokens.size.paneGap);
            compare(at.y, LithicTokens.size.paneTop);
            compare(at.x + work.width, frame.width);
            compare(at.y + work.height, frame.height);
            compare(paneOf(work).fill.a, 0);
        }

        function test_list_pane_takes_the_width_its_host_sets() {
            frame.showList = true;
            compare(paneOf(list).width, LithicTokens.size.listExpanded);
            const before = work.mapToItem(frame, 0, 0).x;
            frame.listWidth = 300;
            compare(paneOf(list).width, 300);
            compare(work.mapToItem(frame, 0, 0).x, before - (LithicTokens.size.listExpanded - 300));
            frame.listWidth = Qt.binding(() => frame.sizeClass === "expanded" ? LithicTokens.size.listExpanded : LithicTokens.size.listMedium);
        }
    }
}
