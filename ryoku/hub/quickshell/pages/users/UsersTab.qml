pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root

    spacing: Tokens.s5

    property var users: []
    property string selfName: ""
    property var permissionDefs: []
    property var storageCaps: ({ "subvolumes": false, "quota": false, "reason": "" })
    property bool listLoaded: false
    property bool capsLoaded: false
    property bool refreshPending: false
    property string loadError: ""
    property string actionError: ""
    property int avatarRevision: 0

    readonly property bool supported: Host.supports("users")
    readonly property string unsupportedReason: Host.reason("users") !== ""
        ? Host.reason("users") : I18n.tr("User accounts are not available on this system.")
    readonly property var selfAccount: {
        for (var i = 0; i < root.users.length; i++)
            if (String(root.users[i].name || "") === root.selfName)
                return root.users[i];
        return null;
    }
    readonly property bool canManage: root.selfAccount !== null && root.selfAccount.role === "admin"
    readonly property int adminCount: {
        let count = 0;
        for (let i = 0; i < root.users.length; i++)
            if (root.users[i].role === "admin")
                count++;
        return count;
    }

    function roleFor(name) {
        for (var i = 0; i < root.users.length; i++)
            if (String(root.users[i].name || "") === name)
                return String(root.users[i].role || "standard");
        return "standard";
    }

    function parseError(text, fallback) {
        try {
            var value = JSON.parse(String(text || ""));
            if (value && value.error)
                return String(value.error);
        } catch (e) {
        }
        var plain = String(text || "").trim();
        return plain !== "" ? plain : fallback;
    }

    function refresh() {
        if (!root.supported)
            return;
        if (listProc.running) {
            root.refreshPending = true;
            return;
        }
        root.refreshPending = false;
        listProc.running = true;
    }

    function loadCapabilities() {
        if (!root.supported || capsProc.running)
            return;
        capsProc.running = true;
    }

    function startIfSupported() {
        if (!root.supported)
            return;
        root.refresh();
        if (!root.capsLoaded)
            root.loadCapabilities();
    }

    function runMutation(args, password) {
        if (!root.canManage || actionProc.running)
            return;
        root.actionError = "";
        actionProc.command = ["pkexec", "ryoku-host", "users"].concat(args);
        actionProc.pendingSecret = password === undefined ? "" : String(password);
        actionProc.hasSecret = password !== undefined;
        actionProc.stdinEnabled = actionProc.hasSecret;
        actionProc.running = true;
    }

    function addPerson(name, fullName, role, password, permissions, limitBytes) {
        var args = ["add", name, "--full-name", fullName, "--role", role];
        for (var i = 0; i < permissions.length; i++)
            args.push("--perm", String(permissions[i]));
        if (Number(limitBytes || 0) > 0)
            args.push("--limit", String(Math.round(limitBytes)));
        root.runMutation(args, password);
    }

    function setRole(name, role) {
        root.runMutation(["set-role", name, role]);
    }

    function setPermissions(name, permissions) {
        var next = [];
        for (var i = 0; i < permissions.length; i++)
            if (String(permissions[i]) !== "admin")
                next.push(String(permissions[i]));
        if (root.roleFor(name) === "admin")
            next.unshift("admin");
        root.runMutation(["set-perms", name, next.join(",")]);
    }

    function setPassword(name, password) {
        root.runMutation(["set-password", name], password);
    }

    function setLimit(name, limitBytes) {
        root.runMutation(["set-limit", name, String(Math.round(limitBytes))]);
    }

    function removePerson(name, eraseFiles) {
        root.runMutation(["delete", name, eraseFiles ? "--erase-files" : "--keep-files"]);
    }

    // The picture lives in ~/.face (any user can write it) and is pushed to
    // AccountsService too, so login managers and other settings apps show the
    // same one. The second step is best effort: it needs the accounts daemon.
    function setAvatar(path) {
        if (avatarProc.running)
            return;
        root.actionError = "";
        avatarProc.command = ["sh", "-c", [
            'src=$1; face="$HOME/.face"; tmp="$face.new"',
            'if command -v magick >/dev/null 2>&1; then',
            '  magick "$src[0]" -auto-orient -thumbnail 512x512^ -gravity center -extent 512x512 -strip "PNG:$tmp" || { echo "Could not read that image."; exit 1; }',
            'else cp -- "$src" "$tmp" || { echo "Could not read that image."; exit 1; }; fi',
            '[ "$(stat -c %s "$tmp")" -gt 1000000 ] && magick "$tmp" -quality 90 "JPEG:$tmp"',
            'mv -f -- "$tmp" "$face" || exit 1',
            'busctl --system call org.freedesktop.Accounts "/org/freedesktop/Accounts/User$(id -u)" org.freedesktop.Accounts.User SetIconFile s "$face" >/dev/null 2>&1 || true'
        ].join("\n"), "sh", path];
        avatarProc.running = true;
    }

    function removeAvatar() {
        if (avatarProc.running)
            return;
        root.actionError = "";
        avatarProc.command = ["sh", "-c", [
            'rm -f -- "$HOME/.face" "$HOME/.face.icon"',
            'busctl --system call org.freedesktop.Accounts "/org/freedesktop/Accounts/User$(id -u)" org.freedesktop.Accounts.User SetIconFile s "" >/dev/null 2>&1 || true'
        ].join("\n")];
        avatarProc.running = true;
    }

    function closeDialogs() {
        addDialog.close();
        passwordDialog.close();
        limitDialog.close();
        removeDialog.close();
    }

    function finishMutation(exitCode, output) {
        if (exitCode === 0) {
            root.actionError = "";
            root.closeDialogs();
        } else {
            root.actionError = root.parseError(output, I18n.tr("The account change could not be completed."));
        }
        root.refresh();
    }

    Component.onCompleted: startIfSupported()
    Connections {
        target: Host
        function onReadyChanged() { root.startIfSupported(); }
        function onUsersChanged() { root.startIfSupported(); }
    }

    Process {
        id: listProc
        command: ["ryoku-host", "users", "list"]
        environment: Spawn.env
        stdout: StdioCollector { id: listOut }
        stderr: StdioCollector { }
        onExited: function(exitCode) {
            if (exitCode === 0) {
                try {
                    var value = JSON.parse(String(listOut.text || "{}"));
                    if (value.error)
                        throw new Error(String(value.error));
                    root.selfName = String(value.self || "");
                    root.users = value.users instanceof Array ? value.users : [];
                    root.listLoaded = true;
                    root.loadError = "";
                } catch (e) {
                    root.loadError = String(e.message || e);
                }
            } else {
                root.loadError = root.parseError(listOut.text, I18n.tr("Could not load user accounts."));
            }
            if (root.refreshPending) {
                root.refreshPending = false;
                root.refresh();
            }
        }
    }

    Process {
        id: capsProc
        command: ["ryoku-host", "users", "caps"]
        environment: Spawn.env
        stdout: StdioCollector { id: capsOut }
        stderr: StdioCollector { }
        onExited: function(exitCode) {
            if (exitCode !== 0) {
                root.loadError = root.parseError(capsOut.text, I18n.tr("Could not load account permissions."));
                return;
            }
            try {
                var value = JSON.parse(String(capsOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
                root.permissionDefs = value.permissions instanceof Array
                    ? value.permissions.filter(function(def) { return String(def.id || "") !== "admin"; })
                    : [];
                root.storageCaps = value.storage || ({ "subvolumes": false, "quota": false, "reason": "" });
                root.capsLoaded = true;
            } catch (e) {
                root.loadError = String(e.message || e);
            }
        }
    }

    Process {
        id: actionProc
        property string pendingSecret: ""
        property bool hasSecret: false
        environment: Spawn.env
        stdinEnabled: false
        stdout: StdioCollector { id: actionOut }
        stderr: StdioCollector { }
        onStarted: {
            if (hasSecret) {
                write(pendingSecret + "\n");
                pendingSecret = "";
            }
        }
        onExited: function(exitCode) {
            pendingSecret = "";
            hasSecret = false;
            stdinEnabled = false;
            root.finishMutation(exitCode, actionOut.text);
        }
    }

    Process {
        id: avatarProc
        environment: Spawn.env
        stdout: StdioCollector { id: avatarOut }
        stderr: StdioCollector { }
        onExited: function(exitCode) {
            if (exitCode === 0)
                root.avatarRevision++;
            else
                root.actionError = root.parseError(avatarOut.text, I18n.tr("The profile picture could not be changed."));
        }
    }

    FileDialogHost {
        id: avatarPicker
        title: I18n.tr("Choose a profile picture")
        nameFilters: [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.bmp *.gif)", I18n.tr("All files") + " (*)"]
        currentFolder: "file://" + (Quickshell.env("XDG_PICTURES_DIR") || ((Quickshell.env("HOME") || "/") + "/Pictures"))
        onAccepted: root.setAvatar(decodeURIComponent(("" + avatarPicker.selectedFile).replace(/^file:\/\//, "")))
    }

    SettingCard {
        visible: Host.ready && !root.supported
        width: parent.width
        title: I18n.tr("USERS")
        kana: "人"
        collapsible: false

        Item {
            width: parent.width
            implicitHeight: unsupportedCopy.implicitHeight + Tokens.s5 * 2
            Text {
                id: unsupportedCopy
                anchors { left: parent.left; right: parent.right; margins: Tokens.s5; verticalCenter: parent.verticalCenter }
                text: root.unsupportedReason
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
                wrapMode: Text.WordWrap
            }
        }
    }

    SettingCard {
        visible: !Host.ready
        width: parent.width
        title: I18n.tr("USERS")
        kana: "人"
        collapsible: false
        Item {
            width: parent.width
            implicitHeight: 68
            Text {
                anchors { left: parent.left; right: parent.right; margins: Tokens.s5; verticalCenter: parent.verticalCenter }
                text: I18n.tr("Checking account support…")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
            }
        }
    }

    SettingCard {
        visible: root.supported
        width: parent.width
        title: I18n.tr("PEOPLE")
        kana: "人"
        collapsible: false

        Item {
            width: parent.width
            implicitHeight: peopleHeader.implicitHeight + Tokens.s4 * 2

            Row {
                id: peopleHeader
                anchors { left: parent.left; right: parent.right; margins: Tokens.s4; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s3

                Column {
                    width: parent.width - (root.canManage ? addButton.width + Tokens.s3 : 0)
                    spacing: Tokens.s1
                    Text {
                        width: parent.width
                        text: root.canManage ? I18n.tr("Accounts on this computer") : I18n.tr("Ask an administrator")
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fRow
                    }
                    Text {
                        width: parent.width
                        text: root.canManage
                            ? I18n.tr("Administrators can add people and manage roles, permissions, passwords and storage.")
                            : I18n.tr("You can view accounts here. An administrator must make account changes.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                }

                Btn {
                    id: addButton
                    visible: root.canManage
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("ADD PERSON")
                    primary: true
                    armed: !actionProc.running && root.capsLoaded
                    onAct: {
                        root.actionError = "";
                        addDialog.openForCreate();
                    }
                }
            }
        }
    }

    Text {
        visible: root.supported && (root.loadError !== "" || root.actionError !== "")
        width: parent.width
        text: root.actionError !== "" ? root.actionError : root.loadError
        color: Tokens.alert
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        font.weight: Font.Medium
        wrapMode: Text.WordWrap
    }

    Text {
        visible: root.supported && !root.listLoaded && root.loadError === ""
        width: parent.width
        text: I18n.tr("Loading people…")
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: Tokens.fBody
    }

    Text {
        visible: root.supported && root.listLoaded && root.users.length === 0 && root.loadError === ""
        width: parent.width
        text: I18n.tr("No user accounts were found.")
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: Tokens.fBody
    }

    Repeater {
        model: root.supported ? root.users : []
        delegate: UserCard {
            required property var modelData
            width: root.width
            userData: modelData
            selfName: root.selfName
            adminCount: root.adminCount
            permissionDefs: root.permissionDefs
            storageCaps: root.storageCaps
            canManage: root.canManage
            avatarRevision: root.avatarRevision
            avatarBusy: avatarProc.running
            onAvatarChooseRequested: avatarPicker.open()
            onAvatarRemoveRequested: root.removeAvatar()
            onRoleRequested: (name, role) => root.setRole(name, role)
            onPermissionsRequested: (name, permissions) => root.setPermissions(name, permissions)
            onPasswordRequested: name => {
                root.actionError = "";
                passwordDialog.openFor(name);
            }
            onLimitRequested: (name, currentBytes) => {
                root.actionError = "";
                limitDialog.openFor(name, currentBytes);
            }
            onRemoveRequested: name => {
                root.actionError = "";
                removeDialog.openFor(name);
            }
        }
    }

    AddPersonDialog {
        id: addDialog
        permissionDefs: root.permissionDefs
        storageCaps: root.storageCaps
        busy: actionProc.running
        error: root.actionError
        onSubmitted: (name, fullName, role, password, permissions, limitBytes) =>
            root.addPerson(name, fullName, role, password, permissions, limitBytes)
    }

    PasswordDialog {
        id: passwordDialog
        busy: actionProc.running
        error: root.actionError
        onSubmitted: (name, password) => root.setPassword(name, password)
    }

    StorageLimitDialog {
        id: limitDialog
        busy: actionProc.running
        error: root.actionError
        onSubmitted: (name, limitBytes) => root.setLimit(name, limitBytes)
    }

    RemovePersonDialog {
        id: removeDialog
        busy: actionProc.running
        error: root.actionError
        onSubmitted: (name, eraseFiles) => root.removePerson(name, eraseFiles)
    }
}
