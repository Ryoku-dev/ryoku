//@ pragma Env QML_XHR_ALLOW_FILE_READ=1
//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
//@ pragma DefaultEnv QS_DROP_EXPENSIVE_FONTS=1
//@ pragma DefaultEnv QT_QUICK_FLICKABLE_WHEEL_DECELERATION=10000
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import "../../../quickshell/components" as Hub

// Run only in a private nested compositor and isolated XDG directories. The
// selected fixture is passed to the real ryoku-hub process on PATH; no decoder,
// media result, or UI command is mocked here.
FloatingWindow {
    id: root
    visible: true
    width: 1100
    height: 820
    readonly property string fixture: Quickshell.env("RYOKU_MEDIA_FIXTURE") || ""
    readonly property bool expectFailure: Quickshell.env("RYOKU_MEDIA_EXPECT_FAILURE") === "1"
    readonly property int expectedFrames: Number(Quickshell.env("RYOKU_MEDIA_EXPECT_FRAMES") || "1")
    property int ticks: 0

    function originalReachedQt(item): bool {
        if (item.source !== undefined && String(item.source) === chooser.urlOf(root.fixture))
            return true;
        var children = item.children || [];
        for (var i = 0; i < children.length; i++)
            if (root.originalReachedQt(children[i]))
                return true;
        return false;
    }
    function finish(ok, detail): void {
        console.log("CARD_MEDIA_CHOOSER " + JSON.stringify({ ok: ok, detail: detail, originalReachedQt: root.originalReachedQt(root.contentItem) }));
        Qt.quit();
    }
    Hub.ImageChooser {
        id: chooser
        inspectImages: true
        overlayParent: root.contentItem
    }
    Timer {
        interval: 100
        repeat: true
        running: true
        onTriggered: {
            root.ticks++;
            if (root.ticks === 1) {
                if (root.fixture.charAt(0) !== "/") {
                    root.finish(false, "absolute RYOKU_MEDIA_FIXTURE required");
                    return;
                }
                chooser.currentFolder = chooser.urlOf(chooser.parentOf(root.fixture));
                chooser.open();
                chooser.chosen = root.fixture;
                if (chooser.previewURL(root.fixture) !== "")
                    root.finish(false, "original URL available before inspection");
            }
            if (root.originalReachedQt(root.contentItem)) {
                root.finish(false, "foreign bytes reached an Image source");
                return;
            }
            var result = chooser.inspectedPreviews[root.fixture];
            if (result) {
                var ok = root.expectFailure ? !!result.error && chooser.previewURL(root.fixture) === ""
                    : !result.error && result.frames === root.expectedFrames && chooser.previewURL(root.fixture) !== "";
                root.finish(ok, result);
            } else if (root.ticks > 350) {
                root.finish(false, "real media preview timed out");
            }
        }
    }
}
