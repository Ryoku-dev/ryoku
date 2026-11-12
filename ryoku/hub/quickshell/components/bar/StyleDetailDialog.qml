import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the bar style's detail, opened by clicking its card: the preview large, the
// description, what it costs, and the action. Escape or clicking the paper
// closes it and returns the keys to the card.
Dialog {
    id: root

    required property var card
    required property var styles
    property Item opener: null

    title: card ? card.name : ""
    dialogWidth: LithicTokens.size.settingsExpandedMax
    actions: root.action ? [{ text: root.actionText, kind: root.actionKind }] : []

    readonly property string action: card ? styles.actionOf(card) : ""
    readonly property string actionText: styles.actionText(action)
    readonly property string actionKind: action === "install" || action === "update" ? "line" : "tonal"

    signal act(string id, string action)

    Column {
        width: parent.width

        Item {
            width: parent.width
            height: Math.min(width * 0.56, 420)

            Rectangle {
                anchors.fill: parent
                visible: !root.card || !root.card.art
                color: Lithic.field

                Hatch {
                    anchors.fill: parent
                }
            }

            Image {
                anchors.fill: parent
                visible: root.card && root.card.art
                source: root.card ? root.card.art : ""
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
            }
        }

        Item { width: 1; height: LithicTokens.space.s4 }

        Label {
            width: parent.width
            text: root.describe()
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Item { width: 1; height: root.facts.length > 0 ? LithicTokens.space.s4 : 0 }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2
            visible: root.facts.length > 0

            Repeater {
                model: root.facts

                Row {
                    required property var modelData
                    width: parent.width
                    spacing: LithicTokens.space.s3

                    Label {
                        width: 140
                        text: modelData.key
                        typeRole: "caption"
                        inkRole: "fg3"
                    }

                    Label {
                        width: parent.width - 140 - LithicTokens.space.s3
                        text: modelData.value
                        typeRole: "body"
                        inkRole: "fg"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }
        }
    }

    onActionTriggered: root.act(root.card.id, root.action)

    function describe() {
        if (!card)
            return "";
        var parts = [];
        if (card.summary)
            parts.push(card.summary);
        if (card.category === "subshells")
            parts.push(I18n.tr("A complete desktop: its own bar, dock and panels"));
        return parts.join(" ");
    }

    readonly property var facts: {
        if (!card)
            return [];
        var out = [];
        var blocked = styles.blockedReason(card);
        if (card.unavailable && card.unavailableReason)
            out.push({ key: I18n.tr("Status"), value: I18n.tr(card.unavailableReason) });
        else if (card.downloadPaused && card.downloadPauseReason)
            out.push({ key: I18n.tr("Status"), value: I18n.tr(card.downloadPauseReason) });
        else if (blocked !== "")
            out.push({ key: I18n.tr("Status"), value: blocked });
        if (styles.updatable(card))
            out.push({ key: I18n.tr("Version"), value: I18n.tr("%1 installed, %2 ready").arg(card.installedVersion).arg(card.version) });
        else if (card.installed && card.source === "shipped")
            out.push({ key: I18n.tr("Version"), value: I18n.tr("Comes with Ryoku") });
        else if (card.installed && card.installedVersion)
            out.push({ key: I18n.tr("Version"), value: I18n.tr("%1 from RyoStore").arg(card.installedVersion) });
        else if (card.version)
            out.push({ key: I18n.tr("Version"), value: I18n.tr("%1 in RyoStore").arg(card.version) });
        if (card.resourceTier)
            out.push({ key: I18n.tr("Cost"), value: card.resourceTier === "light" ? I18n.tr("Light") : card.resourceTier === "heavy" ? I18n.tr("Heavy") : I18n.tr("Standard") });
        return out;
    }
}
