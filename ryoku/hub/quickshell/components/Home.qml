import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../registry/registry.js" as Registry

// the work pane when no page is open: the system's Now board
Item {
    id: root

    required property var hub

    readonly property bool updateKnown: Updates.lastChecked !== null
    readonly property var failures: Changes.failures()
    readonly property bool twoColumns: board.width >= LithicTokens.size.boardColumnsAt
    readonly property string continuePage: {
        var id = Registry.resolve(root.hub.lastSection);
        return id !== "" && root.hub.pageAvailable(id) && !Registry.page(id).handoff ? id : "";
    }
    property string powerMode: ""
    property string network: ""

    function focusRow(rowId) {}

    function title(s) {
        s = String(s || "");
        return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
    }

    readonly property var facts: {
        var out = [];
        var screens = parseInt(SysInfo.sysMonitors);
        if (screens > 0)
            out.push({ label: I18n.tr("Displays"), value: screens === 1 ? I18n.tr("One display") : I18n.tr("%1 displays").replace("%1", screens) });
        if (root.powerMode)
            out.push({ label: I18n.tr("Power mode"), value: root.powerMode });
        if (SysInfo.sysBattery && SysInfo.sysBattery !== "-")
            out.push({ label: I18n.tr("Battery"), value: SysInfo.sysBattery });
        out.push({ label: I18n.tr("Network"), value: root.network ? root.network : I18n.tr("Not connected") });
        if (SysInfo.sysGpu && SysInfo.sysGpu !== "-")
            out.push({ label: I18n.tr("Graphics"), value: SysInfo.sysGpu });
        return out;
    }

    Process {
        running: true
        command: ["powerprofilesctl", "get"]
        stdout: StdioCollector {
            onStreamFinished: {
                var m = { "performance": "Performance", "balanced": "Balanced", "power-saver": "Power saver" };
                var v = this.text.trim();
                root.powerMode = m[v] ? I18n.tr(m[v]) : "";
            }
        }
    }

    Process {
        running: true
        command: ["nmcli", "-t", "-f", "NAME,TYPE", "connection", "show", "--active"]
        stdout: StdioCollector {
            onStreamFinished: {
                var lines = this.text.split("\n").filter(l => l.length > 0 && l.indexOf(":loopback") < 0);
                root.network = lines.length ? lines[0].split(":")[0] : "";
            }
        }
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: board.height + LithicTokens.size.settingsInsetVertical
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Column {
            id: board
            x: Math.max(0, (flick.width - width) / 2)
            width: Math.min(flick.width, LithicTokens.size.boardMax)
            spacing: LithicTokens.space.s5

            Column {
                width: parent.width
                spacing: LithicTokens.space.s2

                Label {
                    width: parent.width
                    text: !root.updateKnown ? I18n.tr("Checking for updates.")
                        : Updates.available ? I18n.tr("An update is ready.") : I18n.tr("Ryoku is up to date.")
                    typeRole: "sentence"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Label {
                    width: parent.width
                    visible: text.length > 0
                    text: {
                        var wm = root.title(Settings.provider), bar = root.title(Settings.get("barStyle"));
                        if (wm && bar)
                            return I18n.tr("You're using %1 with %2.").replace("%1", wm).replace("%2", bar);
                        if (wm)
                            return I18n.tr("You're using %1.").replace("%1", wm);
                        return "";
                    }
                    typeRole: "proof"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Item { width: 1; height: LithicTokens.space.s2; visible: root.continuePage !== "" }
                Plaque {
                    visible: root.continuePage !== ""
                    kind: "tonal"
                    glyph: "arrow_forward"
                    text: root.continuePage !== "" ? I18n.tr("Continue in %1").replace("%1", I18n.tr(Registry.page(root.continuePage).title)) : ""
                    onClicked: root.hub.navigate(root.continuePage, "")
                }
            }

            Flow {
                id: columns
                width: parent.width
                spacing: LithicTokens.space.s5

                Column {
                    width: root.twoColumns ? (columns.width - LithicTokens.space.s5) * 0.6 : columns.width
                    spacing: 0

                    Section {
                        visible: Updates.available || root.failures.length > 0
                        width: parent.width
                        title: I18n.tr("Needs you")

                        Slab {
                            width: parent.width
                            Column {
                                width: parent.width
                                SettingRow {
                                    visible: Updates.available
                                    width: parent.width
                                    first: true
                                    title: I18n.tr("An update is ready")
                                    detail: Updates.latestName ? Updates.latestName : Updates.latestVersion
                                    Plaque {
                                        kind: "gold"
                                        small: true
                                        text: I18n.tr("Open Updates")
                                        onClicked: root.hub.navigate("updates", "")
                                    }
                                }
                                Repeater {
                                    model: root.failures
                                    delegate: SettingRow {
                                        required property var modelData
                                        required property int index
                                        width: parent.width
                                        first: index === 0 && !Updates.available
                                        title: modelData.entry ? I18n.tr("%1 did not change").replace("%1", I18n.tr(modelData.entry.label)) : ""
                                        detail: modelData.error
                                        Plaque {
                                            kind: "line"
                                            small: true
                                            glyph: "refresh"
                                            text: I18n.tr("Retry")
                                            onClicked: Changes.retry(modelData.entry.bind)
                                        }
                                    }
                                }
                            }
                        }
                    }

                    Section {
                        width: parent.width
                        title: I18n.tr("Your desktop")

                        Slab {
                            width: parent.width
                            Column {
                                width: parent.width
                                HandoffRow {
                                    width: parent.width
                                    text: I18n.tr("Wallpaper and colours")
                                    detail: I18n.tr("Choose them in the wallpaper picker")
                                    command: Registry.ryogamiTheme
                                }
                                HandoffRow {
                                    width: parent.width
                                    first: false
                                    glyph: "arrow_forward"
                                    text: I18n.tr("Bar style")
                                    detail: Settings.get("barStyle") ? root.title(Settings.get("barStyle")) : ""
                                    onTriggered: root.hub.navigate("bar", "")
                                }
                                HandoffRow {
                                    visible: root.hub.pageAvailable("window-manager")
                                    width: parent.width
                                    first: false
                                    glyph: "arrow_forward"
                                    text: I18n.tr("Window manager")
                                    detail: root.title(Settings.provider)
                                    onTriggered: root.hub.navigate("window-manager", "")
                                }
                            }
                        }
                    }
                }

                Column {
                    width: root.twoColumns ? (columns.width - LithicTokens.space.s5) * 0.4 : columns.width
                    spacing: 0

                    Section {
                        width: parent.width
                        title: I18n.tr("This computer")

                        Slab {
                            width: parent.width
                            Facts {
                                width: parent.width
                                facts: root.facts
                            }
                        }
                    }

                    Section {
                        visible: Changes.recent.length > 0
                        width: parent.width
                        title: I18n.tr("Recently changed")

                        Slab {
                            width: parent.width
                            Column {
                                width: parent.width
                                Repeater {
                                    model: Changes.recent
                                    delegate: SettingRow {
                                        required property var modelData
                                        required property int index
                                        width: parent.width
                                        first: index === 0
                                        title: I18n.tr(modelData.label)
                                        detail: modelData.page && Registry.page(modelData.page) ? I18n.tr(Registry.page(modelData.page).title) : ""
                                        activatable: modelData.page !== ""
                                        onActivated: root.hub.navigate(modelData.page, "")
                                        Plaque {
                                            kind: "quiet"
                                            small: true
                                            glyph: "undo"
                                            text: I18n.tr("Undo")
                                            onClicked: Changes.undoEntry(modelData)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
}
