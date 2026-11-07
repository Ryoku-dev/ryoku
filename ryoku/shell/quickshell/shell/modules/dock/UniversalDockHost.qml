import QtQuick
import shell.services
import "../stage/Singletons" as StageCfg
import stage as Stage
Item {
    id: root

    required property var screen
    property bool surfaceVisible: true
    readonly property string design: Dock.design
    readonly property var contribution: {
        const choices = Shells.contributions("dock");
        return choices.find(choice => choice.id === root.design) ?? null;
    }
    readonly property bool contributionSelected: root.contribution !== null
    readonly property bool contributionPending: root.contributionSelected
        && (root.contribution.prepared !== true || String(root.contribution.entryUrl ?? "") === "")
    readonly property string contributionUrl: root.contributionSelected
        && !root.contributionPending ? String(root.contribution.entryUrl) : ""
    readonly property string storeUrl: !root.contributionSelected && Dock.storeDesign
        ? DockProducts.urlFor(root.design) : ""
    readonly property string selectedUrl: root.contributionUrl !== ""
        ? root.contributionUrl : root.storeUrl
    property string failedUrl: ""
    readonly property bool externalRuns: root.selectedUrl !== ""
        && root.selectedUrl !== root.failedUrl
    readonly property var item: designLoader.item?.surface ?? null
    function requestContribution(): void {
        if (root.contributionPending)
            Shells.requestSync();
    }
    onContributionChanged: root.requestContribution()
    onSelectedUrlChanged: root.failedUrl = ""
    Component.onCompleted: root.requestContribution()
    // The Stage Editor frames this monitor: every design stays mounted,
    // revealed and drawn inside the editor's card, and publishes the edge it
    // occupies for layouts that keep clear of it (desktop icons). The dock
    // catalogue edits the live surface, so hiding it while that page is open
    // edits an invisible thing.
    readonly property bool stageEditing: StageCfg.StageSession.onMonitor(
        root.screen ? root.screen.name : "")
        || (Stage.GlobalStates.editModeMonitor === (root.screen?.name ?? "")
            && Stage.GlobalStates.editProgress > 0)

    width: 0
    height: 0

    Loader {
        id: designLoader
        active: root.surfaceVisible && Dock.cfg("enabled", false)
            && root.design !== "none" && root.design !== "ii"
            && (root.contributionSelected || !Dock.storeDesign || DockProducts.ready)
        asynchronous: true
        sourceComponent: root.externalRuns ? externalDesign : ryokuDesign
    }

    Component {
        id: externalDesign
        Loader {
            id: externalLoader
            readonly property string url: root.selectedUrl
            readonly property var surface: item?.surface ?? null
            asynchronous: true
            onUrlChanged: externalLoader.load()
            Component.onCompleted: externalLoader.load()
            function load(): void {
                externalLoader.setSource(externalLoader.url, {
                    screen: root.screen,
                    surfaceVisible: root.surfaceVisible,
                    stageEditing: root.stageEditing
                });
            }
            onStatusChanged: {
                if (externalLoader.status !== Loader.Error)
                    return;
                console.warn("dock: " + root.design + " did not load; drawing the Ryoku dock");
                root.failedUrl = externalLoader.url;
            }
            Binding {
                when: externalLoader.item !== null
                target: externalLoader.item
                property: "surfaceVisible"
                value: root.surfaceVisible
            }
            Binding {
                when: externalLoader.item !== null
                target: externalLoader.item
                property: "stageEditing"
                value: root.stageEditing
            }
        }
    }

    Component {
        id: ryokuDesign
        Item {
            property alias surface: dockSurface
            width: 0
            height: 0
            DockSurface {
                id: dockSurface
                screen: root.screen
                stageEditing: root.stageEditing
                visible: root.surfaceVisible && Dock.cfg("enabled", false)
            }
        }
    }

}
