#include "hubsearch.hpp"

#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
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

    void setEntries(const QString &source)
    {
        const QJSValue entries = m_engine->evaluate(source);
        QVERIFY2(!entries.isError(), qPrintable(entries.toString()));
        m_search->setEntries(entries);
    }

    // entries the way Hub.qml builds them in a second language: the English label, then
    // the translation ("Blur Unschärfe"), from ryoku/i18n/catalog/{de,ja}.json
    void setGermanEntries()
    {
        setEntries(QStringLiteral(R"JS([
            {id: "shortcuts", section: "", sectionName: "Keyboard Tastatur", group: "", tab: "", label: "Keyboard shortcuts Tastaturkürzel", desc: "", kw: "", key: "", isPage: true},
            {id: "blur", section: "", sectionName: "Window style Fensterstil", group: "Blur Unschärfe", tab: "", label: "Blur Unschärfe", desc: "", kw: "", key: "", isPage: false},
            {id: "key-sounds", section: "", sectionName: "Keyboard Tastatur", group: "", tab: "", label: "Key sounds Schlüsselgeräusche", desc: "", kw: "", key: "", isPage: false},
            {id: "bundle", section: "", sectionName: "Apps", group: "", tab: "", label: "Bundle Bündel", desc: "", kw: "", key: "", isPage: false},
            {id: "bundel", section: "", sectionName: "Apps", group: "", tab: "", label: "Bundel", desc: "", kw: "", key: "", isPage: false},
            {id: "street", section: "", sectionName: "Weather Wetter", group: "", tab: "", label: "Street Straße", desc: "", kw: "", key: "", isPage: false},
            {id: "night", section: "", sectionName: "Displays Bildschirme", group: "", tab: "", label: "Night light Nachtlicht", desc: "", kw: "", key: "", isPage: false}
        ])JS"));
    }

    void setJapaneseEntries()
    {
        setEntries(QStringLiteral(R"JS([
            {id: "shortcuts", section: "", sectionName: "Keyboard キーボード", group: "", tab: "", label: "Keyboard shortcuts キーボードショートカット", desc: "", kw: "", key: "", isPage: true},
            {id: "keyboard", section: "", sectionName: "Devices デバイス", group: "", tab: "", label: "Keyboard キーボード", desc: "", kw: "", key: "", isPage: true},
            {id: "night", section: "", sectionName: "Displays ディスプレイ", group: "", tab: "", label: "Night light ナイトライト", desc: "", kw: "", key: "", isPage: false},
            {id: "blur", section: "", sectionName: "Window style ウィンドウスタイル", group: "", tab: "", label: "Blur ブラー", desc: "", kw: "", key: "", isPage: false},
            {id: "browser", section: "", sectionName: "Apps アプリ", group: "", tab: "", label: "Browser colour ブラウザのカラー", desc: "", kw: "", key: "", isPage: false},
            {id: "key-sounds", section: "", sectionName: "Keyboard キーボード", group: "", tab: "", label: "Key sounds キー音", desc: "", kw: "", key: "", isPage: false}
        ])JS"));
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
            {id: "blur-behind", section: "global", sectionName: "General", group: "DESKTOP", tab: "Appearance", label: "Blur and dim behind windows", desc: "With an app open, the desktop around it blurs and darkens. Turn off to keep it clear.", kw: "frost haze darken dark dim blurry background wallpaper apps open ", key: "deckFrostEnabled", isPage: false},
            {id: "blur-strength", section: "global", sectionName: "General", group: "DESKTOP", tab: "Appearance", label: "Blur and dim strength", desc: "How strong the blur and darkening get.", kw: " ", key: "deckFrost", isPage: false}
        ])JS"));
        QVERIFY2(!entries.isError(), qPrintable(entries.toString()));
        m_search->setEntries(entries);

        // QV4's dense Array.sort moves equal scores this way in the original Hub.
        QCOMPARE(resultIds(QStringLiteral("wallpaper")),
                 QStringList({"desktop-page", "general-page",
                              "blur-behind", "shell-shortcuts", "blur-strength", "show-as",
                              "show-images", "local-frost"}));
    }

    void limit()
    {
        QCOMPARE(resultIds(QStringLiteral("opacity"), 2), QStringList({"opacity-page", "opacity-setting"}));
        QVERIFY(resultIds(QStringLiteral("opacity"), 0).isEmpty());
    }

    // "Unschärfe" was indexed as "unsch" + "rfe", so its own word found nothing
    void germanWithUmlauts()
    {
        setGermanEntries();
        QCOMPARE(resultIds(QStringLiteral("Unschärfe")), QStringList({"blur"}));
        QCOMPARE(resultIds(QStringLiteral("UNSCHÄRFE")), QStringList({"blur"}));
        QCOMPARE(resultIds(QStringLiteral("Tastaturkürzel")), QStringList({"shortcuts"}));
        QCOMPARE(resultIds(QStringLiteral("Schlüsselgeräusche")), QStringList({"key-sounds"}));
        QCOMPARE(resultIds(QStringLiteral("schlüssel")), QStringList({"key-sounds"}));
        QCOMPARE(resultIds(QStringLiteral("Straße")), QStringList({"street"}));
        // full case folding: "ß" is "ss"
        QCOMPARE(resultIds(QStringLiteral("STRASSE")), QStringList({"street"}));
        // the exact spelling ranks above the same word without its accent
        QCOMPARE(resultIds(QStringLiteral("Bündel")), QStringList({"bundle", "bundel"}));
    }

    void germanWithoutUmlauts()
    {
        setGermanEntries();
        QCOMPARE(resultIds(QStringLiteral("unscharfe")), QStringList({"blur"}));
        QCOMPARE(resultIds(QStringLiteral("tastaturkurzel")), QStringList({"shortcuts"}));
        QCOMPARE(resultIds(QStringLiteral("schlussel")), QStringList({"key-sounds"}));
        QCOMPARE(resultIds(QStringLiteral("schlusselgerausche")), QStringList({"key-sounds"}));
        QCOMPARE(resultIds(QStringLiteral("bundel")), QStringList({"bundel", "bundle"}));
    }

    // Japanese text made no words at all
    void japaneseWholeWord()
    {
        setJapaneseEntries();
        QCOMPARE(resultIds(QStringLiteral("ナイトライト")), QStringList({"night"}));
        QCOMPARE(resultIds(QStringLiteral("ディスプレイ")), QStringList({"night"}));
        QCOMPARE(resultIds(QStringLiteral("キー音")), QStringList({"key-sounds"}));
        // a run's scattered letters are no match: ブ, ラ and ー are all in "ブラウザのカラー"
        QCOMPARE(resultIds(QStringLiteral("ブラー")), QStringList({"blur"}));
    }

    // without spaces a word is found inside the longer run that holds it
    void japaneseInsideCompound()
    {
        setJapaneseEntries();
        QCOMPARE(resultIds(QStringLiteral("ショートカット")), QStringList({"shortcuts"}));
        QCOMPARE(resultIds(QStringLiteral("カラー")), QStringList({"browser"}));
        const QStringList keyboard = resultIds(QStringLiteral("キーボード"));
        QCOMPARE(keyboard, QStringList({"shortcuts", "keyboard", "key-sounds"}));
        // half-width kana are the same letters
        QCOMPARE(resultIds(QStringLiteral("ｷｰﾎﾞｰﾄﾞ")), keyboard);
        // "・" separates words in a query as it does in the index
        QCOMPARE(resultIds(QStringLiteral("キーボード・ショートカット")).value(0), QStringLiteral("shortcuts"));
    }

    void mixedQuery()
    {
        setJapaneseEntries();
        QCOMPARE(resultIds(QStringLiteral("keyboard ショートカット")).value(0), QStringLiteral("shortcuts"));
        // both words count: the row that holds both ranks above a page that holds one
        QCOMPARE(resultIds(QStringLiteral("keyboard キー音")), QStringList({"shortcuts", "key-sounds", "keyboard"}));
        QCOMPARE(resultIds(QStringLiteral("night ディスプレイ")), QStringList({"night"}));
        setGermanEntries();
        QCOMPARE(resultIds(QStringLiteral("blur unscharfe")), QStringList({"blur"}));
        QCOMPARE(resultIds(QStringLiteral("night Bildschirme")), QStringList({"night"}));
    }

    // the real English index (hubsearch-en.json): every listed query keeps its top ten
    void englishUnchanged()
    {
        QFile file(QFINDTESTDATA("hubsearch-en.json"));
        QVERIFY2(file.open(QIODevice::ReadOnly), qPrintable(file.fileName()));
        const QString text = QString::fromUtf8(file.readAll());

        const QJSValue fixture = m_engine->evaluate(QStringLiteral("JSON.parse")).call({text});
        QVERIFY2(!fixture.isError(), qPrintable(fixture.toString()));
        const QJSValue entries = m_engine->evaluate(QStringLiteral(
            "(f => f.entries.map(r => { var o = {}; f.fields.forEach((k, i) => o[k] = r[i]); return o; }))")).call({fixture});
        QVERIFY2(entries.isArray(), qPrintable(entries.toString()));
        QVERIFY(entries.property(QStringLiteral("length")).toUInt() > 700);
        m_search->setEntries(entries);
        m_search->setSynonyms(fixture.property(QStringLiteral("synonyms")));

        const QJsonObject expected = QJsonDocument::fromJson(text.toUtf8()).object().value(QStringLiteral("expected")).toObject();
        QVERIFY(expected.size() > 300);
        for (auto it = expected.constBegin(); it != expected.constEnd(); ++it) {
            QStringList ids;
            for (const QJsonValue &id : it.value().toArray())
                ids.append(id.toString());
            const QStringList actual = resultIds(it.key(), 10);
            QVERIFY2(actual == ids,
                     qPrintable(QStringLiteral("%1: %2 (was %3)").arg(it.key(), actual.join(u' '), ids.join(u' '))));
        }
    }
};

QTEST_GUILESS_MAIN(TestHubSearch)
#include "tst_hubsearch.moc"
