import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/keyboard"
import "../components/keyboard/keymap.js" as Keymap
import "../schema/keyboard.js" as Schema

// layouts, typing, special keys, and copying the layout to the sign-in screen,
// the console and the startup image
Item {
    id: root

    required property var hub

    // [{ code, name }] from the xkb rules this machine has
    property var layouts: []
    property var variants: []
    property bool variantsLoaded: false
    // a read that failed says so under the layouts, with Look again, instead of an empty list
    property bool layoutsFailed: false
    property bool variantsFailed: false
    property bool reading: false
    property string variantsFor: ""

    readonly property var store: Changes.stores["keyboard"] || null
    readonly property string first: String(Changes.value(root.kb("layout")) || "")
    readonly property string second: String(Changes.value(root.kb("second")) || "")
    readonly property string variant: String(Changes.value(root.kb("variant")) || "")
    readonly property string switchKeys: String(Changes.value(root.kb("switch")) || "")
    readonly property string phase: String(Changes.value(root.kb("everywhere")) || "")

    function kb(key) {
        return { store: "keyboard", key: key };
    }

    // a layout missing from the list (it could not be read) shows its code as people write it: "US"
    function layoutName(code) {
        if (!code)
            return "";
        var name = Keymap.nameOf(root.layouts, code);
        return name === code ? code.toUpperCase() : name;
    }

    function variantName(code) {
        return code ? Keymap.nameOf(root.variants, code) : I18n.tr("Default");
    }

    readonly property var variantOptions: [{ code: "", name: I18n.tr("Default") }].concat(root.variants)
    readonly property var secondOptions: [{ code: "", name: I18n.tr("None") }].concat(root.layouts.filter(l => l.code !== root.first))

    function prepared(row) {
        if ((row.id === "layout" || row.id === "second-layout") && root.layoutsFailed)
            return Object.assign({}, row, { unavailable: I18n.tr("The list of layouts could not be read") });
        if (row.id === "variant" && root.variantsFailed)
            return Object.assign({}, row, { unavailable: I18n.tr("The styles of this layout could not be read") });
        if (row.id === "variant" && root.variantsLoaded && root.variants.length === 0)
            return Object.assign({}, row, { unavailable: I18n.tr("This layout has no other styles") });
        return row;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.map(r => root.prepared(r))
    })

    readonly property string typing: {
        var name = root.layoutName(root.first);
        if (!name)
            return "";
        return root.variant ? I18n.tr("%1, %2").arg(name).arg(root.variantName(root.variant)) : name;
    }

    readonly property string stateLine: {
        if (!root.typing)
            return "";
        if (root.second && root.switchKeys)
            return I18n.tr("You type in %1. %2 switches to %3.").arg(root.typing)
                .arg(Changes.value(root.kb("switch")) === "grp:win_space_toggle" ? "Super+Space" : "Alt+Shift")
                .arg(root.layoutName(root.second));
        if (root.second)
            return I18n.tr("You type in %1, with %2 kept ready.").arg(root.typing).arg(root.layoutName(root.second));
        return I18n.tr("You type in %1.").arg(root.typing);
    }

    // the layout other places use, in words; "" when that place records none
    function placeName(code) {
        if (!code)
            return I18n.tr("Not set, so it types US English");
        return root.layoutName(code);
    }

    readonly property var status: root.store ? root.store.status : null
    readonly property bool allAgree: {
        var s = root.status;
        if (!s || !root.first)
            return false;
        var greeter = !s.greeter || s.greeter === root.first;
        var tty = !s.console_as_layout || s.console_as_layout === root.first;
        return greeter && tty && !s.boot_stale;
    }

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    function copyEverywhere() {
        var row = Schema.rows.find(r => r.id === "everywhere");
        Changes.write(row.bind, Date.now(), row.label, "keyboard");
    }

    // the xkb lists, read with a deadline; a missing or failing ryoku-hub reads as a failure
    function loadLayouts() {
        root.layoutsFailed = false;
        root.reading = true;
        Changes.run(["ryoku-hub", "desktop", "layouts"], function (ok, out) {
            root.reading = false;
            try {
                var list = ok ? JSON.parse(out) : null;
                if (!Array.isArray(list))
                    throw "not a list";
                root.layouts = list;
            } catch (e) {
                root.layoutsFailed = true;
            }
        }, { timeout: 15000 });
    }

    function refreshVariants(again) {
        if (root.first.length === 0 || (!again && root.variantsFor === root.first))
            return;
        var wanted = root.first;
        root.variantsFor = wanted;
        root.variantsLoaded = false;
        root.variantsFailed = false;
        Changes.run(["ryoku-hub", "desktop", "variants", wanted], function (ok, out) {
            // an answer for a layout chosen before the current one is dropped
            if (root.variantsFor !== wanted)
                return;
            var found = [];
            try {
                var list = ok ? JSON.parse(out) : null;
                if (!Array.isArray(list))
                    throw "not a list";
                for (var i = 0; i < list.length; i++)
                    found.push({ code: String(list[i].code), name: Keymap.variantLabel(list[i].name) });
            } catch (e) {
                root.variantsFailed = true;
            }
            root.variants = found;
            root.variantsLoaded = true;
        }, { timeout: 15000 });
    }
    onFirstChanged: root.refreshVariants(false)

    function lookAgain() {
        root.loadLayouts();
        root.refreshVariants(true);
    }

    Component.onCompleted: {
        Changes.ensureStore("keyboard", Qt.resolvedUrl("../components/keyboard/KeyboardStore.qml"));
        if (root.store)
            root.store.refreshStatus();
        root.loadLayouts();
        root.refreshVariants(false);
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "keyboard"
        schema: root.schema
        stateLine: root.stateLine
        controls: ({ "layout": layoutControl, "variant": variantControl, "second-layout": secondControl, "everywhere": copyControl })
        footers: root.layoutsFailed || root.variantsFailed ? ({ layouts: readFailed, everywhere: everywhereFooter })
                                                         : ({ everywhere: everywhereFooter })
    }

    LayoutChooser {
        id: chooser
        parent: root.hub.overlayItem
    }

    component Choice: Plaque {
        property var row
        property var value
        property var commit
        property string current: String(value === undefined || value === null ? "" : value)
        kind: "tonal"
    }

    Component {
        id: layoutControl
        Choice {
            id: layoutChoice
            text: root.layoutName(layoutChoice.current) || I18n.tr("Choose")
            accessibleName: I18n.tr("Layout, %1. Choose another").arg(layoutChoice.text)
            onClicked: layoutChoice.activate()
            function activate() {
                chooser.openWith(I18n.tr("Keyboard layout"), root.layouts, layoutChoice.current, code => layoutChoice.commit(code), layoutChoice);
            }
        }
    }

    Component {
        id: variantControl
        Choice {
            id: variantChoice
            text: root.variantName(variantChoice.current)
            accessibleName: I18n.tr("Layout style, %1. Choose another").arg(variantChoice.text)
            onClicked: variantChoice.activate()
            function activate() {
                chooser.openWith(I18n.tr("Style of %1").arg(root.layoutName(root.first)), root.variantOptions, variantChoice.current,
                                 code => variantChoice.commit(code), variantChoice);
            }
        }
    }

    Component {
        id: secondControl
        Choice {
            id: secondChoice
            text: secondChoice.current ? root.layoutName(secondChoice.current) : I18n.tr("None")
            accessibleName: I18n.tr("Second layout, %1. Choose another").arg(secondChoice.text)
            onClicked: secondChoice.activate()
            function activate() {
                chooser.openWith(I18n.tr("Second layout"), root.secondOptions, secondChoice.current, code => secondChoice.commit(code), secondChoice);
            }
        }
    }

    Component {
        id: copyControl
        Row {
            spacing: LithicTokens.space.s2

            Plaque {
                visible: root.phase === "running" && root.store && root.store.verySlow
                kind: "quiet"
                small: true
                text: I18n.tr("Cancel")
                onClicked: root.store.cancel()
            }
            Plaque {
                kind: "line"
                small: true
                glyph: "content_copy"
                text: root.phase === "running" ? I18n.tr("Copying") : I18n.tr("Copy")
                busy: root.phase === "running" && root.store !== null && root.store.slow
                enabled: root.phase !== "running" && root.first.length > 0
                accessibleName: I18n.tr("Copy your layout to the sign-in screen, the console and startup")
                onClicked: root.copyEverywhere()
            }
        }
    }

    Component {
        id: readFailed
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.parent ? parent.parent.width - lookAgain.width - parent.spacing : implicitWidth)
                text: I18n.tr("The keyboard layouts could not be read. Run ryoku doctor in a terminal, then look again.")
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: lookAgain
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Look again")
                busy: root.reading
                onClicked: root.lookAgain()
            }
        }
    }

    Component {
        id: everywhereFooter
        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: {
                    if (root.phase === "running")
                        return root.store && root.store.verySlow
                            ? I18n.tr("Rebuilding the startup image so the disk unlock prompt follows. This can take a few minutes.")
                            : I18n.tr("Copying your layout. Enter your password if it is asked for.");
                    if (root.phase === "stopped")
                        return I18n.tr("Stopped waiting. A step already under way may still finish; the list below shows where each place stands.");
                    if (root.phase === "done" && root.allAgree)
                        return I18n.tr("Done. They all use %1 now.").arg(root.layoutName(root.first));
                    if (!root.status)
                        return I18n.tr("Each place keeps its own layout until you copy yours there.");
                    if (root.allAgree)
                        return I18n.tr("They already use %1.").arg(root.layoutName(root.first));
                    return I18n.tr("Not all of them use %1. Copy it to bring them in line.").arg(root.layoutName(root.first));
                }
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            BladeProgress {
                visible: root.phase === "running" && root.store !== null && root.store.verySlow
                width: parent.width
                active: visible
                indeterminate: true
            }

            Facts {
                visible: root.status !== null
                width: parent.width
                facts: root.status ? [
                    { label: I18n.tr("Sign-in screen"), value: root.placeName(root.status.greeter) },
                    { label: I18n.tr("Text console"), value: root.status.console ? root.layoutName(root.status.console_as_layout || root.status.console) : root.placeName("") },
                    { label: I18n.tr("Disk unlock"), value: !root.status.boot_image ? I18n.tr("No startup image found")
                        : root.status.boot_stale ? I18n.tr("Still has an older layout") : I18n.tr("Same as the console") }
                ] : []
            }
        }
    }
}
