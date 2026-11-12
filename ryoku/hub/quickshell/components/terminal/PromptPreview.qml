import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a sample of the chosen prompt on a terminal plate
CutRect {
    id: preview

    property string layout: "pill"
    readonly property color ink: Lithic.dark ? Lithic.fg : Lithic.ground

    function sample(id) {
        if (id === "minimal")
            return "󰉋 ~/ryoku  󰊢 unstable  λ";
        if (id === "two-line")
            return "╭─ 󰉋 ~/ryoku  󰊢 unstable\n╰─ λ";
        if (id === "powerline")
            return "󰉋 ~/ryoku󰊢 unstable 󰔚 1s  λ";
        if (id === "lean")
            return "󰉋  ~/ryoku  󰊢 unstable\n❯";
        return " 󰉋 ~/ryoku    󰊢 unstable \nλ";
    }

    implicitHeight: sampleText.implicitHeight + LithicTokens.space.s4 * 2
    fill: Lithic.dark ? Lithic.field : Lithic.fg
    cut: LithicTokens.cut.small

    Text {
        id: sampleText
        x: LithicTokens.space.s4
        y: LithicTokens.space.s4
        width: parent.width - LithicTokens.space.s4 * 2
        text: preview.sample(preview.layout)
        color: preview.ink
        font.family: Lithic.font.mono
        font.pixelSize: 13
        lineHeight: 1.3
        textFormat: Text.PlainText
        wrapMode: Text.NoWrap
        elide: Text.ElideRight
        Accessible.name: I18n.tr("Prompt sample")
    }
}
