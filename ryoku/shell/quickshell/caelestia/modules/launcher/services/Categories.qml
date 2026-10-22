pragma Singleton

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.utils

QtObject {
    id: root

    readonly property var definitions: [
        { id: "favorites", name: qsTr("Favorites"), icon: "favorite" },
        { id: "all", name: qsTr("All Applications"), icon: "apps" },
        { id: "audioVideo", name: qsTr("Audio & Video"), icon: "movie", xdg: ["AudioVideo"] },
        { id: "audio", name: qsTr("Audio"), icon: "music_note", xdg: ["Audio"] },
        { id: "video", name: qsTr("Video"), icon: "videocam", xdg: ["Video"] },
        { id: "development", name: qsTr("Development"), icon: "code", xdg: ["Development"] },
        { id: "education", name: qsTr("Education"), icon: "school", xdg: ["Education"] },
        { id: "game", name: qsTr("Games"), icon: "sports_esports", xdg: ["Game"] },
        { id: "graphics", name: qsTr("Graphics"), icon: "palette", xdg: ["Graphics"] },
        { id: "network", name: qsTr("Network"), icon: "public", xdg: ["Network"] },
        { id: "office", name: qsTr("Office"), icon: "description", xdg: ["Office"] },
        { id: "science", name: qsTr("Science"), icon: "science", xdg: ["Science"] },
        { id: "settings", name: qsTr("Settings"), icon: "settings", xdg: ["Settings"] },
        { id: "system", name: qsTr("System"), icon: "computer", xdg: ["System"] },
        { id: "utility", name: qsTr("Utilities"), icon: "build", xdg: ["Utility"] },
        { id: "other", name: qsTr("Other"), icon: "category" }
    ]

    /// The curated category a desktop entry belongs to, or "other".
    function categoryForApp(app): string {
        let cats = [];
        const raw = app.categories;
        if (raw) {
            if (typeof raw === "string") {
                cats = raw.split(";").map(c => c.trim()).filter(c => c.length > 0);
            } else {
                for (let i = 0; i < raw.length; i++) {
                    const c = String(raw[i]).trim();
                    if (c.length > 0) cats.push(c);
                }
            }
        }
        for (const def of root.definitions) {
            if (!def.xdg) continue;
            for (const x of def.xdg)
                if (cats.includes(x)) return def.id;
        }
        return "other";
    }

    function visibleCategories(all: list<var>): list<var> {
        const res = [];
        for (const def of root.definitions) {
            if (def.id === "favorites" || def.id === "all") {
                res.push(def);
            } else if (all.some(a => root.categoryForApp(a) === def.id)) {
                res.push(def);
            }
        }
        return res;
    }

    function appsFor(categoryId: string, all: list<var>): list<var> {
        if (categoryId === "all")
            return all;
        if (categoryId === "favorites")
            return all.filter(a => Strings.testRegexList(GlobalConfig.launcher.favouriteApps, a.id));
        if (categoryId === "other")
            return all.filter(a => root.categoryForApp(a) === "other");
        return all.filter(a => root.categoryForApp(a) === categoryId);
    }
}
