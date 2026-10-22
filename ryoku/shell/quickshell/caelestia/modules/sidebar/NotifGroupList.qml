pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Components
import Caelestia.Config
import caelestia.components
import caelestia.services

LazyListView {
    id: root

    required property Props props
    required property list<var> notifs
    required property bool expanded
    required property Flickable container
    required property DrawerVisibilities visibilities

    signal requestToggleExpand(expand: bool)

    anchors.left: parent.left
    anchors.right: parent.right
    implicitHeight: contentHeight

    spacing: Math.round(Tokens.spacing.extraSmall)
    asynchronous: true

    readyDelay: 1
    cacheBuffer: 400
    removeDuration: Tokens.anim.durations.normal

    useCustomViewport: true
    viewport: {
        tWatcher.transform;
        return Qt.rect(0, container.contentY - mapToItem(container.contentItem, 0, 0).y, width, container.height);
    }

    model: ScriptModel {
        values: {
            if (root.expanded)
                return root.notifs;

            let count = 0;
            let i = 0;
            const previewNum = root.Config.notifs.groupPreviewNum;
            while (i < root.notifs.length && count < previewNum) {
                if (!(root.notifs[i]?.closed ?? true))
                    count++;
                i++;
            }

            return root.notifs.slice(0, i);
        }
    }

    delegate: Component {
        NotifSwipeDelegate {
            id: notif

            required property int index
            required property NotifData modelData

            Component.onCompleted: modelData?.lock(this)
            Component.onDestruction: modelData?.unlock(this)

            LazyListView.preferredHeight: modelData?.closed || LazyListView.removing ? 0 : notifInner.nonAnimHeight
            LazyListView.visibleHeight: modelData?.closed || LazyListView.removing ? 0 : notifInner.implicitHeight
            implicitHeight: notifInner.implicitHeight

            opacity: LazyListView.removing || LazyListView.adding ? 0 : 1
            scale: LazyListView.removing || LazyListView.adding ? 0.7 : 1

            cursorShape: notifInner.body?.hoveredLink ? Qt.PointingHandCursor : pressed ? Qt.ClosedHandCursor : undefined
            preventStealingDrag: true
            allowExpandGesture: false
            enabled: root.expanded && !(modelData?.closed ?? true)
            closed: modelData?.closed ?? true

            onExpandRequested: expanded => root.requestToggleExpand(expanded)
            onCloseRequested: modelData?.close()

            ParallelAnimation {
                running: notif.modelData?.closed ?? false
                onFinished: notif.modelData?.unlock(notif)

                Anim {
                    type: Anim.DefaultEffects
                    target: notif
                    property: "opacity"
                    to: 0
                }
                Anim {
                    target: notif
                    property: "x"
                    to: notif.x >= 0 ? notif.width : -notif.width
                }
            }

            Notif {
                id: notifInner

                anchors.fill: parent
                modelData: notif.modelData
                props: root.props
                expanded: root.expanded
                visibilities: root.visibilities
            }
        }
    }

    TransformWatcher {
        id: tWatcher

        a: root.container.contentItem
        b: root
    }
}
