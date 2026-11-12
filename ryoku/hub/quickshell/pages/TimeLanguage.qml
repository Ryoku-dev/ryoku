import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/time-language"
import "../schema/time-language.js" as Schema

// language, regional formats and clock, time zone, weather location and units
Item {
    id: root

    required property var hub
    readonly property var zoneBind: ({ store: "timezone", key: "zone" })
    readonly property string zone: {
        var v = Changes.value(root.zoneBind);
        return v === undefined ? "" : String(v);
    }
    readonly property var zoneStore: {
        Changes.revision;
        return Changes.stores.timezone || null;
    }
    readonly property bool zoneFailed: root.zone.length === 0 && root.zoneStore !== null && root.zoneStore.readFailed === true && root.zoneStore.reading !== true
    readonly property bool zoneBusy: root.zoneStore !== null && root.zoneStore.busy === true

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    // a stored language may be a code, an English or a native name; it keeps its own spelling
    function languageRow(row) {
        var stored = Settings.get("language");
        var code = I18n._code(stored === undefined || stored === null ? "" : String(stored));
        var autoValue = code === "" && stored !== undefined && stored !== null && String(stored) !== "" ? stored : "auto";
        var options = I18n.pickerOptions.map(c => {
            if (c === "auto")
                return { label: I18n.tr("Auto, as the system"), value: autoValue };
            return { label: I18n.pickerLabels[c] || c, value: code === c ? stored : c };
        });
        return Object.assign({}, row, { options: options, "default": autoValue });
    }

    function localeLabel(code) {
        if (code === "")
            return I18n.tr("Match language");
        var l = Qt.locale(code);
        var territory = l.nativeTerritoryName || "";
        return territory.length > 0 ? l.nativeLanguageName + " (" + territory + ")" : l.nativeLanguageName;
    }

    function formatsRow(row) {
        var options = I18n.localeOptions.map(c => ({ label: root.localeLabel(c), value: c }));
        var stored = Settings.get("formatLocale");
        if (stored && I18n.localeOptions.indexOf(String(stored)) < 0)
            options.push({ label: root.localeLabel(String(stored)), value: stored });
        return Object.assign({}, row, { options: options });
    }

    readonly property var schema: {
        Settings.revision;
        I18n.langs;
        return {
            title: Schema.title,
            lead: Schema.lead,
            sections: Schema.sections,
            related: Schema.related,
            rows: Schema.rows.map(r => r.id === "language" ? root.languageRow(r) : r.id === "formats" ? root.formatsRow(r) : r)
        };
    }

    // an example of the formats in use, not a clock, so nothing ticks
    readonly property string stateLine: {
        Settings.revision;
        var sample = new Date(2026, 10, 11, 18, 30);
        var date = sample.toLocaleDateString(Qt.locale(TimeFormat.formatLocale || I18n.lang), Locale.LongFormat);
        var line = I18n.tr("Dates read %1, times %2").replace("%1", date).replace("%2", TimeFormat.format(sample));
        return root.zone.length > 0 ? line + "  ·  " + root.zone.replace(/_/g, " ") : line;
    }

    Component.onCompleted: Changes.ensureStore("timezone", Qt.resolvedUrl("../components/time-language/TimeZoneStore.qml"))

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "time-language"
        schema: root.schema
        stateLine: root.stateLine
        controls: ({ "time-zone": zoneControl, "location": locationControl })
    }

    Component {
        id: zoneControl
        Row {
            property var row: null
            function activate() {
                if (root.zoneBusy)
                    return;
                zoneMap.currentZone = root.zone;
                zoneMap.opener = changeButton;
                zoneMap.show();
            }
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                text: root.zone.length > 0 ? root.zone.replace(/_/g, " ") : root.zoneFailed ? I18n.tr("Could not be read") : I18n.tr("Reading")
                typeRole: "label"
                inkRole: "fg2"
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.zoneFailed
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                accessibleName: I18n.tr("Read the time zone again")
                onClicked: root.zoneStore.readZone()
            }
            Plaque {
                id: changeButton
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "public"
                text: I18n.tr("Change")
                // the system is still taking the last zone
                busy: root.zoneBusy
                accessibleName: I18n.tr("Change the time zone")
                onClicked: parent.activate()
            }
        }
    }

    Component {
        id: locationControl
        LocationField {
            overlayParent: root.hub.overlayItem
            pageId: "time-language"
        }
    }

    TimeZoneMap {
        id: zoneMap
        parent: root.hub.overlayItem
        onChosen: z => Changes.set(root.zoneBind, z, I18n.tr("Time zone"), "time-language",
                                   I18n.tr("Time zone set to %1").replace("%1", z.replace(/_/g, " ")))
    }
}
