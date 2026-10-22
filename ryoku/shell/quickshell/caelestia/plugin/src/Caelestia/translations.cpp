#include "translations.hpp"

#include <QtQml/qqmlengine.h>
#include <qcoreapplication.h>
#include <qdir.h>
#include <qlocale.h>
#include <qlogging.h>

#include <algorithm>
#include <utility>

namespace caelestia {

using Qt::StringLiterals::operator""_s;

namespace {

const QString& catalogPrefix() {
    static const QString prefix = u"caelestia_"_s;
    return prefix;
}

QString toLocalPath(const QString& path) {
    const QUrl url(path);
    if (url.isLocalFile()) {
        return url.toLocalFile();
    }
    return path;
}

// English is the source language, so it never needs a catalogue.
bool isSourceLanguage(const QString& code) {
    return code == u"en"_s || code.startsWith(u"en_"_s);
}

QString normaliseCode(const QString& code) {
    QString normalised = code;
    normalised.replace(u'-', u'_');
    return normalised;
}

} // namespace

Translations::Translations(QObject* parent)
    : QObject(parent)
    , m_language(u"system"_s)
    , m_activeLanguage(u"en"_s)
    , m_translator(nullptr) {
    reload();
}

QString Translations::language() const {
    return m_language;
}

void Translations::setLanguage(const QString& language) {
    const QString normalised = language.isEmpty() ? u"system"_s : normaliseCode(language);
    if (m_language == normalised) {
        return;
    }

    m_language = normalised;
    emit languageChanged();

    reload();
}

QString Translations::activeLanguage() const {
    return m_activeLanguage;
}

QStringList Translations::extraSearchPaths() const {
    return m_extraSearchPaths;
}

void Translations::setExtraSearchPaths(const QStringList& paths) {
    if (m_extraSearchPaths == paths) {
        return;
    }

    m_extraSearchPaths = paths;
    emit extraSearchPathsChanged();

    reload();
}

QVariantList Translations::available() const {
    return m_available;
}

QString Translations::nativeNameFor(const QString& code) {
    const QLocale locale(normaliseCode(code));
    const QString name = locale.nativeLanguageName();
    return name.isEmpty() ? code : name;
}

QString Translations::nameFor(const QString& code) {
    const QLocale locale(normaliseCode(code));
    const QString name = QLocale::languageToString(locale.language());
    return name.isEmpty() ? code : name;
}

QStringList Translations::searchPaths() const {
    QStringList paths;

    const QString env = qEnvironmentVariable("CAELESTIA_TRANSLATIONS_DIR");
    if (!env.isEmpty()) {
        for (const QString& path : env.split(u':', Qt::SkipEmptyParts)) {
            paths << toLocalPath(path);
        }
    }

    for (const QString& path : m_extraSearchPaths) {
        paths << toLocalPath(path);
    }

#ifdef CAELESTIA_TRANSLATIONS_DIR
    paths << QString::fromUtf8(CAELESTIA_TRANSLATIONS_DIR);
#endif

    paths.removeDuplicates();
    return paths;
}

QStringList Translations::candidates() const {
    QStringList codes;

    if (m_language == u"system"_s) {
        const QLocale system = QLocale::system();
        for (const QString& lang : system.uiLanguages()) {
            codes << normaliseCode(lang);
        }
        codes << system.name();
    } else {
        codes << m_language;
    }

    QStringList expanded;
    for (const QString& code : std::as_const(codes)) {
        expanded << code;
        const QString bare = code.section(u'_', 0, 0);
        if (bare != code) {
            expanded << bare;
        }
    }

    expanded.removeDuplicates();
    return expanded;
}

void Translations::reload() {
    refreshAvailable();

    const QStringList dirs = searchPaths();
    auto* translator = new QTranslator(this);

    QString loaded;
    for (const QString& code : candidates()) {
        if (isSourceLanguage(code)) {
            break;
        }

        for (const QString& dir : dirs) {
            if (translator->load(catalogPrefix() + code, dir)) {
                loaded = code;
                break;
            }
        }

        if (!loaded.isEmpty()) {
            break;
        }
    }

    if (loaded.isEmpty()) {
        delete translator;
        translator = nullptr;
    }

    if (m_translator) {
        QCoreApplication::removeTranslator(m_translator);
        m_translator->deleteLater();
        m_translator = nullptr;
    }

    if (translator) {
        m_translator = translator;
        QCoreApplication::installTranslator(m_translator);
    }

    setActiveLanguage(loaded.isEmpty() ? u"en"_s : loaded);

    if (auto* engine = qmlEngine(this)) {
        engine->retranslate();
    }
}

void Translations::setActiveLanguage(const QString& code) {
    if (m_activeLanguage == code) {
        return;
    }

    m_activeLanguage = code;
    emit activeLanguageChanged();
}

void Translations::refreshAvailable() {
    QStringList codes{ u"en"_s };

    for (const QString& path : searchPaths()) {
        const QDir dir(path);
        if (!dir.exists()) {
            continue;
        }

        for (const QString& file : dir.entryList({ catalogPrefix() + u"*.qm"_s }, QDir::Files)) {
            const QString code = file.mid(catalogPrefix().size(), file.size() - catalogPrefix().size() - 3);
            if (!code.isEmpty() && !isSourceLanguage(code) && !codes.contains(code)) {
                codes << code;
            }
        }
    }

    std::sort(codes.begin(), codes.end());

    QVariantList available;
    for (const QString& code : std::as_const(codes)) {
        available << QVariantMap{
            { u"code"_s, code },
            { u"name"_s, nameFor(code) },
            { u"nativeName"_s, nativeNameFor(code) },
        };
    }

    if (available != m_available) {
        m_available = available;
        emit availableChanged();
    }
}

} // namespace caelestia
