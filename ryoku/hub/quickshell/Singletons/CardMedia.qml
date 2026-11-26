pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

QtObject {
    id: root
    readonly property var hero: ProfileStore.get("hero", ({})) || ({})
    readonly property int revision: ProfileStore.pictureRevision
    readonly property string requestedSource: hero.kind === "custom" ? String(hero.source || "") : ""
    property var checked: ({})
    property string error: ""
    property bool busy: false
    property bool queued: false
    property int generation: 0
    readonly property bool current: requestedSource !== "" && checked.source === requestedSource
    readonly property string file: current ? String(checked.file || "") : ""
    readonly property string poster: current ? String(checked.poster || "") : ""
    readonly property string cutout: current ? String(checked.cutout || "") : ""
    readonly property string digest: current ? String(checked.digest || "") : ""

    onRequestedSourceChanged: root.refresh()
    onRevisionChanged: root.refresh()
    Component.onCompleted: root.refresh()

    function refresh(): void {
        root.generation++;
        root.checked = ({});
        root.error = "";
        if (root.busy) {
            root.queued = true;
            return;
        }
        if (root.requestedSource === "")
            return;
        root.busy = true;
        root.queued = false;
        var generation = root.generation;
        Changes.run(["ryoku-hub", "media", "current"], function(ok, out, err) {
            root.busy = false;
            if (generation === root.generation) {
                var answer = null;
                if (ok) {
                    try { answer = JSON.parse(out); }
                    catch (e) { err = I18n.tr("The media inspector returned an unreadable answer."); }
                }
                if (answer && answer.source === root.requestedSource && /^[0-9a-f]{64}$/.test(String(answer.digest || "")))
                    root.checked = answer;
                else
                    root.error = String(err || I18n.tr("Your card picture did not pass media inspection.")).trim();
            }
            if (root.queued)
                root.refresh();
        }, { timeout: 35000 });
    }
}
