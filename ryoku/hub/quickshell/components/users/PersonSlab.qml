pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "users.js" as Users

// one person: who they are, then what an administrator may change for them
Slab {
    id: root

    required property var store
    required property var person
    // the page, for search landings: its rowItems and highlightId
    property var page: null
    // this person's rows answer the search landings
    property bool spot: false
    property var notes: ({})

    readonly property string name: String(root.person.name || "")
    readonly property bool isSelf: root.name === root.store.self
    readonly property var profile: root.store.profileOf(root.name)
    readonly property bool lastAdmin: root.person.role === "admin" && root.store.adminCount <= 1
    readonly property var storage: root.person.storage || ({ supported: false, reason: "" })
    readonly property string viewOnly: root.store.inProfileAccount
        ? I18n.tr("Profile accounts cannot change other accounts")
        : I18n.tr("Only an administrator can change this")

    signal choosePicture()
    signal passwordRequested(string name)
    signal limitRequested(string name, double limitBytes, double usedBytes)
    signal removeRequested(string name)
    signal profilesRequested()

    function roleBind() { return { store: "users", key: "role/" + root.name }; }
    function permBind(id) { return { store: "users", key: "perm/" + root.name + "/" + id }; }

    Column {
        width: parent.width

        // who
        Item {
            width: parent.width
            implicitHeight: Math.max(avatar.height, who.implicitHeight)

            Avatar {
                id: avatar
                anchors.verticalCenter: parent.verticalCenter
                userName: root.name
                fullName: String(root.person.fullName || "")
                home: String(root.person.home || "")
                revision: root.isSelf ? root.store.pictureRevision : 0
            }

            Column {
                id: who
                anchors.left: avatar.right
                anchors.leftMargin: LithicTokens.space.s3
                anchors.right: signedIn.left
                anchors.rightMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                spacing: LithicTokens.size.settingsDetailGap

                Label {
                    width: parent.width
                    text: String(root.person.fullName || "") !== "" ? String(root.person.fullName) : root.name
                    typeRole: "row"
                    elide: Text.ElideRight
                }
                Row {
                    spacing: LithicTokens.space.s3
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.name
                        typeRole: "monoSm"
                        inkRole: "fg2"
                    }
                    Marks {
                        anchors.verticalCenter: parent.verticalCenter
                        traits: {
                            var t = [];
                            if (root.isSelf)
                                t.push(I18n.tr("You"));
                            t.push(root.person.role === "admin" ? I18n.tr("Administrator") : I18n.tr("Standard"));
                            if (root.profile)
                                t.push(I18n.tr("%1 profile").arg(root.store.profileName(root.profile.profile)));
                            if (root.person.uid !== undefined)
                                t.push(I18n.tr("User ID %1").arg(root.person.uid));
                            return t;
                        }
                    }
                }
            }

            Claim {
                id: signedIn
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                tone: root.person.loggedIn === true ? "alive" : "idle"
                text: root.person.loggedIn === true ? I18n.tr("Signed in") : I18n.tr("Signed out")
            }
        }

        // your picture, which only you change
        PersonRow {
            owner: root
            rowId: "picture"
            visible: root.isSelf
            title: I18n.tr("Picture")
            detail: root.store.pictureError !== "" ? "" : I18n.tr("Beside your name at sign-in and on the lock screen")
            error: root.store.pictureError
            busy: root.store.pictureBusy

            Plaque {
                kind: "quiet"
                small: true
                glyph: "image"
                text: avatar.hasImage ? I18n.tr("Change") : I18n.tr("Choose")
                enabled: !root.store.pictureBusy
                accessibleName: I18n.tr("Choose your picture")
                onClicked: root.choosePicture()
            }
            Plaque {
                visible: avatar.hasImage
                kind: "quiet"
                small: true
                text: I18n.tr("Remove")
                enabled: !root.store.pictureBusy && avatar.hasImage
                accessibleName: I18n.tr("Remove your picture")
                onClicked: root.store.removePicture()
            }
        }

        PersonRow {
            id: roleRow
            owner: root
            rowId: "role"
            title: I18n.tr("Role")
            detail: root.person.role === "admin"
                ? I18n.tr("Can install apps, change the system and manage other accounts")
                : I18n.tr("Uses the computer and changes only their own settings")
            unavailable: !root.store.canManage ? root.viewOnly
                : root.profile ? I18n.tr("Profile accounts stay standard")
                : root.lastAdmin ? I18n.tr("The only administrator stays one. Make someone else an administrator first.")
                : ""
            bind: root.roleBind()
            busy: root.store.runningKey === roleRow.bind.key

            Segmented {
                id: roleChoice
                readonly property string value: String(Changes.value(roleRow.bind) || root.person.role || "standard")
                model: [I18n.tr("Standard"), I18n.tr("Administrator")]
                accessibleName: I18n.tr("Role for %1").arg(root.name)
                onActivated: index => {
                    var next = index === 1 ? "admin" : "standard";
                    if (next !== value)
                        Changes.set(roleRow.bind, next, I18n.tr("Role for %1").arg(root.name), "users");
                }
                // the control assigns its own index on click, so the store keeps driving it
                Binding {
                    target: roleChoice
                    property: "currentIndex"
                    value: roleChoice.value === "admin" ? 1 : 0
                    restoreMode: Binding.RestoreNone
                }
            }
        }

        Repeater {
            model: root.store.permissionDefs
            delegate: PersonRow {
                id: permRow
                required property var modelData
                required property int index
                readonly property string permId: String(modelData.id || "")
                owner: root
                rowId: index === 0 ? "permissions" : ""
                title: Users.permissionTitle(modelData, I18n)
                detail: Users.permissionDetail(modelData, I18n)
                unavailable: !root.store.canManage ? root.viewOnly : ""
                bind: root.permBind(permRow.permId)
                busy: root.store.runningKey === permRow.bind.key
                activatable: root.store.canManage
                onActivated: permToggle.flip()

                Toggle {
                    id: permToggle
                    checked: Changes.value(permRow.bind) === true
                    Accessible.name: permRow.title
                    function flip() {
                        Changes.set(permRow.bind, Changes.value(permRow.bind) !== true,
                                    I18n.tr("%1 for %2").arg(permRow.title).arg(root.name), "users");
                    }
                    onToggled: {
                        // the store's value decides what shows
                        checked = Changes.value(permRow.bind) === true;
                        flip();
                    }
                }
            }
        }

        PersonRow {
            owner: root
            rowId: "storage"
            title: I18n.tr("Home folder")
            detail: {
                if (!root.storage.supported)
                    return String(root.storage.reason || I18n.tr("Its size cannot be read on this system"));
                var used = Users.size(root.storage.usedBytes || 0);
                return Number(root.storage.limitBytes || 0) > 0
                    ? I18n.tr("%1 used of %2").arg(used).arg(Users.size(root.storage.limitBytes))
                    : I18n.tr("%1 used, no limit").arg(used);
            }
            note: root.notes[root.name + "/limit"] || ""

            BladeMeter {
                visible: root.storage.supported === true && Number(root.storage.limitBytes || 0) > 0
                anchors.verticalCenter: parent.verticalCenter
                width: 120
                value: Number(root.storage.limitBytes || 0) > 0 ? Number(root.storage.usedBytes || 0) / Number(root.storage.limitBytes) : 0
            }
            Plaque {
                visible: root.store.canManage && root.store.storageCaps.quota === true && root.storage.supported === true
                kind: "quiet"
                small: true
                text: Number(root.storage.limitBytes || 0) > 0 ? I18n.tr("Change limit") : I18n.tr("Set a limit")
                accessibleName: I18n.tr("Storage limit for %1").arg(root.name)
                onClicked: root.limitRequested(root.name, Number(root.storage.limitBytes || 0), Number(root.storage.usedBytes || 0))
            }
        }

        PersonRow {
            owner: root
            rowId: "password"
            title: I18n.tr("Password")
            detail: root.isSelf ? I18n.tr("The password you sign in and unlock with") : I18n.tr("The password %1 signs in with").arg(root.name)
            note: root.notes[root.name + "/password"] || ""
            unavailable: !root.store.canManage ? root.viewOnly : ""

            Plaque {
                kind: "quiet"
                small: true
                glyph: "password"
                text: I18n.tr("Change")
                accessibleName: I18n.tr("Change the password for %1").arg(root.name)
                onClicked: root.passwordRequested(root.name)
            }
        }

        PersonRow {
            owner: root
            rowId: "remove-person"
            title: I18n.tr("Remove %1").arg(root.name)
            detail: root.profile && root.store.canManage
                ? I18n.tr("Remove it under Account profiles, which also takes its tools away")
                : I18n.tr("Delete this account and archive or erase its home folder")
            // a profile account is removed from its own section, so its link stays usable
            unavailable: !root.store.canManage ? root.viewOnly
                : root.isSelf ? I18n.tr("You cannot remove the account you are signed in to")
                : root.lastAdmin ? I18n.tr("Make someone else an administrator before removing the only one")
                : root.profile ? ""
                : root.person.loggedIn === true ? I18n.tr("%1 is signed in. They need to sign out first.").arg(root.name)
                : ""

            Plaque {
                visible: !root.profile
                kind: "dangerLine"
                small: true
                text: I18n.tr("Remove")
                accessibleName: I18n.tr("Remove %1").arg(root.name)
                onClicked: root.removeRequested(root.name)
            }
            Plaque {
                visible: !!root.profile && root.store.canManage
                kind: "quiet"
                small: true
                glyph: "arrow_downward"
                text: I18n.tr("Account profiles")
                onClicked: root.profilesRequested()
            }
        }
    }
}
