pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    property var hub: null
    property bool live: false
    property string submissionKind: "design"
    property string submissionPath: ""
    property bool rights: false
    property bool policy: false
    property bool review: false
    property bool unlinkConfirmation: false
    property string unlinkAccount: ""
    property string unlinkCard: ""
    readonly property bool cardConsentReady: !!CardAccount.status.consent
        && !!CardAccount.status.consent.account && CardAccount.status.consent.account.accepted === true
        && !!CardAccount.status.consent.cards && CardAccount.status.consent.cards.accepted === true
    readonly property bool linked: !!CardAccount.status.devices && (CardAccount.status.devices.devices || []).some(function(device) {
        return device.cardID === CardAccount.status.cardID && device.revokedAt === null;
    })
    spacing: LithicTokens.space.s4
    onLiveChanged: {
        CardAccount.active = root.live;
        if (root.live && !CardAccount.busy)
            CardAccount.refresh();
    }
    Component.onDestruction: CardAccount.active = false
    function clearAssent() { root.rights = false; root.policy = false; root.review = false; }
    function chooseDesign() { folder.open(); }
    function chooseHero() { image.open(); }
    Connections {
        target: CardAccount
        function onDraftChanged() { root.clearAssent(); }
    }
    FolderChooser {
        id: folder
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose the design folder to submit")
        onAccepted: {
            root.submissionKind = "design";
            root.submissionPath = folder.localPath(folder.selectedFolder);
            CardAccount.draft = null;
        }
    }
    ImageChooser {
        id: image
        inspectImages: true
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose an image or animation to submit")
        videos: false
        onAccepted: {
            root.submissionKind = "hero";
            root.submissionPath = image.localPath(image.selectedFile);
            CardAccount.draft = null;
        }
    }
    Label { width: parent.width; text: I18n.tr("Ryoku Account"); typeRole: "section" }
    Note { text: I18n.tr("Make and wear offline cards without signing in. Account links this device for online submissions; Ember and the separate $2/month Creator subscription share the same account. A verified active badge or Creator can qualify for reviewed animated sharing. Neither skips review.") }
    Label {
        width: parent.width
        text: ({ unread: I18n.tr("Not checked"), unconfigured: I18n.tr("Account server not configured"), signed_out: I18n.tr("Signed out"), pending: I18n.tr("Waiting for browser approval"), signed_in: I18n.tr("Signed in") })[CardAccount.status.state] || String(CardAccount.status.state || "")
        typeRole: "title"
        textFormat: Text.PlainText
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Note { visible: !!CardAccount.status.message; text: String(CardAccount.status.message || "") }
    TextField { width: parent.width; visible: CardAccount.signedIn; readOnly: true; text: String(CardAccount.status.accountID || "") + " · " + String(CardAccount.status.server || ""); Accessible.name: I18n.tr("Signed-in account and service") }
    Note { visible: CardAccount.pending; text: I18n.tr("In your browser, enter code %1. No policies are accepted automatically. This code expires at %2.").replace("%1", String(CardAccount.status.userCode || "")).replace("%2", new Date(Number(CardAccount.status.expiresAt || 0) * 1000).toLocaleTimeString()) }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Plaque { text: I18n.tr("Sign in to Ryoku"); kind: "gold"; visible: !CardAccount.signedIn && !CardAccount.pending; enabled: !CardAccount.busy; onClicked: CardAccount.signIn() }
        Plaque { text: I18n.tr("Cancel sign-in"); kind: "line"; visible: CardAccount.pending; enabled: !CardAccount.busy; onClicked: CardAccount.cancel() }
        Plaque { text: I18n.tr("Check sign-in"); kind: "line"; visible: CardAccount.pending; enabled: !CardAccount.busy; onClicked: CardAccount.poll() }
        Plaque { text: I18n.tr("Sign out"); kind: "line"; visible: CardAccount.signedIn; enabled: !CardAccount.busy; onClicked: CardAccount.signOut() }
        Plaque { text: I18n.tr("Refresh"); kind: "quiet"; enabled: !CardAccount.busy; onClicked: CardAccount.refresh() }
        Plaque { text: I18n.tr("Account, disclosures & Creator"); kind: "line"; enabled: !CardAccount.busy; onClicked: CardAccount.website() }
    }
    Note { visible: CardAccount.error !== ""; text: CardAccount.error; ink: Lithic.jasper }
    Note { visible: CardAccount.message !== ""; text: CardAccount.message }
    Note { visible: CardAccount.busy; text: I18n.tr("Working: %1").replace("%1", CardAccount.job) }
    Column {
        width: parent.width
        visible: CardAccount.signedIn
        Note { text: root.linked ? I18n.tr("This device is linked to this Account.") : I18n.tr("This device is not linked to this Account.") }
        spacing: LithicTokens.space.s3
        Label { text: I18n.tr("Optional online cards and device"); typeRole: "section" }
        Note { text: root.cardConsentReady ? I18n.tr("Current Account terms and separate online-card consent accepted on the website.") : I18n.tr("For online linking or submissions only, accept Account terms and the separate optional card review and local-key consent on the website. Ember and Creator purchases do not require card consent or create a key.") }
        Note { text: CardAccount.status.creator && CardAccount.status.creator.active ? I18n.tr("Creator active until %1. Ember credits are separate.").replace("%1", new Date(Number(CardAccount.status.creator.until)).toLocaleString()) : I18n.tr("Creator is not active. Offline animation remains available, and an independently verified badge can qualify for online review.") }
        TextField { width: parent.width; readOnly: true; text: String(CardAccount.status.cardID || ""); Accessible.name: I18n.tr("This device's public card ID") }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Plaque { text: I18n.tr("Link this device…"); kind: "line"; enabled: !CardAccount.busy && root.cardConsentReady; onClicked: { typedLink.text = ""; CardAccount.prepareLink(); } }
            Plaque { text: I18n.tr("Unlink this device…"); kind: "quiet"; enabled: !CardAccount.busy; onClicked: { root.unlinkAccount = String(CardAccount.status.accountID || ""); root.unlinkCard = String(CardAccount.status.cardID || ""); root.unlinkConfirmation = !root.unlinkConfirmation; } }
        }
        Column {
            width: parent.width
            visible: root.unlinkConfirmation
            spacing: LithicTokens.space.s2
            Note { text: I18n.tr("Unlink account %1, card %2? This revokes its device binding and new online actions, not offline art or friends' existing copies.").replace("%1", root.unlinkAccount).replace("%2", root.unlinkCard) }
            Plaque { text: I18n.tr("Confirm unlink"); kind: "line"; enabled: !CardAccount.busy; onClicked: { root.unlinkConfirmation = false; CardAccount.unlink(root.unlinkAccount, root.unlinkCard); } }
        }
        Column {
            width: parent.width
            visible: !!CardAccount.link
            spacing: LithicTokens.space.s2
            Note { text: CardAccount.link ? I18n.tr("Action: link device\nAccount: %1\nCard: %2\nService: %3\nExpires: %4").replace("%1", CardAccount.link.accountID).replace("%2", CardAccount.link.cardID).replace("%3", CardAccount.link.server).replace("%4", new Date(Number(CardAccount.link.expiresAt)).toLocaleString()) : "" }
            Note { text: CardAccount.link ? String(CardAccount.link.disclosure || "") : "" }
            TextField { id: typedLink; width: parent.width; placeholderText: CardAccount.link ? String(CardAccount.link.confirmation || "") : ""; Accessible.name: I18n.tr("Type the exact device-link confirmation") }
            Plaque { text: I18n.tr("Sign this link request"); kind: "gold"; enabled: !CardAccount.busy && !!CardAccount.link && typedLink.text === CardAccount.link.confirmation; onClicked: CardAccount.confirmLink(typedLink.text) }
        }
        CardSubmissionPanel { width: parent.width; hub: root.hub; accountPanel: root }
    }
    component Note: Label {
        textFormat: Text.PlainText
        width: root.width
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
