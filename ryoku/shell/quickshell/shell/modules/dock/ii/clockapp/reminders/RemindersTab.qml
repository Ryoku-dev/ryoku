pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import stage.modules.common.widgets
import stage.services
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../services/reminders/RemindersLogic.js" as Logic
import "../.."

/**
 * Reminders, after Samsung Reminder (One UI 8): the home screen's list cards (Today,
 * Scheduled, Important, No alert, Completed), the user's categories, the "Try these out"
 * templates and recent reminders; each list grouped by day; search with its filters; a
 * recycle bin; and the "Add reminder" bar along the bottom, with the microphone.
 *
 * Editing happens in the side sheet like everywhere else in the app. The page keeps only
 * which list is open, what is selected and what is being searched; the reminders belong
 * to RemindersService.
 */
Item {
    id: root

    BusTopic {
        id: dictationTopic
        topic: "dictation"
    }

    property date now: new Date()
    property bool compact: false
    property bool wide: false
    property ClockSidePanel panels: null
    property real layoutWidth: root.width

    signal settingsRequested()

    // ── Settings ────────────────────────────────────────────────────────
    readonly property var settings: Config.options.clockApp.reminders
    readonly property string sortBy: root.settings?.sortBy ?? "alertTime"
    readonly property bool pinImportant: root.settings?.pinImportant ?? true
    readonly property bool showCompleted: root.settings?.showCompleted ?? false
    readonly property string itemView: root.settings?.view ?? "card"

    // ── Where we are ────────────────────────────────────────────────────
    /// "home" | "list" | "search" | "trash"
    property string view: "home"
    /// In "list": a smart list id ("today", …) or "cat:<category id>".
    property string listId: ""
    property var selected: []
    property bool selectMode: false
    /// The ⋮ options are a side sheet (RemindersMenu), so "open" is whether it's showing.
    readonly property bool menuOpen: root.panels?.isShowing(optionsSheet) ?? false
    property string searchText: ""
    property var searchFilters: ({ checklist: false, images: false, links: false, completed: false })

    readonly property bool selecting: root.selectMode || root.selected.length > 0
    readonly property string categoryId: root.listId.startsWith("cat:") ? root.listId.slice(4) : ""
    readonly property var smart: RemindersStyle.smartList(root.listId)
    readonly property bool inTrash: root.view === "trash"
    readonly property string listTitle: root.view === "search" ? Translation.tr("Search")
        : root.inTrash ? Translation.tr("Recycle bin")
        : root.categoryId.length > 0 ? RemindersService.categoryName(root.categoryId)
        : (root.smart?.label ?? "")

    // ── Layout ──────────────────────────────────────────────────────────
    // Every decision (columns, tile sizes, type sizes) reads the settled width; the live
    // one only stretches what is already laid out (guide §5.3). Grids are Flows whose
    // items carry their own width, never a layout that stretches its columns.
    readonly property real contentWidth: Math.min(root.width - ClockStyle.gapTiny * 2, 1180)
    readonly property real contentLayoutWidth: Math.min(root.layoutWidth - ClockStyle.gapTiny * 2, 1180)

    readonly property int tileColumns: root.contentLayoutWidth >= 860 ? 5 : root.contentLayoutWidth >= 520 ? 3 : 2
    /// Five lists never fill three or two columns evenly, so Today takes two cells there
    /// and the grid closes without a hole.
    readonly property int todaySpan: root.tileColumns === 5 ? 1 : 2
    readonly property real tileLayoutWidth: (root.contentLayoutWidth
        - ClockStyle.gap * (root.tileColumns - 1)) / root.tileColumns
    readonly property bool listsCollapsed: !(root.settings?.categoriesExpanded ?? true)
    readonly property real tileHeight: root.listsCollapsed ? 52
        : Math.round(Math.max(112, Math.min(148, root.tileLayoutWidth * 0.56)))
    readonly property int tileDigitSize: Math.round(Math.max(34, Math.min(56, root.tileHeight * 0.36)))

    readonly property int listColumns: root.contentLayoutWidth >= 980 && root.itemView === "card" ? 2 : 1
    readonly property real listGap: root.itemView === "list" ? ClockStyle.gapTiny : ClockStyle.gapSmall
    readonly property real itemLayoutWidth: (root.contentLayoutWidth - root.listGap * (root.listColumns - 1))
        / root.listColumns

    /// Home on a wide page: your categories and the templates on the left, recent
    /// reminders beside them instead of under them (guide §5.2).
    readonly property bool homeSplit: root.contentLayoutWidth >= 900 && root.recentIds.length > 0
    readonly property real homePaneWidth: root.homeSplit
        ? Math.floor((root.contentWidth - ClockStyle.gapHuge) / 2) : root.contentWidth
    readonly property real homePaneLayoutWidth: root.homeSplit
        ? Math.floor((root.contentLayoutWidth - ClockStyle.gapHuge) / 2) : root.contentLayoutWidth
    readonly property int categoryColumns: root.homePaneLayoutWidth >= 520 ? 2 : 1
    readonly property real categoryLayoutWidth: (root.homePaneLayoutWidth
        - ClockStyle.gapSmall * (root.categoryColumns - 1)) / root.categoryColumns
    readonly property int templateColumns: Math.max(1,
        Math.floor((root.homePaneLayoutWidth + ClockStyle.gapSmall) / (200 + ClockStyle.gapSmall)))
    readonly property real templateLayoutWidth: (root.homePaneLayoutWidth
        - ClockStyle.gapSmall * (root.templateColumns - 1)) / root.templateColumns
    /// A pane's Flows are never narrower than the settled pane, so they don't re-wrap
    /// while the page width is still animating.
    readonly property real homeFlowWidth: Math.max(root.homePaneWidth, root.homePaneLayoutWidth)

    // ── Data ────────────────────────────────────────────────────────────
    readonly property var counts: Logic.smartCounts(RemindersService.reminders, root.now)
    readonly property var categoryCounts: Logic.categoryCounts(RemindersService.reminders)
    readonly property var categoryOrder: RemindersService.categoryOrder()
    property var memo: ({})
    /// What a delegate shows for the frame between its reminder leaving and the list
    /// dropping it: every field present, so nothing reads off undefined.
    readonly property var placeholder: ({ id: "", title: "", notes: "", categoryId: "default", checklist: [], attachments: [],
        schedule: null, completed: false, important: false, early: null, snoozedUntil: 0, alert: "default" })

    /// The open list as groups of ids — ids, so a reminder edited in place keeps its
    /// delegate (and an unchanged list keeps the very same array).
    readonly property var groups: {
        let list;
        let grouped = false;
        const all = RemindersService.reminders;
        if (root.view === "trash") {
            list = Logic.sortReminders(RemindersService.trashed, "modified", false, {}, RemindersService.allDayTime);
        } else if (root.view === "search") {
            const filtered = Logic.search(all, root.searchText, root.searchFilters);
            list = Logic.sortReminders(filtered, root.sortBy, root.pinImportant, root.categoryOrder, RemindersService.allDayTime);
        } else if (root.view === "list" && root.listId === "completed") {
            list = Logic.sortReminders(all.filter(item => Logic.inSmartList(item, "completed", root.now)), "completed", false, {},
                RemindersService.allDayTime);
        } else if (root.view === "list") {
            const open = root.categoryId.length > 0
                ? all.filter(item => Logic.isLive(item) && item.categoryId === root.categoryId)
                : all.filter(item => Logic.inSmartList(item, root.listId, root.now));
            list = Logic.sortReminders(open, root.sortBy, root.pinImportant, root.categoryOrder, RemindersService.allDayTime);
            grouped = root.sortBy === "alertTime" && root.listId !== "noAlert" && root.listId !== "important";
        } else {
            return ObjectUtils.keep(root.memo, "groups", []);
        }
        let groups = grouped
            ? Logic.groupByDay(list, root.now, RemindersService.allDayTime).map(group => ({ key: group.key, ids: group.reminders.map(item => item.id) }))
            : [{ key: "", ids: list.map(item => item.id) }];
        // A category list shows its finished reminders under them, when asked to.
        if (root.view === "list" && root.categoryId.length > 0 && root.showCompleted) {
            const done = Logic.sortReminders(all.filter(item => !item.deletedAt && item.completed && item.categoryId === root.categoryId),
                "completed", false, {}, RemindersService.allDayTime);
            if (done.length > 0)
                groups = groups.concat([{ key: "completed", ids: done.map(item => item.id) }]);
        }
        return ObjectUtils.keep(root.memo, "groups", groups.filter(group => group.ids.length > 0));
    }
    readonly property int shownCount: root.groups.reduce((sum, group) => sum + group.ids.length, 0)

    readonly property var recentIds: ObjectUtils.keep(root.memo, "recent", RemindersService.reminders
        .filter(item => Logic.isLive(item))
        .sort((a, b) => b.modifiedAt - a.modifiedAt)
        .slice(0, 5)
        .map(item => item.id))

    readonly property var templates: [
        { id: "workout", icon: "fitness_center", title: Translation.tr("Workout schedule"), subtitle: Translation.tr("Tuesdays and Thursdays") },
        { id: "payment", icon: "payments", title: Translation.tr("Monthly payment"), subtitle: Translation.tr("The 1st of every month") },
        { id: "pickup", icon: "local_shipping", title: Translation.tr("Pickup reminder"), subtitle: Translation.tr("Tomorrow at 17:00") },
        { id: "home", icon: "home", title: Translation.tr("When you get home"), subtitle: Translation.tr("This evening") },
        { id: "grocery", icon: "shopping_cart", title: Translation.tr("Grocery list"), subtitle: Translation.tr("A checklist to tick off") }
    ]

    readonly property string pageSubtitle: {
        const today = root.counts.today;
        const next = RemindersService.nextAlert;
        const nextItem = next ? RemindersService.reminder(next.id) : null;
        const parts = [];
        if (today > 0)
            parts.push(today === 1 ? Translation.tr("1 for today") : Translation.tr("%1 for today").arg(String(today)));
        if (nextItem)
            parts.push(Translation.tr("Next: %1").arg(nextItem.title || Translation.tr("Reminder")) + " · " + RemindersService.whenText(nextItem, root.now));
        return parts.length > 0 ? parts.join(" · ") : Translation.tr("Nothing coming up");
    }
    readonly property bool actionAvailable: !root.inTrash && !root.selecting
    readonly property string editingId: root.panels?.isShowing(editorSheet) ? (root.panels.current?.reminderId ?? "") : ""

    // ── Actions ─────────────────────────────────────────────────────────
    function primaryAction(): void {
        root.openEditor("", root.draftForList({}));
    }

    // The options sheet stays open while you move between lists: what it sets (sort,
    // view) applies to whichever list is beside it.
    function openEditor(id: string, draft): void {
        root.panels?.show(editorSheet, { reminderId: id, initialDraft: draft ?? null });
    }

    function toggleOptions(): void {
        if (root.menuOpen)
            root.panels?.close();
        else
            root.panels?.show(optionsSheet, {});
    }

    function openList(id: string): void {
        root.clearSelection();
        root.listId = id;
        root.view = "list";
        flick.contentY = 0;
    }

    function goHome(): void {
        root.clearSelection();
        root.view = "home";
        root.listId = "";
        root.searchText = "";
        flick.contentY = 0;
    }

    function openSearch(): void {
        root.clearSelection();
        root.view = "search";
        Qt.callLater(() => searchField.forceActiveFocus());
    }

    /** What a new reminder starts with, from the list it is added in. */
    function draftForList(base) {
        const draft = Object.assign({}, base);
        if (root.categoryId.length > 0)
            draft.categoryId = root.categoryId;
        if (root.view === "list" && root.listId === "important")
            draft.important = true;
        if (root.view === "list" && (root.listId === "today" || root.listId === "scheduled") && !draft.schedule) {
            const soon = new Date(Date.now() + 3600000);
            draft.schedule = root.listId === "today"
                ? { date: Qt.formatDate(new Date(), "yyyy-MM-dd"), time: "" }
                : { date: Qt.formatDate(soon, "yyyy-MM-dd"), time: Qt.formatTime(soon, "HH:mm") };
        }
        return draft;
    }

    function quickAdd(text: string): void {
        const title = text.trim();
        if (title.length === 0)
            return;
        RemindersService.create(root.draftForList({ title: title }));
        quickField.text = "";
    }

    function useTemplate(id: string): void {
        const draft = Logic.templateDraft(id, new Date(), {
            workout: Translation.tr("Workout"),
            payment: Translation.tr("Pay the bills"),
            pickup: Translation.tr("Pick up the parcel"),
            home: Translation.tr("When I get home"),
            grocery: Translation.tr("Grocery list"),
            groceryItems: [Translation.tr("Milk"), Translation.tr("Bread"), Translation.tr("Eggs"), Translation.tr("Fruit")]
        });
        root.openEditor("", draft);
    }

    function toggleSelected(id: string): void {
        root.selected = root.selected.includes(id) ? root.selected.filter(item => item !== id) : root.selected.concat([id]);
        if (root.selected.length === 0)
            root.selectMode = false;
    }

    function clearSelection(): void {
        root.selected = [];
        root.selectMode = false;
    }

    function selectAll(): void {
        const all = [];
        root.groups.forEach(group => group.ids.forEach(id => all.push(id)));
        root.selected = all;
    }

    function groupTitle(key: string): string {
        switch (key) {
        case "overdue": return Translation.tr("Overdue");
        case "today": return Translation.tr("Today");
        case "tomorrow": return Translation.tr("Tomorrow");
        case "week": return Translation.tr("Next 7 days");
        case "later": return Translation.tr("Later");
        case "none": return Translation.tr("No alert");
        case "completed": return Translation.tr("Completed");
        }
        return "";
    }

    function setOption(key: string, value): void {
        Config.options.clockApp.reminders[key] = value;
    }

    // A notification's Open, or anything else that asks for one reminder.
    function consumePendingOpen(): void {
        const id = GlobalStates.reminderToOpen;
        if (id.length === 0)
            return;
        GlobalStates.reminderToOpen = "";
        if (RemindersService.reminder(id))
            Qt.callLater(() => root.openEditor(id, null));
    }

    Component.onCompleted: root.consumePendingOpen()

    Connections {
        target: GlobalStates
        function onReminderToOpenChanged() {
            root.consumePendingOpen();
        }
    }

    Keys.onPressed: event => {
        const ctrl = event.modifiers & Qt.ControlModifier;
        if (ctrl && event.key === Qt.Key_F) {
            root.openSearch();
            event.accepted = true;
        } else if (event.key === Qt.Key_Escape && root.selecting) {
            root.clearSelection();
            event.accepted = true;
        } else if (event.key === Qt.Key_Escape && root.view !== "home") {
            root.goHome();
            event.accepted = true;
        } else if (event.key === Qt.Key_Delete && root.selected.length > 0) {
            if (root.inTrash)
                RemindersService.deleteForever(root.selected);
            else
                RemindersService.trash(root.selected);
            root.clearSelection();
            event.accepted = true;
        }
    }

    Component {
        id: editorSheet
        ReminderEditorSheet {}
    }

    Component {
        id: categorySheet
        ReminderCategorySheet {}
    }

    Component {
        id: categoriesSheet
        ReminderCategoriesSheet {
            onEditRequested: categoryId => root.panels?.show(categorySheet, { categoryId: categoryId })
            onMoved: root.clearSelection()
        }
    }

    // Built in this tab's context, so its bindings follow the tab while it is open.
    Component {
        id: optionsSheet
        RemindersMenu {
            sortBy: root.sortBy
            pinImportant: root.pinImportant
            showCompleted: root.showCompleted
            itemView: root.itemView
            listId: root.listId
            inTrash: root.inTrash
            onOptionChanged: (key, value) => root.setOption(key, value)
            onManageRequested: root.panels?.show(categoriesSheet, { mode: "manage" })
            onTrashRequested: {
                root.panels?.close();
                root.clearSelection();
                root.view = "trash";
                root.listId = "";
                flick.contentY = 0;
            }
            onSettingsRequested: {
                root.panels?.close();
                root.settingsRequested();
            }
            onSyncRequested: RemindersSync.syncNow()
            onEditCategoryRequested: root.panels?.show(categorySheet, { categoryId: root.categoryId })
        }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: ClockStyle.gapSmall

        // ── Toolbar ─────────────────────────────────────────────────────
        Item {
            Layout.fillWidth: true
            implicitHeight: 48

            // Selection replaces the toolbar with what can be done to the selection.
            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: ClockStyle.gapTiny
                anchors.rightMargin: ClockStyle.gapTiny
                visible: root.selecting
                spacing: 2

                ClockIconButton {
                    symbol: "close"
                    tooltip: Translation.tr("Cancel")
                    onClicked: root.clearSelection()
                }

                StyledText {
                    Layout.fillWidth: true
                    text: root.selected.length === 0 ? Translation.tr("Select reminders")
                        : Translation.tr("%1 selected").arg(String(root.selected.length))
                    font.pixelSize: ClockStyle.textLarge
                    font.weight: Font.DemiBold
                    color: ClockStyle.colOnSurface
                    elide: Text.ElideRight
                }

                ClockIconButton {
                    symbol: "select_all"
                    tooltip: Translation.tr("Select all")
                    onClicked: root.selectAll()
                }

                ClockIconButton {
                    visible: !root.inTrash
                    enabled: root.selected.length > 0
                    symbol: "check_circle"
                    tooltip: Translation.tr("Complete")
                    onClicked: {
                        RemindersService.completeMany(root.selected);
                        root.clearSelection();
                    }
                }

                ClockIconButton {
                    visible: !root.inTrash
                    enabled: root.selected.length > 0
                    symbol: "drive_file_move"
                    tooltip: Translation.tr("Move to category")
                    onClicked: root.panels?.show(categoriesSheet, { mode: "move", reminderIds: root.selected })
                }

                ClockIconButton {
                    visible: !root.inTrash
                    enabled: root.selected.length > 0
                    symbol: "content_copy"
                    tooltip: Translation.tr("Duplicate")
                    onClicked: {
                        root.selected.forEach(id => RemindersService.duplicate(id));
                        root.clearSelection();
                    }
                }

                ClockIconButton {
                    visible: root.inTrash
                    enabled: root.selected.length > 0
                    symbol: "restore_from_trash"
                    tooltip: Translation.tr("Restore")
                    onClicked: {
                        RemindersService.restoreFromTrash(root.selected);
                        root.clearSelection();
                    }
                }

                ClockIconButton {
                    enabled: root.selected.length > 0
                    symbol: root.inTrash ? "delete_forever" : "delete"
                    colIcon: ClockStyle.colError
                    tooltip: root.inTrash ? Translation.tr("Delete for good") : Translation.tr("Move to recycle bin")
                    onClicked: {
                        if (root.inTrash)
                            RemindersService.deleteForever(root.selected);
                        else
                            RemindersService.trash(root.selected);
                        root.clearSelection();
                    }
                }
            }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: ClockStyle.gapTiny
                anchors.rightMargin: ClockStyle.gapTiny
                visible: !root.selecting
                spacing: 2

                ClockIconButton {
                    visible: root.view !== "home"
                    symbol: "arrow_back"
                    tooltip: Translation.tr("Back")
                    onClicked: root.goHome()
                }

                // A list's name, in its category colour.
                RowLayout {
                    Layout.fillWidth: true
                    visible: root.view === "list" || root.view === "trash"
                    spacing: ClockStyle.gapSmall

                    MaterialSymbol {
                        text: root.inTrash ? "delete" : root.categoryId.length > 0 ? RemindersStyle.categoryIcon(root.categoryId) : (root.smart?.icon ?? "")
                        iconSize: ClockStyle.iconNormal
                        fill: 1
                        color: root.categoryId.length > 0 ? RemindersStyle.categoryColor(root.categoryId) : ClockStyle.colPrimary
                    }

                    StyledText {
                        Layout.fillWidth: true
                        text: root.listTitle + (root.shownCount > 0 ? "  " + root.shownCount : "")
                        font.family: ClockStyle.fontTitle
                        font.variableAxes: ClockStyle.axesTitle
                        font.pixelSize: ClockStyle.textTitle - 4
                        color: ClockStyle.colOnSurface
                        elide: Text.ElideRight
                    }
                }

                // Search: the field fills the toolbar, filters below it.
                Rectangle {
                    Layout.fillWidth: true
                    visible: root.view === "search"
                    implicitHeight: 44
                    radius: ClockStyle.pill(height)
                    color: ClockStyle.colSurfaceHigh

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 14
                        anchors.rightMargin: 6
                        spacing: 8

                        MaterialSymbol {
                            text: "search"
                            iconSize: ClockStyle.iconSmall + 4
                            color: ClockStyle.colOnSurfaceVariant
                        }

                        StyledTextInput {
                            id: searchField
                            Layout.fillWidth: true
                            clip: true
                            font.pixelSize: Appearance.font.pixelSize.normal
                            color: ClockStyle.colOnSurface
                            onTextChanged: root.searchText = searchField.text
                            Keys.onEscapePressed: event => {
                                root.goHome();
                                event.accepted = true;
                            }

                            StyledText {
                                anchors.fill: parent
                                visible: searchField.text.length === 0
                                verticalAlignment: Text.AlignVCenter
                                text: Translation.tr("Search reminders")
                                font.pixelSize: Appearance.font.pixelSize.normal
                                color: Appearance.colors.colOnLayer1Inactive
                            }
                        }

                        ClockIconButton {
                            visible: searchField.text.length > 0
                            symbol: "close"
                            size: 32
                            iconSize: ClockStyle.iconSmall + 2
                            tooltip: Translation.tr("Clear")
                            onClicked: searchField.text = ""
                        }
                    }
                }

                Item {
                    Layout.fillWidth: true
                    visible: root.view === "home"
                }

                ClockIconButton {
                    visible: root.view !== "search"
                    symbol: "search"
                    tooltip: Translation.tr("Search") + " (Ctrl+F)"
                    onClicked: root.openSearch()
                }

                ClockIconButton {
                    visible: root.view !== "home"
                    enabled: root.shownCount > 0
                    symbol: "checklist_rtl"
                    tooltip: Translation.tr("Select")
                    onClicked: root.selectMode = true
                }

                ClockIconButton {
                    id: menuButton
                    symbol: "more_vert"
                    toggled: root.menuOpen
                    tooltip: Translation.tr("More options")
                    onClicked: root.toggleOptions()
                }
            }
        }

        // Search filters (Samsung's Checklists / Images / Links / Complete).
        Flow {
            Layout.fillWidth: true
            Layout.leftMargin: ClockStyle.gapTiny
            visible: root.view === "search"
            spacing: 6

            Repeater {
                model: [
                    { id: "checklist", icon: "checklist", label: Translation.tr("Checklists") },
                    { id: "images", icon: "image", label: Translation.tr("Images") },
                    { id: "links", icon: "link", label: Translation.tr("Links") },
                    { id: "completed", icon: "check_circle", label: Translation.tr("Completed") }
                ]

                ClockFormChip {
                    required property var modelData
                    symbol: modelData.icon
                    label: modelData.label
                    selected: root.searchFilters[modelData.id] === true
                    onTriggered: {
                        const next = Object.assign({}, root.searchFilters);
                        next[modelData.id] = !next[modelData.id];
                        root.searchFilters = next;
                    }
                }
            }
        }

        // ── Page ────────────────────────────────────────────────────────
        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            StyledFlickable {
                id: flick
                anchors.fill: parent
                contentWidth: width
                contentHeight: page.implicitHeight + ClockStyle.fabClearance
                clip: true

                ColumnLayout {
                    id: page
                    x: (flick.width - width) / 2
                    y: ClockStyle.gapTiny
                    width: root.contentWidth
                    spacing: ClockStyle.gapHuge

                    // ── Home: the list cards ────────────────────────────
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: root.view === "home"
                        spacing: ClockStyle.gap

                        SectionHeader {
                            title: Translation.tr("Lists")
                            collapsible: true
                            collapsed: root.listsCollapsed
                            onToggled: root.setOption("categoriesExpanded", root.listsCollapsed)
                        }

                        RedealFlow {
                            Layout.preferredWidth: Math.max(root.contentWidth, root.contentLayoutWidth)
                            spacing: ClockStyle.gap

                            Repeater {
                                model: RemindersStyle.smartLists

                                SmartTile {
                                    required property var modelData
                                    required property int index
                                    readonly property int span: modelData.id === "today" ? root.todaySpan : 1
                                    width: Math.floor(root.tileLayoutWidth * span
                                        + ClockStyle.gap * (span - 1))
                                    list: modelData
                                    count: root.counts[modelData.id] ?? 0
                                    collapsed: root.listsCollapsed

                                    StaggeredEntrance {
                                        index: parent.index
                                        active: !ClockStyle.reducedMotion
                                    }
                                }
                            }
                        }
                    }

                    // ── Home: categories and templates | recent ─────────
                    GridLayout {
                        Layout.fillWidth: true
                        visible: root.view === "home"
                        columns: root.homeSplit ? 2 : 1
                        columnSpacing: ClockStyle.gapHuge
                        rowSpacing: ClockStyle.gapHuge

                        ColumnLayout {
                            Layout.fillWidth: true
                            Layout.preferredWidth: root.homePaneWidth
                            Layout.alignment: Qt.AlignTop
                            spacing: ClockStyle.gapHuge

                            // ── My reminders ────────────────────────────
                            ColumnLayout {
                                Layout.fillWidth: true
                                spacing: ClockStyle.gapSmall

                                SectionHeader {
                                    title: Translation.tr("My reminders")

                                    ClockButton {
                                        variant: "text"
                                        symbol: "create_new_folder"
                                        label: Translation.tr("Add")
                                        onClicked: root.panels?.show(categorySheet, {})
                                    }

                                    ClockButton {
                                        variant: "text"
                                        symbol: "tune"
                                        label: Translation.tr("Manage")
                                        onClicked: root.panels?.show(categoriesSheet, { mode: "manage" })
                                    }
                                }

                                RedealFlow {
                                    Layout.preferredWidth: root.homeFlowWidth
                                    spacing: ClockStyle.gapSmall

                                    Repeater {
                                        model: RemindersService.categories

                                        CategoryRow {
                                            required property var modelData
                                            width: Math.floor(root.categoryLayoutWidth)
                                            category: modelData
                                        }
                                    }
                                }
                            }

                            // ── Templates ───────────────────────────────
                            ColumnLayout {
                                Layout.fillWidth: true
                                visible: root.settings?.showTemplates ?? true
                                spacing: ClockStyle.gapSmall

                                SectionHeader {
                                    title: Translation.tr("Try these out")

                                    ClockIconButton {
                                        symbol: "close"
                                        size: 32
                                        iconSize: ClockStyle.iconSmall + 2
                                        tooltip: Translation.tr("Hide templates")
                                        onClicked: root.setOption("showTemplates", false)
                                    }
                                }

                                RedealFlow {
                                    Layout.preferredWidth: root.homeFlowWidth
                                    spacing: ClockStyle.gapSmall

                                    Repeater {
                                        model: root.templates

                                        TemplateCard {
                                            required property var modelData
                                            required property int index
                                            // The last card takes what its row has left, so
                                            // the strip ends flush instead of on a hole.
                                            readonly property int span: index === root.templates.length - 1
                                                ? root.templateColumns - (index % root.templateColumns) : 1
                                            width: Math.floor(root.templateLayoutWidth * span
                                                + ClockStyle.gapSmall * (span - 1))
                                            template: modelData
                                        }
                                    }
                                }
                            }
                        }

                        // ── Recent ──────────────────────────────────────
                        ColumnLayout {
                            Layout.fillWidth: true
                            Layout.preferredWidth: root.homePaneWidth
                            Layout.alignment: Qt.AlignTop
                            visible: root.recentIds.length > 0
                            spacing: ClockStyle.gapSmall

                            SectionHeader {
                                title: Translation.tr("Recent reminders")
                            }

                            Repeater {
                                model: root.recentIds

                                ReminderItem {
                                    required property string modelData
                                    reminder: RemindersService.reminder(modelData) ?? root.placeholder
                                    visible: RemindersService.reminder(modelData) !== null
                                    now: root.now
                                    view: root.itemView
                                    editing: root.editingId === modelData
                                    onOpenRequested: root.openEditor(modelData, null)
                                    onSelectToggled: root.openEditor(modelData, null)
                                }
                            }
                        }
                    }

                    // ── A list ──────────────────────────────────────────
                    Repeater {
                        model: root.view === "home" ? [] : root.groups

                        ColumnLayout {
                            id: group
                            required property var modelData
                            Layout.fillWidth: true
                            spacing: ClockStyle.gapSmall

                            SectionHeader {
                                visible: group.modelData.key.length > 0
                                title: root.groupTitle(group.modelData.key)
                                count: group.modelData.ids.length
                                alert: group.modelData.key === "overdue"
                            }

                            RedealFlow {
                                Layout.preferredWidth: Math.max(root.contentWidth, root.contentLayoutWidth)
                                spacing: root.listGap

                                Repeater {
                                    model: group.modelData.ids

                                    ReminderItem {
                                        required property string modelData
                                        width: Math.floor(root.itemLayoutWidth)
                                        animateWidth: true
                                        reminder: RemindersService.reminder(modelData) ?? root.placeholder
                                        now: root.now
                                        view: root.itemView
                                        selecting: root.selecting
                                        selected: root.selected.includes(modelData)
                                        editing: root.editingId === modelData
                                        showCategory: root.categoryId.length === 0
                                        inTrash: root.inTrash
                                        onOpenRequested: {
                                            if (root.inTrash)
                                                root.toggleSelected(modelData);
                                            else
                                                root.openEditor(modelData, null);
                                        }
                                        onSelectToggled: root.toggleSelected(modelData)
                                    }
                                }
                            }
                        }
                    }

                    // Bin and completed housekeeping, under their lists.
                    RowLayout {
                        Layout.fillWidth: true
                        visible: (root.inTrash || (root.view === "list" && root.listId === "completed")) && root.shownCount > 0
                        spacing: ClockStyle.gapSmall

                        StyledText {
                            Layout.fillWidth: true
                            text: root.inTrash
                                ? (RemindersService.trashDays > 0
                                    ? Translation.tr("Reminders in the recycle bin are deleted after %1 days.").arg(String(RemindersService.trashDays))
                                    : Translation.tr("Reminders stay in the recycle bin until you empty it."))
                                : (RemindersService.autoDeleteCompletedDays > 0
                                    ? Translation.tr("Completed reminders move to the recycle bin after %1 days.").arg(String(RemindersService.autoDeleteCompletedDays))
                                    : "")
                            wrapMode: Text.Wrap
                            font.pixelSize: ClockStyle.textSmall
                            color: ClockStyle.colSubtext
                        }

                        ClockButton {
                            variant: "tonal"
                            danger: true
                            symbol: root.inTrash ? "delete_forever" : "delete_sweep"
                            label: root.inTrash ? Translation.tr("Empty recycle bin") : Translation.tr("Clear completed")
                            onClicked: {
                                if (root.inTrash)
                                    RemindersService.emptyTrash();
                                else
                                    RemindersService.clearCompleted();
                            }
                        }
                    }
                }
            }

            // An empty list says what goes in it.
            ColumnLayout {
                anchors.centerIn: parent
                width: Math.min(parent.width - ClockStyle.gapHuge * 2, 420)
                visible: root.view !== "home" && root.shownCount === 0
                spacing: ClockStyle.gapHuge

                ClockEmptyState {
                    Layout.alignment: Qt.AlignHCenter
                    symbol: root.inTrash ? "delete" : root.view === "search" ? "search" : (root.smart?.icon ?? "checklist")
                    // Shapes no tile, row or badge of this tab wears, so the empty page
                    // doesn't read as a blown-up copy of the tile you came from.
                    shape: root.inTrash ? "Ghostish" : root.view === "search" ? "Arch" : "Bun"
                    title: root.inTrash ? Translation.tr("Recycle bin is empty")
                        : root.view === "search" ? (root.searchText.length > 0 ? Translation.tr("No matches") : Translation.tr("Search your reminders"))
                        : root.listId === "completed" ? Translation.tr("Nothing completed yet")
                        : Translation.tr("No reminders")
                    // Completed has no add bar, so it doesn't point at one.
                    subtitle: root.inTrash ? ""
                        : root.view === "search" ? Translation.tr("Titles, notes, checklists and links are all searched.")
                        : root.listId === "completed" ? Translation.tr("Reminders you complete show up here.")
                        : Translation.tr("Add one below, or press Ctrl+N for the full editor.")
                }
            }
        }

        // ── Add reminder ────────────────────────────────────────────────
        // Samsung's bottom bar: type and press Enter, or dictate.
        Rectangle {
            id: quickBar
            Layout.fillWidth: true
            Layout.rightMargin: ClockStyle.fabSizeLarge + ClockStyle.gapLarge * 2 - ClockStyle.paneGap
            Layout.bottomMargin: ClockStyle.gapLarge + (ClockStyle.fabSizeLarge - implicitHeight) / 2
            visible: !root.inTrash && root.view !== "search" && !(root.view === "list" && root.listId === "completed") && !root.selecting
            implicitHeight: 56
            radius: ClockStyle.pill(height)
            color: quickField.activeFocus ? ClockStyle.colSurfaceHighest : ClockStyle.colSurfaceHigh

            Behavior on color {
                enabled: !ClockStyle.reducedMotion
                animation: ClockStyle.motionFast.colorAnimation.createObject(this)
            }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 8
                anchors.rightMargin: 8
                spacing: 6

                ClockIconButton {
                    symbol: "add"
                    size: 40
                    tooltip: Translation.tr("Add reminder")
                    onClicked: quickField.text.trim().length > 0 ? root.quickAdd(quickField.text) : root.primaryAction()
                }

                StyledTextInput {
                    id: quickField
                    Layout.fillWidth: true
                    clip: true
                    font.pixelSize: Appearance.font.pixelSize.normal
                    color: ClockStyle.colOnSurface
                    onAccepted: root.quickAdd(quickField.text)
                    Keys.onEscapePressed: event => {
                        quickField.text = "";
                        root.forceActiveFocus();
                        event.accepted = true;
                    }

                    StyledText {
                        anchors.fill: parent
                        visible: quickField.text.length === 0
                        verticalAlignment: Text.AlignVCenter
                        text: root.categoryId.length > 0
                            ? Translation.tr("Add reminder to %1").arg(RemindersService.categoryName(root.categoryId))
                            : Translation.tr("Add reminder")
                        elide: Text.ElideRight
                        font.pixelSize: Appearance.font.pixelSize.normal
                        color: Appearance.colors.colOnLayer1Inactive
                    }
                }

                ClockIconButton {
                    visible: quickField.text.trim().length > 0
                    symbol: "open_in_full"
                    size: 40
                    iconSize: ClockStyle.iconSmall + 4
                    tooltip: Translation.tr("More details")
                    onClicked: {
                        root.openEditor("", root.draftForList({ title: quickField.text.trim() }));
                        quickField.text = "";
                    }
                }

                // Dictation types into the focused field, so focus it first. The daemon
                // owns the state; this reads it from the bus topic.
                ClockIconButton {
                    visible: dictationTopic.data ? dictationTopic.data.enabled === true : false
                    symbol: dictationTopic.data && dictationTopic.data.state === "recording"
                        ? "graphic_eq" : "mic"
                    size: 40
                    toggled: dictationTopic.data
                        ? dictationTopic.data.state === "recording"
                          || dictationTopic.data.state === "transcribing"
                        : false
                    tooltip: dictationTopic.data && dictationTopic.data.state === "recording"
                        ? Translation.tr("Stop dictating") : Translation.tr("Dictate")
                    onClicked: {
                        quickField.forceActiveFocus();
                        Bus.call("dictation.toggle");
                    }
                }
            }
        }
    }

    // ── Pieces ──────────────────────────────────────────────────────────
    /// A Flow whose items keep their settled size while it re-deals them: a sheet opening
    /// or the rail folding moves tiles to their new places instead of stretching them
    /// frame by frame (guide §5.3, §8.2).
    component RedealFlow: Flow {
        move: Transition {
            enabled: !ClockStyle.reducedMotion
            NumberAnimation {
                properties: "x,y"
                duration: ClockStyle.motionDefault.duration
                easing.type: Easing.BezierSpline
                easing.bezierCurve: ClockStyle.motionDefault.bezierCurve
            }
        }
    }

    component SectionHeader: RowLayout {
        id: header
        property string title: ""
        property int count: -1
        property bool alert: false
        property bool collapsible: false
        property bool collapsed: false
        default property alias trailing: trailingRow.data

        signal toggled()

        Layout.fillWidth: true
        Layout.topMargin: 2
        spacing: ClockStyle.gapSmall

        StyledText {
            text: header.title
            font.pixelSize: ClockStyle.textNormal + 1
            font.weight: Font.Bold
            color: header.alert ? ClockStyle.colError : ClockStyle.colOnSurfaceVariant
        }

        StyledText {
            visible: header.count >= 0
            text: String(header.count)
            font.pixelSize: ClockStyle.textSmall
            color: ClockStyle.colSubtext
        }

        Item {
            Layout.fillWidth: true
        }

        RowLayout {
            id: trailingRow
            spacing: 2
        }

        ClockIconButton {
            visible: header.collapsible
            symbol: "expand_less"
            size: 32
            iconSize: ClockStyle.iconSmall + 4
            // A chevron turning to say which way it goes, as ClockFormPicker's does: the
            // rotation carries meaning, it isn't decoration (guide §4).
            rotation: header.collapsed ? 180 : 0
            tooltip: header.collapsed ? Translation.tr("Expand") : Translation.tr("Collapse")
            onClicked: header.toggled()

            Behavior on rotation {
                enabled: !ClockStyle.reducedMotion
                animation: ClockStyle.motionSpatial.numberAnimation.createObject(this)
            }
        }
    }

    /// One of the five list cards: icon, name and how many. Collapsed, they shrink to chips.
    /// The badge morphs into its partner shape under the pointer; an empty list's count
    /// thins out to the inactive digit weight, gliding there rather than swapping.
    component SmartTile: RippleButton {
        id: tile
        property var list
        property int count: 0
        property bool collapsed: false
        readonly property var colors: RemindersStyle.smartColors(tile.list.id)

        property real boldness: tile.count > 0 ? 1 : 0
        Behavior on boldness {
            enabled: !ClockStyle.reducedMotion
            animation: ClockStyle.motionDefault.numberAnimation.createObject(this)
        }

        implicitHeight: root.tileHeight
        buttonRadius: tile.collapsed ? ClockStyle.pill(tile.height) : ClockStyle.radiusCard
        buttonRadiusPressed: ClockStyle.radiusNormal
        colBackground: ClockStyle.colIdleCard
        colBackgroundHover: ClockStyle.colIdleCardHover
        colRipple: ClockStyle.colSurfaceActive
        onClicked: root.openList(tile.list.id)

        Behavior on implicitHeight {
            enabled: !ClockStyle.reducedMotion
            animation: ClockStyle.motionSpatial.numberAnimation.createObject(this)
        }
        Behavior on width {
            enabled: !ClockStyle.reducedMotion
            animation: ClockStyle.motionDefault.numberAnimation.createObject(this)
        }

        StyledToolTip {
            text: tile.list.label
            extraVisibleCondition: tileLabel.truncated
        }

        contentItem: Item {
            MaterialShapeWrappedMaterialSymbol {
                id: tileIcon
                anchors.left: parent.left
                anchors.leftMargin: tile.collapsed ? 8 : 16
                anchors.top: tile.collapsed ? undefined : parent.top
                anchors.topMargin: 14
                anchors.verticalCenter: tile.collapsed ? parent.verticalCenter : undefined
                text: tile.list.icon
                iconSize: tile.collapsed ? 16 : 20
                padding: tile.collapsed ? 7 : 10
                shape: RemindersStyle.shapeFor(tile.list.shapeKind, tile.hovered)
                color: tile.colors[0]
                colSymbol: tile.colors[1]
                fill: 1
            }

            StyledText {
                anchors.right: parent.right
                anchors.rightMargin: tile.collapsed ? 16 : 18
                anchors.top: tile.collapsed ? undefined : parent.top
                anchors.topMargin: 8
                anchors.verticalCenter: tile.collapsed ? parent.verticalCenter : undefined
                text: String(tile.count)
                font.family: ClockStyle.fontMain
                font.variableAxes: RemindersStyle.digitAxes(tile.boldness)
                font.pixelSize: tile.collapsed ? ClockStyle.textLarge : root.tileDigitSize
                color: tile.count > 0 ? ClockStyle.colOnSurface : ClockStyle.colSubtext
            }

            StyledText {
                id: tileLabel
                anchors.left: tile.collapsed ? tileIcon.right : parent.left
                anchors.leftMargin: tile.collapsed ? 8 : 18
                anchors.right: parent.right
                anchors.rightMargin: tile.collapsed ? 44 : 12
                anchors.bottom: tile.collapsed ? undefined : parent.bottom
                anchors.bottomMargin: 14
                anchors.verticalCenter: tile.collapsed ? parent.verticalCenter : undefined
                text: tile.list.label
                elide: Text.ElideRight
                font.pixelSize: tile.collapsed ? ClockStyle.textNormal : Appearance.font.pixelSize.normal
                font.weight: Font.DemiBold
                color: ClockStyle.colOnSurface
            }
        }
    }

    component CategoryRow: RippleButton {
        id: row
        property var category
        readonly property color colAccent: RemindersStyle.categoryColor(row.category.id)

        implicitHeight: 60
        buttonRadius: ClockStyle.radiusLarge
        buttonRadiusPressed: ClockStyle.radiusNormal
        colBackground: ClockStyle.colIdleCard
        colBackgroundHover: ClockStyle.colIdleCardHover
        colRipple: ClockStyle.colSurfaceActive
        onClicked: root.openList("cat:" + row.category.id)

        Behavior on width {
            enabled: !ClockStyle.reducedMotion
            animation: ClockStyle.motionDefault.numberAnimation.createObject(this)
        }

        StyledToolTip {
            text: categoryName.text
            extraVisibleCondition: categoryName.truncated
        }

        contentItem: RowLayout {
            spacing: 12

            Rectangle {
                Layout.leftMargin: 10
                implicitWidth: 38
                implicitHeight: 38
                radius: 19
                color: row.colAccent

                MaterialSymbol {
                    anchors.centerIn: parent
                    text: row.category.icon
                    iconSize: 19
                    fill: 1
                    color: RemindersStyle.onColor(row.colAccent)
                }
            }

            StyledText {
                id: categoryName
                Layout.fillWidth: true
                text: RemindersService.categoryName(row.category.id)
                elide: Text.ElideRight
                font.pixelSize: Appearance.font.pixelSize.normal
                font.weight: Font.DemiBold
                color: ClockStyle.colOnSurface
            }

            MaterialSymbol {
                visible: row.category.pinned
                text: "keep"
                fill: 1
                iconSize: ClockStyle.iconSmall
                color: ClockStyle.colOnSurfaceVariant
            }

            MaterialSymbol {
                visible: row.category.remote?.todo?.listId ? true : false
                text: "cloud_done"
                iconSize: ClockStyle.iconSmall
                color: ClockStyle.colOnSurfaceVariant
            }

            StyledText {
                Layout.rightMargin: 16
                text: String(root.categoryCounts[row.category.id] ?? 0)
                font.family: ClockStyle.fontMain
                font.variableAxes: ClockStyle.axesDigitsBold
                font.pixelSize: ClockStyle.textLarge
                color: ClockStyle.colOnSurfaceVariant
            }
        }
    }

    /// A "Try these out" suggestion. Its own badge shape (no list tile wears it) morphs
    /// under the pointer, and its title takes the rounded title face, a voice apart from
    /// the category rows above it.
    component TemplateCard: RippleButton {
        id: card
        property var template

        implicitWidth: 220
        implicitHeight: 72
        buttonRadius: ClockStyle.radiusLarge
        buttonRadiusPressed: ClockStyle.radiusNormal
        colBackground: ClockStyle.colSurfaceHigh
        colBackgroundHover: ClockStyle.colSurfaceHover
        colRipple: ClockStyle.colSurfaceActive
        onClicked: root.useTemplate(card.template.id)

        Behavior on width {
            enabled: !ClockStyle.reducedMotion
            animation: ClockStyle.motionDefault.numberAnimation.createObject(this)
        }

        StyledToolTip {
            text: card.template.title + "\n" + card.template.subtitle
            extraVisibleCondition: templateTitle.truncated || templateSubtitle.truncated
        }

        contentItem: RowLayout {
            spacing: 10

            MaterialShapeWrappedMaterialSymbol {
                Layout.leftMargin: 10
                text: card.template.icon
                iconSize: 18
                padding: 9
                shape: RemindersStyle.shapeFor(MaterialShape.Shape.Clover8Leaf, card.hovered)
                color: ClockStyle.colTertiaryContainer
                colSymbol: ClockStyle.colOnTertiaryContainer
            }

            ColumnLayout {
                Layout.fillWidth: true
                Layout.rightMargin: 10
                spacing: 0

                StyledText {
                    id: templateTitle
                    Layout.fillWidth: true
                    text: card.template.title
                    elide: Text.ElideRight
                    font.family: ClockStyle.fontTitle
                    font.variableAxes: ClockStyle.axesTitle
                    font.pixelSize: ClockStyle.textNormal + 2
                    color: ClockStyle.colOnSurface
                }

                StyledText {
                    id: templateSubtitle
                    Layout.fillWidth: true
                    text: card.template.subtitle
                    elide: Text.ElideRight
                    font.pixelSize: ClockStyle.textSmall
                    color: ClockStyle.colSubtext
                }
            }
        }
    }
}
