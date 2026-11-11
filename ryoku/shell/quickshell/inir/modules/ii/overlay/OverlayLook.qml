pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import inir.modules.iris.style

Singleton {
    id: root
    readonly property bool iris: true

    readonly property color colOnLayer0: IrisStyle.text
    readonly property color colOnLayer1: IrisStyle.text
    readonly property color colOnLayer2: IrisStyle.text
    readonly property color colOnLayer3: IrisStyle.text
    readonly property color colOnSurface: IrisStyle.text
    readonly property color colSubtext: IrisStyle.textSecondary
    readonly property color colLayer0: IrisStyle.bodySurface
    readonly property color colSurfaceContainer: IrisStyle.bodySurface
    readonly property color colLayer1Hover: IrisStyle.fillHover
    readonly property color colLayer2: IrisStyle.fill
    readonly property color colLayer2Hover: IrisStyle.fillHover
    readonly property color colLayer2Active: IrisStyle.fillActive
    readonly property color colLayer3: IrisStyle.fill
    readonly property color colLayer3Hover: IrisStyle.fillHover
    readonly property color colLayer3Active: IrisStyle.fillActive
    readonly property color colPrimary: IrisStyle.accent
    readonly property color colPrimaryContainer: IrisStyle.tintFill(IrisStyle.accent)
    readonly property color colPrimaryContainerHover: IrisStyle.tintFillHover(IrisStyle.accent)
    readonly property color colPrimaryContainerActive: IrisStyle.tintFillHover(IrisStyle.accent)
    readonly property color colOnPrimaryContainer: IrisStyle.accent
    readonly property color colSecondaryContainer: IrisStyle.fill
    readonly property color colRecessed: IrisStyle.fillQuiet
    readonly property color colError: IrisStyle.danger
    readonly property color colErrorActive: IrisStyle.tintFillHover(IrisStyle.danger)
    readonly property color colErrorContainer: IrisStyle.tintFill(IrisStyle.danger)
    readonly property color colOnErrorContainer: IrisStyle.danger
    readonly property real roundingSmall: IrisStyle.radiusRow
    readonly property real roundingNormal: IrisStyle.radiusTile
    readonly property string fontNumbers: IrisStyle.fontNumbers
    readonly property var titles: ({
        crosshair: "Crosshair", fpsLimiter: "FPS limiter", floatingImage: "Floating image",
        resources: "Resources", gamePerformance: "Game performance", notes: "Notes",
        discord: "Discord", volumeMixer: "Volume mixer", notifications: "Notifications"
    })
}
