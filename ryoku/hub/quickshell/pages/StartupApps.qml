import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/default-apps"
import "../components/startup-apps"
import "../schema/startup-apps.js" as Schema

// what starts at sign-in, and the variables the session exports
Item {
    id: root

    required property var hub
    readonly property string pageId: "startup-apps"

    readonly property var appsBind: ({ store: "desktop", key: "autostart" })
    readonly property var envBind: ({ store: "desktop", key: "env" })
    readonly property bool appsSupported: Settings.modelsKey("desktop.autostart")
    readonly property bool envSupported: Settings.modelsKey("desktop.env")
    readonly property var apps: (Changes.value(root.appsBind) || []).filter(a => a && typeof a === "object")
    readonly property var env: (Changes.value(root.envBind) || []).filter(e => e && typeof e === "object")

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows
    })

    // removing everything and the session variables sit in this page's own More options
    function focusRow(rowId) {
        var rows = { "remove-all": root.removeAllRow, "remove-all-variables": root.removeAllEnvRow, "session-variables": root.envBlock };
        if (rows.hasOwnProperty(rowId) && root.envFold) {
            root.envFold.open = true;
            revealFold.target = rows[rowId];
            revealFold.rowId = rowId;
            revealFold.restart();
            return;
        }
        page.focusRow(rowId);
    }

    property Item envFold: null
    property Item removeAllRow: null
    property Item removeAllEnvRow: null
    property Item envBlock: null
    property string landed: ""

    Timer {
        id: revealFold
        property Item target: null
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 20
        onTriggered: {
            var t = revealFold.target && revealFold.target.visible ? revealFold.target : root.envBlock;
            page.reveal(t || root.envFold);
            root.landed = t === revealFold.target ? revealFold.rowId : "";
            clearLanded.restart();
            // the keys follow the landing: the row's own control, or the first one in the block
            var next = t ? t.nextItemInFocusChain(true) : null;
            if (next && page.contains(t, next))
                next.forceActiveFocus(Qt.TabFocusReason);
        }
    }

    Timer {
        id: clearLanded
        interval: LithicTokens.motion.navigate + LithicTokens.motion.highlight + 100
        onTriggered: root.landed = ""
    }

    // each add, edit and remove is its own change with its own notice
    function setApps(list, notice) {
        Changes.set(root.appsBind, list, "Startup apps", root.pageId, notice);
    }

    function setEnv(list, notice) {
        Changes.set(root.envBind, list, "Session variables", root.pageId, notice);
    }

    function appName(item) {
        var name = names.nameOf(item.command);
        return name.length > 0 ? name : String(item.command || "");
    }

    function takenExcept(item) {
        return root.env.filter(e => e !== item && e.key !== (item ? item.key : undefined)).map(e => String(e.key || ""));
    }

    // a list save the window manager refused or never answered: the list is back as it was,
    // so the reason shows under it with Retry
    component SaveError: Row {
        id: line
        property var bind: null
        readonly property string text: line.bind ? Changes.error(line.bind) : ""
        visible: line.text.length > 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, line.width - again.width - LithicTokens.space.s3)
            text: line.text
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: again
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: Changes.retry(line.bind)
        }
    }

    // names installed apps for commands; never shown
    AppFinder {
        id: names
        visible: false
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: root.pageId
        schema: root.schema
        bodies: ({ apps: appsBody })
    }

    Component {
        id: appsBody
        Column {
            spacing: LithicTokens.space.s2

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    Label {
                        visible: !root.appsSupported
                        width: parent.width
                        text: I18n.tr("This window manager cannot start apps at sign-in.")
                        typeRole: "body"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }

                    ListEditor {
                        id: appsList
                        visible: root.appsSupported
                        width: parent.width
                        model: root.apps
                        accessibleName: I18n.tr("Startup apps")
                        addText: I18n.tr("Add a startup app")
                        doneText: I18n.tr("Add")
                        emptyText: I18n.tr("Nothing extra starts when you sign in.")
                        titleOf: item => root.appName(item)
                        detailOf: item => root.appName(item) === String(item.command || "") ? "" : String(item.command || "")
                        editor: Component { StartupEditor {} }
                        onAdded: item => root.setApps(root.apps.concat([item]), I18n.tr("%1 starts at sign-in").arg(root.appName(item)))
                        onEdited: (index, item) => {
                            var list = root.apps.slice();
                            list[index] = item;
                            root.setApps(list, I18n.tr("%1 changed").arg(root.appName(item)));
                        }
                        onRemoved: index => {
                            var list = root.apps.slice();
                            var gone = list.splice(index, 1)[0];
                            root.setApps(list, I18n.tr("%1 no longer starts at sign-in").arg(root.appName(gone)));
                        }
                    }

                    SaveError {
                        width: parent.width
                        topPadding: LithicTokens.space.s2
                        bind: root.appsBind
                    }

                    Item {
                        width: 1
                        height: LithicTokens.size.settingsRowGap
                    }

                    Disclosure {
                        id: fold
                        width: parent.width
                        text: I18n.tr("More options")
                        Component.onCompleted: root.envFold = fold
                        Component.onDestruction: {
                            if (root.envFold === fold)
                                root.envFold = null;
                        }

                        Column {
                            width: parent ? parent.width : 0
                            spacing: LithicTokens.space.s4

                            SettingRow {
                                id: removeAllRow
                                visible: root.appsSupported && root.apps.length > 0
                                width: parent.width
                                first: true
                                title: I18n.tr("Remove all startup apps")
                                detail: I18n.tr("Clear the whole list at once. Undo brings it back.")
                                highlighted: root.landed === "remove-all"

                                Plaque {
                                    kind: "quiet"
                                    small: true
                                    glyph: "delete_sweep"
                                    text: I18n.tr("Remove all")
                                    accessibleName: I18n.tr("Remove all startup apps")
                                    onClicked: root.setApps([], I18n.tr("All startup apps removed"))
                                }
                                Component.onCompleted: root.removeAllRow = removeAllRow
                            }

                            Loader {
                                id: envLoader
                                width: parent.width
                                sourceComponent: envBody
                                Component.onCompleted: root.envBlock = envLoader
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: envBody
        Column {
            spacing: LithicTokens.space.s3

            Column {
                width: parent.width
                spacing: LithicTokens.size.settingsDetailGap

                Label {
                    width: parent.width
                    text: I18n.tr("Session variables")
                    typeRole: "row"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Label {
                    width: parent.width
                    text: root.envSupported
                        ? I18n.tr("Names and values every app in your session sees. They take effect at your next sign-in.")
                        : I18n.tr("This window manager cannot set session variables.")
                    typeRole: "detail"
                    inkRole: root.envSupported ? "fg2" : "fg3"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }

            ListEditor {
                visible: root.envSupported
                width: parent.width
                model: root.env
                accessibleName: I18n.tr("Session variables")
                addText: I18n.tr("Add a variable")
                doneText: I18n.tr("Add")
                emptyText: I18n.tr("No variables of your own yet.")
                titleOf: item => String(item.key || "")
                detailOf: item => String(item.value || "").length > 0 ? String(item.value) : I18n.tr("Empty value")
                editor: Component {
                    VariableEditor {
                        taken: root.takenExcept(item)
                    }
                }
                onAdded: item => root.setEnv(root.env.concat([item]), I18n.tr("%1 added").arg(String(item.key || "")))
                onEdited: (index, item) => {
                    var list = root.env.slice();
                    list[index] = item;
                    root.setEnv(list, I18n.tr("%1 changed").arg(String(item.key || "")));
                }
                onRemoved: index => {
                    var list = root.env.slice();
                    var gone = list.splice(index, 1)[0];
                    root.setEnv(list, I18n.tr("%1 removed").arg(String(gone.key || "")));
                }
            }

            SaveError {
                width: parent.width
                bind: root.envBind
            }

            SettingRow {
                id: removeAllEnvRow
                visible: root.envSupported && root.env.length > 0
                width: parent.width
                first: true
                title: I18n.tr("Remove all session variables")
                detail: I18n.tr("Clear every variable at once. Undo brings them back.")
                highlighted: root.landed === "remove-all-variables"

                Plaque {
                    kind: "quiet"
                    small: true
                    glyph: "delete_sweep"
                    text: I18n.tr("Remove all")
                    accessibleName: I18n.tr("Remove all session variables")
                    onClicked: root.setEnv([], I18n.tr("All session variables removed"))
                }
                Component.onCompleted: root.removeAllEnvRow = removeAllEnvRow
            }
        }
    }
}
