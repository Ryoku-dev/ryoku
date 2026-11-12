import QtQuick
import QtTest
import Lithic

Item {
    width: 400
    height: 300

    SectionIndex {
        id: index
        width: 300
        sizeClass: "compact"
        sections: ["Wi-Fi", "Saved networks", "Hotspot"]
    }

    function pickerOf(item) {
        const kids = item ? item.children || [] : [];
        for (let i = 0; i < kids.length; ++i) {
            if (typeof kids[i].menuItems === "function")
                return kids[i];
            const inner = pickerOf(kids[i]);
            if (inner)
                return inner;
        }
        return null;
    }

    TestCase {
        name: "SectionIndexName"
        when: windowShown

        function test_compactPickerIsNamed() {
            const picker = pickerOf(index);
            verify(picker !== null && picker.visible, "a compact index draws a picker");
            compare(picker.Accessible.name, "Go to a section", "a screen reader hears what the picker is for");
            index.accessibleName = "Jump to a part of Wi-Fi";
            compare(picker.Accessible.name, "Jump to a part of Wi-Fi");
        }
    }
}
