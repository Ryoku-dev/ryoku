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

    TextField {
        id: longError
        y: 100
        width: 200
        errorText: "That file could not be used. Open /dev/shm/somewhere/long/art.txt and check that it is readable."
    }

    TextField {
        id: shortError
        y: 100
        x: 210
        width: 200
        errorText: "Too short."
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

        function test_longErrorReadsWhole() {
            const help = textItem(longError, longError.errorText);
            verify(help !== null, "the error is drawn");
            verify(!help.truncated, "an error is never cut short");
            verify(help.lineCount > 1, "a long error wraps");
            verify(longError.implicitHeight > shortError.implicitHeight, "the field grows to hold every line");
        }
    }
}
