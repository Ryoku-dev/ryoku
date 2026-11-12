import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// General's app-icon choice. The backend owns discovery, validation and every
// toolkit write; this surface only presents that catalogue and its applied value.
Item {
    id: root

    property var options: []
    property var labels: ({})
    property string current: ""
    property string error: ""
    property bool applying: false
    signal pickRequested()

    implicitHeight: card.implicitHeight

    function refresh() {
        catalog.running = false;
        currentTheme.running = false;
        catalog.running = true;
        currentTheme.running = true;
    }

    function apply(theme) {
        if (!theme || root.applying)
            return;
        root.error = "";
        root.applying = true;
        applyTheme.command = ["ryoku-hub", "desktop", "icon-theme", "set", theme];
        applyTheme.running = false;
        applyTheme.running = true;
    }

    Process {
        id: catalog
        running: false
        command: ["ryoku-hub", "desktop", "icons"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var rows = JSON.parse(this.text);
                    if (!Array.isArray(rows))
                        return;
                    var codes = [];
                    var names = {};
                    for (var i = 0; i < rows.length; i++) {
                        var code = String(rows[i].code || "");
                        if (code === "")
                            continue;
                        codes.push(code);
                        names[code] = String(rows[i].name || code);
                    }
                    root.options = codes;
                    root.labels = names;
                } catch (e) {
                    root.error = I18n.tr("Couldn't read the installed icon themes.");
                }
            }
        }
    }

    Process {
        id: currentTheme
        running: false
        command: ["ryoku-hub", "desktop", "icon-theme"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var state = JSON.parse(this.text);
                    root.current = String(state.theme || "");
                } catch (e) {
                    root.error = I18n.tr("Couldn't read the applied icon theme.");
                }
            }
        }
    }

    Process {
        id: applyTheme
        running: false
        stderr: StdioCollector { }
        onExited: function(code) {
            root.applying = false;
            if (code === 0) {
                root.error = "";
                currentTheme.running = false;
                currentTheme.running = true;
            } else {
                root.error = I18n.tr("Couldn't apply the icon theme.");
            }
        }
    }

    Component.onCompleted: root.refresh()

    SettingCard {
        id: card
        width: parent.width
        title: I18n.tr("ICONS")
        summary: I18n.tr("App icon theme")

        SettingRow {
            width: parent.width
            footH: 32
            label: I18n.tr("Icon theme")
            desc: root.error !== ""
                ? root.error
                : I18n.tr("The installed icon set used by apps and the Ryoku shell.")

            PickBar {
                anchors.fill: parent
                enabled: !root.applying && root.options.length > 0
                opacity: enabled ? 1 : 0.45
                value: root.current
                count: root.options.length
                labels: root.labels
                onOpened: root.pickRequested()
            }
        }
    }
}
