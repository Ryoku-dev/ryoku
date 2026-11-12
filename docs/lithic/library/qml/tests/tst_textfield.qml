import QtQuick
import QtTest
import Lithic

Item {
    width: 400
    height: 200

    TextField {
        id: limit
        width: 300
        label: "Storage limit"
        placeholderText: "128"
    }

    function textItem(item, wanted) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (kids[i].text === wanted && kids[i].color !== undefined)
                return kids[i];
            const inner = textItem(kids[i], wanted);
            if (inner)
                return inner;
        }
        return null;
    }

    TestCase {
        name: "TextFieldPlaceholder"
        when: windowShown

        function test_placeholderReadsAsAHint() {
            const hint = textItem(limit, "128");
            verify(hint !== null, "the placeholder is drawn");
            verify(Qt.colorEqual(hint.color, Lithic.fg3), "an empty field must not look filled in: the hint uses fg3");
            limit.text = "64";
            verify(!hint.visible, "the hint goes once there is text");
        }
    }
}
