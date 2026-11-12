import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 640
    height: 480

    Section {
        id: plain
        width: 480
        title: "Blur"
        Rectangle {
            width: parent.width
            implicitHeight: 40
        }
    }
    Section {
        id: ruled
        y: 120
        width: 480
        title: "Blur"
        ruled: true
        mark: "01"
        Rectangle {
            width: parent.width
            implicitHeight: 40
        }
    }
    Section {
        id: unmarked
        y: 240
        width: 480
        title: "Shadows"
        ruled: true
        Rectangle {
            width: parent.width
            implicitHeight: 40
        }
    }

    // the head's parts by what they are: the title and the mark are text, the rule a hairline
    function texts(section) {
        var out = [];
        for (var i = 0; i < section.children.length; i++) {
            var c = section.children[i];
            if (c.text !== undefined && c.visible)
                out.push(c);
        }
        return out;
    }
    function rule(section) {
        for (var i = 0; i < section.children.length; i++) {
            var c = section.children[i];
            if (c.text === undefined && c.color !== undefined && c.height === LithicTokens.size.hairline)
                return c;
        }
        return null;
    }

    TestCase {
        name: "SectionRuled"
        when: windowShown

        function test_plain_section_draws_no_rule_and_no_mark() {
            verify(!rule(plain).visible);
            compare(texts(plain).length, 1);
        }

        function test_rule_runs_from_the_title_to_the_mark() {
            var parts = texts(ruled);
            compare(parts.length, 2);
            var title = parts[0], mark = parts[1];
            var line = rule(ruled);
            verify(line.visible);
            compare(mark.text, "01");
            compare(mark.x + mark.width, ruled.width);
            compare(line.x, title.width + LithicTokens.space.s3);
            compare(line.x + line.width, mark.x - LithicTokens.space.s3);
            // centred on the title, so it reads as the head's own line
            verify(Math.abs(line.y + line.height / 2 - (title.y + title.height / 2)) <= 1);
            verify(Qt.colorEqual(line.color, Lithic.vein));
        }

        function test_rule_without_a_mark_reaches_the_far_edge() {
            var line = rule(unmarked);
            verify(line.visible);
            compare(texts(unmarked).length, 1);
            compare(line.x + line.width, unmarked.width);
        }

        function test_the_head_keeps_the_section_geometry() {
            compare(ruled.implicitHeight, plain.implicitHeight);
            compare(ruled.bodyItem.y, plain.bodyItem.y);
        }
    }
}
