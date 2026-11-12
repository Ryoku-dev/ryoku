import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/users"
import "../schema/users.js" as Schema

// the people on this computer and the accounts set up from a profile
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    // a short word under a row after its dialog finished: "<name>/password" and the like
    property var notes: ({})
    property string peopleNote: ""
    property string profileNote: ""
    // whose rows answer search landings; the newest person after an add
    property string spotName: ""
    property string wantedProfile: ""
    // a person just added, whose rows the landing waits for until the list read shows them
    property string awaitName: ""
    // their slab is made at once but placed when the next frame is drawn; the landing waits for that
    property bool awaitPlaced: false

    readonly property var store: Changes.stores["users"] || null
    readonly property Item pageItem: page
    readonly property bool ready: root.store !== null
    readonly property bool inProfile: root.ready && root.store.inProfileAccount
    readonly property string spotPerson: {
        if (!root.ready)
            return "";
        if (root.spotName !== "" && root.store.user(root.spotName))
            return root.spotName;
        if (root.store.selfAccount)
            return root.store.self;
        return root.store.users.length > 0 ? String(root.store.users[0].name || "") : "";
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        // a profile account draws only its own panel; filtered here so the frame does not count
        // the store link as an option the window manager lacks
        rows: Schema.rows.filter(r => r.template !== true && !(root.inProfile && r.section === "profiles"))
    })

    readonly property string stateLine: {
        if (!Host.ready)
            return I18n.tr("Checking what this system supports");
        if (!root.ready || !root.store.supported)
            return Host.reason("users") !== "" ? Host.reason("users") : I18n.tr("Accounts cannot be managed on this system.");
        if (!root.store.listLoaded)
            return "";
        var n = root.store.users.length;
        var people = n === 1 ? I18n.tr("One person uses this computer.") : I18n.tr("%1 people use this computer.").arg(n);
        if (root.store.inProfileAccount)
            return people + " " + I18n.tr("This is a profile account, so changes to other accounts happen from an administrator's account.");
        return people + " " + (root.store.canManage ? I18n.tr("You are an administrator.") : I18n.tr("Only an administrator can change accounts."));
    }

    Component.onCompleted: {
        Changes.ensureStore("users", Qt.resolvedUrl("../components/users/AccountsStore.qml"));
        if (root.store)
            root.store.start();
        root.takeProfileRequest();
    }

    // ryostore's Create account and `ryoku-shell hub open users <id>` land here
    Connections {
        target: root.hub
        function onUsersProfileIdChanged() { root.takeProfileRequest(); }
    }

    function takeProfileRequest() {
        var id = String(root.hub.usersProfileId || "");
        if (id === "")
            return;
        root.hub.usersProfileId = "";
        root.wantedProfile = id;
        if (root.ready)
            root.store.refreshProfiles();
    }

    function openWantedProfile() {
        if (root.wantedProfile === "" || !root.ready || !root.store.profilesLoaded)
            return;
        var id = root.wantedProfile;
        root.wantedProfile = "";
        var def = root.store.profileDef(id);
        if (def && def.installed === true) {
            root.profileNote = "";
            dialogs.item.setup.openFor(def, null);
        } else {
            root.profileNote = I18n.tr("The %1 profile is not installed. Install it from RyoStore, then create its account here.").arg(id);
            root.focusRow("create-profile");
        }
    }

    Connections {
        target: root.store
        ignoreUnknownSignals: true
        function onProfilesFound() { Qt.callLater(root.openWantedProfile); }
        function onUsersChanged() { root.retryFocus(); }
        function onProfileAccountsChanged() { root.retryFocus(); }
    }
    onReadyChanged: {
        if (root.ready) {
            root.store.start();
            Qt.callLater(root.openWantedProfile);
        }
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // search lands on a drawn row, on a row of the person it answers for, or on the section
    function focusRow(rowId) {
        if (!rowId)
            return;
        root.pendingRow = rowId;
        root.retryFocus();
    }

    function retryFocus() {
        if (root.pendingRow === "")
            return;
        Qt.callLater(root.land);
    }

    function land() {
        var id = root.pendingRow;
        if (id === "" || page.layout.length === 0)
            return;
        if (root.awaitName !== "") {
            if (!root.ready || !root.store.user(root.awaitName))
                return;
            root.awaitName = "";
            root.awaitPlaced = true;
            return;
        }
        if (root.awaitPlaced)
            return;
        if (page.rowItems[id]) {
            root.pendingRow = "";
            page.land(id);
            return;
        }
        if (page.sectionOfRow(id)) {
            root.pendingRow = "";
            page.focusRow(id);
            return;
        }
        var row = Schema.row(id);
        // a deep link may name a section itself, such as users/profiles
        var section = row ? row.section : id;
        // in a profile account its own panel answers for reset and checkpoints
        if (root.inProfile && (section === "profiles" || section === "people"))
            section = "this-profile";
        // the data is not read yet: try again when it is
        if (!root.ready || !root.store.listLoaded || !root.store.profilesLoaded)
            return;
        root.pendingRow = "";
        var at = root.sectionIndex(section);
        if (at >= 0)
            page.chooseSection(at);
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "users"
        schema: root.schema
        stateLine: root.stateLine
        bodies: root.ready ? (root.inProfile ? { "this-profile": thisProfileBody } : { people: peopleBody, profiles: profilesBody }) : ({})
        onLayoutChanged: root.retryFocus()
    }

    Connections {
        target: root.Window.window
        enabled: root.awaitPlaced
        function onFrameSwapped() {
            root.awaitPlaced = false;
            root.land();
        }
    }

    Component {
        id: peopleBody

        Column {
            spacing: LithicTokens.space.s2

            Label {
                visible: Host.ready && !root.store.supported
                width: parent.width
                text: Host.reason("users") !== "" ? Host.reason("users") : I18n.tr("Accounts cannot be managed on this system.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Row {
                visible: root.store.supported && !root.store.listLoaded && root.store.loadError === ""
                spacing: LithicTokens.space.s2
                Working {
                    anchors.verticalCenter: parent.verticalCenter
                    active: parent.visible
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Reading the accounts on this computer")
                    typeRole: "detail"
                }
            }

            Row {
                visible: root.store.loadError !== ""
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, parent.width - reload.width - LithicTokens.space.s3)
                    text: root.store.loadError
                    typeRole: "detail"
                    ink: Lithic.jasper
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Plaque {
                    id: reload
                    kind: "quiet"
                    small: true
                    glyph: "refresh"
                    text: I18n.tr("Retry")
                    onClicked: {
                        root.store.loadError = "";
                        root.store.start();
                    }
                }
            }

            Label {
                visible: root.store.supported && root.store.listLoaded && root.store.users.length === 0
                width: parent.width
                text: I18n.tr("No accounts were found on this computer.")
                typeRole: "body"
                inkRole: "fg2"
            }

            // counted, not the list itself, so a list read after a change keeps each slab, its focus and the scroll
            Repeater {
                model: root.store.supported ? root.store.users.length : 0
                delegate: PersonSlab {
                    required property int index
                    readonly property var entry: root.store.users[index] || ({})
                    width: parent ? parent.width : 0
                    store: root.store
                    person: entry
                    page: root.pageItem
                    spot: String(entry.name || "") === root.spotPerson
                    notes: root.notes
                    onChoosePicture: picker.open()
                    onPasswordRequested: name => dialogs.item.password.openFor(name)
                    onLimitRequested: (name, limitBytes, usedBytes) => dialogs.item.limit.openFor(name, limitBytes, usedBytes)
                    onRemoveRequested: name => dialogs.item.remove.openFor(name)
                    onProfilesRequested: page.chooseSection(root.sectionIndex("profiles"))
                }
            }

            Slab {
                visible: root.store.supported && root.store.listLoaded
                width: parent.width

                PersonRow {
                    owner: QtObject {
                        readonly property var store: root.store
                        readonly property var page: root.pageItem
                        readonly property bool spot: true
                    }
                    rowId: "add-person"
                    width: parent.width
                    settingRow.first: true
                    title: I18n.tr("Add a person")
                    detail: I18n.tr("Their own sign-in, home folder and desktop")
                    note: root.peopleNote
                    unavailable: root.store.canManage ? "" : (root.store.inProfileAccount
                        ? I18n.tr("Profile accounts cannot add people") : I18n.tr("Only an administrator can add people"))

                    Plaque {
                        kind: "line"
                        small: true
                        glyph: "person_add"
                        text: I18n.tr("Add")
                        enabled: root.store.capsLoaded
                        accessibleName: I18n.tr("Add a person")
                        onClicked: dialogs.item.add.openForCreate()
                    }
                }
            }
        }
    }

    Component {
        id: profilesBody

        Column {
            spacing: LithicTokens.space.s2

            Label {
                width: parent.width
                text: I18n.tr("A profile from RyoStore sets up a separate standard account with its own tools, menus and settings, for work you keep apart.")
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Repeater {
                model: root.store.profileAccounts.length
                delegate: ProfileSlab {
                    required property int index
                    width: parent ? parent.width : 0
                    store: root.store
                    status: root.store.profileAccounts[index] || ({})
                    page: root.pageItem
                    spot: index === 0
                    onToolsRequested: status => {
                        var def = root.store.profileDef(status.profile);
                        if (def)
                            dialogs.item.setup.openFor(def, status);
                        else
                            root.profileNote = I18n.tr("The %1 profile's definition is missing, so its tools cannot be changed here.").arg(root.store.profileName(status.profile));
                    }
                    onCheckpointsRequested: status => dialogs.item.checkpoints.openFor(String(status.account || ""), status.loggedIn === true)
                    onResetRequested: status => dialogs.item.reset.openFor(String(status.account || ""))
                    onRemoveRequested: status => dialogs.item.removeProfile.openFor(status, root.store.profileName(status.profile))
                }
            }

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    Label {
                        visible: root.store.profilesLoaded && root.store.installedProfiles.length === 0
                        width: parent.width
                        text: I18n.tr("No profiles are installed. Find one in RyoStore, then create its account here.")
                        typeRole: "body"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }

                    Repeater {
                        model: root.store.installedProfiles.length
                        delegate: PersonRow {
                            id: createRow
                            required property int index
                            readonly property var modelData: root.store.installedProfiles[index] || ({})
                            owner: QtObject {
                                readonly property var store: root.store
                                readonly property var page: root.pageItem
                                readonly property bool spot: createRow.index === 0
                            }
                            rowId: index === 0 ? "create-profile" : ""
                            settingRow.first: index === 0
                            title: I18n.tr("Create a %1 account").arg(String(modelData.name || modelData.id))
                            detail: {
                                var m = modelData.manifest || ({});
                                var text = String(m.summary || m.description || "");
                                var version = String(modelData.version || "");
                                return version !== "" ? (text !== "" ? I18n.tr("Version %1. %2").arg(version).arg(text) : I18n.tr("Version %1").arg(version)) : text;
                            }
                            unavailable: root.store.canManage ? "" : root.store.listLoaded
                                ? I18n.tr("Only an administrator can create accounts") : I18n.tr("Waiting for the account list")

                            Plaque {
                                kind: "line"
                                small: true
                                glyph: "person_add"
                                text: I18n.tr("Create")
                                accessibleName: I18n.tr("Create a %1 account").arg(String(modelData.name || modelData.id))
                                onClicked: dialogs.item.setup.openFor(modelData, null)
                            }
                        }
                    }
                }
            }

            Label {
                visible: root.profileNote !== ""
                width: parent.width
                text: root.profileNote
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    Component {
        id: thisProfileBody

        ThisProfile {
            store: root.store
            page: root.pageItem
            onResetRequested: dialogs.item.reset.openFor("")
        }
    }

    RU.FileDialogHost {
        id: picker
        title: I18n.tr("Choose your picture")
        nameFilters: [I18n.tr("Pictures") + " (*.png *.jpg *.jpeg *.webp *.bmp *.gif)", I18n.tr("All files") + " (*)"]
        currentFolder: "file://" + (Quickshell.env("XDG_PICTURES_DIR") || ((Quickshell.env("HOME") || "/") + "/Pictures"))
        onAccepted: root.store.setPicture(decodeURIComponent(String(picker.selectedFile).replace(/^file:\/\//, "")))
    }

    function note(key, text) {
        var n = Object.assign({}, root.notes);
        n[key] = text;
        root.notes = n;
    }

    Loader {
        id: dialogs
        active: root.ready
        sourceComponent: Item {
            readonly property alias add: addDialog
            readonly property alias password: passwordDialog
            readonly property alias limit: limitDialog
            readonly property alias remove: removeDialog
            readonly property alias setup: setupDialog
            readonly property alias checkpoints: checkpointsDialog
            readonly property alias reset: resetDialog
            readonly property alias removeProfile: removeProfileDialog

            AddPersonDialog {
                id: addDialog
                parent: root.hub.overlayItem
                store: root.store
                onProfileRequested: definition => setupDialog.openFor(definition, null)
                onAdded: name => {
                    root.peopleNote = I18n.tr("%1 can sign in now. Their desktop is set up the first time they do.").arg(name);
                    root.spotName = name;
                    root.awaitName = name;
                    root.focusRow("role");
                }
            }
            PasswordDialog {
                id: passwordDialog
                parent: root.hub.overlayItem
                store: root.store
                onChanged: name => root.note(name + "/password", I18n.tr("Password changed"))
            }
            StorageLimitDialog {
                id: limitDialog
                parent: root.hub.overlayItem
                store: root.store
                onChanged: name => root.note(name + "/limit", I18n.tr("Limit saved"))
            }
            RemovePersonDialog {
                id: removeDialog
                parent: root.hub.overlayItem
                store: root.store
                onRemoved: name => root.peopleNote = removeDialog.erase
                    ? I18n.tr("%1 was removed and their files erased.").arg(name)
                    : I18n.tr("%1 was removed. Their home folder is in /home/.removed.").arg(name)
            }
            ProfileSetupDialog {
                id: setupDialog
                parent: root.hub.overlayItem
                store: root.store
                onCompleted: account => {
                    root.store.refresh();
                    root.store.refreshProfiles();
                }
            }
            ProfileCheckpointsDialog {
                id: checkpointsDialog
                parent: root.hub.overlayItem
                hub: root.hub
                onChanged: root.store.refreshProfiles()
            }
            ProfileResetDialog {
                id: resetDialog
                parent: root.hub.overlayItem
                onCompleted: root.store.refreshProfiles()
            }
            ProfileRemoveDialog {
                id: removeProfileDialog
                parent: root.hub.overlayItem
                onCompleted: {
                    root.store.refresh();
                    root.store.refreshProfiles();
                }
            }
        }
    }
}
