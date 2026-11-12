import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/launcher"
import "../schema/launcher.js" as Schema

// the launcher on Super+Space: which one, how it looks, what it lists, and its greeting
Item {
    id: root

    required property var hub

    Component.onCompleted: Changes.ensureStore("launcher", Qt.resolvedUrl("../components/launcher/LauncherStore.qml"))

    function offered(row) {
        return !row.offers || styles.supports(row.offers);
    }

    // the launcher's own settings, plus the desktop's weather units the preview shows
    readonly property var previewSettings: {
        Changes.revision;
        var out = {};
        var keys = ["variant", "radius", "bgBlur", "heroImage", "heroStrength", "heroPosX", "heroPosY", "showWeather",
                    "showGreeting", "resultSettleMs", "horizonMode", "horizonColor", "overlayOpacity", "hiddenApps",
                    "columns", "matWidth", "sectionOrder", "hiddenSections", "pinnedApps"];
        keys.forEach(k => out[k] = Changes.value({ store: "launcher", key: k }));
        out.weatherUnit = String(Settings.get("weatherUnit") || "auto");
        return out;
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => root.offered(r))
    })

    readonly property var lacking: {
        var groups = [];
        if (!styles.supports("shape") && !styles.supports("background") && !styles.supports("overlay"))
            groups.push(I18n.tr("its look"));
        if (!["hiddenApps", "pinnedApps", "sections", "grid", "results"].some(c => styles.supports(c)))
            groups.push(I18n.tr("its results"));
        if (!styles.supports("hero"))
            groups.push(I18n.tr("a greeting"));
        return groups;
    }
    readonly property string lackingNote: {
        var c = styles.activeCard;
        if (!c || root.lacking.length === 0)
            return "";
        if (root.lacking.length === 3)
            return I18n.tr("%1 has no options of its own here.").arg(c.name);
        return I18n.tr("%1 has no options here for %2.").arg(c.name).arg(root.lacking.join(I18n.tr(" or ")));
    }

    readonly property string stateLine: styles.activeCard ? I18n.tr("In use: %1").arg(styles.activeCard.name) : ""

    // a deep link waits until the launcher in use says what it offers, or lands anyway after a moment
    property string pendingRow: ""

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!styles.ready) {
            root.pendingRow = rowId;
            readyWait.restart();
            return;
        }
        root.land(rowId);
    }

    function land(rowId) {
        root.pendingRow = "";
        readyWait.stop();
        var row = Schema.rows.find(r => r.id === rowId);
        // a setting, or a whole section, the launcher in use does not offer lands on the style it would need
        var missingSection = Schema.sections.some(s => s.id === rowId) && !page.layout.some(s => s.id === rowId);
        page.focusRow((row && !root.offered(row)) || missingSection ? "launcher-style" : rowId);
    }

    Connections {
        target: styles
        function onReadyChanged() {
            if (styles.ready && root.pendingRow !== "")
                Qt.callLater(root.land, root.pendingRow);
        }
    }
    Timer {
        id: readyWait
        interval: 2500
        onTriggered: if (root.pendingRow !== "") root.land(root.pendingRow)
    }

    function edit(key, value) {
        var labels = { heroPosX: "Picture focus across", heroPosY: "Picture focus down" };
        if (labels[key])
            Changes.set({ store: "launcher", key: key }, Math.round(Number(value) * 1000) / 1000, labels[key], "launcher");
    }

    function listOf(key) {
        var v = Changes.value({ store: "launcher", key: key });
        return v && v.length !== undefined ? Array.prototype.slice.call(v).map(String) : [];
    }

    LauncherStyles { id: styles }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "launcher"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({
            style: styleBody,
            results: ["hiddenApps", "pinnedApps", "sections"].some(c => styles.supports(c)) ? resultsBody : null
        })
    }

    Component {
        id: styleBody
        Column {
            spacing: LithicTokens.space.s3

            LauncherPreview {
                width: parent.width
                shelf: styles
                cardId: chooser.selectedId
                settings: root.previewSettings
                onEdit: (key, value) => root.edit(key, value)
            }

            DesignChooser {
                id: chooser
                width: parent.width
                shelf: styles
                accessibleName: I18n.tr("Launchers")
                ownerNote: root.lackingNote
            }

            Label {
                width: parent.width
                visible: styles.catalogMissing
                text: I18n.tr("The launchers that come with Ryoku were not found. Update Ryoku to restore them.")
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Component {
        id: resultsBody
        Column {
            spacing: LithicTokens.space.s4

            Column {
                visible: styles.supports("hiddenApps")
                width: parent.width
                spacing: LithicTokens.space.s2
                Label {
                    width: parent.width
                    text: I18n.tr("Hidden apps")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    text: I18n.tr("The launcher leaves these out; they stay installed.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Slab {
                    width: parent.width
                    AppListEditor {
                        width: parent.width
                        bind: ({ store: "launcher", key: "hiddenApps" })
                        changeLabel: "Hidden apps"
                        addText: I18n.tr("Hide an app")
                        emptyText: I18n.tr("Every app shows in the launcher.")
                        clearText: I18n.tr("Show every app")
                        accessibleName: I18n.tr("Hidden apps")
                        // an app is hidden or a favourite, never both
                        excluded: root.listOf("pinnedApps")
                    }
                }
            }

            Column {
                visible: styles.supports("pinnedApps")
                width: parent.width
                spacing: LithicTokens.space.s2
                Label {
                    width: parent.width
                    text: I18n.tr("Favourites")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    text: I18n.tr("Listed first in every section and in search, in this order.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Slab {
                    width: parent.width
                    AppListEditor {
                        width: parent.width
                        bind: ({ store: "launcher", key: "pinnedApps" })
                        changeLabel: "Favourites"
                        addText: I18n.tr("Add a favourite")
                        emptyText: I18n.tr("No favourites yet.")
                        clearText: I18n.tr("Remove all favourites")
                        reorderable: true
                        accessibleName: I18n.tr("Favourites")
                        excluded: root.listOf("hiddenApps")
                    }
                }
            }

            Column {
                visible: styles.supports("sections") && styles.storeSettings.sections.length > 0
                width: parent.width
                spacing: LithicTokens.space.s2
                Label {
                    width: parent.width
                    text: I18n.tr("Sections")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    text: I18n.tr("A hidden section loses its tab only; its apps stay under All and in search.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Slab {
                    width: parent.width
                    SectionOrder {
                        width: parent.width
                        host: page
                        known: styles.storeSettings.sections
                    }
                }
            }
        }
    }
}
