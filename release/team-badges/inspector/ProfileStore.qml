pragma Singleton
import QtQuick

// Inspector-only data seam. The card and all visual components remain the real
// pinned Ryoku bundle. No submitted expression is ever evaluated as QML/JS.
QtObject {
    readonly property int pictureRevision: 0
    function get(path, fallback) {
        if (path === "text.name") return RenderData.name;
        if (path === "style") return "classic";
        if (path === "hero") return RenderData.hero;
        return fallback;
    }
}
