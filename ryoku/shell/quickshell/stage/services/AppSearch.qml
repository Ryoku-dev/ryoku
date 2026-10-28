pragma Singleton

import stage.modules.common
import stage.modules.common.functions
import QtQuick
import Quickshell

/**
 * - Eases fuzzy searching for applications by name
 * - Guesses icon name for window class name
 * - Supports frecency ranking and alias resolution
 */
Singleton {
    id: root
    property bool sloppySearch: Config.options?.search.sloppy ?? false
    property bool frecencySearch: Config.options?.search.frecency ?? false
    property real scoreThreshold: 0.2
    property var substitutions: ({
            "code-url-handler": "visual-studio-code",
            "Code": "visual-studio-code",
            "gnome-tweaks": "org.gnome.tweaks",
            "pavucontrol-qt": "pavucontrol",
            "wps": "wps-office2019-kprometheus",
            "wpsoffice": "wps-office2019-kprometheus",
            "footclient": "foot",
            "jetbrains-studio": "android-studio",
            "zen": "zen-browser"
        })
    property var regexSubstitutions: [
        {
            "regex": /^steam_app_(\d+)$/,
            "replace": "steam_icon_$1"
        },
        {
            "regex": /Minecraft.*/,
            "replace": "minecraft"
        },
        {
            "regex": /.*polkit.*/,
            "replace": "system-lock-screen"
        },
        {
            "regex": /gcr.prompter/,
            "replace": "system-lock-screen"
        }
    ]

    // Deduped list to fix double icons, pre-sorted alphabetically to avoid sorting on every query.
    // Deduped through a Set rather than findIndex per entry: this rebuilds on every
    // desktop entry rescan, and the quadratic version made each rescan a visible hitch.
    readonly property list<DesktopEntry> list: {
        const seen = new Set();
        const arr = [];
        for (const app of DesktopEntries.applications.values) {
            if (seen.has(app.id))
                continue;
            seen.add(app.id);
            arr.push(app);
        }
        arr.sort((a, b) => (a.name || "").localeCompare(b.name || ""));
        return arr;
    }
    function lookupDesktopEntry(appId, includeNoDisplay = false) {
        const id = String(appId ?? "").trim();
        if (!id)
            return null;
        const plain = id.replace(/\.desktop$/i, "");
        const exact = root.list.find(entry => entry.id === id || entry.id === plain);
        if (exact)
            return exact;
        const match = DesktopEntries.heuristicLookup(id);
        return match && (includeNoDisplay || !match.noDisplay) ? match : null;
    }

    readonly property var preppedNames: list.map(a => ({
                name: Fuzzy.prepare(`${a.name} `),
                entry: a
            }))

    readonly property var preppedIcons: list.map(a => ({
                name: Fuzzy.prepare(`${a.icon} `),
                entry: a
            }))

    /**
     * Frecency search: combines fuzzy matching with app launch frequency
     */
    function frecencyQuery(search: string): var {
        if (search === "") {
            // When empty, show all apps sorted by frecency then alphabetical
            const scored = list.map(obj => ({
                        entry: obj,
                        score: AppUsage.getScore(obj.id)
                    }));
            // Split: apps with score > 0 sorted by score desc, then rest alphabetically (since list is already sorted, filter preserves order)
            const used = scored.filter(item => item.score > 0).sort((a, b) => b.score - a.score);
            const unused = scored.filter(item => item.score === 0);
            return used.concat(unused).map(item => item.entry);
        }

        // Use Fuzzy.go to get matches with scores
        const fuzzyResults = root.trimFuzzyResults(Fuzzy.go(search, preppedNames, {
            all: false,
            threshold: root.fuzzyThreshold,
            key: "name"
        }));

        if (fuzzyResults.length === 0)
            return [];

        // Find max score for normalization
        let maxFuzzy = 0;
        for (let i = 0; i < fuzzyResults.length; i++) {
            if (fuzzyResults[i].score > maxFuzzy)
                maxFuzzy = fuzzyResults[i].score;
        }

        const results = fuzzyResults.map(r => {
            const entry = r.obj.entry;
            const fuzzyScore = r.score;
            const normalizedFuzzy = maxFuzzy > 0 ? fuzzyScore / maxFuzzy : 1;
            const usageScore = AppUsage.getScore(entry.id);
            // Normalize usage score (log scale to prevent single high-freq app dominating)
            const normalizedUsage = usageScore > 0 ? Math.min(1, Math.log(usageScore + 1) / Math.log(100)) : 0;

            // Boost score if the app name starts with the search string
            const startsWithQuery = entry.name.toLowerCase().startsWith(search.toLowerCase());
            const prefixBonus = startsWithQuery ? 1.0 : 0.0;

            return {
                entry: entry,
                combinedScore: normalizedFuzzy * 0.6 + normalizedUsage * 0.4 + prefixBonus,
                isAlias: false
            };
        });

        return results.sort((a, b) => b.combinedScore - a.combinedScore).map(item => item.entry);
    }

    readonly property real fuzzyThreshold: Config.options?.search.fuzzyThreshold ?? 0.35
    readonly property real fuzzyRelativeCutoff: Config.options?.search.fuzzyRelativeCutoff ?? 0.35

    /**
     * Drop the tail of a fuzzysort run.
     *
     * fuzzysort matches any subsequence, so without a floor a three-letter
     * query reaches half the menu. Two cuts, because one is not enough: an
     * absolute floor for queries where everything is weak, and a cut relative
     * to the best hit for queries where one result is obviously right and the
     * rest only share letters with it.
     */
    function trimFuzzyResults(results: var): var {
        if (results.length === 0)
            return results;
        const best = results[0].score;
        const floor = Math.max(root.fuzzyThreshold, best * root.fuzzyRelativeCutoff);
        const kept = [];
        for (let i = 0; i < results.length; i++) {
            if (results[i].score < floor)
                break;
            kept.push(results[i]);
        }
        return kept;
    }

    /**
     * Typo-tolerant matching over the whole app list (Myers bit-parallel edit
     * distance). Not a replacement for fuzzysort — a last resort for when the
     * exact passes found nothing, where returning a near miss beats returning
     * an empty list.
     */
    function typoQuery(search: string): var {
        if (!search)
            return [];
        const settings = Config.options?.search.typoTolerance;
        const prepared = BitwiseFuzzy.prepare(search);
        return BitwiseFuzzy.search(prepared, list, {
            key: "name",
            threshold: settings?.threshold ?? 0.30,
            // A typo is a few wrong keys, not most of the word: one in three at most.
            maxEdits: Math.max(1, Math.ceil(search.length / 3)),
            limit: Math.max(1, settings?.maxResults ?? 8)
        });
    }

    function fuzzyQuery(search) { // Idk why list<DesktopEntry> doesn't work
        if (search === "") {
            if (root.frecencySearch) {
                return frecencyQuery(search);
            }
            return list;
        }

        // Frecency mode: combine fuzzy with usage frequency
        if (root.frecencySearch) {
            return frecencyQuery(search);
        }

        // Sloppy mode: levenshtein distance
        if (root.sloppySearch) {
            const results = list.map(obj => ({
                        entry: obj,
                        score: Levendist.computeScore(obj.name.toLowerCase(), search.toLowerCase())
                    })).filter(item => item.score > root.scoreThreshold).sort((a, b) => b.score - a.score);
            return results.map(item => item.entry);
        }

        // Default: fuzzy sort
        return root.trimFuzzyResults(Fuzzy.go(search, preppedNames, {
            limit: 100,
            threshold: root.fuzzyThreshold,
            key: "name"
        })).map(r => {
            return r.obj.entry;
        });
    }

    // Quickshell.iconPath() walks the icon theme on disk and caches nothing, and
    // guessIcon calls it up to eight times for a single name it fails to resolve.
    property var _iconExistsCache: ({})

    function iconExists(iconName) {
        if (!iconName || iconName.length == 0)
            return false;
        const cached = _iconExistsCache[iconName];
        if (cached !== undefined)
            return cached;

        const result = (Quickshell.iconPath(iconName, true).length > 0) && !iconName.includes("image-missing");
        _iconExistsCache[iconName] = result;
        return result;
    }

    // A rescan can bring in entries whose icons were not on disk when a name was last
    // guessed, so both caches are dropped rather than left to answer from before.
    Connections {
        target: DesktopEntries
        function onApplicationsChanged() {
            root._iconExistsCache = ({});
            root._iconCache = ({});
        }
    }

    // A regenerated or switched icon theme adds and removes names just the same.
    Connections {
        target: TaskbarApps
        function onIconThemeRevisionChanged() {
            root._iconExistsCache = ({});
            root._iconCache = ({});
        }
    }

    function getReverseDomainNameAppName(str) {
        return str.split('.').slice(-1)[0];
    }

    function getKebabNormalizedAppName(str) {
        return str.toLowerCase().replace(/\s+/g, "-");
    }

    function getUndescoreToKebabAppName(str) {
        return str.toLowerCase().replace(/_/g, "-");
    }

    property var _iconCache: ({})

    /**
     * The icon to draw for a desktop entry. Themed icons can't recolor an absolute-path
     * icon in place, so recolor_icons.py injects a copy named after the .desktop file; use
     * it only while themed icons are on, so a custom icon stays custom otherwise.
     */
    function entryIcon(entry) {
        const icon = String(entry?.icon ?? "");
        if (!icon.startsWith("/") || !Config.options.appearance.icons.enableThemed)
            return icon;
        const id = String(entry.id ?? "").replace(/\.desktop$/, "");
        return iconExists(id) ? id : icon;
    }

    function guessIcon(str) {
        if (!str || str.length == 0)
            return "image-missing";
        if (_iconCache[str] !== undefined)
            return _iconCache[str];
        
        let result = _guessIconImpl(str);
        _iconCache[str] = result;
        return result;
    }

    function _guessIconImpl(str) {
        // Try common substitutions first
        if (substitutions[str])
            return substitutions[str];
        if (substitutions[str.toLowerCase()])
            return substitutions[str.toLowerCase()];

        // Handle common variations for user's requested apps
        if (str.includes("android-studio") && iconExists("android-studio"))
            return "android-studio";
        if (str.includes("zen")) {
            if (iconExists("zen-browser")) return "zen-browser";
            if (iconExists("zen")) return "zen";
        }
        if (str.includes("prism")) {
            if (iconExists("prismlauncher")) return "prismlauncher";
            if (iconExists("prism_launcher")) return "prism_launcher";
            if (iconExists("org.prismlauncher.PrismLauncher")) return "org.prismlauncher.PrismLauncher";
        }
        if (str.includes("helium") && iconExists("helium"))
            return "helium";
        if (str.includes("zed")) {
            if (iconExists("dev.zed.Zed")) return "dev.zed.Zed";
            if (iconExists("zed")) return "zed";
        }

        // Try to see if there's a themed icon matching the name (for absolute path icons)
        // This is important for apps like Zen Browser where the .desktop has an absolute path
        // but the theme script generates a themed icon with the desktop entry's ID
        let nameWithoutExt = str;
        if (str.endsWith(".desktop"))
            nameWithoutExt = str.slice(0, -8);
        if (iconExists(nameWithoutExt))
            return nameWithoutExt;

        const noUnderscores = nameWithoutExt.replace(/_/g, "");
        if (iconExists(noUnderscores))
            return noUnderscores;

        // Quickshell's desktop entry lookup
        const entry = DesktopEntries.byId(str);
        if (entry) {
            // Even if we have an entry, check if its ID (basename) has a themed version
            // because the entry.icon might be an absolute path
            const entryId = entry.id.endsWith(".desktop") ? entry.id.slice(0, -8) : entry.id;
            if (iconExists(entryId))
                return entryId;
            return entry.icon;
        }

        // Regex substitutions
        for (let i = 0; i < regexSubstitutions.length; i++) {
            const substitution = regexSubstitutions[i];
            const replacedName = str.replace(substitution.regex, substitution.replace);
            if (replacedName != str)
                return replacedName;
        }

        // Icon exists -> return as is
        if (iconExists(str))
            return str;

        // Simple guesses
        const lowercased = str.toLowerCase();
        if (iconExists(lowercased))
            return lowercased;

        const reverseDomainNameAppName = getReverseDomainNameAppName(str);
        if (iconExists(reverseDomainNameAppName))
            return reverseDomainNameAppName;

        const lowercasedDomainNameAppName = reverseDomainNameAppName.toLowerCase();
        if (iconExists(lowercasedDomainNameAppName))
            return lowercasedDomainNameAppName;

        const kebabNormalizedGuess = getKebabNormalizedAppName(str);
        if (iconExists(kebabNormalizedGuess))
            return kebabNormalizedGuess;

        const undescoreToKebabGuess = getUndescoreToKebabAppName(str);
        if (iconExists(undescoreToKebabGuess))
            return undescoreToKebabGuess;

        // Search in desktop entries
        const iconSearchResults = Fuzzy.go(str, preppedIcons, {
            limit: 10,
            key: "name"
        }).map(r => {
            return r.obj.entry;
        });
        if (iconSearchResults.length > 0) {
            const guess = iconSearchResults[0].icon;
            if (iconExists(guess))
                return guess;
        }

        const nameSearchResults = root.fuzzyQuery(str);
        if (nameSearchResults.length > 0) {
            const guess = nameSearchResults[0].icon;
            if (iconExists(guess))
                return guess;
        }

        // Quickshell's desktop entry lookup
        const heuristicEntry = DesktopEntries.heuristicLookup(str);
        if (heuristicEntry)
            return heuristicEntry.icon;

        // Give up
        return "application-x-executable";
    }
}
