import QtQuick
import "../Singletons"
import "../components"
import "../schema/game-bar.js" as Schema

// the game bar overlay: its look, widget snapping and settle motion, and the Discord voice widget
SettingsPage {
    pageId: "game-bar"
    schema: Schema
    adaptRows: rows => Schema.adapt(rows, bind => Changes.value(bind))
}
