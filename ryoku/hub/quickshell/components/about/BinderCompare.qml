import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../registry/rarity.js" as Rarity

Column {
    id: root
    required property var entry
    spacing: LithicTokens.space.s3
    readonly property var mine: ({ threads: CardCode.threads, ramGb: CardCode.ramGb, refresh: CardCode.refreshRate,
        width: CardCode.displayWidth, gpuClass: CardCode.graphicsClass })
    readonly property var ownRarity: Rarity.of(root.mine)
    readonly property var theirRarity: Rarity.of(root.entry.hw)
    readonly property var rows: [
        { label: "Threads", own: root.mine.threads, theirs: root.entry.hw.threads, unit: "" },
        { label: "Memory", own: root.mine.ramGb, theirs: root.entry.hw.ramGb, unit: " GB" },
        { label: "Refresh rate", own: root.mine.refresh, theirs: root.entry.hw.refresh, unit: " Hz" },
        { label: "Display width", own: root.mine.width, theirs: root.entry.hw.width, unit: " px" },
        { label: "Graphics class", own: root.mine.gpuClass, theirs: root.entry.hw.gpuClass, unit: " / 5" },
        { label: "Rarity points", own: root.ownRarity.points, theirs: root.theirRarity.points, unit: "" },
        { label: "Power", own: Rarity.power(root.mine.threads, root.mine.ramGb, root.mine.gpuClass > 0), theirs: Rarity.power(root.entry.hw.threads, root.entry.hw.ramGb, root.entry.hw.gpuClass > 0), unit: "" }
    ]
    Component.onCompleted: { if (!SysInfo.loaded && !SysInfo.loading) SysInfo.load(); }
    Label { text: I18n.tr("Compare"); typeRole: "title" }
    Row {
        width: parent.width
        Label { width: parent.width * 0.46; text: I18n.tr("Machine"); typeRole: "caption"; inkRole: "fg3" }
        Label { width: parent.width * 0.27; text: I18n.tr("Yours"); typeRole: "caption"; horizontalAlignment: Text.AlignRight; inkRole: "fg2" }
        Label { width: parent.width * 0.27; text: root.entry.name; typeRole: "caption"; horizontalAlignment: Text.AlignRight; elide: Text.ElideRight; inkRole: "fg2" }
    }
    Column {
        width: parent.width
        Repeater {
            model: root.rows
            delegate: Item {
                required property var modelData
                width: root.width
                height: 36
                Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Lithic.alpha(Lithic.fg, 0.12) }
                Row {
                    anchors.fill: parent
                    Label { width: parent.width * 0.46; anchors.verticalCenter: parent.verticalCenter; text: I18n.tr(modelData.label); typeRole: "detail"; inkRole: "fg2" }
                    Label {
                        readonly property bool wins: SysInfo.loaded && modelData.own > modelData.theirs
                        width: parent.width * 0.27
                        anchors.verticalCenter: parent.verticalCenter
                        text: SysInfo.loaded ? (wins ? "▲ " : "") + modelData.own + modelData.unit : "…"
                        typeRole: "mono"
                        font.weight: wins ? Font.Bold : Font.Normal
                        horizontalAlignment: Text.AlignRight
                        ink: wins ? Lithic.gold : Lithic.fg2
                    }
                    Label {
                        readonly property bool wins: SysInfo.loaded && modelData.theirs > modelData.own
                        width: parent.width * 0.27
                        anchors.verticalCenter: parent.verticalCenter
                        text: (wins ? "▲ " : "") + modelData.theirs + modelData.unit
                        typeRole: "mono"
                        font.weight: wins ? Font.Bold : Font.Normal
                        horizontalAlignment: Text.AlignRight
                        ink: wins ? Lithic.gold : Lithic.fg2
                    }
                }
            }
        }
    }
    Label {
        width: parent.width
        text: !SysInfo.loaded ? I18n.tr("Reading your machine…")
            : root.ownRarity.points === root.theirRarity.points ? I18n.tr("Your cards share the same rarity score.")
            : root.ownRarity.points > root.theirRarity.points ? I18n.tr("Your machine earns the higher rarity score.")
            : I18n.tr("Their machine earns the higher rarity score.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
