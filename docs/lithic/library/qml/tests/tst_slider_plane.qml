import QtQuick
import QtTest
import Lithic

// A slider's track is `field`; on a field plane (a ListEditor form, a row the
// host marks as lying on one) it carries the edge hairline a field has there,
// so its range stays visible. Anywhere else it is drawn as before.
Item {
    id: host
    width: 700
    height: 600

    Rectangle {
        width: parent.width
        height: 140
        color: Lithic.field
        SettingRow {
            width: parent.width
            plane: Lithic.field
            title: "Opacity"
            Slider { id: onField; width: 200; value: 20 }
        }
    }

    Rectangle {
        y: 160
        width: parent.width
        height: 120
        color: Lithic.slab
        SettingRow {
            width: parent.width
            title: "Opacity"
            Slider { id: onSlab; width: 200; value: 20 }
        }
    }

    ListEditor {
        id: list
        y: 300
        width: 600
        model: [{ title: "kitty" }]
        editor: Component {
            Item {
                property var item
                implicitHeight: 40
                Slider { objectName: "formSlider"; width: 200; value: 20 }
            }
        }
    }

    TestCase {
        name: "SliderPlane"
        when: windowShown

        // pixels in a column of the unfilled track, well past the handle, that are not the plane
        function column(slider) {
            const image = grabImage(host);
            const top = slider.mapToItem(host, Math.round(slider.width * 0.85), 0);
            const colours = [];
            for (let y = 0; y < slider.height; y++)
                colours.push(image.pixel(Math.round(top.x), Math.round(top.y) + y));
            return colours;
        }

        function strokeOn(slider, plane) {
            return column(slider).filter(c => !Qt.colorEqual(c, plane)).length;
        }

        function test_trackOnAFieldRowShowsItsRange() {
            verify(strokeOn(onField, Lithic.field) > 0, "the unfilled track shows on a field plane");
        }

        function test_trackOnASlabIsUnchanged() {
            const colours = column(onSlab);
            verify(colours.some(c => Qt.colorEqual(c, Lithic.field)), "the track is field on a slab");
            compare(colours.filter(c => Qt.colorEqual(c, Lithic.edge)).length, 0, "no hairline on a slab");
        }

        function test_trackInAListEditorFormShowsItsRange() {
            list.openEditor(0);
            const slider = findChild(list, "formSlider");
            verify(slider !== null, "the form opened");
            waitForRendering(slider);
            verify(strokeOn(slider, Lithic.field) > 0, "the form lies on the field plane");
        }
    }
}
