import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../shared/Singletons"
import "../../shared/lib/results.js" as Results
import "../../shared/lib/lifecycle.js" as Lifecycle
import "../../shared" as Shared
import "." as HeroVariant

Item {
    id: root

    property real s: 1
    property bool open: false
    property bool animationsEnabled: true
    property bool snapping: false
    property bool suppressResultDeckAnimation: false
    property bool frostActive: false
    property real maxHeight: 640
    property var results: []
    property var selectedResult: null
    property string selectedResultKey: ""
    property bool shelfOpen: false
    property var secondaryActions: []
    property var packedLayout: []
    property string focusedActionId: ""
    property bool helpMode: false
    property bool askMode: false
    property bool answerMode: false
    property bool actionBrowse: false
    property bool searching: false
    property string emptyText: I18n.tr("NO MATCHES")
    property string errorText: ""
    property int windowCount: 0
    property bool windowFocusActive: false
    property string askQuestion: ""
    property var answer: ({ available: false })

    signal leadActivated()
    signal resultSelected(string resultKey, int rank)
    signal actionFocusRequested(string actionId)
    signal actionExecuteRequested(string actionId)
    signal askFinished()

    readonly property real pad: 10 * s
    readonly property bool hasResults: selectedResult !== null && results.length > 0
    readonly property real progressHeight: searching && hasResults ? 18 * s : 0
    readonly property real leadHeight: Results.LAYOUT.leadHeight * s
    readonly property real prefixHeight: actionBrowse || answerMode
        ? prefixStack.implicitHeight + 8 * s : 0
    readonly property real resultBodyHeight: hasResults
        ? leadHeight + actionShelf.targetHeight + ledger.desiredHeight
        : leadHeight + ledger.desiredHeight
    readonly property real standardDesiredHeight: pad * 2 + prefixHeight + resultBodyHeight
    readonly property real rawDesiredHeight: helpMode
        ? pad * 2 + helpPanel.implicitHeight
        : (askMode ? pad * 2 + askPanel.implicitHeight : standardDesiredHeight)
    readonly property real desiredHeight: open
        ? Math.min(Math.max(0, maxHeight), rawDesiredHeight) : 0
    readonly property string activeCategory: tabs.activeCategory
    readonly property bool askBusy: askPanel.busy
    readonly property bool askResumeMode: askPanel.resumeMode
    readonly property bool askAnswerCurrent: askPanel.answerCurrent
    readonly property bool askPermissionPending: askPanel.permPending
    readonly property int askChipCount: askPanel.chips.length
    readonly property string actionVisibleRange: actionShelf.visibleRange
    readonly property real actionShelfHeight: actionShelf.height
    readonly property real resultDeckOpacity: resultDeck.opacity

    implicitHeight: desiredHeight
    height: desiredHeight
    opacity: open ? 1 : 0
    visible: height > 0.1 || opacity > 0.01
    clip: true

    Behavior on height {
        enabled: root.animationsEnabled && !root.snapping
        NumberAnimation {
            id: heightMotion
            duration: Motion.drawer
            easing.type: Easing.OutCubic
        }
    }
    Behavior on opacity {
        enabled: root.animationsEnabled && !root.snapping
        NumberAnimation {
            id: opacityMotion
            duration: Motion.drawer
            easing.type: Easing.OutCubic
        }
    }
    function snapAnimations() {
        snapping = true;
        height = Qt.binding(function () { return root.desiredHeight; });
        opacity = Qt.binding(function () { return root.open ? 1 : 0; });
        actionShelf.snapAnimations();
        snapping = false;
    }

    function hideResultDeckImmediately() {
        resultDeckFade.stop();
        suppressResultDeckAnimation = true;
        resultDeck.opacity = 0;
    }

    onAnimationsEnabledChanged: {
        if (!animationsEnabled)
            snapAnimations();
    }

    function resetCategory() {
        tabs.activeIndex = 0;
    }

    function cycleCategory(delta) {
        tabs.cycle(delta);
    }

    function resetAsk() {
        askPanel.reset();
    }

    function runAsk() {
        askPanel.run();
    }

    function moveAsk(delta) {
        askPanel.move(delta);
    }

    function activateAsk() {
        askPanel.activate();
    }

    function cancelAsk() {
        askPanel.cancel();
    }

    function revealRank(rank) {
        ledger.revealRank(rank);
    }

    Rectangle {
        anchors.fill: parent
        color: root.frostActive
            ? Qt.rgba(Theme.drawer.r, Theme.drawer.g, Theme.drawer.b,
                Lifecycle.frostBackdropOpacity(root.frostActive))
            : Theme.drawer
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 1
        color: Theme.frame
        z: 20
    }

    Item {
        id: content
        anchors.fill: parent
        anchors.margins: root.pad

        Column {
            id: prefixStack
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            spacing: 8 * root.s

            Shared.CategoryTabs {
                id: tabs
                width: prefixStack.width
                height: visible ? implicitHeight : 0
                visible: root.actionBrowse && !root.helpMode && !root.askMode
                s: root.s
            }

            Shared.AnswerPanel {
                width: prefixStack.width
                height: visible ? implicitHeight : 0
                visible: root.answerMode && !root.helpMode && !root.askMode
                s: root.s
                answer: root.answer
            }
        }

        Flickable {
            id: helpViewport
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            visible: root.helpMode
            clip: true
            contentWidth: width
            contentHeight: helpPanel.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height

            Shared.HelpPanel {
                id: helpPanel
                width: helpViewport.width
                s: root.s
            }
        }

        Flickable {
            id: askViewport
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            visible: root.askMode
            clip: true
            contentWidth: width
            contentHeight: askPanel.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height

            Shared.AskPanel {
                id: askPanel
                width: askViewport.width
                s: root.s
                question: root.askQuestion
                onFinished: root.askFinished()
            }
        }

        Item {
            id: standardBody
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: prefixStack.bottom
            anchors.topMargin: prefixStack.height > 0 ? 8 * root.s : 0
            anchors.bottom: parent.bottom
            visible: !root.helpMode && !root.askMode

            Item {
                id: resultDeck
                anchors.fill: parent
                opacity: 0
                visible: root.hasResults || opacity > 0.001
                layer.enabled: opacity > 0.001 && opacity < 0.999
                layer.smooth: true

                Behavior on opacity {
                    enabled: root.animationsEnabled && !root.snapping
                        && !root.suppressResultDeckAnimation
                    // NumberAnimation, not OpacityAnimator: the deck's layer
                    // toggles inside the fade, and a render-thread animator
                    // racing that toggle faults the scene graph (and the
                    // warning it logs names this very node). Every other fade
                    // in the launcher animates opacity on the main thread.
                    NumberAnimation {
                        id: resultDeckFade
                        duration: 120
                        easing.type: Easing.OutCubic
                    }
                }

                Connections {
                    target: root
                    function onHasResultsChanged() {
                        resultDeck.opacity = root.hasResults ? 1 : 0;
                    }
                }

                Item {
                    id: progress
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: root.progressHeight
                    z: 10
                    visible: height > 0

                    Row {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 5 * root.s

                        Shared.Spinner {
                            anchors.verticalCenter: parent.verticalCenter
                            size: 10 * root.s
                            thickness: Math.max(1, root.s)
                            color: Theme.providerOther
                        }

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("UPDATING")
                            color: Theme.faint
                            font.family: Theme.mono
                            font.pixelSize: 7.5 * root.s
                            font.weight: Font.DemiBold
                            font.letterSpacing: 0.8 * root.s
                        }
                    }
                }

                HeroVariant.LeadResult {
                    id: lead
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: root.hasResults ? root.leadHeight : 0
                    visible: root.hasResults
                    s: root.s
                    entry: root.selectedResult
                    errorText: root.errorText
                    windowCount: root.windowCount
                    windowFocusActive: root.windowFocusActive
                    onActivated: root.leadActivated()
                }

                HeroVariant.ActionShelf {
                    id: actionShelf
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: lead.bottom
                    s: root.s
                    open: root.shelfOpen && root.hasResults
                    animationsEnabled: root.animationsEnabled
                    actions: root.secondaryActions
                    packedLayout: root.packedLayout
                    focusedActionId: root.focusedActionId
                    onFocusRequested: actionId => root.actionFocusRequested(actionId)
                    onExecuteRequested: actionId => root.actionExecuteRequested(actionId)
                }

                HeroVariant.ResultLedger {
                    id: ledger
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: actionShelf.bottom
                    height: root.hasResults
                        ? Math.max(0, Math.min(desiredHeight,
                            standardBody.height - progress.height
                                - lead.height - actionShelf.height))
                        : 0
                    visible: root.hasResults && height > 0
                    s: root.s
                    results: root.results
                    selectedResultKey: root.selectedResultKey
                    onResultSelected: (resultKey, rank) => root.resultSelected(resultKey, rank)
                }
            }

            Row {
                anchors.centerIn: parent
                spacing: 8 * root.s
                visible: !root.hasResults

                Shared.Spinner {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.searching
                    size: 14 * root.s
                    color: Theme.providerOther
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.searching ? I18n.tr("SEARCHING") : root.emptyText
                    color: Theme.faint
                    font.family: Theme.mono
                    font.pixelSize: 9 * root.s
                    font.weight: Font.DemiBold
                    font.letterSpacing: 1.1 * root.s
                }
            }
        }
    }
}
