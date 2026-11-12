import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/animations"
import "../registry/provider-map.js" as ProviderMap
import "../components/animations/anim.js" as Anim
import "../schema/animations.js" as Schema

// how fast and how much the desktop moves, and the window manager's own animations
Item {
    id: root

    required property var hub

    // the provider's rows placed on this page, raw, with their place
    readonly property var placed: {
        ProviderSchema.revision;
        var out = [];
        (ProviderSchema.rows || []).forEach(r => {
            var p = ProviderMap.placeOf(r);
            if (p.known && p.place && p.place[0] === "animations")
                out.push({ raw: r, section: p.place[1], row: ProviderMap.normalize(r, p.place) });
        });
        return out;
    }

    // the animation tree a provider lets you edit leaf by leaf, or ""
    readonly property string base: Anim.curveBase(root.placed.filter(p => p.section === "curves").map(p => p.raw))
    readonly property var rawRows: root.placed.map(p => p.raw)

    readonly property var leaves: {
        Changes.revision;
        if (root.base === "" || !Changes.desktopDefaultsReady)
            return [];
        var list = Changes.pathGet(Changes.desktopDefaults, Changes.storePath(root.base + ".items"));
        return Array.isArray(list) ? list.filter(it => it && it.leaf) : [];
    }
    readonly property var curveOptions: {
        Changes.revision;
        if (root.base === "")
            return [];
        var key = Changes.storePath(root.base + ".curves");
        var defaults = Changes.desktopDefaultsReady ? Changes.pathGet(Changes.desktopDefaults, key) : [];
        return Anim.names(defaults, Changes.value({ store: "desktop", key: root.base + ".curves" }))
            .map(n => ({ value: n, label: Anim.curveLabel(n) }));
    }

    // provider groups of more than one row, drawn as one row with an Edit dialog
    readonly property var groups: {
        var by = {}, order = [];
        root.placed.forEach(p => {
            if ((p.section !== "per-animation" && p.section !== "open-close") || !p.row.control || !p.raw.group
                    || String(p.raw.key || "").indexOf("[") >= 0)
                return;
            var k = p.section + "|" + p.raw.group;
            if (!by[k]) {
                by[k] = { section: p.section, group: p.raw.group, rows: [] };
                order.push(k);
            }
            // a change says which animation it touched: "Workspace switch changed", not "Style changed"
            by[k].rows.push(Object.assign({}, p.row, { changeLabel: Anim.groupName(p.raw.group) }));
        });
        return order.map(k => by[k]).filter(g => g.rows.length > 1);
    }
    readonly property var grouped: {
        var keys = {};
        root.groups.forEach(g => g.rows.forEach(r => keys[r.id] = root.groupRowId(g)));
        return keys;
    }

    function groupRowId(g) {
        return "anim-" + Anim.slug(g.section + " " + g.group);
    }

    readonly property bool hasStyle: root.base !== ""

    // leaf rows and group rows of the style section, under More options when
    // the section has its own style to pick
    readonly property var syntheticRows: {
        var out = [];
        root.leaves.forEach(it => {
            var name = Anim.leafName(root.rawRows, it.leaf);
            out.push({
                id: "anim-" + Anim.slug(it.leaf), section: "style", label: name,
                changeLabel: name, keywords: ["animation", it.leaf],
                control: "custom", component: leafControl, leaf: it.leaf, more: true, activatable: true,
                bind: { store: "anim-leaves", key: it.leaf },
                "default": Anim.leafShape(it.leaf, it)
            });
        });
        root.groups.forEach(g => {
            var target = g.section === "per-animation" ? "style" : g.section;
            out.push({
                id: root.groupRowId(g), section: target, label: Anim.groupName(g.group),
                keywords: g.rows.map(r => r.label), control: "custom", component: groupControl,
                activatable: true, groupRows: g.rows, more: target === "style" && root.hasStyle
            });
        });
        return out;
    }

    function adapt(rows) {
        var kept = rows.filter(r => {
            if (r.id === "anim-preset")
                return root.hasStyle;
            if (!r.provider)
                return true;
            if (r.section === "curves" || r.section === "per-animation" || r.section === "style")
                return false;
            return root.grouped[r.id] === undefined;
        });
        return kept.concat(root.syntheticRows);
    }

    // performance settings that hold motion reduced whatever this switch says
    readonly property var performance: Live.config("performance.json")
    readonly property var powerProfile: Live.topic("powerprofiles")
    readonly property bool lowPower: !!root.performance && root.performance.lowPowerMode === true
    readonly property bool saver: !!root.powerProfile && root.powerProfile.active_profile === "power-saver"
        && !(root.performance && root.performance.powerProfileEffects === false)

    // with no focus flash rows at all the section still says why it is empty
    readonly property bool flashKnown: root.placed.some(p => p.section === "focus-flash")

    function focusRow(rowId) {
        if (root.grouped[rowId] !== undefined) {
            page.focusRow(root.grouped[rowId]);
            return;
        }
        if (rowId === "each-animation") {
            var first = root.syntheticRows.find(r => r.section === "style");
            page.focusRow(first ? first.id : "anim-preset");
            return;
        }
        if (rowId === "anim-preset" && !root.hasStyle) {
            page.focusRow("each-animation");
            return;
        }
        if (rowId === "reset-animations" && root.hasStyle) {
            page.focusRow("curves");
            resetLand.restart();
            return;
        }
        page.focusRow(rowId);
    }

    Timer {
        id: resetLand
        interval: LithicTokens.motion.unfold + LithicTokens.motion.navigate + 120
        onTriggered: {
            var sec = page.sectionItems["style"];
            if (sec && sec.moreBodyItem)
                page.reveal(sec.moreBodyItem.resetRow);
        }
    }

    function ensureStores() {
        if (!root.hasStyle)
            return;
        Changes.ensureStore("animations", Qt.resolvedUrl("../components/animations/PresetStore.qml"));
        Changes.ensureStore("anim-leaves", Qt.resolvedUrl("../components/animations/LeafStore.qml"));
        var leaves = Changes.stores["anim-leaves"];
        if (leaves)
            leaves.itemsKey = root.base + ".items";
    }

    onHasStyleChanged: root.ensureStores()
    Component.onCompleted: root.ensureStores()

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "animations"
        schema: Schema
        adaptRows: rows => root.adapt(rows)
        footers: {
            var f = {};
            if (root.lowPower || root.saver)
                f.reduce = reduceNote;
            if (!root.flashKnown)
                f["focus-flash"] = noFlash;
            return f;
        }
        moreBodies: root.hasStyle ? { style: workshop } : ({})
    }

    Component {
        id: reduceNote
        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: root.lowPower
                    ? I18n.tr("Low power mode keeps motion reduced while it is on, whatever this switch says.")
                    : I18n.tr("Power saver keeps motion reduced while it is on, whatever this switch says.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                kind: "quiet"
                small: true
                glyph: "arrow_forward"
                text: root.lowPower ? I18n.tr("Open Performance") : I18n.tr("Open Power & battery")
                onClicked: root.hub.navigate(root.lowPower ? "performance" : "power", "")
            }
        }
    }

    Component {
        id: noFlash
        Label {
            text: I18n.tr("Focus flash needs a window manager plugin that draws it.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: workshop
        CurveWorkshop {
            base: root.base
            overlayParent: root.hub.overlayItem
        }
    }

    // a leaf's state at a glance, its switch, and Edit for the rest
    Component {
        id: leafControl
        Row {
            id: leafCtl
            property var row: null
            property var value
            property var commit: null
            spacing: LithicTokens.space.s3

            function activate() {
                leafDialog.open();
            }

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, 150)
                text: !leafCtl.value ? "" : !leafCtl.value.enabled ? I18n.tr("Off")
                    : I18n.tr("%1 ms").arg(Math.round(Number(leafCtl.value.speed) * 100))
                      + (leafCtl.value.bezier ? ", " + Anim.curveLabel(leafCtl.value.bezier) : "")
                typeRole: "detail"
                inkRole: "fg2"
            }
            Toggle {
                anchors.verticalCenter: parent.verticalCenter
                checked: !!leafCtl.value && leafCtl.value.enabled === true
                Accessible.name: leafCtl.row ? I18n.tr(leafCtl.row.label) : ""
                onToggled: {
                    if (leafCtl.value && typeof leafCtl.commit === "function")
                        leafCtl.commit(Object.assign({}, leafCtl.value, { enabled: checked }));
                }
            }
            IconPlaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "tune"
                accessibleName: leafCtl.row ? I18n.tr("Edit %1").arg(I18n.tr(leafCtl.row.label)) : ""
                onClicked: leafCtl.activate()
            }

            LeafDialog {
                id: leafDialog
                parent: root.hub.overlayItem
                row: leafCtl.row
                curves: root.curveOptions
                styles: leafCtl.row ? Anim.styleOptions(root.rawRows, leafCtl.row.leaf, leafCtl.value ? leafCtl.value.style : undefined) : []
                overlayParent: root.hub.overlayItem
            }
        }
    }

    // a group's state at a glance and Edit, which opens its settings
    Component {
        id: groupControl
        Row {
            id: groupCtl
            property var row: null
            spacing: LithicTokens.space.s3

            function activate() {
                groupDialog.open();
            }

            readonly property string summary: {
                Changes.revision;
                if (!groupCtl.row)
                    return "";
                return Anim.groupSummary(groupCtl.row.groupRows, b => Changes.value(b), r => groupDialog.shows(r), s => I18n.tr(s));
            }

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, 200)
                text: groupCtl.summary
                typeRole: "detail"
                inkRole: "fg2"
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "tune"
                text: I18n.tr("Edit")
                accessibleName: groupCtl.row ? I18n.tr("Edit %1").arg(I18n.tr(groupCtl.row.label)) : ""
                onClicked: groupCtl.activate()
            }

            GroupDialog {
                id: groupDialog
                parent: root.hub.overlayItem
                heading: groupCtl.row ? groupCtl.row.label : ""
                rows: groupCtl.row ? groupCtl.row.groupRows : []
                hub: root.hub
            }
        }
    }
}
