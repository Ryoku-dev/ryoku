pragma Singleton
import QtQuick

// Inspector-only data seam: the pinned design renderer disables ambient hero
// motion. Enable it only for the sanitized hero being reviewed in this worker.
QtObject {
    readonly property bool on: RenderData.hero.kind === "custom"
}
