import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 600
    height: 400

    readonly property string longLabel: "Ryoku shortcuts left as they were before the import"
    readonly property string longValue: "/var/lib/ryoport/snapshots/ubuntu-24.04/before-update/".repeat(6)

    Facts {
        id: facts
        width: 600
        facts: [
            { label: "Shortcuts added", value: "2" },
            { label: host.longLabel, value: "1" },
            { label: "Backup", value: host.longValue, literal: true }
        ]
    }

    TestCase {
        name: "Facts"
        when: windowShown

        // The rows are the delegates, found by their model data rather than by name
        function row(index) {
            var found = null;
            (function walk(item) {
                for (var i = 0; i < item.children.length && found === null; i++) {
                    var child = item.children[i];
                    if (child.modelData !== undefined && child.index === index)
                        found = child;
                    else
                        walk(child);
                }
            })(facts);
            verify(found !== null, "row " + index + " exists");
            return found;
        }

        // The label is the text that reads the fact's label; the value is the other one
        function side(r, wantLabel) {
            for (var i = 0; i < r.children.length; i++) {
                var child = r.children[i];
                if (child.text !== undefined && (child.text === r.modelData.label) === wantLabel)
                    return child;
            }
            return null;
        }

        function test_longLabelWrapsAndGrowsItsRow() {
            var short = row(0), long = row(1);
            var label = side(long, true);
            verify(label.lineCount > 1, "a long label wraps");
            verify(!label.truncated, "a long label never elides");
            compare(label.width, LithicTokens.size.factsLabelWidth, "the label keeps the 140 column");
            compare(short.height, LithicTokens.size.factsRowHeight, "a one-line row stays 44");
            verify(long.height > LithicTokens.size.factsRowHeight, "the row grows past 44");
            verify(long.height >= label.implicitHeight, "the row fits its taller side");
            compare(long.y, short.y + short.height, "rows stack without overlap");
        }

        function test_valuesKeepTheirColumnAndGap() {
            var column = LithicTokens.size.factsLabelWidth + LithicTokens.space.s4;
            for (var i = 0; i < facts.facts.length; i++) {
                var value = side(row(i), false);
                compare(value.x, column, "value " + i + " starts s4 after the label column");
                compare(value.width, facts.width - column);
            }
        }

        function test_longValueWrapsAndBothSidesCentre() {
            var r = row(2);
            var value = side(r, false);
            verify(value.lineCount > 1, "a long value wraps");
            verify(!value.truncated, "a long value never elides");
            compare(r.height, Math.max(LithicTokens.size.factsRowHeight, value.implicitHeight));
            var label = side(r, true);
            fuzzyCompare(label.y + label.height / 2, r.height / 2, 1, "the label centres in its row");
            fuzzyCompare(value.y + value.height / 2, r.height / 2, 1, "the value centres in its row");
        }

        function test_heightIsTheSumOfRows() {
            compare(facts.implicitHeight, row(0).height + row(1).height + row(2).height);
        }
    }
}
