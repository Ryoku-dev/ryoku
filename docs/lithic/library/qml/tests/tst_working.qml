import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 400
    height: 200

    Working {
        id: spinner
        x: 40
        y: 20
        active: true
        size: 24
    }

    Plaque {
        id: glyphPlaque
        y: 80
        text: "Apply"
        glyph: "check"
    }

    SplitPlaque {
        id: split
        y: 140
        text: "Update"
        glyph: "download"
    }

    TestCase {
        name: "Working"
        when: windowShown

        function flakesOf(item, out) {
            const kids = item.children || [];
            for (let i = 0; i < kids.length; ++i) {
                if (kids[i].fillFraction !== undefined && kids[i].outlineWidth !== undefined)
                    out.push(kids[i]);
                else
                    flakesOf(kids[i], out);
            }
            return out;
        }

        function test_theFlakesDrawInsideTheSpinnersBox() {
            const flakes = flakesOf(spinner, []);
            verify(flakes.length > 0, "the spinner draws its flakes");
            let left = Infinity, right = -Infinity;
            for (const flake of flakes) {
                // a flake's slanted bar runs from (width - height) to height along its own x
                const from = flake.mapToItem(spinner, Math.min(0, flake.width - flake.height), 0).x;
                const to = flake.mapToItem(spinner, Math.max(flake.width, flake.height), 0).x;
                left = Math.min(left, from);
                right = Math.max(right, to);
            }
            verify(left >= -0.5, "the band starts inside the spinner, at " + left);
            verify(right <= spinner.width + 0.5, "the band ends inside the spinner, at " + right + " of " + spinner.width);
        }

        function test_aBusyPlaqueKeepsItsWidth() {
            const plaqueIdle = glyphPlaque.width;
            glyphPlaque.busy = true;
            compare(glyphPlaque.width, plaqueIdle, "a busy plaque keeps the glyph's slot");
            glyphPlaque.busy = false;
            const splitIdle = split.width;
            split.busy = true;
            compare(split.width, splitIdle, "a busy split plaque keeps its width");
            split.busy = false;
        }
    }
}
