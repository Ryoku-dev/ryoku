pragma Singleton
import QtQuick

// Bundled so the chrome renders the same whether or not the host has these faces.
QtObject {
    id: fonts

    readonly property FontLoader uiLoader: FontLoader {
        source: Qt.resolvedUrl("fonts/RobotoCondensed-Bold-UI.ttf")
    }
    readonly property FontLoader iconLoader: FontLoader {
        source: Qt.resolvedUrl("fonts/SymbolsNerdFont-Regular.ttf")
    }

    // The bundled face is a Bold cut, so text sets weight Bold.
    readonly property string ui: uiLoader.status === FontLoader.Ready ? uiLoader.name : "Roboto Condensed"
    readonly property int uiWeight: Font.Bold

    readonly property string icon: iconLoader.status === FontLoader.Ready ? iconLoader.name : "Symbols Nerd Font"

    // Ryoku's brand stack (docs/ui-ux.md), installed system-wide by the desktop packages:
    // Fraunces for numerals and titles, Geologica for words, Noto CJK for kanji seals.
    readonly property string display: "Fraunces"
    readonly property string sans: "Geologica"
    readonly property string jp: "Noto Sans CJK JP"
}
