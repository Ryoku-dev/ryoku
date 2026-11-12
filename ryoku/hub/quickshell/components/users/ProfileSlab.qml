import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "users.js" as Users

// one account set up from a profile: its tools, checkpoints, reset and removal
Slab {
    id: root

    required property var store
    required property var status
    property var page: null
    property bool spot: false

    readonly property string account: String(root.status.account || "")
    readonly property string profileName: root.store.profileName(root.status.profile)
    readonly property int checkpointCount: Users.list(root.status.checkpoints).length
    // who may change it is known once the account list is read
    readonly property string viewOnly: root.store.listLoaded ? I18n.tr("Only an administrator can change this") : I18n.tr("Waiting for the account list")

    signal toolsRequested(var status)
    signal checkpointsRequested(var status)
    signal resetRequested(var status)
    signal removeRequested(var status)

    Column {
        width: parent.width

        Item {
            width: parent.width
            implicitHeight: Math.max(LithicTokens.size.headerMark, who.implicitHeight)

            Column {
                id: who
                anchors.left: parent.left
                anchors.right: signedIn.left
                anchors.rightMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                spacing: LithicTokens.size.settingsDetailGap

                Label {
                    width: parent.width
                    text: I18n.tr("%1 profile").arg(root.profileName)
                    typeRole: "row"
                    elide: Text.ElideRight
                }
                Row {
                    spacing: LithicTokens.space.s3
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.account
                        typeRole: "monoSm"
                        inkRole: "fg2"
                    }
                    Label {
                        visible: String(root.status.version || "") !== ""
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Version %1").arg(String(root.status.version || ""))
                        typeRole: "detail"
                    }
                }
            }

            Claim {
                id: signedIn
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                tone: root.status.loggedIn === true ? "alive" : "idle"
                text: root.status.loggedIn === true ? I18n.tr("Signed in") : I18n.tr("Signed out")
            }
        }

        PersonRow {
            owner: root
            rowId: "profile-tools"
            title: I18n.tr("Tools")
            detail: {
                var n = Users.list(root.status.packages).length;
                return n === 1 ? I18n.tr("One package from this profile") : I18n.tr("%1 packages from this profile").arg(n);
            }
            unavailable: root.store.canManage ? "" : root.viewOnly

            Plaque {
                kind: "quiet"
                small: true
                glyph: "construction"
                text: I18n.tr("Edit tools")
                accessibleName: I18n.tr("Edit the tools of %1").arg(root.account)
                onClicked: root.toolsRequested(root.status)
            }
        }

        PersonRow {
            owner: root
            rowId: "checkpoints"
            title: I18n.tr("Checkpoints")
            detail: root.checkpointCount === 0 ? I18n.tr("None saved yet. Save one before a big change.")
                : root.checkpointCount === 1 ? I18n.tr("One saved")
                : I18n.tr("%1 saved").arg(root.checkpointCount)
            unavailable: root.store.canManage ? "" : root.viewOnly

            Plaque {
                kind: "quiet"
                small: true
                glyph: "history"
                text: I18n.tr("Open")
                accessibleName: I18n.tr("Checkpoints of %1").arg(root.account)
                onClicked: root.checkpointsRequested(root.status)
            }
        }

        PersonRow {
            owner: root
            rowId: "profile-reset"
            title: I18n.tr("Reset settings")
            detail: I18n.tr("Put back every file the profile manages. Projects, notes and VPN files stay.")
            unavailable: root.store.canManage ? "" : root.viewOnly

            Plaque {
                kind: "quiet"
                small: true
                glyph: "restart_alt"
                text: I18n.tr("Reset")
                accessibleName: I18n.tr("Reset the settings of %1").arg(root.account)
                onClicked: root.resetRequested(root.status)
            }
        }

        PersonRow {
            owner: root
            rowId: "remove-profile"
            title: I18n.tr("Remove this profile account")
            detail: I18n.tr("Take away its tools, and archive, keep or erase the account")
            unavailable: !root.store.canManage ? root.viewOnly
                : root.account === root.store.self ? I18n.tr("You cannot remove the account you are signed in to")
                : ""

            Plaque {
                kind: "dangerLine"
                small: true
                text: I18n.tr("Remove")
                accessibleName: I18n.tr("Remove the %1 profile account").arg(root.account)
                onClicked: root.removeRequested(root.status)
            }
        }
    }
}
