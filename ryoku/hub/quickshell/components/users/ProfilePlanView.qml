pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// what applying a profile changes, as ryoku-host planned it
Column {
    id: view

    property var plan: ({})

    spacing: LithicTokens.space.s4

    function list(v) { return Users.list(v); }

    function packageGroups() {
        var groups = [], by = {};
        view.list(view.plan.packages).forEach(p => {
            var name = String(p.group || I18n.tr("Other"));
            if (!by[name]) {
                by[name] = { name: name, packages: [] };
                groups.push(by[name]);
            }
            by[name].packages.push(p);
        });
        return groups;
    }

    function installed(p) {
        return p.preExisting === true
            || (p.preExisting === undefined && Number(p.download || 0) === 0 && p.target === true && Number(p.installed || 0) > 0);
    }

    component Head: Label {
        width: parent ? parent.width : 0
        typeRole: "label"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    component Line: Column {
        id: line
        property string title: ""
        property bool literal: false
        property string detail: ""
        property string tag: ""
        property bool warning: false
        width: parent ? parent.width : 0
        spacing: LithicTokens.size.settingsDetailGap

        Row {
            width: parent.width
            spacing: LithicTokens.space.s3
            Label {
                width: parent.width - (tagLabel.visible ? tagLabel.implicitWidth + LithicTokens.space.s3 : 0)
                text: line.title
                typeRole: line.literal ? "monoSm" : "body"
                ink: line.warning ? Lithic.jasper : Lithic.fg
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                id: tagLabel
                visible: line.tag.length > 0
                text: line.tag
                typeRole: "detail"
                inkRole: "fg3"
            }
        }
        Label {
            visible: line.detail.length > 0
            width: parent.width
            text: line.detail
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Account") }
        Line {
            readonly property var account: view.plan.account || ({})
            title: String(account.name || "")
            literal: true
            tag: account.exists === true ? I18n.tr("Updated") : I18n.tr("New")
            detail: I18n.tr("Standard account, home %1, shell %2").arg(String(account.home || "")).arg(String(account.shell || ""))
        }
    }

    Column {
        visible: view.list(view.plan.repos).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Software sources") }
        Repeater {
            model: view.list(view.plan.repos)
            delegate: Line {
                required property var modelData
                title: String(modelData.id || "")
                literal: true
                detail: String(modelData.detail || "")
                tag: Users.planWord(modelData.action, I18n)
            }
        }
        Label {
            width: parent.width
            text: I18n.tr("Each source's signing key is checked before the source is turned on.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Column {
        visible: view.list(view.plan.packages).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Packages") }
        Label {
            width: parent.width
            readonly property var totals: view.plan.totals || ({})
            text: I18n.tr("%1 chosen and %2 they need: %3 to download, %4 on disk")
                .arg(Number(totals.targets || 0)).arg(Number(totals.dependencies || 0))
                .arg(Users.size(totals.download || 0)).arg(Users.size(totals.installed || 0))
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Repeater {
            model: view.packageGroups()
            delegate: Column {
                id: group
                required property var modelData
                width: view.width
                spacing: LithicTokens.space.s2
                Label {
                    width: parent.width
                    topPadding: LithicTokens.space.s1
                    text: I18n.tr(String(group.modelData.name || ""))
                    typeRole: "detail"
                    inkRole: "fg3"
                }
                Repeater {
                    model: group.modelData.packages || []
                    delegate: Line {
                        required property var modelData
                        title: String(modelData.name || "") + (modelData.version ? "  " + String(modelData.version) : "")
                        literal: true
                        tag: String(modelData.repo || "")
                        detail: view.installed(modelData) ? I18n.tr("Already installed")
                            : I18n.tr("%1 to download, %2 on disk").arg(Users.size(modelData.download || 0)).arg(Users.size(modelData.installed || 0))
                    }
                }
            }
        }
    }

    Column {
        visible: view.list(view.plan.remove).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Packages removed") }
        Repeater {
            model: view.list(view.plan.remove)
            delegate: Line {
                required property var modelData
                title: String(modelData.name || "")
                literal: true
                tag: String(modelData.repo || "")
                detail: I18n.tr("Frees %1").arg(Users.size(modelData.installed || 0))
            }
        }
    }

    Column {
        visible: view.list(view.plan.system).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Changes to the system") }
        Repeater {
            model: view.list(view.plan.system)
            delegate: Line {
                required property var modelData
                title: String(modelData.detail || modelData.kind || "")
                detail: String(modelData.path || "")
                tag: Users.planWord(modelData.kind, I18n)
            }
        }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Files and session") }
        Line {
            readonly property var files: view.plan.files || ({})
            title: I18n.tr("%1 files the profile keeps up to date, %2 set up once").arg(Number(files.managed || 0)).arg(Number(files.seedOnce || 0))
            detail: String(files.home || "")
        }
        Repeater {
            model: view.list(view.plan.env)
            delegate: Line {
                required property var modelData
                title: String(modelData.key || "") + "=" + String(modelData.value || "")
                literal: true
                tag: I18n.tr("Session variable")
            }
        }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Also shown in other accounts") }
        Label {
            visible: view.list(view.plan.bleed).length === 0
            width: parent.width
            text: I18n.tr("No apps from this plan show up in other accounts.")
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Repeater {
            model: view.list(view.plan.bleed)
            delegate: Line {
                required property var modelData
                title: String(modelData.name || modelData.desktopId || "")
                detail: String(modelData.desktopId || "")
                tag: String(modelData["package"] || "")
            }
        }
    }

    Column {
        visible: view.list(view.plan.warnings).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("Worth knowing") }
        Repeater {
            model: view.list(view.plan.warnings)
            delegate: Line {
                required property var modelData
                title: String(modelData)
            }
        }
    }

    Column {
        visible: view.list(view.plan.blockers).length > 0
        width: parent.width
        spacing: LithicTokens.space.s2
        Head { text: I18n.tr("What stops this plan") }
        Repeater {
            model: view.list(view.plan.blockers)
            delegate: Line {
                required property var modelData
                title: String(modelData)
                warning: true
            }
        }
    }
}
