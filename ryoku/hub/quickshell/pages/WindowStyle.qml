import QtQuick
import "../components"
import "../components/previews"
import "../schema/window-style.js" as Schema

SettingsPage {
    pageId: "window-style"
    schema: Schema
    headerPreview: Component { WindowStylePreview {} }
}
