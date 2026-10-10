import QtQuick
import Quickshell
import "ryostore" as Ryo
import "ryostore/Singletons" as RyoState

ShellRoot {
    id: root
    property string lastPreview: ""
    property string lastSelected: ""
    property int previewCount: 0
    property int selectionCount: 0
    property string installedKey: ""
    property string detailsKey: ""
    property string settingsKey: ""
    property string routeView: ""
    property string routeCategory: ""
    property bool searchOpened: false
    property bool searchClosed: false
    property string editedQuery: ""
    property string retryKey: ""
    property string activatedKey: ""
    property int closeCount: 0
    property var lastInstallComponents: null
    property bool sawVerifying: false
    property var probeDimensions: String(Quickshell.env("RYOSTORE_PROBE_SIZE") || "980x640").split("x")
    readonly property int probeWidth: Number(probeDimensions[0]) || 980
    readonly property int probeHeight: Number(probeDimensions[1]) || 640

    function require(condition, label) {
        if (!condition)
            throw new Error("RYOSTORE-COMPONENTS-PROBE-FAIL " + label);
    }

    function findObject(item, name) {
        if (!item)
            return null;
        if (item.objectName === name)
            return item;
        const children = item.children || [];
        for (const child of children) {
            const found = findObject(child, name);
            if (found)
                return found;
        }
        return null;
    }

    function inside(item, container) {
        if (!item)
            return false;
        const point = item.mapToItem(container, 0, 0);
        return point.x >= 0 && point.y >= 0
                && point.x + item.width <= container.width
                && point.y + item.height <= container.height;
    }

    Ryo.ProductCover {
        id: realCover
        width: 320
        height: 180
        item: ({
            id: "hero",
            category: "rices",
            categoryName: "Rices",
            name: "Hero",
            art: Qt.resolvedUrl("ryostore/logo.svg"),
            installed: false
        })
    }

    Ryo.ProductCover {
        id: missingCover
        objectName: "missing-cover"
        x: 340
        width: 320
        height: 180
        item: ({
            id: "plain",
            category: "rices",
            categoryName: "Rices",
            name: "Plain",
            art: "",
            accent: "#d75f5f",
            surface: "#101010",
            installed: false
        })
    }

    Ryo.StatusReadout {
        id: active
        objectName: "active-readout"
        item: ({ category: "rices", id: "active", active: true, installed: true })
    }

    Ryo.StatusReadout {
        id: partial
        objectName: "partial-readout"
        item: ({ category: "bundles", id: "pack", installedCount: 2, totalCount: 4 })
    }

    Ryo.StatusReadout {
        id: progress
        objectName: "progress-readout"
        item: ({ category: "lockscreens", id: "clock" })
        busyKey: "lockscreens:clock"
        installStage: "DOWNLOADING"
    }

    Ryo.StatusReadout {
        id: offline
        objectName: "offline-readout"
        item: ({ category: "plugins", id: "market" })
        offline: true
    }

    Ryo.StatusReadout {
        id: failed
        objectName: "failure-readout"
        item: ({ category: "barstyles", id: "broken" })
        installErrorKey: "barstyles:broken"
        installError: "fixture install failed"
    }

    FloatingWindow {
        title: "ryostore-components-probe"
        minimumSize: Qt.size(root.probeWidth, root.probeHeight)
        maximumSize: minimumSize
        color: "#080a0d"

        Ryo.ProductMedia {
            id: pausedGif
            objectName: "ryostore-probe-paused-gif"
            width: 64
            height: 64
            source: "file:///tmp/ryostore-probe.gif"
            active: true
            visible: false
        }
        Ryo.ProductMedia {
            id: webpMedia
            objectName: "ryostore-probe-animated-webp"
            width: 64
            height: 64
            source: "file:///tmp/ryostore-probe.webp"
            active: false
            visible: false
        }
        Ryo.ProductMedia {
            id: videoMedia
            objectName: "ryostore-probe-video"
            width: 64
            height: 64
            source: "file:///tmp/ryostore-probe.mp4"
            active: false
            visible: false
        }
        Ryo.ProductMedia {
            id: aspectGif
            width: 400
            height: 300
            source: Quickshell.env("RYOSTORE_ASPECT_GIF")
            mode: "view"
            active: false
            opacity: 0
        }
        Ryo.StoreHeader {
            id: header
            z: 10
            width: implicitWidth
            height: parent.height
            view: "discover"
            categoryID: ""
            categories: [
                { id: "rices", name: "Rices" },
                { id: "lockscreens", name: "Locks" },
                { id: "plugins", name: "Plugins" },
                { id: "barstyles", name: "Bar Styles" },
                { id: "fastfetch", name: "Fastfetch" },
                { id: "bundles", name: "Bundles" }
            ]
            query: ""
            libraryCount: 3
            updateCount: 1
            offline: false
            searchActive: false
            resultCount: 4
            onRouteRequested: (view, categoryID) => {
                root.routeView = view;
                root.routeCategory = categoryID;
            }
            onQueryEdited: value => root.editedQuery = value
            onSearchActivated: root.searchOpened = true
            onSearchEscaped: root.searchClosed = true
        }

        Ryo.ShowroomStage {
            id: stage
            objectName: "showroom-stage"
            width: parent.width
            height: parent.height - 240
            item: ({
                id: "a",
                category: "rices",
                categoryName: "Rices",
                name: "Committed A",
                summary: "Committed product copy remains attached to its actions.",
                art: "",
                accent: "#b23a48",
                surface: "#171113",
                installed: false
            })
            previewItem: ({
                id: "b",
                category: "rices",
                categoryName: "Rices",
                name: "Preview B",
                art: Qt.resolvedUrl("ryostore/logo.svg"),
                accent: "#d99b50",
                surface: "#18140f",
                installed: true
            })
            positionText: "02 / 04"
            onInstallRequested: item => root.installedKey = item.category + ":" + item.id
            onDetailsRequested: item => root.detailsKey = item.category + ":" + item.id
            onSettingsRequested: item => root.settingsKey = item.category + ":" + item.id
        }

        Ryo.ProductGrid {
            id: grid
            objectName: "product-grid"
            x: 0
            y: stage.height + 20
            width: parent.width
            height: 260
            selectedKey: "rices:b"
            items: [
                { id: "a", category: "rices", name: "A", art: "", accent: "#b23a48", surface: "#171113" },
                { id: "b", category: "rices", name: "B", art: "", accent: "#d99b50", surface: "#18140f" },
                { id: "c", category: "rices", name: "C", art: "", accent: "#4d8f72", surface: "#101815" },
                { id: "d", category: "rices", name: "D", art: "", accent: "#5876a8", surface: "#11141b" }
            ]
            onPreviewRequested: item => {
                root.previewCount++;
                root.lastPreview = item ? item.category + ":" + item.id : "";
            }
            onSelectionRequested: item => {
                root.selectionCount++;
                root.lastSelected = item ? item.category + ":" + item.id : "";
                grid.selectedKey = root.lastSelected;
            }
            onActivated: item => {
                root.activatedKey = item ? item.category + ":" + item.id : "";
            }
        }
        Ryo.ProductDetail {
            id: detail
            z: 20
            anchors.fill: parent
            open: true
            originRect: Qt.rect(60, 420, 220, 180)
            item: ({
                id: "broken",
                category: "lockscreens",
                categoryName: "Locks",
                name: "Broken Clock",
                description: "A lockscreen fixture with exact failure feedback.",
                art: "",
                screenshots: [Qt.resolvedUrl("ryostore/logo.svg"), Qt.resolvedUrl("ryostore/logo.svg")],
                compatibility: ["Hyprland 0.50+", "Ryoku"],
                contents: ["lockscreen QML", "wallpaper"],
                author: "Fixture Author",
                version: "1.2.3",
                size: "4 MiB",
                accent: "#d75f5f",
                surface: "#101010",
                installed: false
            })
            busyKey: ""
            installStage: "FAILED"
            installErrorKey: "lockscreens:broken"
            installError: "fixture install failed"
            reducedMotion: false
            onRetryRequested: item => {
                root.retryKey = item.category + ":" + item.id;
                RyoState.Store.retryInstall(item);
            }
            onCloseRequested: root.closeCount++
            onInstallRequested: item => root.installedKey = item.category + ":" + item.id
            onSettingsRequested: item => root.settingsKey = item.category + ":" + item.id
        }

        Ryo.ProductDetail {
            id: decorDetail
            objectName: "ryostore-decor-detail"
            z: 21
            anchors.fill: parent
            open: true
            item: ({
                id: "carceri",
                category: "decors",
                categoryName: "Decors",
                name: "Carceri",
                description: "A decor fixture with raw and dithered previews.",
                art: "img://dithered.webp",
                artRaw: "img://raw.webp",
                screenshots: [],
                accent: "#e8d8c9",
                surface: "#101010",
                installed: false
            })
            reducedMotion: true
        }

        Ryo.ProductDetail {
            id: bundleDetail
            objectName: "ryostore-bundle-detail"
            z: 22
            anchors.fill: parent
            open: true
            item: ({
                id: "kit",
                category: "bundles",
                categoryName: "Bundles",
                name: "Kit",
                description: "A bundle fixture with core and optional components.",
                art: "",
                screenshots: [],
                installedCount: 1,
                totalCount: 3,
                metadata: ({ items: [
                    ({ type: "package", name: "core-a", tier: "core", summary: "core a", installed: false }),
                    ({ type: "package", name: "core-b", tier: "core", summary: "core b", installed: true }),
                    ({ type: "package", name: "opt-a", tier: "optional", summary: "optional a", installed: false })
                ] })
            })
            reducedMotion: true
            onInstallRequested: (item, dither, components) => {
                root.installedKey = item.category + ":" + item.id;
                root.lastInstallComponents = components;
            }
        }

    }

    Timer {
        interval: 50
        running: true
        onTriggered: {
            root.require(realCover.hasArtwork === true, "real artwork path");
            root.require(missingCover.hasArtwork === false, "metadata cover path");
            root.require(missingCover.coverTitle === "Plain", "missing art retains identity");
            root.require(missingCover.Accessible.name.indexOf("Plain") !== -1, "cover has accessible identity");
            const missingFallback = root.findObject(missingCover, "ryostore-media-fallback");
            const missingFallbackText = root.findObject(missingCover, "ryostore-media-fallback-text");
            root.require(missingFallback && missingFallback.visible, "missing art shows identity fallback");
            root.require(missingFallbackText && missingFallbackText.text === "PL",
                         "identity fallback derives product initials");
            root.require(active.labels.indexOf("ACTIVE") !== -1, "active state explicit");
            root.require(partial.labels.indexOf("2 / 4 INSTALLED") !== -1, "partial state explicit");
            root.require(progress.labels.indexOf("DOWNLOADING") !== -1, "matching progress explicit");
            root.require(offline.labels.indexOf("OFFLINE") !== -1, "offline state explicit");
            root.require(failed.labels.indexOf("fixture install failed") !== -1, "exact failure preserved");
            root.require(pausedGif.animated && !pausedGif.shouldPlay && !pausedGif.playing,
                    "off-screen GIF is classified and paused");
            root.require(webpMedia.animated, "animated WebP uses the animated renderer");
            root.require(videoMedia.video, "video preview uses the muted media renderer");
            root.require(aspectGif.ready, "aspect fixture decoded");
            root.require(Math.abs(aspectGif.nativeLogicalWidth / aspectGif.nativeLogicalHeight - 2) < 0.01,
                         "animated media retains its decoded aspect "
                         + aspectGif.nativeLogicalWidth + "x" + aspectGif.nativeLogicalHeight);
            root.require(aspectGif.nativeLogicalWidth <= 4 && aspectGif.nativeLogicalHeight <= 2,
                         "fitted animated media does not upscale past native pixels");
            root.require(detail.open && detail.item.id === "broken", "failure keeps dossier open");
            root.require(detail.transitionMode === "shared", "standard motion uses shared transition");
            root.require(detail.errorText.indexOf("fixture install failed") !== -1, "exact failure shown");
            root.require(detail.screenshotCount === 2, "detail exposes real screenshots");
            const detailCover = root.findObject(detail, "ryostore-detail-cover");
            const detailDescription = root.findObject(detail, "ryostore-detail-description");
            const detailScreenshots = root.findObject(detail, "ryostore-detail-screenshots");
            const detailInstall = root.findObject(detail, "ryostore-detail-install");
            const detailRetry = root.findObject(detail, "ryostore-detail-retry");
            const detailSettings = root.findObject(detail, "ryostore-detail-settings");
            root.require(Math.abs(detailCover.x - detail.targetX) < 0.5
                    && Math.abs(detailCover.y - detail.targetY) < 0.5
                    && Math.abs(detailCover.width - detail.targetWidth) < 0.5
                    && Math.abs(detailCover.height - (detail.targetHeight - detail.filmstripHeight)) < 0.5,
                    "open shared cover reaches the media plate");
            root.require(detailDescription.text.indexOf("lockscreen fixture") !== -1,
                    "detail exposes description");
            root.require(detail.metadataText.indexOf("Fixture Author") !== -1
                    && detail.metadataText.indexOf("1.2.3") !== -1
                    && detail.metadataText.indexOf("4 MiB") !== -1
                    && detail.metadataText.indexOf("Hyprland 0.50+") !== -1
                    && detail.metadataText.indexOf("lockscreen QML") !== -1,
                    "detail exposes all product metadata");
            const decorCover = root.findObject(decorDetail, "ryostore-detail-cover");
            root.require(decorDetail.hasDither === true, "decor detail exposes dither variants");
            decorDetail.ditherOn = true;
            root.require(String(decorCover.artOverride) === "img://dithered.webp", "dither ON shows the dithered preview");
            decorDetail.ditherOn = false;
            root.require(String(decorCover.artOverride) === "", "dither OFF shows the raw preview");
            decorDetail.item = Object.assign({}, decorDetail.item, { installed: true });
            const decorSettings = root.findObject(decorDetail, "ryostore-detail-settings");
            root.require(!decorSettings.visible, "decor detail hides Settings without a settings page");
            root.require(detailScreenshots && detailScreenshots.visible && detailRetry.visible,
                    "multi-preview detail exposes its filmstrip and failure action");
            detail.openLightbox(0);
            const lightbox = root.findObject(detail, "ryostore-detail-lightbox");
            const zoom = root.findObject(detail, "ryostore-lightbox-zoom");
            root.require(detail.lightboxOpen && lightbox.visible && zoom.text === "100%",
                    "gallery opens its lightbox at fitted scale");
            zoom.Accessible.pressAction();
            root.require(detail.lightboxActualPixels && zoom.text === "FIT",
                    "lightbox exposes a native-pixel zoom");
            detail.stepLightbox(1);
            root.require(detail.lightboxIndex === 1, "lightbox arrows advance the gallery");
            detail.closeLightbox();
            root.installedKey = "";
            detailInstall.Accessible.pressAction();
            root.require(root.installedKey === "lockscreens:broken",
                    "accessible detail install targets selected item");
            root.installedKey = "";
            RyoState.Store.installErrorKey = "plugins:other";
            RyoState.Store.installError = "keep this error";
            RyoState.Store.installStage = "FAILED";
            RyoState.Store.clearInstallError(detail.item);
            root.require(RyoState.Store.installErrorKey === "plugins:other"
                    && RyoState.Store.installError === "keep this error"
                    && RyoState.Store.installStage === "FAILED",
                    "nonmatching clear preserves failure");
            detail.busyKey = "plugins:other";
            root.retryKey = "";
            detailRetry.Accessible.pressAction();
            root.require(root.retryKey === "", "busy state suppresses retry");
            detail.busyKey = "";
            RyoState.Store.busyKey = "";
            RyoState.Store.installErrorKey = "lockscreens:broken";
            RyoState.Store.installError = "fixture install failed";
            RyoState.Store.installStage = "FAILED";
            detailRetry.Accessible.pressAction();
            root.require(root.retryKey === "lockscreens:broken", "accessible retry targets selected item");
            root.require(RyoState.Store.installError === ""
                    && RyoState.Store.installErrorKey === "", "retry clears matching error");
            verificationWatcher.start();
            detail.item = Object.assign({}, detail.item, { installed: true, hasSettings: true });
            root.require(detailSettings.visible, "installed detail exposes Settings");
            root.settingsKey = "";
            detailSettings.Accessible.pressAction();
            root.require(root.settingsKey === "lockscreens:broken",
                    "accessible detail Settings targets selected item");
            root.settingsKey = "";
            detail.open = false;
            root.require(detail.visible && detail.transitionProgress > 0,
                    "shared close remains visible during reverse travel");
            const headerDiscover = root.findObject(header, "ryostore-header-discover");
            const headerCategories = root.findObject(header, "ryostore-header-categories");
            const headerSearch = root.findObject(header, "ryostore-header-search");
            const headerSearchField = root.findObject(header, "ryostore-header-search-field");
            const headerLibrary = root.findObject(header, "ryostore-header-library");
            const firstCategory = root.findObject(header, "ryostore-category-rices");
            const lastCategory = root.findObject(header, "ryostore-category-bundles");
            firstCategory.Accessible.pressAction();
            root.require(root.routeView === "discover" && root.routeCategory === "rices", "accessible category route");
            headerLibrary.Accessible.pressAction();
            root.require(root.routeView === "library" && root.routeCategory === "", "accessible library route");
            headerDiscover.Accessible.pressAction();
            root.require(root.routeView === "discover" && root.routeCategory === "", "accessible Discover route");
            root.searchOpened = false;
            headerSearchField.forceActiveFocus();
            root.require(headerSearchField.activeFocus, "search field takes focus");
            root.require(root.searchOpened, "focusing search activates search mode");
            root.editedQuery = "";
            headerSearchField.text = "rice";
            headerSearchField.textEdited();
            root.require(root.editedQuery === "rice", "search field edit delegates query");
            headerCategories.contentY = 0;
            const searchPt = headerSearch.mapToItem(header, 0, 0);
            const libPt = headerLibrary.mapToItem(header, 0, 0);
            const discPt = headerDiscover.mapToItem(header, 0, 0);
            const catPt = firstCategory.mapToItem(header, 0, 0);
            root.require(searchPt.y < discPt.y, "masthead search sits above catalogue navigation");
            root.require(discPt.y < libPt.y, "Library follows browse categories in the rail");
            root.require(Math.abs(discPt.x - catPt.x) < 1, "rail entries share one alignment");
            root.require(root.inside(headerSearch, header), "search stays visible at responsive size");
            root.require(root.inside(headerLibrary, header), "library stays visible at responsive size");
            root.require(headerCategories.width > 0, "categories retain scroll region");
            const focusOrder = [
                "ryostore-category-rices",
                "ryostore-category-lockscreens",
                "ryostore-category-plugins",
                "ryostore-category-barstyles",
                "ryostore-category-fastfetch",
                "ryostore-category-bundles"
            ];
            var focusItem = headerDiscover;
            for (const expectedName of focusOrder) {
                focusItem = focusItem.nextItemInFocusChain(true);
                root.require(focusItem.objectName === expectedName, "semantic tab order reaches " + expectedName);
            }
            headerCategories.contentY = 0;
            lastCategory.forceActiveFocus(Qt.TabFocusReason);
            root.require(lastCategory.activeFocus, "overflow category takes keyboard focus");
            root.require(lastCategory.y >= headerCategories.contentY
                    && lastCategory.y + lastCategory.height <= headerCategories.contentY + headerCategories.height,
                    "focused category scrolls into view");
            headerLibrary.forceActiveFocus(Qt.TabFocusReason);
            root.require(headerLibrary.y >= headerCategories.contentY
                    && headerLibrary.y + headerLibrary.height <= headerCategories.contentY + headerCategories.height,
                    "focused Library scrolls into view");
            const navigationRows = [
                headerDiscover,
                firstCategory,
                root.findObject(header, "ryostore-category-lockscreens"),
                root.findObject(header, "ryostore-category-plugins"),
                root.findObject(header, "ryostore-category-barstyles"),
                root.findObject(header, "ryostore-category-fastfetch"),
                lastCategory,
                headerLibrary
            ];
            var firstVisibleRow = null;
            for (const row of navigationRows) {
                if (row.y + row.height > headerCategories.contentY) {
                    firstVisibleRow = row;
                    break;
                }
            }
            root.require(firstVisibleRow && firstVisibleRow.y >= headerCategories.contentY - 1,
                         "rail scroll begins on a whole navigation row");
            root.require(headerLibrary.Accessible.name.indexOf("3") !== -1
                    && headerLibrary.Accessible.name.toLowerCase().indexOf("1 update ready") !== -1,
                    "library counts remain explicit");
            header.offline = true;
            root.require(headerSearchField.Accessible.name.toLowerCase().indexOf("offline") !== -1, "search field exposes offline state");
            const stageTitle = root.findObject(stage, "ryostore-stage-title");
            const stageStatus = root.findObject(stage, "ryostore-stage-status");
            const stagePrimary = root.findObject(stage, "ryostore-stage-primary");
            const stageArtwork = root.findObject(stage, "ryostore-stage-artwork");
            const stageScrim = root.findObject(stage, "ryostore-stage-scrim");
            const stagePosition = root.findObject(stage, "ryostore-stage-position");
            const stageDetails = root.findObject(stage, "ryostore-stage-details");
            const stageSettings = root.findObject(stage, "ryostore-stage-settings");
            root.require(stage.displayItem.id === "b", "preview owns stage artwork");
            root.require(stage.actionItem.id === "a", "preview cannot retarget action");
            root.require(stageArtwork.item.name === "Preview B", "preview owns cover title");
            root.require(stageArtwork.item.category === "rices", "cover category remains committed");
            root.require(stageArtwork.item.installed === false, "cover state remains committed");
            const previewItem = stage.previewItem;
            stage.previewItem = null;
            root.require(root.findObject(stageArtwork, "ryostore-cover-metadata") === null,
                         "stage cover keeps product copy in the story column");
            const committedItem = stage.item;
            stage.item = null;
            root.require(stageArtwork.Accessible.ignored,
                    "empty stage cover leaves the accessibility tree");
            root.require(!stageDetails.visible, "empty stage hides product actions");
            stage.item = committedItem;
            stage.previewItem = previewItem;
            stage.triggerInstall();
            root.require(root.installedKey === "rices:a", "install targets committed selection");
            root.installedKey = "";
            stagePrimary.Accessible.pressAction();
            root.require(root.installedKey === "rices:a", "accessible install targets committed selection");
            stageDetails.Accessible.pressAction();
            root.require(root.detailsKey === "rices:a", "details targets committed selection");
            stage.triggerSettings();
            root.require(root.settingsKey === "", "settings unavailable before install");
            root.installedKey = "";
            stage.busyKey = "plugins:other";
            stage.triggerInstall();
            root.require(root.installedKey === "", "global busy state suppresses install");
            stage.busyKey = "";
            stage.item = ({
                id: "a",
                category: "rices",
                categoryName: "Rices",
                name: "Committed A",
                summary: "Committed product copy remains attached to its actions.",
                art: "",
                accent: "#b23a48",
                surface: "#171113",
                installed: true,
                hasSettings: true
            });
            stage.triggerInstall();
            root.require(root.installedKey === "", "installed state suppresses install");
            stage.item = Object.assign({}, stage.item, { updateAvailable: true });
            root.require(stagePrimary.text === "UPDATE", "installed update is a first-class primary action");
            stage.triggerInstall();
            root.require(root.installedKey === "rices:a", "update uses the product transaction");
            root.installedKey = "";
            stageSettings.Accessible.pressAction();
            root.require(root.settingsKey === "rices:a", "settings targets committed selection");
            stage.busyKey = "rices:a";
            stage.installStage = "VERIFYING";
            stage.offline = true;
            stage.installErrorKey = "rices:a";
            stage.installError = "fixture install failed";
            root.require(stageStatus.labels.indexOf("VERIFYING") !== -1, "stage shows matching progress");
            root.require(stageStatus.labels.indexOf("OFFLINE") !== -1, "stage shows offline state");
            root.require(stageStatus.labels.indexOf("fixture install failed") !== -1, "stage preserves exact error");
            stage.previewItem = null;
            root.require(stage.displayItem.id === "a", "preview clears to committed selection");
            stage.reducedMotion = true;
            root.require(stage.motionDuration === 0, "reduced motion disables stage travel");
            root.require(stageArtwork.width === stage.width && stageArtwork.height === stage.height, "stage artwork is full bleed");
            root.require(stageScrim.height === stage.height, "stage scrim covers artwork height");
            root.require(root.inside(stagePosition, stage), "stage position remains visible");
            root.require(stageDetails.visible, "details action remains visible");
            root.require(stageSettings.visible, "settings action visible when installed");
            root.require(stagePrimary.Accessible.role === Accessible.Button, "primary action exposes button role");
            root.require(stagePrimary.Accessible.name !== "", "primary action exposes accessible name");
            root.require(root.inside(stageTitle, stage), "stage title remains visible at responsive size");
            root.require(root.inside(stageStatus, stage), "stage status remains visible at responsive size");
            root.require(root.inside(stagePrimary, stage), "stage primary action remains visible at responsive size");
            root.require(root.inside(stageDetails, stage), "stage details action remains visible at responsive size");
            root.require(root.inside(stageSettings, stage), "stage settings action remains visible at responsive size");
            grid.forceActiveFocus();
            root.require(grid.activeFocus, "grid takes keyboard focus");
            root.require(grid.positionFor("rices:c") === 2, "positionFor maps key to index");
            root.selectionCount = 0;
            root.lastSelected = "";
            grid.moveBy(-10);
            root.require(root.lastSelected === "rices:a", "boundary clamps to first item");
            grid.select(99);
            root.require(root.lastSelected === "rices:d", "select clamps to last item");
            grid.selectedKey = "rices:a";
            grid.moveBy(1);
            root.require(root.lastSelected === "rices:b", "moveBy advances the selection");
            root.require(root.selectionCount >= 3, "each move emits one selection");
            root.activatedKey = "";
            grid.activated(grid.items[2]);
            root.require(root.activatedKey === "rices:c", "activation targets the chosen item");
            grid.selectedKey = "rices:b";
            const cell = grid.cellRectFor("rices:b");
            root.require(cell.width > 0 && cell.height > 0, "cellRectFor returns the selected tile bounds");
            grid.restoreOffset(0);
            root.require(grid.contentY === 0, "restoreOffset sets the scroll position");
            root.require(bundleDetail.isBundle === true, "bundle detail flagged as bundle");
            root.require(bundleDetail.components.length === 3, "bundle detail exposes components");
            const bundleList = root.findObject(bundleDetail, "ryostore-detail-components");
            root.require(bundleList && bundleList.visible, "bundle detail shows component list");
            root.require(bundleDetail.selectedNames.indexOf("core-a") !== -1
                    && bundleDetail.selectedNames.indexOf("core-b") !== -1, "core components default selected");
            root.require(bundleDetail.selectedNames.indexOf("opt-a") === -1, "optional components default unselected");
            const bundleInstallSelected = root.findObject(bundleDetail, "ryostore-detail-install-selected");
            const bundleInstallAll = root.findObject(bundleDetail, "ryostore-detail-install-all");
            const bundlePlainInstall = root.findObject(bundleDetail, "ryostore-detail-install");
            root.require(bundleInstallSelected.visible && bundleInstallAll.visible, "bundle exposes select and all install actions");
            root.require(!bundlePlainInstall.visible, "bundle hides the single install action");
            bundleDetail.toggleSel("opt-a");
            root.require(bundleDetail.selectedNames.indexOf("opt-a") !== -1, "toggling an optional component selects it");
            bundleDetail.toggleSel("opt-a");
            root.require(bundleDetail.selectedNames.indexOf("opt-a") === -1, "toggling again deselects it");
            root.installedKey = "";
            root.lastInstallComponents = null;
            bundleInstallSelected.Accessible.pressAction();
            root.require(root.installedKey === "bundles:kit", "install selected targets the bundle");
            root.require(root.lastInstallComponents.indexOf("core-a") !== -1
                    && root.lastInstallComponents.indexOf("opt-a") === -1,
                    "install selected passes only the selected components");
            root.installedKey = "";
            root.lastInstallComponents = null;
            bundleInstallAll.Accessible.pressAction();
            root.require(root.lastInstallComponents.length === 3
                    && root.lastInstallComponents.indexOf("opt-a") !== -1,
                    "install all passes every component");
            finishTimer.start();
        }

    }

    Timer {
        id: verificationWatcher
        interval: 10
        repeat: true
        onTriggered: {
            if (RyoState.Store.installStage === "VERIFYING"
                    && RyoState.Store.busyKey === "lockscreens:broken")
                root.sawVerifying = true;
            if (root.sawVerifying && RyoState.Store.busyKey === "")
                stop();
        }
    }

    Timer {
        id: finishTimer
        interval: 350
        onTriggered: {
            const detailCover = root.findObject(detail, "ryostore-detail-cover");
            root.require(root.sawVerifying, "retry remains busy through VERIFYING refresh");
            root.require(RyoState.Store.busyKey === ""
                    && RyoState.Store.installStage === "COMPLETE",
                    "successful refresh clears busy state after verification");
            root.require(Math.abs(detailCover.x - detail.originRect.x) < 0.5
                    && Math.abs(detailCover.y - detail.originRect.y) < 0.5
                    && Math.abs(detailCover.width - detail.originRect.width) < 0.5
                    && Math.abs(detailCover.height - detail.originRect.height) < 0.5,
                    "shared close reverses cover to origin");
            detail.reducedMotion = true;
            detail.open = true;
            root.require(detail.transitionMode === "immediate"
                    && detail.transitionProgress === 1,
                    "reduced motion removes shared-element travel");
            const detailClose = root.findObject(detail, "ryostore-detail-close");
            detailClose.Accessible.pressAction();
            root.require(root.closeCount === 1, "accessible Back delegates reversible close");
            root.require(detail.open && detail.item.id === "broken",
                    "close does not mutate detail state");
            console.log("RYOSTORE-COMPONENTS-PROBE-PASS");
            Qt.quit();
        }
    }
}
