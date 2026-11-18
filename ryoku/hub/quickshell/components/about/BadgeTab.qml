pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../badge"
import ".."

Column {
    id: root
    property var hub: null
    property bool live: false
    property string folder: ""
    property string copyError: ""
    // every badge, tester to founder, may have its own card design
    readonly property bool holder: !!Badge.badge
    readonly property bool idle: Badge.job === ""
    readonly property var roles: ["tester", "contributor", "nixos", "core", "founder"]
    readonly property var emblemSpec: Badge.ownDesign && Badge.ownDesign.spec ? Badge.ownDesign.spec.badge : null
    spacing: LithicTokens.space.s4

    function copy(text) {
        root.copyError = "";
        Changes.run(["wl-copy", "--", String(text)], function(ok) {
            if (!ok)
                root.copyError = I18n.tr("The code could not be copied. Select it and copy it yourself.");
        });
    }
    function filename() {
        var name = Badge.candidate ? Badge.candidate.name : "card";
        return "ryoku-design-" + String(name).toLowerCase().replace(/[^a-z0-9_-]+/g, "-") + ".zip";
    }
    FolderChooser {
        id: folderPick
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Load a design folder")
        onAccepted: {
            root.folder = folderPick.localPath(folderPick.selectedFolder);
            Badge.previewFolder(root.folder);
        }
    }
    FolderChooser {
        id: savePick
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose where to save the design pack")
        currentFolder: "file://" + (Quickshell.env("HOME") || "") + "/Downloads"
        onAccepted: output.text = savePick.localPath(savePick.selectedFolder) + "/" + root.filename()
    }
    Certificate {
        id: certificate
        parent: root.hub ? root.hub.overlayItem : root
        badge: Badge.badge
        spec: root.emblemSpec
    }
    Row {
        width: parent.width
        spacing: LithicTokens.space.s4
        Emblem {
            visible: !!Badge.badge
            role: Badge.badge ? Badge.badge.role : ""
            spec: root.emblemSpec
            live: root.live && root.visible
            size: 150
        }
        Column {
            width: Math.max(0, parent.width - 150 - parent.spacing)
            spacing: LithicTokens.space.s2
            Label { width: parent.width; text: Badge.badge ? Badge.badge.role.charAt(0).toUpperCase() + Badge.badge.role.slice(1) : I18n.tr("No badge yet"); typeRole: "title"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            Label { width: parent.width; text: Badge.badge ? Badge.badge.login : I18n.tr("Activate a code below"); typeRole: "body"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            Label {
                width: parent.width
                text: Badge.badge ? I18n.tr("Issued by %1 · %2").replace("%1", "neur0map").replace("%2", Badge.badge.issued) : ""
                visible: !!Badge.badge
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                text: Badge.badge && Badge.badge.program ? I18n.tr("Program: %1").replace("%1", Badge.badge.program) : ""
                visible: text !== ""
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque { visible: !!Badge.badge; small: true; kind: "line"; text: I18n.tr("View certificate"); onClicked: certificate.open() }
        }
    }
    Note {
        text: Badge.error || root.copyError || Badge.message
        visible: text !== ""
        ink: Badge.error || root.copyError ? Lithic.jasper : Lithic.fg2
    }
    CardOnlineSwitch {
        width: parent.width
        hub: root.hub
        showsError: false
    }
    Label { text: I18n.tr("Your request code"); typeRole: "section" }
    Note { text: I18n.tr("Testers and contributors: run /badge in the claim-badge channel on Discord. Team members: send this request code to nero. A code works only on this machine.") }
    TextField { width: parent.width; text: Badge.request; readOnly: true; Accessible.name: I18n.tr("Your request code") }
    Plaque { small: true; kind: "line"; glyph: "content_copy"; text: I18n.tr("Copy request code"); enabled: Badge.request !== ""; onClicked: root.copy(Badge.request) }
    TextField { id: activation; width: parent.width; placeholderText: I18n.tr("Paste your badge code"); Accessible.name: I18n.tr("Badge activation code") }
    Plaque { kind: "gold"; text: I18n.tr("Activate"); busy: Badge.job === "activate"; enabled: root.idle && activation.text.trim() !== ""; onClicked: Badge.activate(activation.text) }

    Column {
        width: parent.width
        visible: root.holder
        spacing: LithicTokens.space.s3
        Label { text: I18n.tr("Your card design"); typeRole: "section" }
        Note {
            text: Badge.design ? I18n.tr("Approved design: %1. Friends see it on your card.").replace("%1", Badge.design.name)
                : Badge.worn ? I18n.tr("Wearing %1 on this machine. Friends see your classic card until nero approves it.").replace("%1", Badge.worn.name)
                : I18n.tr("No design yet. Wear one on this machine now, and send it to Mika so friends see it too.")
        }
        Note {
            visible: Badge.wornError !== ""
            text: I18n.tr("The design you wear no longer passes the check, so your card is classic: %1").replace("%1", Badge.wornError)
            ink: Lithic.jasper
        }
        Note { text: I18n.tr("A design folder holds design.json and its pictures, including animated WebP. It cannot contain QML, shaders or scripts.") }
        Plaque { kind: "line"; text: I18n.tr("Load design folder"); enabled: root.idle; busy: Badge.job === "check"; onClicked: folderPick.open() }
        Note { visible: root.folder !== ""; text: root.folder }
        Note {
            visible: !!Badge.checked
            text: !Badge.checked ? "" : Badge.checked.ok ? I18n.tr("Design checked · %1 files · %2 bytes").replace("%1", Badge.checked.files.length).replace("%2", Badge.checked.bytes) : I18n.tr("The design needs changes before it can be worn or packed.")
        }
        Repeater {
            model: Badge.checked ? Badge.checked.errors || [] : []
            delegate: Note { required property string modelData; text: modelData; ink: Lithic.jasper }
        }
        SettingRow {
            width: parent.width
            title: I18n.tr("Preview on my card")
            detail: I18n.tr("Shows it while the studio is open")
            activatable: !!Badge.candidate
            onActivated: Badge.preview = Badge.previewing ? null : Badge.candidate
            Toggle { checked: Badge.previewing; enabled: !!Badge.candidate; Accessible.name: I18n.tr("Preview on my card"); onToggled: Badge.preview = checked ? Badge.candidate : null }
        }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Plaque {
                kind: "line"
                text: I18n.tr("Wear on this machine")
                visible: !Badge.design
                enabled: root.idle && !!Badge.checked && Badge.checked.ok && root.folder !== ""
                busy: Badge.job === "wear"
                onClicked: Badge.wear(root.folder)
            }
            Plaque { kind: "quiet"; text: I18n.tr("Stop wearing"); visible: !!Badge.worn; enabled: root.idle; busy: Badge.job === "unwear"; onClicked: Badge.unwear() }
        }
        Note { visible: !Badge.design; text: I18n.tr("A worn design stays on this machine's card. It is never sent to friends or the team service.") }
        TextField {
            id: output
            width: parent.width
            text: (Quickshell.env("HOME") || "") + "/Downloads/" + root.filename()
            Accessible.name: I18n.tr("Design pack save path")
        }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Plaque { small: true; kind: "quiet"; text: I18n.tr("Choose save folder"); onClicked: savePick.open() }
            Plaque { kind: "gold"; text: I18n.tr("Save for Mika"); enabled: root.idle && !!Badge.checked && Badge.checked.ok && root.folder !== "" && output.text.charAt(0) === "/"; busy: Badge.job === "pack"; onClicked: Badge.saveDesign(root.folder, output.text) }
        }
        Note { text: I18n.tr("To have friends see it, use /card submit in the claim-badge channel to send the zip to Mika. nero approves it first.") }
    }
    Column {
        width: parent.width
        visible: Badge.owner
        spacing: LithicTokens.space.s3
        Label { text: I18n.tr("Owner tools"); typeRole: "section" }
        TextField { id: memberRequest; width: parent.width; placeholderText: I18n.tr("Member's request code"); Accessible.name: placeholderText }
        TextField { id: login; width: parent.width; placeholderText: I18n.tr("GitHub login"); Accessible.name: placeholderText }
        Segmented { id: role; width: parent.width; model: [I18n.tr("Tester"), I18n.tr("Contributor"), I18n.tr("NixOS"), I18n.tr("Core"), I18n.tr("Founder")]; accessibleName: I18n.tr("Badge role") }
        TextField { id: program; width: parent.width; placeholderText: I18n.tr("Program tested, optional (for example Ryoku v2 Hyperion)"); Accessible.name: placeholderText }
        Plaque { kind: "gold"; text: I18n.tr("Make a code"); enabled: root.idle && memberRequest.text.trim() !== "" && login.text.trim() !== ""; busy: Badge.job === "issue"; onClicked: Badge.issue({ request: memberRequest.text, login: login.text, role: root.roles[role.currentIndex], program: program.text }) }
        TextField { width: parent.width; visible: Badge.issuedCode !== ""; text: Badge.issuedCode; readOnly: true; Accessible.name: I18n.tr("Issued badge code") }
        Plaque { small: true; visible: Badge.issuedCode !== ""; kind: "line"; text: I18n.tr("Copy code"); onClicked: root.copy(Badge.issuedCode) }
        Label { text: I18n.tr("Preview a submitted design"); typeRole: "label" }
        TextField { id: pending; width: parent.width; placeholderText: I18n.tr("Paste the SHA from Mika's review"); Accessible.name: placeholderText }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Plaque { kind: "line"; text: I18n.tr("Preview submitted design"); enabled: root.idle && pending.text.trim() !== ""; busy: Badge.job === "fetch"; onClicked: Badge.previewPending(pending.text) }
            Plaque { kind: "quiet"; text: I18n.tr("Clear preview"); enabled: Badge.previewing; onClicked: Badge.clearPreview() }
        }
        TextField { id: revoked; width: parent.width; placeholderText: I18n.tr("Badge code or badge id to revoke"); Accessible.name: placeholderText }
        Plaque { kind: "line"; text: I18n.tr("Revoke"); enabled: root.idle && revoked.text.trim() !== ""; busy: Badge.job === "revoke"; onClicked: Badge.revoke(revoked.text) }
    }
    component Note: Label {
        width: root.width
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
