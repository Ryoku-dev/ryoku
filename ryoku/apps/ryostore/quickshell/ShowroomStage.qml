import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

Item {
    id: stage

    property var item: null
    property var previewItem: null
    property string busyKey: ""
    property string installStage: ""
    property string installErrorKey: ""
    property string installError: ""
    property string positionText: ""
    property bool offline: false
    property bool reducedMotion: false
    property var shownHero: ({})
    property var outgoingHero: ({})
    property real heroProgress: 1

    function safeAccent(value) {
        const raw = String(value || "").trim().toLowerCase();
        if (/^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/.test(raw))
            return Qt.color(raw);
        if (raw !== "") {
            var hash = 0;
            for (var i = 0; i < raw.length; i++)
                hash = ((hash << 5) - hash + raw.charCodeAt(i)) | 0;
            return Qt.hsla(Math.abs(hash % 360) / 360, 0.52, 0.42, 1);
        }
        return Tokens.sun;
    }

    signal installRequested(var item)
    signal detailsRequested(var item)
    signal settingsRequested(var item)
    signal removeRequested(var item)

    readonly property var displayItem: previewItem || item || ({})
    readonly property var actionItem: item || ({})
    readonly property int motionDuration: reducedMotion ? 0 : Tokens.swap
    readonly property string actionKey: StoreLogic.itemKey(actionItem)
    readonly property string primaryLabel: busyKey === actionKey && installStage !== ""
            ? installStage
            : StoreLogic.primaryAction(actionItem)
    readonly property string secondaryLabel: StoreLogic.secondaryAction(actionItem)
    readonly property bool hasActionItem: item !== null && item !== undefined
    readonly property color stageSurface: displayItem.surface || Tokens.paper
    readonly property color stageAccent: safeAccent(displayItem.accent)
    function heroItem(value) {
        const shown = value || {};
        return {
            id: shown.id,
            name: shown.name || shown.id,
            art: shown.art || "",
            artRaw: shown.artRaw || "",
            category: shown.category,
            categoryName: shown.categoryName,
            accent: shown.accent,
            surface: shown.surface,
            installed: actionItem.installed,
            active: actionItem.active,
            enabled: actionItem.enabled,
            installedCount: actionItem.installedCount,
            totalCount: actionItem.totalCount,
            updateAvailable: actionItem.updateAvailable
        };
    }

    clip: true

    function triggerInstall() {
        if (hasActionItem && StoreLogic.primaryAction(actionItem) !== "INSTALLED" && busyKey === ""
                && !StoreLogic.isDownloadPaused(actionItem) && !StoreLogic.isUnavailable(actionItem))
            installRequested(actionItem);
    }

    function triggerDetails() {
        if (hasActionItem)
            detailsRequested(actionItem);
    }

    function triggerSettings() {
        if (hasActionItem && secondaryLabel !== "")
            settingsRequested(actionItem);
    }

    function triggerRemove() {
        if (hasActionItem && busyKey === "" && StoreLogic.isInstalled(actionItem))
            removeRequested(actionItem);
    }

    function syncHero() {
        const next = heroItem(displayItem);
        const nextKey = StoreLogic.itemKey(next) + "|" + String(next.artRaw || next.art || "");
        const shownKey = StoreLogic.itemKey(shownHero)
                + "|" + String(shownHero.artRaw || shownHero.art || "");
        if (!shownHero || (!shownHero.id && !shownHero.name)) {
            shownHero = next;
            outgoingHero = next;
            heroProgress = 1;
            return;
        }
        if (nextKey === shownKey) {
            shownHero = next;
            return;
        }
        heroFade.stop();
        outgoingHero = shownHero;
        shownHero = next;
        if (reducedMotion) {
            heroProgress = 1;
            return;
        }
        heroProgress = 0;
        heroFade.restart();
    }

    onDisplayItemChanged: syncHero()
    onActionItemChanged: syncHero()
    onReducedMotionChanged: {
        if (reducedMotion) {
            heroFade.stop();
            heroProgress = 1;
        }
    }
    Component.onCompleted: syncHero()

    NumberAnimation {
        id: heroFade
        target: stage
        property: "heroProgress"
        from: 0
        to: 1
        duration: Tokens.swap
        easing.type: Tokens.ease
    }

    ProductCover {
        objectName: "ryostore-stage-artwork-outgoing"
        anchors.fill: parent
        item: stage.outgoingHero
        mode: "hero"
        active: false
        opacity: 1 - stage.heroProgress
        scale: 1.008 - stage.heroProgress * 0.008
        visible: opacity > 0
        reducedMotion: stage.reducedMotion
    }

    ProductCover {
        id: artwork
        objectName: "ryostore-stage-artwork"
        anchors.fill: parent
        item: stage.shownHero
        mode: "hero"
        active: stage.visible
        opacity: stage.heroProgress
        scale: 0.992 + stage.heroProgress * 0.008
        reducedMotion: stage.reducedMotion
    }

    Rectangle {
        objectName: "ryostore-stage-scrim"
        anchors.fill: parent
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop {
                position: 0
                color: Qt.rgba(stage.stageSurface.r, stage.stageSurface.g,
                               stage.stageSurface.b, 0.94)
            }
            GradientStop {
                position: 0.34
                color: Qt.rgba(stage.stageSurface.r, stage.stageSurface.g,
                               stage.stageSurface.b, 0.6)
            }
            GradientStop {
                position: 0.64
                color: Qt.rgba(stage.stageSurface.r, stage.stageSurface.g,
                               stage.stageSurface.b, 0.1)
            }
            GradientStop { position: 1; color: "#00000000" }
        }
    }

    // seat the stage in the surface at the bottom so the filmstrip reads as one
    // continuous shelf, not a seam
    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            GradientStop { position: 0; color: "#00000000" }
            GradientStop { position: 0.7; color: "#00000000" }
            GradientStop {
                position: 1
                color: Qt.rgba(stage.stageSurface.r, stage.stageSurface.g,
                               stage.stageSurface.b, 0.85)
            }
        }
    }

    // a low wash of the product's own accent under the story text, so each
    // hero carries a hint of the product's colour
    Rectangle {
        anchors.fill: parent
        visible: stage.hasActionItem
        opacity: 0.5 * stage.heroProgress
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop {
                position: 0
                color: Qt.rgba(stage.stageAccent.r, stage.stageAccent.g,
                               stage.stageAccent.b, 0.16)
            }
            GradientStop { position: 0.5; color: "#00000000" }
            GradientStop { position: 1; color: "#00000000" }
        }
        Behavior on opacity {
            enabled: !stage.reducedMotion
            NumberAnimation { duration: stage.motionDuration; easing.type: Tokens.ease }
        }
    }

    Text {
        objectName: "ryostore-stage-position"
        anchors { top: parent.top; right: parent.right; margins: Tokens.s5 }
        text: stage.positionText
        visible: text !== ""
        color: Tokens.ink
        font.family: Tokens.mono
        font.pixelSize: Tokens.fMicro
        font.letterSpacing: Tokens.trackLabel
    }

    Column {
        id: story
        anchors { left: parent.left; bottom: parent.bottom; margins: Tokens.s6 }
        width: Math.min(stage.width * 0.48, 520)
        spacing: Tokens.s3
        visible: stage.hasActionItem
        opacity: 0.45 + stage.heroProgress * 0.55

        Text {
            width: parent.width
            text: String(stage.displayItem && (stage.displayItem.categoryName || stage.displayItem.category) || "").toUpperCase()
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackMark
            elide: Text.ElideRight
        }

        Text {
            objectName: "ryostore-stage-title"
            width: parent.width
            text: String(stage.displayItem && (stage.displayItem.name || stage.displayItem.id) || "")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fHero
            font.weight: Font.Medium
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        Text {
            width: parent.width
            text: String(stage.displayItem && (stage.displayItem.summary || stage.displayItem.description) || "")
            visible: text !== ""
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.Wrap
            maximumLineCount: 4
            elide: Text.ElideRight
        }

        StatusReadout {
            objectName: "ryostore-stage-status"
            item: stage.actionItem
            busyKey: stage.busyKey
            installStage: stage.installStage
            installErrorKey: stage.installErrorKey
            installError: stage.installError
            offline: stage.offline
        }

        Text {
            objectName: "ryostore-stage-foreign-wm"
            width: parent.width
            visible: StoreLogic.isUnavailable(stage.displayItem)
            text: StoreLogic.unavailableReason(stage.displayItem).length > 0
                ? StoreLogic.unavailableReason(stage.displayItem)
                : StoreLogic.unavailableLabel(stage.displayItem)
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            textFormat: Text.PlainText
            maximumLineCount: 3
            elide: Text.ElideRight
        }

        Text {
            objectName: "ryostore-stage-pause"
            width: parent.width
            visible: StoreLogic.isDownloadPaused(stage.displayItem)
            text: I18n.tr("Under construction.") + " " + StoreLogic.downloadPauseReason(stage.displayItem)
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            textFormat: Text.PlainText
            maximumLineCount: 3
            elide: Text.ElideRight
        }

        Flow {
            width: parent.width
            spacing: Tokens.s2

            InstallAction {
                objectName: "ryostore-stage-primary"
                text: I18n.tr(stage.primaryLabel)
                primary: true
                busy: stage.busyKey === stage.actionKey
                installed: StoreLogic.primaryAction(stage.actionItem) === "INSTALLED"
                reducedMotion: stage.reducedMotion
                armed: stage.hasActionItem
                        && !installed
                        && !StoreLogic.isDownloadPaused(stage.actionItem)
                        && !StoreLogic.isUnavailable(stage.actionItem)
                onAct: stage.triggerInstall()
            }

            Btn {
                objectName: "ryostore-stage-details"
                text: I18n.tr("VIEW DETAILS")
                armed: stage.hasActionItem
                Accessible.role: Accessible.Button
                Accessible.name: text
                onAct: stage.triggerDetails()
                Accessible.onPressAction: stage.triggerDetails()
            }

            Btn {
                objectName: "ryostore-stage-settings"
                text: I18n.tr(stage.secondaryLabel)
                visible: text !== ""
                armed: visible && stage.hasActionItem
                Accessible.role: Accessible.Button
                Accessible.name: text
                onAct: stage.triggerSettings()
                Accessible.onPressAction: stage.triggerSettings()
            }

            Btn {
                objectName: "ryostore-stage-remove"
                text: I18n.tr("REMOVE")
                visible: StoreLogic.isInstalled(stage.actionItem)
                armed: visible && stage.busyKey === ""
                Accessible.role: Accessible.Button
                Accessible.name: text
                onAct: stage.triggerRemove()
                Accessible.onPressAction: stage.triggerRemove()
            }
        }
    }
}
