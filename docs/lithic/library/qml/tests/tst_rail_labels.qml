import QtQuick
import QtTest
import Lithic

// An expanded rail label wraps only where a line may break: between words, or
// between characters in a script written without spaces. A word too wide for
// the tile on its own is shortened with an ellipsis, never split, and keeps its
// whole name for a screen reader.
Item {
    id: host
    width: 400
    height: 900

    Rail {
        id: german
        height: 900
        sizeClass: "expanded"
        showNew: false
        destinations: [
            { label: "Darstellung", glyph: "palette" },
            { label: "Verbindungen", glyph: "wifi" },
            { label: "Aktualisierungen", glyph: "system_update" },
            { label: "Bildschirm und Fenster", glyph: "monitor" }
        ]
    }

    Rail {
        id: japanese
        x: 120
        height: 900
        sizeClass: "expanded"
        showNew: false
        destinations: [
            { label: "外観", glyph: "palette" },
            { label: "ネットワークと接続の設定", glyph: "wifi" }
        ]
    }

    Rail {
        id: english
        x: 240
        height: 900
        sizeClass: "expanded"
        showNew: false
        destinations: [
            { label: "Appearance", glyph: "palette" },
            { label: "Instant machine", glyph: "bolt" }
        ]
    }

    TestCase {
        name: "RailLabels"
        when: windowShown

        // the caption a tile draws, found by the destination's accessible name
        function caption(rail, name) {
            let tile = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !tile; i++) {
                    const child = item.children[i];
                    if (child.Accessible.name === name && child.destinationLabel !== undefined && child.visible)
                        tile = child;
                    else
                        walk(child);
                }
            })(rail);
            verify(tile !== null, name + " has a tile");
            let text = null;
            (function walk(item) {
                for (let i = 0; i < item.children.length && !text; i++) {
                    const child = item.children[i];
                    if (child.typeRole === "caption" && child.visible)
                        text = child;
                    else
                        walk(child);
                }
            })(tile.contentItem);
            verify(text !== null, name + " draws a caption");
            return { tile: tile, text: text };
        }

        function test_aLongCompoundIsShortenedNotSplit() {
            const label = caption(german, "Aktualisierungen");
            compare(label.text.lineCount, 1, "the word stays on one line");
            verify(label.text.text.endsWith("…"), "it is shortened visibly: " + label.text.text);
            verify("Aktualisierungen".startsWith(label.text.text.slice(0, -1)), "with the start of the word");
            verify(label.text.contentWidth <= label.text.width + 0.5, "and fits the tile");
            compare(label.tile.Accessible.name, "Aktualisierungen", "the whole word is still its name");
            compare(label.tile.height, LithicTokens.size.railDestinationExpanded, "its tile does not grow");
        }

        function test_aSingleWordIsNeverBrokenInside() {
            for (const name of ["Darstellung", "Verbindungen", "Aktualisierungen"]) {
                const label = caption(german, name);
                compare(label.text.lineCount, 1, name + " stays on one line");
                const shown = label.text.text;
                verify(shown === name || (shown.endsWith("…") && name.startsWith(shown.slice(0, -1))),
                       name + " is drawn whole or visibly shortened, not split: " + shown);
            }
        }

        function test_wordsWrapBetweenWords() {
            const label = caption(german, "Bildschirm und Fenster");
            compare(label.text.text, "Bildschirm und Fenster");
            compare(label.text.lineCount, 2);
            compare(label.tile.height, LithicTokens.size.railDestinationExpanded + Lithic.type.caption.lineHeight,
                    "the tile grows by one caption line");
        }

        function test_japaneseBreaksBetweenCharacters() {
            const label = caption(japanese, "ネットワークと接続の設定");
            verify(!label.text.text.includes("…"), "a label without spaces wraps rather than shortens");
            compare(label.text.text, "ネットワークと接続の設定");
            verify(label.text.lineCount >= 2);
            verify(label.text.contentWidth <= label.text.width + 0.5, "every line fits the tile");
        }

        function test_englishIsUnchanged() {
            compare(caption(english, "Appearance").text.lineCount, 1);
            const two = caption(english, "Instant machine");
            compare(two.text.text, "Instant machine");
        }
    }
}
