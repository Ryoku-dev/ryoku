import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "lib/apps.js" as AppsLogic
import "lib/store.js" as StoreLogic

// The Apps search results: Flathub, the system packages and the AUR in one
// list, each row naming where it comes from. Flatpaks install as the user,
// system packages ask for the password once, the AUR opens a terminal.
Item {
    id: view

    property string query: ""
    property bool reducedMotion: false
    // a Flathub row the catalogue also lists opens its full page
    property var catalogueKeys: ({})

    signal openCatalogue(string key)

    readonly property bool stale: AppSearch.resultQuery !== view.query.trim()
    readonly property bool anyUnreviewed: AppSearch.rows.some(row => row.kind === "flatpak" || row.kind === "aur")
    readonly property bool animate: !view.reducedMotion && view.visible
            && !!view.Window.window && view.Window.window.active
    property real sweep: 0
    property string confirmKey: ""
    // what still needs the package a removal is being confirmed for
    property var neededBy: []
    property bool planReading: false
    property string _planOut: ""

    // the user's terminal ($TERMINAL, kitty when unset) runs $1 under the window id $2
    readonly property string terminalScript: "t=\"${TERMINAL:-kitty}\"; id=\"$2\"; case \"${t##*/}\" in "
        + "foot|footclient) exec \"$t\" --app-id=\"$id\" sh -c \"$1\";; "
        + "kitty) exec \"$t\" --class=\"$id\" sh -c \"$1\";; "
        + "wezterm) exec \"$t\" start -- sh -c \"$1\";; "
        + "ghostty|alacritty) exec \"$t\" --class=\"$id\" -e sh -c \"$1\";; "
        + "*) exec \"$t\" -e sh -c \"$1\";; esac"

    function run(row, action) {
        view.confirmKey = "";
        if (row.kind === "aur") {
            if (terminal.running)
                return;
            const full = AppsLogic.aurLine(row, action) + "; printf '\\n%s\\n' "
                    + AppsLogic.shellQuote(I18n.tr("Press Enter to close this window.")) + "; read -r _";
            terminal.command = ["sh", "-c", view.terminalScript, "sh", full, "dev.ryoku.ryostore"];
            terminal.running = true;
            return;
        }
        // an older gpk would fail halfway; the notice above says why
        const command = Gpk.canInstall ? AppsLogic.packageCommand(row, action) : null;
        if (command)
            Jobs.app.start(row.key, action, row.name, command);
    }

    function doing(row) {
        if (Jobs.app.action === "remove")
            return I18n.tr("Removing %1").arg(row.name);
        return I18n.tr("Installing %1").arg(row.name);
    }

    // a removal is asked once more, with gpk's plan read first: packages that
    // still need this one make the system refuse, and the user should know why
    function askRemove(row) {
        view.confirmKey = row.key;
        view.neededBy = [];
        const command = AppsLogic.removePlanCommand(row);
        if (!command)
            return;
        plan.running = false;
        view._planOut = "";
        view.planReading = true;
        plan.command = command;
        plan.running = true;
    }

    onConfirmKeyChanged: if (confirmKey === "") { plan.running = false; view.planReading = false; }

    Process {
        id: plan
        environment: Spawn.env
        stdout: StdioCollector { onStreamFinished: view._planOut = text }
        onExited: code => {
            view.planReading = false;
            if (code !== 0)
                return;
            try {
                view.neededBy = AppsLogic.requiredBy(view._planOut);
            } catch (e) {
                view.neededBy = [];
            }
        }
    }

    // the terminal is our child, so the list can catch up once it closes
    Process {
        id: terminal
        environment: Spawn.env
        onExited: AppSearch.again()
    }

    NumberAnimation {
        target: view
        property: "sweep"
        from: 0
        to: 1
        duration: 1100
        easing.type: Easing.InOutSine
        loops: Animation.Infinite
        running: AppSearch.searching && view.animate
    }

    // searching again over rows already shown: a hairline sweep, not a blank list
    Rectangle {
        id: searchTrack
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: Tokens.border * 2
        color: "transparent"
        clip: true
        visible: AppSearch.searching

        Rectangle {
            width: parent.width * 0.22
            height: parent.height
            color: Tokens.bone
            x: view.reducedMotion ? (parent.width - width) / 2 : view.sweep * (parent.width + width) - width
        }
    }

    // an older gpk searches but cannot install or remove here; said before any job starts
    Rectangle {
        id: gpkNotice
        objectName: "ryostore-apps-gpk-old"
        anchors { left: parent.left; right: parent.right; top: searchTrack.bottom; topMargin: Tokens.s2 }
        visible: !Gpk.canInstall
        height: visible ? gpkRow.implicitHeight + Tokens.s3 * 2 : 0
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.alert

        Row {
            id: gpkRow
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; margins: Tokens.s3 }
            spacing: Tokens.s3

            Text {
                text: "!"
                color: Tokens.alert
                font.family: Tokens.mono
                font.pixelSize: Tokens.fBody
                font.weight: Font.Bold
            }
            Text {
                width: parent.width - Tokens.s5
                text: I18n.tr("Installing and removing apps here needs gpk 0.7.0, and this computer has %1. It comes with the next Ryoku update: run ryoku update, then open Ryostore again. Search works meanwhile, and AUR packages still install in a terminal.")
                      .arg(Gpk.version.replace(/^gpk\s+/, ""))
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
            }
        }
    }

    Rectangle {
        id: notice
        objectName: "ryostore-apps-search-warning"
        anchors { left: parent.left; right: parent.right; top: gpkNotice.bottom; topMargin: gpkNotice.visible ? Tokens.s2 : 0 }
        visible: view.anyUnreviewed && AppSearch.rows.length > 0
        height: visible ? noticeRow.implicitHeight + Tokens.s3 * 2 : 0
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        Row {
            id: noticeRow
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; margins: Tokens.s3 }
            spacing: Tokens.s3

            Text {
                text: "!"
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fBody
                font.weight: Font.Bold
            }
            Text {
                width: parent.width - Tokens.s5
                text: I18n.tr("Flathub and AUR apps come from third parties: Ryostore does not review them. Check an app's source and permissions before you install it. AUR packages are built from user scripts, so they install in a terminal.")
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
            }
        }
    }

    ListView {
        id: list
        objectName: "ryostore-apps-search-results"
        anchors { left: parent.left; right: parent.right; top: notice.bottom; topMargin: Tokens.s2; bottom: parent.bottom }
        clip: true
        model: AppSearch.rows
        spacing: 0
        boundsBehavior: Flickable.StopAtBounds
        opacity: view.stale && AppSearch.searching ? 0.55 : 1
        Behavior on opacity { enabled: !view.reducedMotion; NumberAnimation { duration: Tokens.snap } }

        delegate: Item {
            id: rowItem
            required property var modelData
            required property int index
            readonly property var row: modelData
            readonly property bool busy: Jobs.app.busyFor(row.key)
            readonly property bool confirming: view.confirmKey === row.key
            readonly property string catalogueKey: row.kind === "flatpak" ? "flatpak:" + row.installName : ""
            readonly property bool inCatalogue: catalogueKey !== "" && view.catalogueKeys[catalogueKey] === true
            width: list.width
            height: rowBody.implicitHeight + Tokens.s3 * 2

            Rectangle {
                anchors.fill: parent
                color: rowHover.hovered ? Tokens.tint5 : "transparent"
                Behavior on color { enabled: !view.reducedMotion; ColorAnimation { duration: Tokens.snap } }
            }
            Rectangle {
                anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
                height: Tokens.border
                color: Tokens.lineSoft
            }
            HoverHandler { id: rowHover }

            Column {
                id: rowBody
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                spacing: Tokens.s2

                Item {
                    width: parent.width
                    height: Math.max(plate.height, textCol.implicitHeight, actions.implicitHeight)

                    Rectangle {
                        id: plate
                        anchors.verticalCenter: parent.verticalCenter
                        width: Tokens.px(40)
                        height: Tokens.px(40)
                        radius: Tokens.radius
                        color: Tokens.paperLift
                        border.width: Tokens.border
                        border.color: Tokens.line

                        // the icon theme's own icon when it has one (installed apps,
                        // most system packages), the initials otherwise
                        readonly property string icon: Quickshell.iconPath(rowItem.row.installName, true)
                        Image {
                            anchors.centerIn: parent
                            width: parent.width - Tokens.s2 * 2
                            height: width
                            source: plate.icon
                            sourceSize: Qt.size(width * 2, height * 2)
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                            visible: status === Image.Ready
                        }
                        Text {
                            anchors.centerIn: parent
                            visible: plate.icon === ""
                            text: StoreLogic.initials(null, rowItem.row.name)
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                        }
                    }

                    Column {
                        id: textCol
                        anchors {
                            left: plate.right; leftMargin: Tokens.s3
                            right: actions.left; rightMargin: Tokens.s4
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: Tokens.s1

                        Row {
                            width: parent.width
                            spacing: Tokens.s2

                            Text {
                                id: nameText
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.min(implicitWidth, parent.width - chip.width - versionText.width - Tokens.s2 * 2)
                                text: rowItem.row.name
                                color: Tokens.ink
                                font.family: Tokens.display
                                font.pixelSize: Tokens.fRow
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                                font.underline: rowItem.inCatalogue && nameTap.hovered
                                HoverHandler { id: nameTap; enabled: rowItem.inCatalogue; cursorShape: Qt.PointingHandCursor }
                                TapHandler { enabled: rowItem.inCatalogue; onTapped: view.openCatalogue(rowItem.catalogueKey) }
                            }
                            Rectangle {
                                id: chip
                                objectName: "ryostore-apps-source-" + rowItem.row.source
                                anchors.verticalCenter: parent.verticalCenter
                                width: chipText.implicitWidth + Tokens.s2 * 2
                                height: chipText.implicitHeight + 4
                                radius: Tokens.radius
                                color: rowItem.row.kind === "system" ? Tokens.tint10 : "transparent"
                                border.width: Tokens.border
                                border.color: rowItem.row.kind === "system" ? Tokens.lineStrong : Tokens.line

                                Text {
                                    id: chipText
                                    anchors.centerIn: parent
                                    text: AppsLogic.sourceChip(rowItem.row.source)
                                    color: rowItem.row.kind === "system" ? Tokens.ink : Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                    font.letterSpacing: Tokens.trackLabel
                                }
                            }
                            Text {
                                id: versionText
                                anchors.verticalCenter: parent.verticalCenter
                                text: rowItem.row.version
                                color: Tokens.inkMuted
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny
                            }
                        }
                        Text {
                            width: parent.width
                            text: rowItem.row.summary !== "" ? rowItem.row.summary : rowItem.row.installName
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            elide: Text.ElideRight
                            maximumLineCount: 1
                        }
                    }

                    Column {
                        id: actions
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        spacing: Tokens.s1

                        Row {
                            anchors.right: parent.right
                            spacing: Tokens.s2
                            visible: !rowItem.confirming

                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: rowItem.row.installed
                                text: "✓  " + I18n.tr("INSTALLED")
                                color: Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny
                                font.letterSpacing: Tokens.trackLabel
                            }
                            Btn {
                                objectName: "ryostore-apps-action-" + rowItem.row.key
                                compact: true
                                primary: !rowItem.row.installed && rowItem.row.kind !== "aur" && Gpk.canInstall
                                armed: rowItem.row.kind === "aur" ? !terminal.running
                                       : !Jobs.app.running && Gpk.canInstall
                                motionEnabled: !view.reducedMotion
                                text: rowItem.row.installed ? I18n.tr("REMOVE")
                                      : (rowItem.row.kind === "aur" ? I18n.tr("INSTALL IN TERMINAL") : I18n.tr("INSTALL"))
                                onAct: {
                                    if (rowItem.row.installed && rowItem.row.kind !== "aur")
                                        view.askRemove(rowItem.row);
                                    else
                                        view.run(rowItem.row, rowItem.row.installed ? "remove" : "install");
                                }
                                Accessible.role: Accessible.Button
                                Accessible.name: text + " " + rowItem.row.name
                            }
                        }
                        Row {
                            anchors.right: parent.right
                            spacing: Tokens.s2
                            visible: rowItem.confirming

                            readonly property bool blocked: !view.planReading && view.neededBy.length > 0

                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: parent.blocked ? I18n.tr("%1 can't be removed yet.").arg(rowItem.row.name)
                                      : I18n.tr("Remove %1?").arg(rowItem.row.name)
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                            }
                            // offered only once the plan says nothing else needs it
                            Btn {
                                objectName: "ryostore-apps-confirm-remove"
                                visible: !parent.blocked
                                compact: true
                                primary: true
                                text: I18n.tr("REMOVE")
                                armed: !Jobs.app.running && Gpk.canInstall && !view.planReading && view.neededBy.length === 0
                                motionEnabled: !view.reducedMotion
                                onAct: view.run(rowItem.row, "remove")
                            }
                            Btn {
                                compact: true
                                text: parent.blocked ? I18n.tr("OK") : I18n.tr("KEEP")
                                motionEnabled: !view.reducedMotion
                                onAct: view.confirmKey = ""
                            }
                        }
                        Text {
                            anchors.right: parent.right
                            visible: rowItem.row.kind === "aur"
                            text: I18n.tr("Opens a terminal")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                        }
                    }
                }

                Text {
                    objectName: "ryostore-apps-remove-needed-by"
                    width: parent.width
                    visible: rowItem.confirming && (view.planReading || view.neededBy.length > 0)
                    text: view.planReading ? I18n.tr("Checking what else needs it.")
                          : I18n.tr("Still needed by %1. Remove those first, then this one.")
                            .arg(view.neededBy.join(", "))
                    color: view.planReading ? Tokens.inkMuted : Tokens.alert
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                    horizontalAlignment: Text.AlignRight
                }

                JobLine {
                    width: parent.width
                    target: rowItem.row.key
                    doing: view.doing(rowItem.row)
                    reducedMotion: view.reducedMotion
                }
            }
        }

        WheelScroll { }
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
    }

    // until this query has an answer (the debounce included) it reads as searching
    readonly property bool pending: AppSearch.searching || (view.stale && AppSearch.error === "")

    Column {
        anchors.centerIn: parent
        spacing: Tokens.s3
        visible: AppSearch.rows.length === 0
        width: Math.min(parent.width - Tokens.s7 * 2, 460)

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: view.pending ? I18n.tr("SEARCHING")
                  : (AppSearch.error !== "" ? I18n.tr("SEARCH FAILED") : I18n.tr("NO MATCHES"))
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            text: view.pending
                  ? I18n.tr("Looking through Flathub, the system packages and the AUR.")
                  : (AppSearch.error !== "" ? AppSearch.error
                     : I18n.tr("Nothing on Flathub, the system packages or the AUR matches \"%1\".").arg(view.query.trim()))
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
        }
        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: !AppSearch.searching && AppSearch.error !== ""
            text: I18n.tr("RETRY")
            onAct: AppSearch.again()
        }
    }
}
