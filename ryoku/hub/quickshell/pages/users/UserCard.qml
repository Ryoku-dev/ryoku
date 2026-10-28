pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: cardRoot

    required property var userData
    required property var permissionDefs
    required property string selfName
    required property var storageCaps
    required property int adminCount
    property bool canManage: false
    property var selectedPermissions: []
    property int avatarRevision: 0
    property bool avatarBusy: false

    signal avatarChooseRequested()
    signal avatarRemoveRequested()
    signal roleRequested(string name, string role)
    signal permissionsRequested(string name, var permissions)
    signal passwordRequested(string name)
    signal limitRequested(string name, double currentBytes)
    signal removeRequested(string name)

    implicitHeight: card.implicitHeight

    function initialFor(user) {
        var value = String(user.fullName || user.name || "?").trim();
        return value.length > 0 ? value.charAt(0).toUpperCase() : "?";
    }

    function formatBytes(value) {
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

    function permissionLabel(id) {
        for (var i = 0; i < cardRoot.permissionDefs.length; i++) {
            var def = cardRoot.permissionDefs[i];
            if (String(def.id) === id)
                return I18n.tr(String(def.label || def.id));
        }
        return id;
    }

    function samePermissions(left, right) {
        var a = (left || []).slice().sort();
        var b = (right || []).slice().sort();
        return JSON.stringify(a) === JSON.stringify(b);
    }

    function syncPermissions() {
        cardRoot.selectedPermissions = (cardRoot.userData.permissions || []).slice();
    }

    function togglePermission(id) {
        var next = cardRoot.selectedPermissions.slice();
        var index = next.indexOf(id);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(id);
        cardRoot.selectedPermissions = next;
    }

    readonly property bool permissionsDirty: !samePermissions(cardRoot.selectedPermissions, cardRoot.userData.permissions || [])
    readonly property var storage: cardRoot.userData.storage || ({ "supported": false, "reason": I18n.tr("Storage information is unavailable.") })
    readonly property real storageFraction: Number(storage.limitBytes || 0) > 0
        ? Math.max(0, Math.min(1, Number(storage.usedBytes || 0) / Number(storage.limitBytes))) : 0
    readonly property bool isSelf: String(cardRoot.userData.name || "") === cardRoot.selfName
    readonly property bool lastAdministrator: cardRoot.userData.role === "admin" && cardRoot.adminCount <= 1

    onUserDataChanged: syncPermissions()
    Component.onCompleted: syncPermissions()

    SettingCard {
        id: card
        width: parent.width
        title: String(cardRoot.userData.name || "").toUpperCase()
        kana: cardRoot.initialFor(cardRoot.userData)
        collapsible: false

        Item {
            width: parent.width
            implicitHeight: 88

            Rectangle {
                id: avatar
                anchors { left: parent.left; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                width: 48
                height: 48
                radius: 24
                color: Tokens.bone

                Text {
                    anchors.centerIn: parent
                    visible: !avatarImage.hasImage
                    text: cardRoot.initialFor(cardRoot.userData)
                    color: Tokens.inkOnBone
                    font.family: Tokens.display
                    font.pixelSize: 24
                }

                AvatarImage {
                    id: avatarImage
                    anchors.fill: parent
                    userName: String(cardRoot.userData.name || "")
                    home: String(cardRoot.userData.home || "")
                    revision: cardRoot.avatarRevision
                }
            }

            Column {
                anchors { left: avatar.right; right: stateColumn.left; leftMargin: Tokens.s3; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s1

                Text {
                    width: parent.width
                    text: String(cardRoot.userData.fullName || cardRoot.userData.name || "")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fValue
                    elide: Text.ElideRight
                }
                Row {
                    spacing: Tokens.s2
                    Rectangle {
                        width: roleText.implicitWidth + Tokens.s3
                        height: 22
                        radius: Tokens.radius
                        color: Tokens.tint10
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Text {
                            id: roleText
                            anchors.centerIn: parent
                            text: cardRoot.userData.role === "admin" ? I18n.tr("Administrator") : I18n.tr("Standard")
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                        }
                    }
                    Rectangle {
                        visible: cardRoot.userData.name === cardRoot.selfName
                        width: youText.implicitWidth + Tokens.s3
                        height: 22
                        radius: Tokens.radius
                        color: Tokens.bone
                        Text {
                            id: youText
                            anchors.centerIn: parent
                            text: I18n.tr("You")
                            color: Tokens.inkOnBone
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                        }
                    }
                }
            }

            Column {
                id: stateColumn
                anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s2
                Row {
                    anchors.right: parent.right
                    spacing: Tokens.s2
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 8
                        height: 8
                        radius: 4
                        color: cardRoot.userData.loggedIn ? Tokens.sun : Tokens.lineStrong
                    }
                    Text {
                        text: cardRoot.userData.loggedIn ? I18n.tr("Signed in") : I18n.tr("Signed out")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                    }
                }
                Text {
                    anchors.right: parent.right
                    text: "UID " + String(cardRoot.userData.uid || "")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                }
            }
        }

        Rectangle { visible: cardRoot.isSelf; width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            visible: cardRoot.isSelf
            width: parent.width
            implicitHeight: visible ? photoRow.implicitHeight + Tokens.s4 * 2 : 0

            Row {
                id: photoRow
                anchors { left: parent.left; right: parent.right; margins: Tokens.s4; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s3

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - photoButtons.width - Tokens.s3
                    text: I18n.tr("PROFILE PICTURE")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }
                Row {
                    id: photoButtons
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Btn {
                        compact: true
                        text: avatarImage.hasImage ? I18n.tr("CHANGE") : I18n.tr("CHOOSE")
                        armed: !cardRoot.avatarBusy
                        onAct: cardRoot.avatarChooseRequested()
                    }
                    Btn {
                        visible: avatarImage.hasImage
                        compact: true
                        text: I18n.tr("REMOVE")
                        armed: !cardRoot.avatarBusy
                        onAct: cardRoot.avatarRemoveRequested()
                    }
                }
            }
        }

        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            width: parent.width
            implicitHeight: 76

            Column {
                anchors { left: parent.left; right: parent.right; margins: Tokens.s4; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s2
                Row {
                    width: parent.width
                    Text {
                        width: parent.width - storageValue.width - Tokens.s3
                        text: I18n.tr("HOME STORAGE")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Text {
                        id: storageValue
                        text: {
                            if (!cardRoot.storage.supported)
                                return I18n.tr(String(cardRoot.storage.reason || "Storage information is unavailable."));
                            var used = cardRoot.formatBytes(cardRoot.storage.usedBytes || 0);
                            return Number(cardRoot.storage.limitBytes || 0) > 0
                                ? used + " / " + cardRoot.formatBytes(cardRoot.storage.limitBytes)
                                : used + " · " + I18n.tr("No limit");
                        }
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                    }
                }
                Rectangle {
                    width: parent.width
                    height: 5
                    radius: 2
                    color: Tokens.tint10
                    Rectangle {
                        width: parent.width * cardRoot.storageFraction
                        height: parent.height
                        radius: parent.radius
                        color: cardRoot.storageFraction > 0.9 ? Tokens.alert : Tokens.bone
                    }
                }
            }
        }

        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            width: parent.width
            implicitHeight: permissionColumn.implicitHeight + Tokens.s4 * 2

            Column {
                id: permissionColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                spacing: Tokens.s3

                Row {
                    width: parent.width
                    Text {
                        width: parent.width - savePermissions.width - Tokens.s3
                        text: I18n.tr("PERMISSIONS")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Btn {
                        id: savePermissions
                        visible: cardRoot.canManage
                        compact: true
                        text: I18n.tr("SAVE")
                        primary: cardRoot.permissionsDirty
                        armed: cardRoot.permissionsDirty
                        onAct: cardRoot.permissionsRequested(String(cardRoot.userData.name), cardRoot.selectedPermissions.slice())
                    }
                }

                Flow {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: cardRoot.permissionDefs.filter(function(def) {
                            return String(def.id || "") !== "admin";
                        })
                        delegate: Rectangle {
                            id: permissionChip
                            required property var modelData
                            readonly property string permissionId: String(modelData.id || "")
                            readonly property bool selected: cardRoot.selectedPermissions.indexOf(permissionId) >= 0
                            width: permissionRow.implicitWidth + Tokens.s3 * 2
                            height: 25
                            radius: Tokens.radius
                            color: selected ? Tokens.bone : "transparent"
                            border.width: Tokens.border
                            border.color: selected ? Tokens.bone : Tokens.line
                            opacity: cardRoot.canManage ? 1 : 0.78

                            Row {
                                id: permissionRow
                                anchors.centerIn: parent
                                spacing: Tokens.s1
                                Text {
                                    text: permissionChip.selected ? "✓" : "+"
                                    color: permissionChip.selected ? Tokens.inkOnBone : Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                }
                                Text {
                                    text: cardRoot.permissionLabel(permissionChip.permissionId)
                                    color: permissionChip.selected ? Tokens.inkOnBone : Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fMicro
                                }
                            }
                            HoverHandler { enabled: cardRoot.canManage; cursorShape: Qt.PointingHandCursor }
                            TapHandler { enabled: cardRoot.canManage; onTapped: cardRoot.togglePermission(permissionChip.permissionId) }
                        }
                    }
                    Text {
                        visible: cardRoot.permissionDefs.length === 0
                        text: I18n.tr("No optional permissions are available on this system.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                    }
                }
            }
        }

        Rectangle { visible: cardRoot.canManage; width: parent.width; height: 1; color: Tokens.lineSoft }

        Item {
            visible: cardRoot.canManage
            width: parent.width
            implicitHeight: visible ? adminColumn.implicitHeight + Tokens.s4 * 2 : 0

            Column {
                id: adminColumn
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                spacing: Tokens.s3

                Row {
                    spacing: Tokens.s3
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("ROLE")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Chips {
                        options: ["admin", "standard"]
                        labels: ({ "admin": I18n.tr("Administrator"), "standard": I18n.tr("Standard") })
                        current: String(cardRoot.userData.role || "standard")
                        enabled: !cardRoot.lastAdministrator
                        opacity: enabled ? 1 : 0.5
                        onChose: role => {
                            if (role !== cardRoot.userData.role)
                                cardRoot.roleRequested(String(cardRoot.userData.name), role);
                        }
                    }
                }

                Flow {
                    width: parent.width
                    spacing: Tokens.s2
                    Btn {
                        compact: true
                        text: I18n.tr("RESET PASSWORD")
                        onAct: cardRoot.passwordRequested(String(cardRoot.userData.name))
                    }
                    Btn {
                        visible: cardRoot.storageCaps && cardRoot.storageCaps.quota === true
                        compact: true
                        text: I18n.tr("SET LIMIT")
                        onAct: cardRoot.limitRequested(String(cardRoot.userData.name), Number(cardRoot.storage.limitBytes || 0))
                    }
                    Btn {
                        compact: true
                        text: I18n.tr("REMOVE")
                        armed: !cardRoot.isSelf && !cardRoot.lastAdministrator
                        onAct: cardRoot.removeRequested(String(cardRoot.userData.name))
                    }
                }

                Text {
                    visible: cardRoot.isSelf
                    width: parent.width
                    text: I18n.tr("You can't remove yourself.")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
                Text {
                    visible: cardRoot.lastAdministrator
                    width: parent.width
                    text: I18n.tr("Add another administrator first.")
                    color: Tokens.inkFaint
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
            }
        }
    }
}
