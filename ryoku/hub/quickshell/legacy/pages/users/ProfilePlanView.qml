pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: view

    property var plan: ({})

    width: parent ? parent.width : implicitWidth
    spacing: Tokens.s5

    function list(value) {
        return value instanceof Array ? value : [];
    }

    function humanSize(value) {
        var bytes = Number(value || 0);
        if (bytes < 1024)
            return Math.round(bytes) + " B";
        var units = ["KiB", "MiB", "GiB", "TiB"];
        var index = -1;
        do {
            bytes /= 1024;
            index++;
        } while (bytes >= 1024 && index < units.length - 1);
        return bytes.toFixed(bytes >= 10 ? 1 : 2).replace(/\.0+$/, "") + " " + units[index];
    }

    function packageGroups() {
        var groups = [];
        var byName = ({});
        var packages = view.list(view.plan.packages);
        for (var i = 0; i < packages.length; i++) {
            var name = String(packages[i].group || I18n.tr("Other"));
            if (!byName[name]) {
                byName[name] = { name: name, packages: [] };
                groups.push(byName[name]);
            }
            byName[name].packages.push(packages[i]);
        }
        return groups;
    }

    component SectionHead: Row {
        property alias text: title.text
        spacing: Tokens.s2
        Text {
            text: "//"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
        }
        Text {
            id: title
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
        }
    }

    component PlanRow: Item {
        property string label: ""
        property string detail: ""
        property string badge: ""
        property bool warning: false

        width: parent ? parent.width : 0
        implicitHeight: Math.max(42, rowCopy.implicitHeight + Tokens.s3 * 2)

        Column {
            id: rowCopy
            anchors {
                left: parent.left
                right: badgePlate.visible ? badgePlate.left : parent.right
                rightMargin: badgePlate.visible ? Tokens.s3 : 0
                verticalCenter: parent.verticalCenter
            }
            spacing: Tokens.s1
            Text {
                width: parent.width
                text: parent.parent.label
                color: parent.parent.warning ? Tokens.alert : Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
            }
            Text {
                visible: text !== ""
                width: parent.width
                text: parent.parent.detail
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                wrapMode: Text.WordWrap
            }
        }

        Rectangle {
            id: badgePlate
            visible: parent.badge !== ""
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            width: badgeText.implicitWidth + Tokens.s3
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: badgeText
                anchors.centerIn: parent
                text: badgePlate.parent.badge
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
            }
        }

        Rectangle {
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: 1
            color: Tokens.lineSoft
        }
    }

    Column {
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("ACCOUNT") }
        PlanRow {
            readonly property var account: view.plan.account || ({})
            label: I18n.tr("%1 · %2").arg(String(account.name || "")).arg(I18n.tr(String(account.role || "standard")))
            detail: I18n.tr("Home %1 · shell %2").arg(String(account.home || "")).arg(String(account.shell || ""))
            badge: account.exists === true ? I18n.tr("UPDATE") : I18n.tr("NEW")
        }
    }

    Column {
        visible: view.list(view.plan.repos).length > 0
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("REPOSITORIES") }
        Repeater {
            model: view.list(view.plan.repos)
            delegate: PlanRow {
                required property var modelData
                label: I18n.tr(String(modelData.id || ""))
                detail: I18n.tr(String(modelData.detail || ""))
                badge: I18n.tr(String(modelData.action || "").toUpperCase())
            }
        }
        Text {
            width: parent.width
            text: I18n.tr("Repository keys are verified before a repository is enabled.")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            wrapMode: Text.WordWrap
        }
    }

    Column {
        visible: view.list(view.plan.packages).length > 0
        width: parent.width
        spacing: Tokens.s3
        SectionHead { text: I18n.tr("PACKAGES") }
        Text {
            width: parent.width
            readonly property var totals: view.plan.totals || ({})
            readonly property int targetCount: Number(totals.targets || 0)
            readonly property int dependencyCount: Number(totals.dependencies || 0)
            readonly property string targetSummary: targetCount === 1
                ? I18n.tr("1 target")
                : I18n.tr("%1 targets").arg(targetCount)
            readonly property string dependencySummary: dependencyCount === 1
                ? I18n.tr("1 dependency")
                : I18n.tr("%1 dependencies").arg(dependencyCount)
            text: targetSummary + " · " + dependencySummary
                + " · " + I18n.tr("%1 download").arg(view.humanSize(totals.download || 0))
                + " · " + I18n.tr("%1 installed").arg(view.humanSize(totals.installed || 0))
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            wrapMode: Text.WordWrap
        }
        Repeater {
            model: view.packageGroups()
            delegate: Column {
                id: packageGroup
                required property var modelData
                width: view.width
                spacing: Tokens.s1
                Text {
                    text: I18n.tr(String(packageGroup.modelData.name || "").toUpperCase())
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }
                Repeater {
                    model: packageGroup.modelData.packages || []
                    delegate: PlanRow {
                        required property var modelData
                        readonly property bool alreadyInstalled: modelData.preExisting === true
                            || (modelData.preExisting === undefined
                                && Number(modelData.download || 0) === 0
                                && modelData.target === true
                                && Number(modelData.installed || 0) > 0)
                        label: String(modelData.name || "") + (modelData.version ? "  " + String(modelData.version) : "")
                        detail: alreadyInstalled
                            ? I18n.tr("Already installed")
                            : I18n.tr("%1 download · %2 installed")
                                .arg(view.humanSize(modelData.download || 0))
                                .arg(view.humanSize(modelData.installed || 0))
                        badge: String(modelData.repo || "")
                    }
                }
            }
        }
    }

    Column {
        visible: view.list(view.plan.remove).length > 0
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("PACKAGES REMOVED") }
        Repeater {
            model: view.list(view.plan.remove)
            delegate: PlanRow {
                required property var modelData
                label: String(modelData.name || "")
                detail: view.humanSize(modelData.installed || 0)
                badge: String(modelData.repo || "")
            }
        }
    }

    Column {
        visible: view.list(view.plan.system).length > 0
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("SYSTEM CHANGES") }
        Repeater {
            model: view.list(view.plan.system)
            delegate: PlanRow {
                required property var modelData
                label: I18n.tr(String(modelData.detail || modelData.kind || ""))
                detail: String(modelData.path || "")
                badge: I18n.tr(String(modelData.kind || "").toUpperCase())
            }
        }
    }

    Column {
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("FILES AND SESSION") }
        PlanRow {
            readonly property var files: view.plan.files || ({})
            label: I18n.tr("%1 managed files · %2 first-use files")
                .arg(Number(files.managed || 0)).arg(Number(files.seedOnce || 0))
            detail: String(files.home || "")
        }
        Repeater {
            model: view.list(view.plan.env)
            delegate: PlanRow {
                required property var modelData
                label: String(modelData.key || "")
                detail: String(modelData.value || "")
                badge: I18n.tr("ENV")
            }
        }
    }
    Column {
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("ALSO VISIBLE IN OTHER ACCOUNTS") }
        Text {
            visible: view.list(view.plan.bleed).length === 0
            width: parent.width
            text: I18n.tr("No desktop apps from this plan appear in other accounts.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
        Repeater {
            model: view.list(view.plan.bleed)
            delegate: PlanRow {
                required property var modelData
                label: I18n.tr(String(modelData.name || modelData.desktopId || ""))
                detail: String(modelData.desktopId || "")
                badge: String(modelData.package || "")
            }
        }
    }

    Column {
        visible: view.list(view.plan.warnings).length > 0
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("WARNINGS") }
        Repeater {
            model: view.list(view.plan.warnings)
            delegate: PlanRow {
                required property var modelData
                label: I18n.tr(String(modelData))
                warning: true
            }
        }
    }

    Column {
        width: parent.width
        spacing: Tokens.s2
        SectionHead { text: I18n.tr("BLOCKERS") }
        Text {
            visible: view.list(view.plan.blockers).length === 0
            width: parent.width
            text: I18n.tr("No blockers.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
        Repeater {
            model: view.list(view.plan.blockers)
            delegate: PlanRow {
                required property var modelData
                label: I18n.tr(String(modelData))
                warning: true
            }
        }
    }
}
