#include "hubsearch.hpp"

#include <QQmlContext>
#include <QQmlEngine>
#include <QTest>

#include <memory>

class TestHubSearch : public QObject
{
    Q_OBJECT

private:
    std::unique_ptr<QQmlEngine> m_engine;
    std::unique_ptr<HubSearchIndex> m_search;

    QStringList resultIds(const QString &query, int limit = 60) const
    {
        const QJSValue result = m_search->query(query, limit);
        QStringList ids;
        const quint32 length = result.property(QStringLiteral("length")).toUInt();
        ids.reserve(length);
        for (quint32 i = 0; i < length; ++i)
            ids.append(result.property(i).property(QStringLiteral("id")).toString());
        return ids;
    }

private slots:
    void init()
    {
        m_engine = std::make_unique<QQmlEngine>();
        m_search = std::make_unique<HubSearchIndex>();
        QQmlEngine::setContextForObject(m_search.get(), m_engine->rootContext());

        const QJSValue entries = m_engine->evaluate(QStringLiteral(R"JS([
            {id: "opacity-page", section: "appearance", sectionName: "Appearance", group: "", tab: "", label: "Opacity", desc: "", kw: "", key: "", isPage: true},
            {id: "opacity-setting", section: "appearance", sectionName: "Appearance", group: "", tab: "", label: "Opacity strength", desc: "", kw: "", key: "opacity", isPage: false},
            {id: "window-opacity", section: "windows", sectionName: "Windows", group: "", tab: "", label: "Window opacity", desc: "", kw: "", key: "windowOpacity", isPage: false},
            {id: "window-spacing", section: "windows", sectionName: "Windows", group: "", tab: "", label: "Window spacing", desc: "", kw: "", key: "windowSpacing", isPage: false},
            {id: "network-a", section: "connections", sectionName: "Connections", group: "", tab: "", label: "Network", desc: "", kw: "", key: "networkA", isPage: false},
            {id: "network-b", section: "connections", sectionName: "Connections", group: "", tab: "", label: "Network", desc: "", kw: "", key: "networkB", isPage: false}
        ])JS"));
        QVERIFY2(!entries.isError(), qPrintable(entries.toString()));
        m_search->setEntries(entries);

        const QJSValue synonyms = m_engine->evaluate(
            QStringLiteral("({transparency: 'opacity', wallpaper: 'appearance background'})"));
        QVERIFY2(!synonyms.isError(), qPrintable(synonyms.toString()));
        m_search->setSynonyms(synonyms);
    }

    void cleanup()
    {
        m_search.reset();
        m_engine.reset();
    }

    void exactAndPrefix()
    {
        // These orders were hand-run through Hub.qml's original JavaScript scorer.
        const QStringList expected{"opacity-page", "opacity-setting", "window-opacity"};
        QCOMPARE(resultIds(QStringLiteral("opacity")), expected);
        QCOMPARE(resultIds(QStringLiteral("opac")), expected);
    }

    void typo()
    {
        QCOMPARE(resultIds(QStringLiteral("opaciti")),
                 QStringList({"opacity-page", "opacity-setting", "window-opacity"}));
    }

    void synonym()
    {
        QCOMPARE(resultIds(QStringLiteral("transparency")),
                 QStringList({"opacity-page", "opacity-setting", "window-opacity"}));
    }

    void multiWordCoverage()
    {
        QCOMPARE(resultIds(QStringLiteral("window opacity")),
                 QStringList({"window-opacity", "window-spacing", "opacity-page", "opacity-setting"}));
    }

    void stableTies()
    {
        QCOMPARE(resultIds(QStringLiteral("network")), QStringList({"network-a", "network-b"}));
    }

    void qmlDenseArrayTieOrder()
    {
        const QJSValue entries = m_engine->evaluate(QStringLiteral(R"JS([
            {id: "general-page", section: "global", sectionName: "General", group: "", tab: "", label: "General", desc: "", kw: "language locale region formats format date time number currency text size font scale icons icon theme appearance location city weather timezone international i18n l10n users accounts people profile", key: "", isPage: true},
            {id: "desktop-page", section: "desktop", sectionName: "Desktop", group: "", tab: "", label: "Desktop", desc: "", kw: "desktop brand logo mark name wallpaper picker clipboard sidebar sidebars controls today panel corner", key: "", isPage: true},
            {id: "show-images", section: "desktop", sectionName: "Desktop", group: "APPEARANCE", tab: "Notifications", label: "Show images in notifications", desc: "Shows a sent image, or the app icon when no image is attached", kw: " ", key: "notifications.show_images", isPage: false},
            {id: "shell-shortcuts", section: "keybinds", sectionName: "Keybinds", group: "SHELL", tab: "Shortcuts", label: "Shell shortcuts", desc: "Launcher, Kit, lock, clipboard, screenshot, wallpaper and more", kw: " launcher cheatsheet lock clipboard screenshot wallpaper settings", key: "", isPage: false},
            {id: "local-frost", section: "launcher", sectionName: "App Launcher", group: "BACKGROUND", tab: "", label: "Local frost", desc: "Frosts the frozen desktop behind the result drawer", kw: " ", key: "bgBlur", isPage: false},
            {id: "show-as", section: "addons", sectionName: "Add-ons", group: "Placement", tab: "detail", label: "Show as", desc: "Where it appears: popout, wallpaper tile, bar glyph or sidebar card", kw: " ", key: "<pluginId>.host", isPage: false},
            {id: "wallpaper-60", section: "performance", sectionName: "Performance", group: "MOTION", tab: "", label: "60fps live wallpaper", desc: "Smoother video wallpaper, at a decode cost", kw: " ", key: "liveWallpaper60", isPage: false},
            {id: "pause-wallpaper", section: "performance", sectionName: "Performance", group: "MOTION", tab: "", label: "Pause video wallpaper", desc: "Stops the video behind a fullscreen window", kw: " ", key: "pauseLiveWallpaperWhenFullscreen", isPage: false},
            {id: "blur-behind", section: "global", sectionName: "General", group: "DESKTOP", tab: "Appearance", label: "Blur and dim behind windows", desc: "With an app open, the desktop around it blurs and darkens. Turn off to keep it clear.", kw: "frost haze darken dark dim blurry background wallpaper apps open ", key: "deckFrostEnabled", isPage: false},
            {id: "blur-strength", section: "global", sectionName: "General", group: "DESKTOP", tab: "Appearance", label: "Blur and dim strength", desc: "How strong the blur and darkening get.", kw: " ", key: "deckFrost", isPage: false}
        ])JS"));
        QVERIFY2(!entries.isError(), qPrintable(entries.toString()));
        m_search->setEntries(entries);

        // QV4's dense Array.sort moves equal scores this way in the original Hub.
        QCOMPARE(resultIds(QStringLiteral("wallpaper")),
                 QStringList({"desktop-page", "general-page", "pause-wallpaper", "wallpaper-60",
                              "blur-behind", "shell-shortcuts", "blur-strength", "show-as",
                              "show-images", "local-frost"}));
    }

    void limit()
    {
        QCOMPARE(resultIds(QStringLiteral("opacity"), 2), QStringList({"opacity-page", "opacity-setting"}));
        QVERIFY(resultIds(QStringLiteral("opacity"), 0).isEmpty());
    }
};

QTEST_GUILESS_MAIN(TestHubSearch)
#include "tst_hubsearch.moc"
