import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../Singletons"

// the control on the right of a setting row, chosen by the row's `control`
Item {
    id: root

    required property var row
    property string pageId: ""
    property Item overlayParent: null
    // the frame, for link rows that open another page
    property var hub: null
    // a control the page draws itself, for control: "custom"; its root may
    // declare `property var row` (set on load) and `function activate()`
    property Component custom: null
    readonly property var value: root.row ? Changes.value(root.row.bind) : undefined
    readonly property real maxWidth: LithicTokens.size.settingsControlMax

    // the width the control really takes, so the row's text wraps before it
    implicitWidth: loader.item ? loader.item.width : 0
    implicitHeight: loader.item ? loader.item.implicitHeight : 0

    function commit(v) {
        Changes.set(root.row.bind, v, root.row.changeLabel || root.row.label, root.pageId);
        resyncLater.restart();
    }

    // dies with the row, so a commit that rebuilds the section never calls into a gone control
    Timer {
        id: resyncLater
        interval: 0
        onTriggered: root.resync()
    }

    // the bindings below hold every control to the stored value; turning them
    // off and on re-applies it when a write failed and the value never moved
    property bool bound: true
    readonly property string writeError: root.row ? Changes.error(root.row.bind) : ""
    onWriteErrorChanged: root.resync()
    function resync() {
        root.bound = false;
        root.bound = true;
    }

    // the toggle row is clickable as a whole
    function activate() {
        if (root.row.control === "toggle")
            root.commit(root.value !== true);
        else if (root.row.control === "handoff" || root.row.control === "action")
            // an app of its own (its own scope), so it outlives Settings closing
            Spawn.runApp(root.row.command, { label: String(root.row.command[0]).replace(/^.*\//, "") });
        else if (root.row.control === "link" && root.hub)
            root.hub.navigate(root.row.link, "");
        else if (root.row.control === "custom" && loader.item && typeof loader.item.activate === "function")
            loader.item.activate();
    }

    // keyboard focus on the control itself (a slider or path sits inside a wrapper); reason
    // defaults to the keyboard's
    function focusControl(reason) {
        var why = reason === undefined ? Qt.TabFocusReason : reason;
        var item = loader.item;
        // a control that is off (view only) takes no focus; Qt would pass it on down the page
        if (!item || !item.enabled || !root.enabled)
            return;
        if ((root.row.control === "slider" || root.row.control === "path") && item.children.length > 0)
            item = item.children[0];
        // a wrapper that is no Tab stop itself (a path field) hands focus to its first one, so
        // Tab carries on from inside it
        if (!item.activeFocusOnTab) {
            var inner = item.nextItemInFocusChain(true);
            for (var p = inner; p && inner !== item; p = p.parent) {
                if (p === item) {
                    inner.forceActiveFocus(why);
                    return;
                }
            }
        }
        item.forceActiveFocus(why);
    }

    function optionIndex() {
        var opts = root.row.options || [];
        for (var i = 0; i < opts.length; i++)
            if (JSON.stringify(opts[i].value) === JSON.stringify(root.value))
                return i;
        return -1;
    }

    function optionLabels() {
        return (root.row.options || []).map(o => I18n.tr(o.label));
    }

    function shown(v) {
        if (v === undefined || v === null)
            return "";
        if (root.row.pct)
            return Math.round(Number(v) * 100) + "%";
        var n = Number(v);
        var digits = root.row.step !== undefined && root.row.step < 0.01 ? 3 : 2;
        var text = Number.isInteger(n) ? String(n) : n.toFixed(digits).replace(/0+$/, "").replace(/\.$/, "");
        if (!root.row.unit)
            return text;
        return root.row.unit === "%" ? text + "%" : text + " " + root.row.unit;
    }

    Loader {
        id: loader
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        sourceComponent: {
            switch (root.row ? root.row.control : "") {
            case "toggle": return toggleControl;
            case "segmented": return segmentedControl;
            case "slider": return sliderControl;
            case "stepper": return stepperControl;
            case "picker": return pickerControl;
            case "text": return textControl;
            case "color": return colorControl;
            case "chord": return chordControl;
            case "action":
            case "handoff":
            case "link": return openControl;
            case "custom": return root.custom || root.row.component || null;
            case "path": return pathControl;
            }
            return null;
        }
        // a custom control may declare row, value and commit; value follows the store
        onLoaded: {
            root.takesValue = root.row.control === "custom" && "value" in loader.item;
            if (root.row.control !== "custom")
                return;
            if ("row" in loader.item)
                loader.item.row = root.row;
            if ("commit" in loader.item)
                loader.item.commit = root.commit;
        }
    }
    property bool takesValue: false

    Binding {
        target: loader.item
        property: "value"
        value: root.value
        when: root.bound && root.takesValue
    }

    Component {
        id: toggleControl
        Toggle {
            id: toggle
            Accessible.name: I18n.tr(root.row.label)
            onToggled: root.commit(checked)
            // controls assign their own state on input; the store keeps driving them, so Undo shows
            Binding {
                target: toggle
                property: "checked"
                value: root.value === true
                when: root.bound
            }
        }
    }

    Component {
        id: segmentedControl
        Segmented {
            id: seg
            width: Math.min(implicitWidth, root.maxWidth)
            model: root.optionLabels()
            accessibleName: I18n.tr(root.row.label)
            onActivated: index => root.commit(root.row.options[index].value)
            // the control assigns its own index on click, so the store keeps driving it
            Binding {
                target: seg
                property: "currentIndex"
                value: Math.max(0, root.optionIndex())
                when: root.bound
            }
        }
    }

    Component {
        id: pickerControl
        Picker {
            id: picker
            // wide enough for the widest option as drawn (scripts differ in width per letter),
            // within the control column
            width: Math.min(Math.max(picker.widest + 2 * LithicTokens.size.pickerInset + LithicTokens.size.pickerArrowGap
                                     + LithicTokens.size.pickerArrow, LithicTokens.size.pickerWidthMin), root.maxWidth)
            property real widest: 0
            function measure() {
                var w = 0;
                (picker.options || []).forEach(o => {
                    metrics.text = String(o);
                    w = Math.max(w, metrics.advanceWidth);
                });
                picker.widest = Math.ceil(w);
            }
            Label {
                id: fontSource
                visible: false
                typeRole: "body"
            }
            TextMetrics {
                id: metrics
                font: fontSource.font
            }
            options: root.optionLabels()
            onOptionsChanged: picker.measure()
            Component.onCompleted: picker.measure()
            Connections {
                target: fontSource
                function onFontChanged() { picker.measure(); }
            }
            Binding {
                target: picker
                property: "currentIndex"
                value: root.optionIndex()
                when: root.bound
            }
            overlayParent: root.overlayParent || null
            Accessible.name: I18n.tr(root.row.label)
            onActivated: index => root.commit(root.row.options[index].value)
        }
    }

    Component {
        id: sliderControl
        Row {
            spacing: LithicTokens.space.s3
            Slider {
                id: slider
                anchors.verticalCenter: parent.verticalCenter
                width: 200
                from: root.row.min !== undefined ? root.row.min : 0
                to: root.row.max !== undefined ? root.row.max : 1
                stepSize: root.row.step !== undefined ? root.row.step : 0
                Binding {
                    target: slider
                    property: "value"
                    value: Number(root.value) || 0
                    when: root.bound && !slider.pressed
                }
                accessibleName: I18n.tr(root.row.label)
                onMoved: root.commit(Math.round(value * 1000) / 1000)
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: 72
                horizontalAlignment: Text.AlignRight
                text: root.shown(slider.value)
                typeRole: "label"
                inkRole: "fg2"
                numeric: true
            }
        }
    }

    Component {
        id: stepperControl
        Stepper {
            id: stepper
            from: root.row.min !== undefined ? root.row.min : 0
            to: root.row.max !== undefined ? root.row.max : 100
            stepSize: root.row.step !== undefined ? root.row.step : 1
            unit: root.row.unit ? root.row.unit : ""
            editable: true
            accessibleName: I18n.tr(root.row.label)
            onValueModified: root.commit(value)
            Binding {
                target: stepper
                property: "value"
                value: Math.round(Number(root.value) || 0)
                when: root.bound
            }
        }
    }

    Component {
        id: textControl
        TextField {
            id: textField
            // only text the user typed is ever written; until then the field follows the store,
            // focused or not, so a field focused before its value arrived never writes ""
            property bool edited: false
            width: root.maxWidth
            placeholderText: root.row.placeholder ? root.row.placeholder : ""
            mono: root.row.mono === true
            Accessible.name: I18n.tr(root.row.label)
            onTextEdited: textField.edited = true
            onEditingFinished: {
                if (!textField.edited)
                    return;
                var typed = textField.text;
                if (typed !== String(root.value === undefined || root.value === null ? "" : root.value))
                    root.commit(typed);
                textField.edited = false;
            }
            Binding {
                target: textField
                property: "text"
                value: root.value === undefined || root.value === null ? "" : String(root.value)
                when: root.bound && !textField.edited
            }
        }
    }

    Component {
        id: colorControl
        ColorField {
            id: colorField
            width: 220
            overlayParent: root.overlayParent || null
            accessibleName: I18n.tr(root.row.label)
            swatches: [
                { label: I18n.tr("Accent"), color: Lithic.gold },
                { label: I18n.tr("Jade"), color: Lithic.jade },
                { label: I18n.tr("Lapis"), color: Lithic.lapis },
                { label: I18n.tr("Amber"), color: Lithic.amber },
                { label: I18n.tr("Ink"), color: Lithic.fg }
            ]
            onPicked: c => root.commit(colorField.formatHex(c))
            Binding {
                target: colorField
                property: "color"
                value: root.value ? String(root.value) : "#000000"
                when: root.bound
            }
        }
    }

    Component {
        id: chordControl
        ChordField {
            id: chordField
            Binding {
                target: chordField
                property: "chord"
                value: root.value ? String(root.value) : ""
                when: root.bound
            }
            accessibleName: I18n.tr(root.row.label)
            onRecorded: chord => root.commit(chord)
            onCleared: root.commit("")
        }
    }

    Component {
        id: openControl
        Plaque {
            kind: "quiet"
            small: true
            glyph: root.row.control === "handoff" ? "open_in_new" : root.row.control === "link" ? "arrow_forward" : ""
            text: root.row.actionText ? I18n.tr(root.row.actionText) : I18n.tr("Open")
            accessibleName: I18n.tr(root.row.label)
            onClicked: root.activate()
        }
    }

    // a file or folder: row.kind "file"|"folder", row.nameFilters, row.asUrl stores file:// urls
    Component {
        id: pathControl
        Item {
            implicitWidth: field.implicitWidth
            implicitHeight: field.implicitHeight
            width: root.maxWidth

            function stored(p) {
                return p.length > 0 && root.row.asUrl === true ? "file://" + p : p;
            }

            PathField {
                id: field
                width: parent.width
                kind: root.row.kind === "file" ? "file" : "folder"
                Binding {
                    target: field
                    property: "path"
                    value: root.value ? String(root.value).replace(/^file:\/\//, "") : ""
                    when: root.bound
                }
                placeholder: root.row.placeholder ? I18n.tr(root.row.placeholder)
                    : kind === "file" ? I18n.tr("Choose a file") : I18n.tr("Choose a folder")
                accessibleName: I18n.tr(root.row.label)
                onChooseRequested: field.kind === "folder" ? folderChooser.open() : fileDialog.open()
                onEdited: p => root.commit(parent.stored(p))
            }
            RyokuUi.FileDialogHost {
                id: fileDialog
                title: I18n.tr(root.row.label)
                nameFilters: root.row.nameFilters || []
                onAccepted: root.commit(parent.stored(decodeURIComponent(String(fileDialog.selectedFile).replace(/^file:\/\//, ""))))
            }
            // Qt's folder dialog saves the first subfolder it lists, so folders use the chooser that takes the shown one
            FolderChooser {
                id: folderChooser
                title: I18n.tr(root.row.label)
                currentFolder: field.path.indexOf("/") === 0 ? "file://" + field.path.split("/").map(encodeURIComponent).join("/") : field.path
                overlayParent: root.hub ? root.hub.overlayItem : null
                onAccepted: root.commit(parent.stored(decodeURIComponent(String(folderChooser.selectedFolder).replace(/^file:\/\//, ""))))
            }
        }
    }
}
