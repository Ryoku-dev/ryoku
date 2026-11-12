pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the weather city: typed or picked from matching places, applied on Enter,
// on leaving the field or on a pick
Item {
    id: root

    property var row: null
    property Item overlayParent: null
    property string pageId: ""
    property var places: []
    readonly property var bind: root.row ? root.row.bind : null
    readonly property string stored: {
        var v = Changes.value(root.bind);
        return v === undefined || v === null ? "" : String(v);
    }

    implicitWidth: LithicTokens.size.settingsControlMax
    implicitHeight: field.implicitHeight

    function store(text) {
        var v = String(text).trim();
        if (v !== root.stored)
            Changes.set(root.bind, v, root.row.changeLabel || root.row.label, root.pageId);
    }

    function place(p) {
        return [p.name, p.admin1, p.country].filter(s => s && s.length > 0).join(", ");
    }

    onStoredChanged: {
        if (!field.activeFocus)
            field.text = root.stored;
    }

    TextField {
        id: field
        width: parent.width
        text: root.stored
        placeholderText: I18n.tr("Search a city")
        Accessible.name: I18n.tr("Location")
        onTextEdited: lookup.restart()
        onEditingFinished: {
            if (!suggestions.opened)
                root.store(text);
        }
        Keys.onDownPressed: {
            if (suggestions.opened)
                list.focusItem(0, true);
        }
        Keys.onEscapePressed: event => {
            if (suggestions.opened) {
                suggestions.close();
                event.accepted = true;
            } else {
                event.accepted = false;
            }
        }
    }

    // matching places, asked for once typing pauses
    Timer {
        id: lookup
        interval: 350
        onTriggered: {
            var q = field.text.trim();
            if (q.length < 2) {
                suggestions.close();
                return;
            }
            geo.command = ["curl", "-s", "--max-time", "6",
                "https://geocoding-api.open-meteo.com/v1/search?count=6&format=json&language=" + encodeURIComponent(I18n.lang) + "&name=" + encodeURIComponent(q)];
            geo.running = true;
        }
    }

    Process {
        id: geo
        stdout: StdioCollector {
            onStreamFinished: {
                var out = [];
                try {
                    var j = JSON.parse(this.text);
                    (j && Array.isArray(j.results) ? j.results : []).forEach(r => {
                        if (typeof r.latitude === "number" && typeof r.longitude === "number")
                            out.push({ name: r.name || "", admin1: r.admin1 || "", country: r.country || "" });
                    });
                } catch (e) {}
                root.places = out;
                if (out.length > 0 && field.activeFocus)
                    suggestions.open();
                else
                    suggestions.close();
            }
        }
    }

    Popover {
        id: suggestions
        parent: root.overlayParent ? root.overlayParent : root
        anchorItem: field
        opener: field
        focus: false
        // typing keeps the field; Down hands the list focus
        menuFocus: false

        Menu {
            id: list
            menuWidth: field.width
            items: root.places.map(p => ({ label: root.place(p), glyph: "location_on" }))
            onTriggered: index => {
                var p = root.places[index];
                field.text = p.name;
                suggestions.close();
                root.store(p.name);
            }
        }
    }
}
