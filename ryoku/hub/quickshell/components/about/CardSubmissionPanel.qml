pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    property var hub: null
    required property var accountPanel
    readonly property var draft: CardAccount.draft
    readonly property var submission: CardAccount.submission
    spacing: LithicTokens.space.s3
    Label { text: I18n.tr("Submit for online review"); typeRole: "section" }
    Note { text: I18n.tr("The exact original ZIP and a separate device-signed inventory go to private quarantine. Local checking is not an approval or a proof of rights. Independent inspection and authorized content review must finish before publication. Your signature confirms this action and these bytes, not a mathematical proof that a local check happened.") }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Plaque { text: I18n.tr("Design folder"); kind: "line"; enabled: !CardAccount.busy; onClicked: root.accountPanel.chooseDesign() }
        Plaque { text: I18n.tr("Image or animation"); kind: "line"; enabled: !CardAccount.busy; onClicked: root.accountPanel.chooseHero() }
    }
    TextField {
        width: parent.width
        text: root.accountPanel.submissionPath
        placeholderText: I18n.tr("Local design folder or ZIP, or image path")
        Accessible.name: I18n.tr("Submission source path")
        onTextEdited: { root.accountPanel.submissionPath = text; CardAccount.draft = null; }
    }
    Segmented { width: parent.width; model: [I18n.tr("Design pack"), I18n.tr("Hero image")]; currentIndex: root.accountPanel.submissionKind === "design" ? 0 : 1; accessibleName: I18n.tr("Submission kind"); onActivated: index => { root.accountPanel.submissionKind = index === 0 ? "design" : "hero"; CardAccount.draft = null; } }
    Plaque { text: I18n.tr("Inspect exact submission"); kind: "line"; busy: CardAccount.job.indexOf("submission prepare") === 0; enabled: !CardAccount.busy && root.accountPanel.cardConsentReady && root.accountPanel.submissionPath !== ""; onClicked: CardAccount.prepare(root.accountPanel.submissionKind, root.accountPanel.submissionPath) }
    Column {
        width: parent.width
        visible: !!root.draft
        spacing: LithicTokens.space.s3
        Note { text: root.draft ? I18n.tr("Prepared %1 · %2 files\nAccount %3 · card %4\nGate %5\nOriginal ZIP SHA-256 %6\nAccount terms version %7\nCard consent version %8\nExpires ten minutes after preparation.").replace("%1", root.draft.kind).replace("%2", root.draft.files.length).replace("%3", root.draft.accountID).replace("%4", root.draft.cardID).replace("%5", root.draft.gate).replace("%6", root.draft.originalSha).replace("%7", root.draft.termsVersion).replace("%8", root.draft.attestationVersion) : "" }
        AnimatedImage {
            width: parent.width
            height: visible ? 220 : 0
            visible: !!root.draft && root.draft.kind === "hero"
            source: visible ? "file://" + root.draft.preview : ""
            fillMode: Image.PreserveAspectFit
            playing: visible && root.accountPanel.live
            cache: false
        }
        Plaque { text: I18n.tr("Preview checked design on card"); kind: "line"; visible: !!root.draft && root.draft.kind === "design"; enabled: visible && !CardAccount.busy && Badge.job === ""; onClicked: Badge.previewFolder(root.draft.preview) }
        Repeater {
            model: root.draft ? root.draft.files : []
            delegate: Label { required property var modelData; width: parent.width; text: modelData.name + " · " + modelData.bytes + " bytes · " + modelData.sha; typeRole: "caption"; textFormat: Text.PlainText; wrapMode: Text.Wrap; elide: Text.ElideNone }
        }
        SettingRow {
            width: parent.width
            title: I18n.tr("I own this content or have permission to publish it")
            detail: I18n.tr("Including depicted people's consent and every bundled asset")
            Toggle { checked: root.accountPanel.rights; Accessible.name: I18n.tr("Confirm content rights"); onToggled: root.accountPanel.rights = checked }
        }
        SettingRow {
            width: parent.width
            title: I18n.tr("I accept the disclosed current online-card policy")
            detail: I18n.tr("For this exact submission; no future signatures or payments")
            Toggle { checked: root.accountPanel.policy; Accessible.name: I18n.tr("Confirm submission policy"); onToggled: root.accountPanel.policy = checked }
        }
        SettingRow {
            width: parent.width
            title: I18n.tr("I reviewed the full intended animation and permit inspection")
            detail: I18n.tr("Private quarantine, technical inspection and authorized content review as disclosed")
            Toggle { checked: root.accountPanel.review; Accessible.name: I18n.tr("Confirm inspection and review consent"); onToggled: root.accountPanel.review = checked }
        }
        Plaque { text: I18n.tr("Sign exact bytes and submit"); kind: "gold"; enabled: !CardAccount.busy && root.accountPanel.rights && root.accountPanel.policy && root.accountPanel.review; onClicked: CardAccount.submit() }
    }
    Column {
        width: parent.width
        visible: !!root.submission && !!root.submission.id
        spacing: LithicTokens.space.s2
        Label { text: I18n.tr("Submission status"); typeRole: "section" }
        TextField { width: parent.width; readOnly: true; text: root.submission ? root.submission.id : ""; Accessible.name: I18n.tr("Submission job UUID") }
        Note { text: root.submission ? I18n.tr("State: %1. Uploading, queued, inspecting, needs review and awaiting signature are not approval. Refresh to read the service's current state.").replace("%1", root.submission.state) : "" }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Plaque { text: I18n.tr("Refresh job"); kind: "line"; enabled: !CardAccount.busy; onClicked: CardAccount.submissionAction("status") }
            Plaque { text: I18n.tr("Retry immutable upload"); kind: "line"; visible: !!root.submission && root.submission.state === "uploading"; enabled: !CardAccount.busy; onClicked: CardAccount.submissionAction("upload") }
            Plaque { text: I18n.tr("Cancel job"); kind: "quiet"; visible: !!root.submission && ["uploading", "queued"].indexOf(root.submission.state) >= 0; enabled: !CardAccount.busy; onClicked: CardAccount.submissionAction("cancel") }
            Plaque { text: I18n.tr("Use approved publication"); kind: "gold"; visible: !!root.submission && root.submission.state === "approved"; enabled: !CardAccount.busy; onClicked: CardAccount.submissionAction("select") }
        }
    }
    Note { text: CardAccount.status.selection ? I18n.tr("Selected approved publication: %1. Selection does not turn live-card sync on.").replace("%1", CardAccount.status.selection) : I18n.tr("No approved publication selected. Signed static sharing remains separate; live-card sync stays off unless you choose it.") }
    Plaque { text: I18n.tr("Clear approved selection"); kind: "quiet"; visible: !!CardAccount.status.selection; enabled: !CardAccount.busy; onClicked: CardAccount.submissionAction("clear") }
    component Note: Label {
        textFormat: Text.PlainText
        width: root.width
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
