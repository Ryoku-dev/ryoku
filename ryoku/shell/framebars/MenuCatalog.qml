pragma Singleton
import QtQuick
import "MenuCatalog.js" as Lib

// The fixed catalog of frame menus, surfaces, quick actions and menu widgets.
// Wrapped as an installed-module singleton so any config root reaches it by
// module import.
QtObject {
    function anchors() { return Lib.anchors(); }
    function widgetIds() { return Lib.widgetIds(); }
    function widget(id) { return Lib.widget(id); }
    function menu(id) { return Lib.menu(id); }
    function surface(id) { return Lib.surface(id); }
    function quickAction(id) { return Lib.quickAction(id); }
    function quickActionIds() { return Lib.quickActionIds(); }
}
