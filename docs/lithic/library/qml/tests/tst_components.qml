import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 800
    height: 900

    ChordField { id: chord; x: 10; y: 10; width: 300 }
    SignalSpy { id: recordedSpy; target: chord; signalName: "recorded" }
    SignalSpy { id: clearedSpy; target: chord; signalName: "cleared" }

    ColorField { id: colorField; x: 10; y: 100; width: 300; color: "#ce5e32"; allowAlpha: false
        swatches: [{ label: "Jade", color: "#7ab8a0" }] }
    SignalSpy { id: pickedSpy; target: colorField; signalName: "picked" }

    Column {
        x: 10; y: 200; width: 600
        SettingRow { id: row; width: parent.width; first: true; activatable: true; title: "Night light"; changed: true
            Toggle { id: toggle } }
    }
    SignalSpy { id: activatedSpy; target: row; signalName: "activated" }
    SignalSpy { id: resetSpy; target: row; signalName: "resetRequested" }

    Rectangle { id: pane; x: 10; y: 400; width: 300; height: 100
        Flickable { id: flick; anchors.fill: parent; contentHeight: 400; clip: true }
        ScrollRail { id: rail; anchors.right: parent.right; anchors.top: parent.top; anchors.bottom: parent.bottom; flickable: flick }
    }

    DesignGallery {
        id: gallery; x: 10; y: 520; width: 650
        model: [{ title: "A" }, { title: "B" }, { title: "C" }, { title: "D" }, { title: "E" }]
    }

    Item { id: confirmStage; x: 1000; y: 0; width: 800; height: 600 }
    Confirm { id: confirm; overlayParent: confirmStage; title: "Forget the studio display"; previewEmbedded: true }
    DesignTile { id: tile; x: 700; y: 10; width: 240; title: "Iris"; resource: "heavy"; busy: true; status: "Installing" }
    Rail { id: fleetRail; x: 1000; y: 700; width: 88; height: 640; sizeClass: "expanded" }
    Rail {
        id: wideRail; x: 1100; y: 700; width: 88; height: 640; sizeClass: "expanded"; showNew: false
        destinations: [{ label: "Erscheinungsbild", glyph: "palette" }, { label: "Bar", glyph: "view_day" },
                       { label: "Updates", glyph: "system_update" }]
        brandMark: Component { Rectangle { objectName: "hostMark"; color: Lithic.onSel } }
    }
    Confirm { id: keepConfirm; cancelText: "Revert"; actionText: "Keep" }

    component AppEditor: Column {
        property var item: ({})
        readonly property bool valid: nameField.text.trim().length > 0
        function value() { return { title: nameField.text } }
        TextField { id: nameField; objectName: "nameField"; width: parent.width; label: "Name"; text: parent.item.title || "" }
    }
    ListEditor {
        id: listEditor; x: 1000; y: 10; width: 500
        reorderable: true
        addText: "Add a startup app"
        model: [{ title: "Firefox" }, { title: "Discord" }, { title: "Ryotunes" }]
        editor: Component { AppEditor {} }
    }
    SignalSpy { id: addedSpy; target: listEditor; signalName: "added" }
    SignalSpy { id: editedSpy; target: listEditor; signalName: "edited" }
    SignalSpy { id: removedSpy; target: listEditor; signalName: "removed" }
    SignalSpy { id: movedSpy; target: listEditor; signalName: "moved" }
    PathField { id: pathField; x: 10; y: 700; width: 320; label: "Screenshots"; path: "/home/ops/Pictures" }
    SignalSpy { id: chooseSpy; target: pathField; signalName: "chooseRequested" }
    SignalSpy { id: pathEditedSpy; target: pathField; signalName: "edited" }
    SignalSpy { id: galleryActivated; target: gallery; signalName: "activated" }

    TestCase {
        name: "Components"
        when: windowShown

        function test_chord() {
            chord.forceActiveFocus();
            keyClick(Qt.Key_Return);
            verify(chord.recording);
            keyClick(Qt.Key_A);
            verify(chord.recording, "bare letter keeps waiting");
            keyClick(Qt.Key_Return, Qt.MetaModifier | Qt.ShiftModifier);
            verify(!chord.recording);
            compare(recordedSpy.count, 1);
            compare(chord.chord, "Super+Shift+Return");
            compare(chord.keys, ["Super", "Shift", "Enter"]);
            chord.startRecording();
            keyClick(Qt.Key_Exclam, Qt.MetaModifier | Qt.ShiftModifier);
            compare(chord.chord, "Super+Shift+1");
            chord.startRecording();
            keyClick(Qt.Key_F5);
            compare(chord.chord, "F5");
            chord.startRecording();
            keyClick(Qt.Key_3, Qt.ControlModifier | Qt.KeypadModifier);
            compare(chord.chord, "Ctrl+KP_3");
            compare(chord.keys, ["Ctrl", "Num 3"]);
            chord.startRecording();
            keyClick(Qt.Key_Escape);
            verify(!chord.recording);
            compare(chord.chord, "Ctrl+KP_3");
            chord.startRecording();
            keyClick(Qt.Key_Backspace, Qt.MetaModifier);
            compare(chord.chord, "Super+BackSpace");
            chord.startRecording();
            keyClick(Qt.Key_Backspace);
            compare(chord.chord, "");
            compare(clearedSpy.count, 1);
            compare(chord.canonical("SHIFT + SUPER + q"), "Super+Shift+q");
            mouseClick(chord);
            verify(chord.recording);
            colorField.forceActiveFocus();
            verify(!chord.recording, "focus loss cancels");
        }

        function test_color() {
            compare(colorField.hex, "#ce5e32");
            verify(colorField.parseHex("zz") === null);
            verify(colorField.parseHex("#ce5e3280") === null, "alpha needs allowAlpha");
            colorField.choose("#7ab8a0");
            compare(pickedSpy.count, 1);
            compare(colorField.hex, "#7ab8a0");
            colorField.allowAlpha = true;
            const c = colorField.parseHex("#ce5e3280");
            verify(c !== null);
            compare(Math.round(c.a * 255), 128);
            colorField.choose(c);
            compare(colorField.hex, "#ce5e3280");
            compare(pickedSpy.count, 2);
        }

        function test_row() {
            row.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Return);
            compare(activatedSpy.count, 1);
            row.unavailable = "Not here";
            keyClick(Qt.Key_Return);
            compare(activatedSpy.count, 1);
            verify(!toggle.enabled);
            row.unavailable = "";
        }

        function test_rail() {
            verify(rail.visible);
            compare(flick.contentY, 0);
            mouseClick(rail, 1, 90);
            tryCompare(flick, "contentY", 100);
            rail.scrollTo(1000);
            compare(flick.contentY, 300);
        }

        function test_railLayout() {
            compare(fleetRail.tileWidth, 64, "Ryoport's short labels keep the 64 tile");
            compare(fleetRail.newY, 168);
            compare(fleetRail.destinationY(0), 96);
            compare(fleetRail.destinationY(1), 168 + 64 + 8);
            compare(fleetRail.destinationY(2), 640 - 16 - 64);
            compare(wideRail.tileWidth, 80, "a long label widens every tile up to the rail less s1 a side");
            compare(wideRail.tileHeight(0), 64 + Lithic.type.caption.lineHeight, "a wrapped label grows its tile");
            compare(wideRail.tileHeight(1), 64);
            compare(wideRail.destinationY(1), 96 + 80 + 8, "tiles below a grown tile move down");
            wideRail.currentIndex = -1;
            compare(wideRail._shownIndex, -1);
            wideRail.currentIndex = 1;
            compare(wideRail._glide, false, "the tile reappears in place");
            wideRail.currentIndex = 2;
            compare(wideRail._glide, true);
            verify(findObject(wideRail, "hostMark") !== null, "the host mark reaches the rail's BrandMark");
            compare(keepConfirm.cancelText, "Revert");
        }

        function findObject(item, name) {
            if (!item)
                return null;
            if (item.objectName === name)
                return item;
            const kids = item.children || [];
            for (let i = 0; i < kids.length; ++i) {
                const found = findObject(kids[i], name);
                if (found)
                    return found;
            }
            return null;
        }

        function test_listEditor() {
            listEditor.openEditor(1);
            compare(listEditor.editingIndex, 1);
            const field = findObject(listEditor, "nameField");
            verify(field !== null);
            compare(field.text, "Discord", "the editor gets a copy of the item");
            field.forceActiveFocus();
            field.selectAll();
            keyClick(Qt.Key_V);
            keyClick(Qt.Key_Return);
            compare(editedSpy.count, 1, "Enter in a single-line field means Done");
            compare(editedSpy.signalArguments[0][0], 1);
            compare(editedSpy.signalArguments[0][1].title, "v");
            compare(listEditor.editingIndex, -1);
            compare(listEditor.model[1].title, "Discord", "the component never changes its model");

            listEditor.openEditor(0);
            findObject(listEditor, "nameField").forceActiveFocus();
            keyClick(Qt.Key_Escape);
            compare(listEditor.editingIndex, -1, "Escape cancels");
            compare(editedSpy.count, 1);

            listEditor.openEditor(listEditor.count);
            const fresh = findObject(listEditor, "nameField");
            compare(fresh.text, "", "a new item starts empty");
            verify(!listEditor.editorValid);
            listEditor.done();
            compare(addedSpy.count, 0, "Done waits for a valid editor");
            fresh.text = "Vesktop";
            verify(listEditor.editorValid);
            listEditor.done();
            compare(addedSpy.count, 1);
            compare(addedSpy.signalArguments[0][0].title, "Vesktop");

            listEditor.openEditor(2);
            listEditor.openEditor(0);
            compare(listEditor.editingIndex, 0, "one editor at a time");
            listEditor.cancel();

            listEditor.remove(2);
            compare(removedSpy.count, 1);
            compare(removedSpy.signalArguments[0][0], 2);

            const edit = findByName(listEditor, "Edit Discord");
            verify(edit !== null);
            edit.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Up, Qt.AltModifier);
            compare(movedSpy.count, 1, "Alt+Up moves the focused item");
            compare(movedSpy.signalArguments[0][0], 1);
            compare(movedSpy.signalArguments[0][1], 0);
            listEditor.model = [{ title: "Discord" }, { title: "Firefox" }, { title: "Ryotunes" }];
            tryVerify(function() { return findByName(listEditor, "Edit Discord").activeFocus; }, 1000,
                      "focus follows the moved item once the host reorders");
            keyClick(Qt.Key_Down, Qt.AltModifier);
            compare(movedSpy.signalArguments[1][0], 0);
            compare(movedSpy.signalArguments[1][1], 1);
            keyClick(Qt.Key_Up, Qt.AltModifier);
            compare(movedSpy.count, 2, "Alt+Up from the top row reports nothing");
        }

        function test_listEditorHeightAfterAdd() {
            listEditor.model = [{ title: "Firefox" }];
            waitForRendering(listEditor);
            wait(50);
            const closed = listEditor.implicitHeight;
            listEditor.openEditor(listEditor.count);
            tryVerify(function() { return listEditor.implicitHeight > closed; }, 1000);
            findObject(listEditor, "nameField").text = "Vesktop";
            listEditor.done();
            listEditor.model = [{ title: "Firefox" }, { title: "Vesktop" }];
            const rowStep = LithicTokens.size.settingsRowGap + LithicTokens.size.factsRowHeight;
            tryCompare(listEditor, "implicitHeight", closed + rowStep, 1000,
                       "the add editor's height goes when the new model arrives");
            listEditor.openEditor(0);
            listEditor.cancel();
            tryCompare(listEditor, "implicitHeight", closed + rowStep, 1000,
                       "a row editor's height goes on cancel");
        }

        function findByName(item, wanted) {
            if (!item)
                return null;
            if (item.accessibleName === wanted)
                return item;
            const kids = item.children || [];
            for (let i = 0; i < kids.length; ++i) {
                const found = findByName(kids[i], wanted);
                if (found)
                    return found;
            }
            return null;
        }

        function test_pathField() {
            compare(pathField.placeholder, "Choose a folder");
            const choose = findByName(pathField, "Choose Screenshots");
            verify(choose !== null);
            mouseClick(choose);
            compare(chooseSpy.count, 1);
            pathField.contentItem.children[0].forceActiveFocus();
            pathField.contentItem.children[0].text = "/home/ops/Shots";
            keyClick(Qt.Key_Return);
            compare(pathEditedSpy.count, 1);
            compare(pathEditedSpy.signalArguments[0][0], "/home/ops/Shots");
            pathField.kind = "file";
            compare(pathField.placeholder, "Choose a file");
        }

        function findText(item, wanted) {
            if (!item)
                return false;
            if (item.text === wanted && item.visible !== false)
                return true;
            const kids = item.children || [];
            for (let i = 0; i < kids.length; ++i) {
                if (findText(kids[i], wanted))
                    return true;
            }
            return false;
        }

        function test_translation() {
            const german = { "Changed": "Geändert", "Cancel": "Abbrechen", "Reset %1": "%1 zurücksetzen",
                             "Heavy": "Schwer", "%1, busy": "%1, beschäftigt", "Enter": "Eingabe" };
            verify(findText(row, "Changed"));
            verify(findText(confirmStage, "Cancel"));
            Lithic.translator = s => german[s] !== undefined ? german[s] : s;
            verify(findText(row, "Geändert"), "SettingRow mark follows the translator");
            verify(!findText(row, "Changed"));
            verify(findText(row, "Night light zurücksetzen") || findAccessible(row, "Night light zurücksetzen"));
            verify(findText(confirmStage, "Abbrechen"), "Confirm's Cancel follows the translator");
            verify(findText(tile, "Schwer"));
            compare(tile.Accessible.description, "Installing, beschäftigt");
            chord.chord = "Super+Return";
            compare(chord.keys, ["Super", "Eingabe"]);
            compare(chord.chord, "Super+Return", "chord text stays canonical");
            Lithic.translator = null;
            verify(findText(row, "Changed"), "null restores English");
            verify(findText(confirmStage, "Cancel"));
            verify(findText(tile, "Heavy"));
            compare(chord.keys, ["Super", "Enter"]);
        }

        function findAccessible(item, wanted) {
            if (!item)
                return false;
            if (item.accessibleName === wanted)
                return true;
            const kids = item.children || [];
            for (let i = 0; i < kids.length; ++i) {
                if (findAccessible(kids[i], wanted))
                    return true;
            }
            return false;
        }

        function test_gallery() {
            verify(gallery.columns >= 3);
            gallery.forceActiveFocus(Qt.TabFocusReason);
            gallery.currentIndex = 0;
            keyClick(Qt.Key_Right);
            compare(gallery.currentIndex, 1);
            keyClick(Qt.Key_Down);
            compare(gallery.currentIndex, 1 + gallery.columns < 5 ? 1 + gallery.columns : 1);
            keyClick(Qt.Key_End);
            compare(gallery.currentIndex, 4);
            keyClick(Qt.Key_Return);
            compare(galleryActivated.count, 1);
            compare(galleryActivated.signalArguments[0][0], 4);
        }
    }
}
