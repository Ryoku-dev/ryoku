pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

Popup {
    id: dialog

    property var permissionDefs: []
    property var storageCaps: ({})
    property bool busy: false
    property string error: ""
    property var selectedPermissions: []
    property var installedProfiles: []
    property string localError: ""

    signal submitted(string name, string fullName, string role, string password, var permissions, double limitBytes)
    signal profileRequested(var profile)

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(540, parent ? parent.width - Tokens.s6 * 2 : 540)
    height: Math.min(form.implicitHeight + padding * 2, parent ? parent.height - Tokens.s6 * 2 : form.implicitHeight + padding * 2)
    padding: Tokens.s5
    closePolicy: busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.55) }

    function reset() {
        nameField.text = "";
        fullNameField.text = "";
        roleChoice.current = "standard";
        passwordField.text = "";
        confirmField.text = "";
        limitField.text = "";
        var defaults = [];
        for (var i = 0; i < dialog.permissionDefs.length; i++)
            if (String(dialog.permissionDefs[i].id || "") === "devices")
                defaults.push("devices");
        dialog.selectedPermissions = defaults;
        dialog.localError = "";
    }

    function openForCreate() {
        reset();
        open();
        nameField.grabFocus();
    }

    function togglePermission(id) {
        var next = dialog.selectedPermissions.slice();
        var index = next.indexOf(id);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(id);
        dialog.selectedPermissions = next;
    }

    function permissionLabel(def) {
        return I18n.tr(String(def.label || def.id || ""));
    }

    function submit() {
        dialog.localError = "";
        var name = nameField.text.trim();
        if (name === "") {
            dialog.localError = I18n.tr("Enter a user name.");
            nameField.grabFocus();
            return;
        }
        if (passwordField.text === "") {
            dialog.localError = I18n.tr("Enter a password.");
            passwordField.grabFocus();
            return;
        }
        if (passwordField.text !== confirmField.text) {
            dialog.localError = I18n.tr("Passwords do not match.");
            confirmField.grabFocus();
            return;
        }
        var limitBytes = 0;
        var rawLimit = limitField.text.trim();
        if (rawLimit !== "") {
            var gib = Number(rawLimit);
            if (!isFinite(gib) || gib <= 0) {
                dialog.localError = I18n.tr("Enter a storage limit greater than zero.");
                limitField.grabFocus();
                return;
            }
            limitBytes = Math.round(gib * 1073741824);
        }
        dialog.submitted(name, fullNameField.text.trim(), roleChoice.current,
                         passwordField.text, dialog.selectedPermissions.slice(), limitBytes);
        passwordField.text = "";
        confirmField.text = "";
    }

    contentItem: Flickable {
        id: scroll
        clip: true
        contentWidth: width
        contentHeight: form.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

        Column {
            id: form
            width: scroll.width - (scroll.contentHeight > scroll.height ? Tokens.s3 : 0)
            spacing: Tokens.s4

            Column {
                width: parent.width
                spacing: Tokens.s1
                Text {
                    text: I18n.tr("ADD PERSON")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                }
                Text {
                    text: I18n.tr("A separate Ryoku account")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fValue
                }
                Text {
                    width: parent.width
                    text: I18n.tr("They get a private home and their own desktop settings on first sign-in.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
            }

            Column {
                visible: dialog.installedProfiles.length > 0
                width: parent.width
                spacing: Tokens.s2
                Text {
                    text: I18n.tr("FROM A PROFILE")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Create a separate account from an installed Ryostore profile.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
                Flow {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: dialog.installedProfiles
                        delegate: Btn {
                            required property var modelData
                            text: I18n.tr("CREATE %1").arg(String(modelData.name || modelData.id || "").toUpperCase())
                            onAct: {
                                dialog.close();
                                dialog.profileRequested(modelData);
                            }
                        }
                    }
                }
                Rectangle {
                    width: parent.width
                    height: 1
                    color: Tokens.lineSoft
                }
                Text {
                    text: I18n.tr("OR CREATE A BLANK ACCOUNT")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
            }

            Row {
                width: parent.width
                spacing: Tokens.s3
                Column {
                    width: (parent.width - Tokens.s3) / 2
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("USER NAME")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Field { id: nameField; width: parent.width; tabular: true; placeholder: I18n.tr("sam") }
                }
                Column {
                    width: (parent.width - Tokens.s3) / 2
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("FULL NAME")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Field { id: fullNameField; width: parent.width; placeholder: I18n.tr("Sam Rivera") }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s2
                Text {
                    text: I18n.tr("ROLE")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Chips {
                    id: roleChoice
                    options: ["admin", "standard"]
                    labels: ({ "admin": I18n.tr("Administrator"), "standard": I18n.tr("Standard") })
                    current: "standard"
                    onChose: value => roleChoice.current = value
                }
            }

            Row {
                width: parent.width
                spacing: Tokens.s3
                Column {
                    width: (parent.width - Tokens.s3) / 2
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("PASSWORD")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Field { id: passwordField; width: parent.width; secret: true; placeholder: I18n.tr("Password") }
                }
                Column {
                    width: (parent.width - Tokens.s3) / 2
                    spacing: Tokens.s1
                    Text {
                        text: I18n.tr("CONFIRM")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Field {
                        id: confirmField
                        width: parent.width
                        secret: true
                        placeholder: I18n.tr("Repeat password")
                        onAccepted: dialog.submit()
                    }
                }
            }

            Column {
                width: parent.width
                spacing: Tokens.s2
                Text {
                    text: I18n.tr("PERMISSIONS")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Flow {
                    width: parent.width
                    spacing: Tokens.s2
                    Repeater {
                        model: dialog.permissionDefs.filter(function(def) {
                            return String(def.id || "") !== "admin";
                        })
                        delegate: Rectangle {
                            id: permissionChip
                            required property var modelData
                            readonly property string permissionId: String(modelData.id || "")
                            readonly property bool selected: dialog.selectedPermissions.indexOf(permissionId) >= 0
                            width: permissionText.implicitWidth + Tokens.s5
                            height: 26
                            radius: Tokens.radius
                            color: selected ? Tokens.bone : "transparent"
                            border.width: Tokens.border
                            border.color: selected ? Tokens.bone : Tokens.line
                            Text {
                                id: permissionText
                                anchors.centerIn: parent
                                text: (permissionChip.selected ? "✓  " : "+  ") + dialog.permissionLabel(permissionChip.modelData)
                                color: permissionChip.selected ? Tokens.inkOnBone : Tokens.inkMuted
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro
                            }
                            HoverHandler { cursorShape: Qt.PointingHandCursor }
                            TapHandler { onTapped: dialog.togglePermission(permissionChip.permissionId) }
                        }
                    }
                }
            }

            Column {
                visible: dialog.storageCaps && dialog.storageCaps.quota === true
                width: parent.width
                spacing: Tokens.s1
                Text {
                    text: I18n.tr("STORAGE LIMIT · OPTIONAL")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Field {
                    id: limitField
                    width: parent.width
                    tabular: true
                    placeholder: I18n.tr("GiB · blank means no limit")
                }
            }

            Text {
                visible: dialog.localError !== "" || dialog.error !== ""
                width: parent.width
                text: dialog.localError !== "" ? dialog.localError : dialog.error
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
            }

            Row {
                anchors.right: parent.right
                spacing: Tokens.s2
                Btn {
                    text: I18n.tr("CANCEL")
                    armed: !dialog.busy
                    onAct: dialog.close()
                }
                Btn {
                    text: dialog.busy ? I18n.tr("ADDING…") : I18n.tr("ADD PERSON")
                    primary: true
                    armed: !dialog.busy
                    onAct: dialog.submit()
                }
            }
        }
    }
}
