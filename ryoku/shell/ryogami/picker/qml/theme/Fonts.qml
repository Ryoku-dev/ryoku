pragma Singleton
import QtQuick
import Lithic

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

    readonly property string display: Lithic.font.sans
    readonly property string sans: Lithic.font.sans
    readonly property string mono: Lithic.font.mono
    readonly property string jp: "Noto Sans CJK JP"
}
