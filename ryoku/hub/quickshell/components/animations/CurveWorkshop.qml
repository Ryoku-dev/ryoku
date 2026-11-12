import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "anim.js" as Anim

// the window manager's named easing curves: pick one, start from a shape,
// drag or type its handles, and watch it play; animations using it follow
Column {
    id: shop

    // the provider's animation tree in the desktop store, such as "wm.x.anim"
    property string base: ""
    property Item overlayParent: null

    readonly property var curvesBind: ({ store: "desktop", key: shop.base + ".curves" })
    readonly property var baseBind: ({ store: "desktop", key: shop.base })
    readonly property var defaults: {
        Changes.revision;
        var v = Changes.desktopDefaultsReady ? Changes.pathGet(Changes.desktopDefaults, Changes.storePath(shop.curvesBind.key)) : null;
        return Array.isArray(v) ? v : [];
    }
    readonly property var draft: {
        var v = Changes.value(shop.curvesBind);
        return Array.isArray(v) ? v : [];
    }
    readonly property var names: Anim.names(shop.defaults, shop.draft)
    property string selected: ""
    readonly property string current: shop.names.indexOf(shop.selected) >= 0 ? shop.selected : (shop.names.length > 0 ? shop.names[0] : "")
    readonly property var curve: Anim.curveOf(shop.defaults, shop.draft, shop.current)
    readonly property var builtIn: Anim.find(shop.defaults, shop.current)
    readonly property bool custom: shop.current !== "" && shop.builtIn === null
    readonly property bool edited: shop.builtIn !== null && Anim.find(shop.draft, shop.current) !== null
        && !Anim.sameShape(Anim.curveOf([], shop.draft, shop.current), Anim.curveOf(shop.defaults, [], shop.current))
    // where a search for resetting lands
    readonly property Item resetRow: resetLine

    function label() {
        return I18n.tr("Curve %1").arg(Anim.curveLabel(shop.current));
    }

    function setShape(x0, y0, x1, y1) {
        if (shop.current === "")
            return;
        var next = Anim.upsert(shop.draft, { name: shop.current, x0: x0, y0: y0, x1: x1, y1: y1 });
        Changes.set(shop.curvesBind, next, shop.label(), "animations");
    }

    function addCurve() {
        var name = Anim.freshName(shop.names);
        Changes.set(shop.curvesBind, Anim.upsert(shop.draft, { name: name, x0: 0.25, y0: 0.1, x1: 0.25, y1: 1 }),
                    I18n.tr("Curve %1").arg(Anim.curveLabel(name)), "animations", I18n.tr("Added curve %1").arg(Anim.curveLabel(name)));
        shop.selected = name;
    }

    // a custom curve goes; a built-in one loses its changes
    function dropCurve() {
        var name = shop.current;
        var shown = Anim.curveLabel(name);
        Changes.set(shop.curvesBind, Anim.without(shop.draft, name), I18n.tr("Curve %1").arg(shown), "animations",
                    shop.custom ? I18n.tr("Deleted curve %1").arg(shown) : I18n.tr("Curve %1 is back to its default").arg(shown));
        if (shop.custom)
            shop.selected = "";
    }

    function resetAll() {
        var now = Changes.value(shop.baseBind);
        var next = Object.assign({}, now && typeof now === "object" ? now : {}, { items: [], curves: [] });
        Changes.set(shop.baseBind, next, I18n.tr("Every animation"), "animations", I18n.tr("Every animation is back to its default"));
    }

    function typed(field, text) {
        var n = Number(String(text).replace(",", "."));
        if (isNaN(n))
            return false;
        var c = shop.curve;
        var v = { x0: c.x0, y0: c.y0, x1: c.x1, y1: c.y1 };
        v[field] = field.charAt(0) === "x" ? Math.max(0, Math.min(1, n)) : Math.max(-1, Math.min(2, n));
        shop.setShape(v.x0, v.y0, v.x1, v.y1);
        return true;
    }

    spacing: LithicTokens.space.s4
    topPadding: LithicTokens.space.s3

    Label {
        width: parent.width
        text: I18n.tr("Curves shape how an animation speeds up and settles. Every animation that uses a curve follows your edits at once.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2

        Picker {
            id: picker
            width: Math.min(260, shop.width)
            options: shop.names.map(n => Anim.curveLabel(n))
            currentIndex: shop.names.indexOf(shop.current)
            overlayParent: shop.overlayParent
            Accessible.name: I18n.tr("Curve")
            onActivated: index => shop.selected = shop.names[index]
        }
        Plaque {
            kind: "quiet"
            glyph: "add"
            text: I18n.tr("New curve")
            onClicked: shop.addCurve()
        }
        Plaque {
            kind: "quiet"
            glyph: shop.custom ? "delete" : "restart_alt"
            text: shop.custom ? I18n.tr("Delete curve") : I18n.tr("Reset curve")
            enabled: shop.custom || shop.edited
            onClicked: shop.dropCurve()
        }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s2

        Label {
            text: I18n.tr("Start from a shape")
            typeRole: "detail"
            inkRole: "fg2"
        }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2

            Repeater {
                model: Anim.feels
                delegate: Plaque {
                    required property var modelData
                    small: true
                    kind: Anim.feelMatches(shop.curve, modelData.c) ? "on" : "tonal"
                    text: I18n.tr(modelData.name)
                    onClicked: shop.setShape(modelData.c[0], modelData.c[1], modelData.c[2], modelData.c[3])
                }
            }
        }
    }

    MotionPreview {
        width: parent.width
        x0: shop.curve.x0
        y0: shop.curve.y0
        x1: shop.curve.x1
        y1: shop.curve.y1
    }

    Flow {
        width: parent.width
        spacing: LithicTokens.space.s4

        CurveEditor {
            id: editor
            width: Math.min(360, shop.width)
            height: 300
            x0: shop.curve.x0
            y0: shop.curve.y0
            x1: shop.curve.x1
            y1: shop.curve.y1
            onMoved: (a, b, c, d) => shop.setShape(a, b, c, d)
        }

        Column {
            width: Math.min(260, shop.width)
            spacing: LithicTokens.space.s3

            Repeater {
                model: [
                    { title: I18n.tr("First handle"), x: "x0", y: "y0" },
                    { title: I18n.tr("Second handle"), x: "x1", y: "y1" }
                ]
                delegate: Column {
                    id: pair
                    required property var modelData
                    width: parent.width
                    spacing: LithicTokens.space.s1

                    Label {
                        text: pair.modelData.title
                        typeRole: "row"
                    }
                    Row {
                        spacing: LithicTokens.space.s2
                        HandleField {
                            label: I18n.tr("Time, 0 to 1")
                            field: pair.modelData.x
                        }
                        HandleField {
                            label: I18n.tr("Progress")
                            field: pair.modelData.y
                        }
                    }
                }
            }
            Label {
                width: parent.width
                text: I18n.tr("Drag the handles or type their place. Progress above 1 overshoots, below 0 pulls back first.")
                typeRole: "caption"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Item {
        width: parent.width
        height: LithicTokens.size.hairline
        Rectangle {
            anchors.fill: parent
            color: Lithic.vein
        }
    }

    Row {
        id: resetLine
        width: parent.width
        spacing: LithicTokens.space.s4

        Column {
            width: parent.width - resetPlaque.width - parent.spacing
            anchors.verticalCenter: parent.verticalCenter
            Label {
                width: parent.width
                text: I18n.tr("Reset every animation")
                typeRole: "row"
            }
            Label {
                width: parent.width
                text: I18n.tr("Clears your changes to single animations and your own curves")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
        Plaque {
            id: resetPlaque
            anchors.verticalCenter: parent.verticalCenter
            kind: "line"
            small: true
            text: I18n.tr("Reset")
            accessibleName: I18n.tr("Reset every animation")
            onClicked: shop.resetAll()
        }
    }

    component HandleField: TextField {
        id: tf
        property string field: ""
        readonly property string shown: Number(shop.curve[tf.field]).toFixed(2)
        readonly property bool edited: tf.text !== tf.shown || tf.errorText !== ""
        width: 120
        mono: true
        text: tf.shown
        errorText: ""
        onEditingFinished: {
            if (!shop.typed(tf.field, tf.text)) {
                tf.errorText = I18n.tr("Type a number, such as 0.25.");
                return;
            }
            tf.errorText = "";
            tf.text = Qt.binding(() => tf.shown);
        }
        // Escape drops an unsaved edit; with nothing typed it goes on, so the page can go back
        Keys.onEscapePressed: event => {
            event.accepted = tf.edited;
            tf.errorText = "";
            tf.text = Qt.binding(() => tf.shown);
        }
        // with nothing unsaved, Ctrl+Z undoes the page's last change, not the field's typing
        Keys.onPressed: event => {
            if (event.key !== Qt.Key_Z || !(event.modifiers & Qt.ControlModifier) || tf.edited)
                return;
            Changes.undo("animations");
            event.accepted = true;
        }
    }
}
