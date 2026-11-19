pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons

QtObject {
    id: languages

    required property var store
    property var catalogue: []
    property var currentPack: null
    property string currentLanguageId: ""
    property string requestedLanguageId: ""
    property bool ready: false
    property bool loading: false
    property string errorText: ""
    property var cache: ({})
    readonly property string assetsPath: String(Qt.resolvedUrl("assets")).replace(/^file:\/\//, "")
    readonly property string fallbackLanguageId: "english_1k"
    readonly property string currentBaseLanguageId: currentLanguageId.replace(/_extended$/, "")

    function languageFor(id) {
        return languages.catalogue.find(entry => entry.id === id) ?? null;
    }

    function resolveId(baseId) {
        if (languages.store.wordlistSize === "extended") {
            const extended = String(baseId) + "_extended";
            if (languageFor(extended))
                return extended;
        }
        return String(baseId);
    }

    function request(baseId) {
        const resolved = resolveId(baseId || fallbackLanguageId);
        languages.requestedLanguageId = languageFor(resolved) ? resolved : fallbackLanguageId;
        if (!languages.ready) {
            languages.loading = true;
            manifest.reload();
            return;
        }
        loadRequested();
    }

    function loadRequested() {
        const entry = languageFor(requestedLanguageId) ?? languageFor(fallbackLanguageId);
        if (!entry) {
            loading = false;
            errorText = I18n.tr("No bundled typing language is available.");
            return;
        }
        const cached = cache[entry.id];
        if (cached) {
            currentPack = cached;
            currentLanguageId = entry.id;
            loading = false;
            errorText = "";
            return;
        }
        loading = true;
        errorText = "";
        pack.path = assetsPath + "/" + entry.file;
        pack.reload();
    }

    property FileView manifest: FileView {
        path: languages.assetsPath + "/languages-manifest.json"
        blockLoading: true
        onLoaded: {
            try {
                const parsed = JSON.parse(text());
                const entries = Array.from(parsed.languages || []).filter(entry =>
                    typeof entry.id === "string" && typeof entry.file === "string");
                if (entries.length === 0)
                    throw new Error("empty manifest");
                languages.catalogue = entries;
                languages.ready = true;
                languages.loadRequested();
            } catch (error) {
                languages.loading = false;
                languages.errorText = I18n.tr("Typing language catalogue could not be read.");
            }
        }
        onLoadFailed: {
            languages.loading = false;
            languages.errorText = I18n.tr("Typing language catalogue is unavailable.");
        }
    }

    property FileView pack: FileView {
        blockLoading: true
        onLoaded: {
            try {
                const parsed = JSON.parse(text());
                const words = Array.from(parsed.words || []).filter(word =>
                    typeof word === "string" && word.trim().length > 0);
                if (words.length === 0)
                    throw new Error("empty word list");
                const entry = languages.languageFor(languages.requestedLanguageId)
                    ?? languages.languageFor(languages.fallbackLanguageId);
                const loaded = {
                    id: entry.id,
                    name: String(parsed.name || entry.label || entry.id),
                    bcp47: String(parsed.bcp47 || entry.bcp47 || ""),
                    rightToLeft: Boolean(parsed.rightToLeft || entry.rightToLeft),
                    joiningScript: Boolean(parsed.joiningScript || entry.joiningScript),
                    preferredFont: String(parsed.preferredFont || ""),
                    words: words
                };
                const next = Object.assign({}, languages.cache);
                next[loaded.id] = loaded;
                languages.cache = next;
                languages.currentPack = loaded;
                languages.currentLanguageId = loaded.id;
                languages.loading = false;
                languages.errorText = "";
            } catch (error) {
                languages.loading = false;
                languages.errorText = I18n.tr("Typing language pack could not be read.");
            }
        }
        onLoadFailed: {
            if (languages.requestedLanguageId !== languages.fallbackLanguageId
                    && languages.languageFor(languages.fallbackLanguageId)) {
                languages.requestedLanguageId = languages.fallbackLanguageId;
                languages.errorText = I18n.tr("Language pack unavailable; using English.");
                languages.loadRequested();
            } else {
                languages.loading = false;
                languages.errorText = I18n.tr("Typing language pack is unavailable.");
            }
        }
    }

    property Connections settingsWatch: Connections {
        target: languages.store
        function onWordlistSizeChanged() {
            languages.request(languages.currentBaseLanguageId || languages.store.language);
        }
        function onLanguageChanged() {
            languages.request(languages.store.language);
        }
    }

    Component.onCompleted: request(store.language)
}
