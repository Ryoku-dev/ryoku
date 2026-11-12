pragma Singleton
pragma ComponentBehavior: Bound

import stage.modules.common
import stage
import stage.services
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Notifications
import Quickshell.Wayland
import Ryoku.Ui.Singletons

/**
 * Provides extra features not in Quickshell.Services.Notifications:
 *  - Persistent storage
 *  - Popup notifications, with timeout
 *  - Notification groups by app
 */
Singleton {
	id: root
    component Notif: QtObject {
        id: wrapper
        required property int notificationId // Could just be `id` but it conflicts with the default prop in QtObject
        property Notification notification
        // Internal notifications have no freedesktop sender to keep alive.
        // They still use the same cards/actions/popup path as server traffic.
        property bool internal: false
        property list<var> customActions: []
        property string _qsFilePath: ""
        property var internalActionPayload: ({})
        property int internalExpireTimeout: -1
        property list<var> actions: {
            var base = notification ? notification.actions.map(function(action) {
                return { "identifier": action.identifier, "text": action.text };
            }) : [];
            if (customActions.length > 0) return base.concat(customActions);
            return base;
        }
        /**
         * The notification already offers to copy its file, so the card's own copy-text
         * button steps aside: two copy buttons side by side read as one ambiguous one.
         */
        readonly property bool hasFileCopyAction: actions.some(action => action.identifier === "__qs_copy_file")
        property bool popup: false
        property bool isTransient: notification?.hints.transient ?? false
        property string appIcon: notification?.appIcon ?? ""
        property string appName: notification?.appName ?? ""
        property string body: notification?.body ?? ""
        property string image: notification?.image ?? ""
        property string summary: notification?.summary ?? ""
        property double time
        property string urgency: notification?.urgency.toString() ?? "normal"
        property Timer timer
        // Discarding a server notification clears its `notification` property,
        // which normally triggers discardNotification again. Mark the wrapper
        // first so the lifecycle cleanup is one-way and re-entrant safe.
        property bool _releasing: false

        onNotificationChanged: {
            if (notification === null && !internal && !_releasing) {
                root.discardNotification(notificationId);
            }
        }
    }

    function notifToJSON(notif) {
        return {
            "notificationId": notif.notificationId,
            "actions": notif.actions,
            "appIcon": notif.appIcon,
            "appName": notif.appName,
            "body": notif.body,
            "image": notif.image,
            "summary": notif.summary,
            "time": notif.time,
            "urgency": notif.urgency,
        }
    }
    function notifToString(notif) {
        return JSON.stringify(notifToJSON(notif), null, 2);
    }

    component NotifTimer: Timer {
        required property int notificationId
        interval: 7000
        running: true
        onTriggered: () => {
            const notifObject = root.findTrackedNotification(notificationId);
            const timer = notifObject?.timer ?? null;
            if (notifObject)
                notifObject.timer = null;
            print("[Notifications] Notification timer triggered for ID: " + notificationId + ", transient: " + notifObject?.isTransient);
            if (notifObject) {
                if (notifObject.isTransient) root.discardNotification(notificationId);
                else root.timeoutNotification(notificationId);
            }
            if (timer)
                timer.destroy();
            else
                destroy();
        }
    }

    property bool silent: false
    readonly property bool focusedWindowFullscreen:
        Wm.outputHasFullscreen(Wm.focusedOutput ?? "")

    readonly property bool autoSilent: (Config?.options.notifications.autoDndFullscreen ?? true) && focusedWindowFullscreen
    readonly property bool effectiveSilent: silent || autoSilent
    property int unread: 0
    property var filePath: Directories.notificationsPath
    // Keep the list typed to the stable Qt base class. A list<Notif> retains the
    // generated QML type revision in its element type; after a hot reload, existing
    // Notif objects then fail assignment to the regenerated Notif type and become
    // null entries ("Cannot append Notif(...) to a QML list").
    property list<QtObject> list: []
    property var popupList: list.filter((notif) => notif && notif.popup);
    property bool popupInhibited: (GlobalStates?.sidebarRightOpen ?? false) || effectiveSilent
    property var latestTimeForApp: ({})
    // Notification history is useful context, but retaining every wrapper and
    // image reference for the lifetime of the shell is not. Keep the newest
    // entries while the active notification center remains responsive.
    readonly property int maximumHistoryEntries: 200
    // See Config.qml for the rationale on these guards.
    property real initTimestamp: Date.now()
    property int missingFileGracePeriod: 2000
    property int missingFileRetryInterval: 1500

    // Debounced disk write timer - batches rapid notification changes
    property bool _pendingDiskWrite: false
    Timer {
        id: diskWriteTimer
        interval: 100
        repeat: false
        onTriggered: {
            notifFileView.setText(stringifyList(root.list));
            root._pendingDiskWrite = false;
        }
    }
    function scheduleDiskWrite() {
        root._pendingDiskWrite = true;
        diskWriteTimer.restart();
    }
    function flushDiskWrite() {
        diskWriteTimer.stop();
        if (root._pendingDiskWrite) {
            notifFileView.setText(stringifyList(root.list));
            root._pendingDiskWrite = false;
        }
    }

    // Pending notifications queue for batching
    property var _pendingNotifications: []
    Timer {
        id: batchNotificationTimer
        interval: 50
        repeat: false
        onTriggered: {
            if (root._pendingNotifications.length > 0) {
                const pending = root._pendingNotifications.slice();
                root._pendingNotifications = [];
                root.list = root.trimHistory(root.list.concat(pending.filter(notif => notif && !notif._releasing)));
                root.scheduleDiskWrite();
            }
        }
    }
    Component {
        id: notifComponent
        Notif {}
    }
    Component {
        id: notifTimerComponent
        NotifTimer {}
    }

    function stringifyList(list) {
        return JSON.stringify(list.filter((notif) => notif).map((notif) => notifToJSON(notif)), null, 2);
    }

    function trimHistory(values) {
        const entries = Array.from(values ?? []).filter(notif => notif && !notif._releasing);
        if (entries.length <= root.maximumHistoryEntries)
            return entries;
        const firstRetainedIndex = entries.length - root.maximumHistoryEntries;
        entries.slice(0, firstRetainedIndex).forEach(notif => root.releaseNotificationObject(notif));
        return entries.slice(firstRetainedIndex);
    }

    function releaseNotificationObject(notifObject) {
        if (!notifObject || notifObject._releasing)
            return;

        notifObject._releasing = true;
        const timer = notifObject.timer;
        notifObject.timer = null;
        if (timer) {
            timer.stop();
            timer.destroy();
        }

        // Drop the server-side QObject/image reference before destroying the
        // wrapper. The guard above prevents this assignment from recursively
        // entering discardNotification.
        notifObject.notification = null;
        notifObject.customActions = [];
        notifObject.internalActionPayload = ({});
        notifObject._qsFilePath = "";
        // Destroy on the next event-loop turn: Repeater releases pooled
        // delegates deferred, and binding re-evaluation against a wrapper
        // destroyed in this same tick logs "assign [undefined]" per property
        // and races in-flight image requests. The fields are already cleared
        // above, so those reads land on empty strings instead.
        Qt.callLater(() => notifObject.destroy());
    }
    
    onListChanged: {
        // Update latest time for each app reactively via reassignment
        const nextLatestTime = Object.assign({}, root.latestTimeForApp);
        root.list.forEach((notif) => {
            if (!notif) return;
            if (!nextLatestTime[notif.appName] || notif.time > nextLatestTime[notif.appName]) {
                nextLatestTime[notif.appName] = Math.max(nextLatestTime[notif.appName] || 0, notif.time);
            }
        });
        // Remove apps that no longer have notifications
        Object.keys(nextLatestTime).forEach((appName) => {
            if (!root.list.some((notif) => notif && notif.appName === appName)) {
                delete nextLatestTime[appName];
            }
        });
        root.latestTimeForApp = nextLatestTime;
    }

    function appNameListForGroups(groups) {
        return Object.keys(groups).sort((a, b) => {
            // Sort by time, descending
            return groups[b].time - groups[a].time;
        });
    }

    // Phone notifications are routed to the Phone tab only when the user opted out of
    // mirroring them here and a phone is actually there to show them.
    readonly property bool hidePhoneNotifications: KdeConnectService._enabled
        && KdeConnectService.activeReachable
        && !(Config.options?.phone?.mirrorNotificationsToDesktop ?? true)

    function groupsForList(list) {
        const groups = {};
        list.forEach((notif) => {
            if (!notif) return;
            const appNameLower = (notif.appName || "").toLowerCase();
            const isKdeConnect = appNameLower === "kdeconnect"
                || appNameLower === "kde connect"
                || appNameLower === "org.kde.kdeconnect"
                || KdeConnectService.devices.some(d => d.name && d.name.toLowerCase() === appNameLower);

            if (isKdeConnect && (root.hidePhoneNotifications || KdeConnectService.isIgnoredNotification(notif))) {
                return;
            }

            if (!String(notif.summary || "").trim() && !String(notif.body || "").trim()) {
                return;
            }

            if (!groups[notif.appName]) {
                groups[notif.appName] = {
                    appName: notif.appName,
                    appIcon: notif.appIcon,
                    notifications: [],
                    time: 0
                };
            }
            groups[notif.appName].notifications.push(notif);
            // Always set to the latest time in the group
            groups[notif.appName].time = latestTimeForApp[notif.appName] || notif.time;
        });
        return groups;
    }

    // Computed group bindings - automatically cached by the QML engine and re-evaluated reactively.
    property var groupsByAppName: groupsForList(root.list)
    property var popupGroupsByAppName: groupsForList(root.popupList)
    property list<string> appNameList: appNameListForGroups(root.groupsByAppName)
    property list<string> popupAppNameList: appNameListForGroups(root.popupGroupsByAppName)

    function appSoundsMuted(appName) {
        const conf = Config.options?.sounds;
        if (!conf) return false;
        const lower = (appName || "").toLowerCase();
        const neverApps = conf.neverPlayApps ?? [];
        return neverApps.some(app => app.toLowerCase() === lower);
    }

    function toggleAppSoundMute(appName) {
        if (!appName) return;
        const conf = Config.options?.sounds;
        if (!conf) return;
        const lower = appName.toLowerCase();
        const neverApps = conf.neverPlayApps ?? [];
        const alwaysApps = conf.alwaysPlayApps ?? [];
        if (root.appSoundsMuted(appName)) {
            conf.neverPlayApps = neverApps.filter(app => app.toLowerCase() !== lower);
        } else {
            conf.alwaysPlayApps = alwaysApps.filter(app => app.toLowerCase() !== lower);
            conf.neverPlayApps = [...neverApps, appName];
        }
    }

    // Quickshell's notification IDs starts at 1 on each run, while saved notifications
    // can already contain higher IDs. This is for avoiding id collisions
    property int idOffset
    signal initDone();
    signal notify(notification: var);
    signal discard(id: int);
    signal discardAll();
    signal timeout(id: var);
    // The sender of an internal notification is this process, so custom action
    // identifiers come back here rather than trying to call a dead notify-send.
    signal internalActionInvoked(string identifier, int notificationId, var payload);

    // A replacement notification keeps the freedesktop notification id. Keep
    // the same wrapper too, otherwise a long-running task such as an Ollama
    // pull would fill the center with one entry per percentage update.
    function findTrackedNotification(notificationId: int): var {
        const listed = root.list.find(notif => notif && notif.notificationId === notificationId);
        if (listed)
            return listed;
        return root._pendingNotifications.find(notif => notif && notif.notificationId === notificationId) ?? null;
    }

    function armNotificationTimeout(notifObject, notification) {
        const expireTimeout = notification?.expireTimeout ?? notifObject?.internalExpireTimeout ?? -1;
        if (root.popupInhibited || expireTimeout === 0)
            return;
        const interval = expireTimeout < 0
            ? (Config?.options.notifications.timeout ?? 7000)
            : expireTimeout;
        notifObject.popup = true;
        if (notifObject.timer) {
            notifObject.timer.interval = interval;
            notifObject.timer.restart();
        } else {
            notifObject.timer = notifTimerComponent.createObject(root, {
                "notificationId": notifObject.notificationId,
                "interval": interval,
            });
        }
    }

    function nextInternalNotificationId(): int {
        let candidate = -1;
        const used = new Set(Array.from(root.list ?? []).concat(root._pendingNotifications ?? [])
            .filter(notif => notif !== null && notif !== undefined)
            .map(notif => Number(notif.notificationId)));
        while (used.has(candidate))
            candidate--;
        return candidate;
    }

    /**
     * Adds a shell-owned notification without spawning notify-send. Actions
     * are regular `customActions`, so every notification surface dispatches
     * them through executeShellAction and the caller receives one signal.
     */
    function publishInternalNotification(options: var): var {
        if (root.effectiveSilent)
            return null;
        const notificationId = root.nextInternalNotificationId();
        const appName = String(options?.appName ?? "Quickshell");
        const notifObject = notifComponent.createObject(root, {
            "notificationId": notificationId,
            "internal": true,
            "appName": appName,
            "appIcon": String(options?.appIcon ?? ""),
            "summary": String(options?.summary ?? ""),
            "body": String(options?.body ?? ""),
            "urgency": String(options?.urgency ?? "normal"),
            "time": Date.now(),
            "internalExpireTimeout": Number(options?.expireTimeout ?? -1)
        });
        if (!notifObject)
            return null;
        notifObject.customActions = Array.from(options?.actions ?? []).filter(action => {
            return String(action?.identifier ?? "").startsWith("__qs_")
                && String(action?.text ?? "").length > 0;
        });
        notifObject.internalActionPayload = options?.actionPayload ?? ({});
        root._pendingNotifications.push(notifObject);
        batchNotificationTimer.restart();
        if (!root.popupInhibited) {
            root.armNotificationTimeout(notifObject, null);
            root.unread++;
        }
        if (options?.sound === true && !root.appSoundsMuted(appName))
            SoundService.playNotificationCue(notifObject.urgency === "critical");
        root.notify(notifObject);
        root.scheduleDiskWrite();
        return notificationId;
    }

    // Quickshell keeps one notification server per process and hands every
    // notification to each NotificationServer, so this one hears the same
    // traffic as shell/services/Notifs.qml. That server draws the banner and
    // plays the arrival chime under Settings > Desktop > Sounds. A sound from here
    // would be a second chime that neither the Hub nor Do Not Disturb silences.
	NotificationServer {
        id: notifServer
        // actionIconsSupported: true
        actionsSupported: true
        bodyHyperlinksSupported: true
        bodyImagesSupported: true
        bodyMarkupSupported: true
        bodySupported: true
        // The vendored Stage service stores image metadata but has no image
        // renderer. Current iRiS bridges to shell.services.Notifs instead.
        imageSupported: false
        keepOnReload: false
        persistenceSupported: true

        onNotification: (notification) => {
            const appNameLower = (notification.appName || "").toLowerCase();
            const isKdeConnect = appNameLower === "kdeconnect"
                || appNameLower === "kde connect"
                || appNameLower === "org.kde.kdeconnect"
                || KdeConnectService.devices.some(d => d.name && d.name.toLowerCase() === appNameLower);

            // The KDE Connect daemon reports outgoing file transfer results only
            // through this notification. The service consumes it, publishes a
            // translated shell notification instead, and the raw English one
            // stays hidden.
            if (isKdeConnect && KdeConnectService.considerTransferNotification(notification)) {
                notification.tracked = true;
                return;
            }

            // Filter out empty or unwanted System UI / lockscreen placeholder notifications from phone
            if (isKdeConnect && KdeConnectService.isIgnoredNotification(notification)) {
                notification.tracked = true;
                const existingNotifObject = root.findTrackedNotification(notification.id + root.idOffset);
                if (existingNotifObject) {
                    root.discardNotification(notification.id + root.idOffset);
                }
                return;
            }

            // Also ignore general notifications that have completely empty summary and body
            const summaryTrimmed = String(notification.summary || "").trim();
            const bodyTrimmed = String(notification.body || "").trim();
            if (!summaryTrimmed && !bodyTrimmed) {
                notification.tracked = true;
                return;
            }

            if (isKdeConnect && root.hidePhoneNotifications) {
                notification.tracked = true;
                return;
            }

            notification.tracked = true
            const notificationId = notification.id + root.idOffset;
            const existingNotifObject = root.findTrackedNotification(notificationId);
            if (existingNotifObject) {
                existingNotifObject.notification = notification;
                existingNotifObject.time = Date.now();
                root._handleShellNotification(existingNotifObject);
                root.armNotificationTimeout(existingNotifObject, notification);
                root.notify(existingNotifObject);
                root.triggerListChange();
                root.scheduleDiskWrite();
                return;
            }

            const newNotifObject = notifComponent.createObject(root, {
                "notificationId": notificationId,
                "notification": notification,
                "time": Date.now(),
            });

            root._handleShellNotification(newNotifObject);

            // Batch notifications to avoid rapid list updates
            root._pendingNotifications.push(newNotifObject);
            batchNotificationTimer.restart();

            // Popup
            if (!root.popupInhibited) {
                root.armNotificationTimeout(newNotifObject, notification);
                root.unread++;
            }
            root.notify(newNotifObject);
            // Schedule disk write instead of immediate write
            root.scheduleDiskWrite();
        }
    }

    function markAllRead() {
        root.unread = 0;
    }

    // Inject QML-handled action buttons for shell-internal notifications (screenshots, recordings).
    // Actions prefixed with "__qs_" are handled directly in QML via Quickshell.execDetached
    // instead of action.invoke(), since the original sender (notify-send) has already exited.
    function _handleShellNotification(notifObj) {
        var appName = notifObj.appName || "";
        var appIcon = notifObj.appIcon || "";
        var body = notifObj.body || "";

        // notify-send -i camera-photo → appIcon === "camera-photo"
        var isScreenshot = appIcon === "camera-photo";
        // notify-send -a 'Recorder' → appName === "Recorder"
        var isRecording = appName === "Recorder";

        // The battery warns before it suspends the machine, and the countdown is cancellable.
        if (notifObj.notification?.hints?.["x-qs-notif"] === "battery-suspend-warn") {
            notifObj.customActions = [
                { "identifier": "__qs_battery_suspend_now", "text": Translation.tr("Suspend now") },
                { "identifier": "__qs_battery_suspend_cancel", "text": Translation.tr("Stay awake") }
            ];
            return;
        }

        // The "Keep awake" timer warns before it expires and offers to push the deadline out.
        // Matched on a private hint rather than the icon: notify-send's -i doesn't reach appIcon.
        if (notifObj.notification?.hints?.["x-qs-notif"] === "keepawake-warn") {
            notifObj.customActions = [
                { "identifier": "__qs_keepawake_extend", "text": Translation.tr("Extend %1").arg(Idle.formatMinutes(Idle.extendMinutes)) },
                { "identifier": "__qs_keepawake_off", "text": Translation.tr("Stop") }
            ];
            return;
        }

        if (!isScreenshot && !isRecording) return;

        // Parse file path from body: "Saved to: /path" or "Saved to /path"
        var match = body.match(/(?:Saved to:?|Copied to)\s*(.+)/i);
        if (!match) return;
        var filePath = match[1].trim();
        if (!filePath || filePath.charAt(0) !== "/") return;

        var actions = [];
        if (isScreenshot) {
            actions.push(
                { "identifier": "__qs_open_file", "text": Translation.tr("Open"), "icon": "open_in_new", "iconOnly": true },
                { "identifier": "__qs_open_folder", "text": Translation.tr("Folder"), "icon": "folder_open" },
                { "identifier": "__qs_delete_file", "text": Translation.tr("Delete"), "icon": "delete" }
            );
        } else if (isRecording) {
            actions.push(
                { "identifier": "__qs_open_file", "text": Translation.tr("Open"), "icon": "open_in_new", "iconOnly": true },
                { "identifier": "__qs_open_folder", "text": Translation.tr("Folder"), "icon": "folder_open" },
                // The file itself, not the body text the card's own copy button takes -
                // label and glyph both say so, since the two sit in the same card.
                { "identifier": "__qs_copy_file", "text": Translation.tr("Copy video"), "icon": "movie" }
            );
        }

        notifObj.customActions = actions;
        notifObj._qsFilePath = filePath;
    }

    // Execute a QML-handled notification action (identified by "__qs_" prefix).
    function executeShellAction(notifObj, identifier) {
        if (String(identifier).startsWith("__qs_calendar_") || String(identifier).startsWith("__qs_reminder_")) {
            root.internalActionInvoked(identifier, notifObj?.notificationId ?? 0, notifObj?.internalActionPayload ?? ({}));
            return;
        }
        if (identifier === "__qs_battery_suspend_now") {
            Battery.suspendNow();
            return;
        } else if (identifier === "__qs_battery_suspend_cancel") {
            Battery.cancelAutomaticSuspend(true);
            return;
        }
        // Keep-awake actions carry no file, so they're handled before the file-path guard
        if (identifier === "__qs_keepawake_extend") {
            Idle.extendBy(Idle.extendMinutes);
            return;
        } else if (identifier === "__qs_keepawake_off") {
            Idle.toggleInhibit(false);
            return;
        }

        var filePath = notifObj._qsFilePath || "";
        if (!filePath) return;

        if (identifier === "__qs_open_file") {
            Spawn.runApp(["xdg-open", filePath], { label: "file" });
        } else if (identifier === "__qs_open_folder") {
            var dirPath = filePath.replace(/\/[^\/]*$/, "");
            Spawn.runApp(["xdg-open", dirPath], { label: "folder" });
        } else if (identifier === "__qs_delete_file") {
            Quickshell.execDetached(["bash", "-c", 'rm -f "$1"', "_", filePath]);
        } else if (identifier === "__qs_copy_file") {
            // A real file copy (uri-list + file-manager targets), not the path as
            // text: the paste lands as the video itself in chats and editors.
            Quickshell.execDetached(["python3",
                Directories.scriptPath + "/clipboard/copy_file_to_clipboard.py", filePath]);
        }
    }

    function discardNotification(id) {
        console.log("[Notifications] Discarding notification with ID: " + id);
        const matchesId = notif => notif && notif.notificationId === id;
        const targets = Array.from(root.list ?? []).concat(root._pendingNotifications ?? [])
            .filter(matchesId)
            .filter((notif, index, values) => values.indexOf(notif) === index);
        const notifServerIndex = notifServer.trackedNotifications.values.findIndex((notif) => notif.id + root.idOffset === id);
        if (targets.length > 0) {
            root.list = root.list.filter(notif => !matchesId(notif));
            root._pendingNotifications = root._pendingNotifications.filter(notif => !matchesId(notif));
            targets.forEach(notif => root.releaseNotificationObject(notif));
            root.scheduleDiskWrite();
        }
        if (notifServerIndex !== -1) {
            notifServer.trackedNotifications.values[notifServerIndex].dismiss()
        }
        root.discard(id); // Emit signal
    }

    function discardMultipleNotifications(ids) {
        if (!ids || ids.length === 0) return;
        const idSet = new Set(ids);
        const matchesId = notif => notif && idSet.has(notif.notificationId);
        const targets = Array.from(root.list ?? []).concat(root._pendingNotifications ?? [])
            .filter(matchesId)
            .filter((notif, index, values) => values.indexOf(notif) === index);
        root.list = root.list.filter(notif => !matchesId(notif));
        root._pendingNotifications = root._pendingNotifications.filter(notif => !matchesId(notif));
        targets.forEach(notif => root.releaseNotificationObject(notif));
        root.scheduleDiskWrite();
        notifServer.trackedNotifications.values.forEach(notif => {
            if (idSet.has(notif.id + root.idOffset)) {
                notif.dismiss();
            }
        });
        ids.forEach(id => root.discard(id));
    }

    function discardAllNotifications() {
        const targets = Array.from(root.list ?? []).concat(root._pendingNotifications ?? [])
            .filter(notif => notif)
            .filter((notif, index, values) => values.indexOf(notif) === index);
        root.list = [];
        root._pendingNotifications = [];
        targets.forEach(notif => root.releaseNotificationObject(notif));
        root.scheduleDiskWrite();
        notifServer.trackedNotifications.values.forEach((notif) => {
            notif.dismiss()
        })
        root.discardAll();
    }

    function cancelTimeout(id) {
        const index = root.list.findIndex((notif) => notif && notif.notificationId === id);
        // The timer is gone once it has fired.
        if (root.list[index] != null)
            root.list[index].timer?.stop();
    }

    function timeoutNotification(id) {
        const index = root.list.findIndex((notif) => notif && notif.notificationId === id);
        if (root.list[index] != null)
            root.list[index].popup = false;
        root.timeout(id);
    }

    function timeoutAll() {
        root.popupList.forEach((notif) => {
            root.timeout(notif.notificationId);
        })
        root.popupList.forEach((notif) => {
            notif.popup = false;
        });
    }

    function attemptInvokeAction(id, notifIdentifier) {
        console.log("[Notifications] Attempting to invoke action with identifier: " + notifIdentifier + " for notification ID: " + id);
        const notifServerIndex = notifServer.trackedNotifications.values.findIndex((notif) => notif.id + root.idOffset === id);
        console.log("Notification server index: " + notifServerIndex);
        if (notifServerIndex !== -1) {
            const notifServerNotif = notifServer.trackedNotifications.values[notifServerIndex];
            const action = notifServerNotif.actions.find((action) => action.identifier === notifIdentifier);
            // console.log("Action found: " + JSON.stringify(action));
            action.invoke()
        } 
        else {
            console.log("Notification not found in server: " + id)
        }
        root.discardNotification(id);
    }

    function triggerListChange() {
        root.list = root.list.slice(0)
    }

    function refresh() {
        notifFileView.reload()
    }

    Component.onCompleted: {
        refresh()
    }

    property bool _initialized: false

    FileView {
        id: notifFileView
        path: Qt.resolvedUrl(filePath)
        atomicWrites: true
        onLoaded: {
            if (root._initialized) return;
            const fileContents = notifFileView.text();
            try {
                const parsed = JSON.parse(fileContents || "[]");
                const filtered = parsed.filter(notif => {
                    if (!notif) return false;
                    const appLower = (notif.appName || "").toLowerCase();
                    const isKde = appLower === "kdeconnect"
                        || appLower === "kde connect"
                        || appLower === "org.kde.kdeconnect"
                        || KdeConnectService.devices.some(d => d.name && d.name.toLowerCase() === appLower);
                    if (isKde && KdeConnectService.isIgnoredNotification(notif)) return false;
                    if (!String(notif.summary || "").trim() && !String(notif.body || "").trim()) return false;
                    return true;
                });
                const retained = filtered.length > root.maximumHistoryEntries
                    ? filtered.slice(filtered.length - root.maximumHistoryEntries)
                    : filtered;
                root.list = retained.map((notif) => {
                    return notifComponent.createObject(root, {
                        "notificationId": notif.notificationId,
                        "actions": [], // Notification actions are meaningless if they're not tracked by the server or the sender is dead
                        "appIcon": notif.appIcon,
                        "appName": notif.appName,
                        "body": notif.body,
                        "image": notif.image,
                        "summary": notif.summary,
                        "time": notif.time,
                        "urgency": notif.urgency,
                    });
                });
                if (filtered.length !== parsed.length || parsed.length > root.maximumHistoryEntries)
                    root.scheduleDiskWrite();
            } catch (e) {
                console.log("[Notifications] Error parsing notifications JSON: " + e);
            }
            // Find largest notificationId
            let maxId = 0;
            root.list.forEach((notif) => {
                if (!notif) return;
                maxId = Math.max(maxId, notif.notificationId);
            });

            console.log("[Notifications] File loaded");
            root.idOffset = maxId;
            root._initialized = true;
            root.initDone();
        }
        onLoadFailed: (error) => {
            if(error != FileViewError.FileNotFound) {
                console.log("[Notifications] Error loading file: " + error);
                return;
            }
            // Lazy-rstoration: a transient missing file (hot-reload / restart /
            // partial disk I/O) should not erase the user's existing
            // notifications history. Only seed an empty list past the grace
            // window if the file is genuinely absent.
            if (Date.now() - root.initTimestamp > root.missingFileGracePeriod) {
                console.log("[Notifications] File genuinely missing, creating new file.")
                root.list = []
                root.scheduleDiskWrite();
            } else {
                missingFileRetryTimer.restart()
            }
        }
    }

    Timer {
        id: missingFileRetryTimer
        interval: root.missingFileRetryInterval
        repeat: false
        onTriggered: notifFileView.reload()
    }
}
