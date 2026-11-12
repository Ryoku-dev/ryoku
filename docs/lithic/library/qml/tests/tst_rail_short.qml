import QtQuick
import QtTest
import Lithic

// A host with many destinations keeps its rail whole in a short window: the stacked tiles
// share the room above the foot, and a tile without a label names itself in a tooltip.
Item {
    id: host
    width: 1000
    height: 520

    readonly property var names: ["Appearance", "Desktop", "Windows", "Devices", "Connections", "Apps",
                                  "Accounts", "System", "Updates"]

    AppFrame {
        id: frame
        anchors.fill: parent
        destinations: host.names.map(name => ({ label: name, glyph: "settings" }))
        showNew: false
        footIndex: host.names.length - 1
        compactBar: false
        Item { anchors.fill: parent }
    }

    function named(item, name) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (kids[i].visible && kids[i].Accessible.name === name && kids[i].clicked !== undefined)
                return kids[i];
            const inner = named(kids[i], name);
            if (inner)
                return inner;
        }
        return null;
    }

    TestCase {
        name: "RailShort"
        when: windowShown

        function box(name) {
            const tile = host.named(frame, name);
            verify(tile, name + " has a tile");
            const at = tile.mapToItem(frame, 0, 0);
            return { top: at.y, bottom: at.y + tile.height, height: tile.height, tile: tile };
        }

        function resize(w, h) {
            host.width = w;
            host.height = h;
            wait(50);
        }

        function init() {
            resize(1000, 520);
        }

        function test_tiles_stay_above_the_foot_in_the_shortest_window() {
            const foot = box("Updates");
            compare(foot.bottom, host.height - LithicTokens.size.railItemBottomInset);
            let last = box(host.names[0]);
            for (let i = 1; i < host.names.length - 1; ++i) {
                const next = box(host.names[i]);
                verify(next.top >= last.bottom, host.names[i] + " starts under " + host.names[i - 1]);
                verify(next.height >= LithicTokens.size.iconPlaqueSmall, host.names[i] + " keeps a pressable height");
                last = next;
            }
            verify(last.bottom < foot.top, "the last stacked tile ends above Updates");
        }

        function test_tiles_keep_their_size_where_there_is_room() {
            resize(1000, 800);
            compare(box("Desktop").height, LithicTokens.size.railDestinationMedium);
            compare(box("Desktop").top - box("Appearance").bottom, LithicTokens.size.railTileGap);
        }

        function test_an_icon_only_tile_names_itself_in_a_tooltip() {
            const tile = box("Devices").tile;
            verify(tile.tip, "a medium tile has a tooltip");
            compare(tile.tip.text, "Devices");
        }

        function test_a_labelled_tile_has_no_tooltip_until_its_label_goes() {
            resize(1280, 800);
            verify(!box("Devices").tile.tip, "an expanded tile shows its label, so it has no tooltip");
            resize(1280, 520);
            verify(box("Devices").tile.tip, "a tile too short for its label gets the tooltip");
            verify(box("System").bottom < box("Updates").top, "expanded tiles stay above the foot too");
        }
    }
}
