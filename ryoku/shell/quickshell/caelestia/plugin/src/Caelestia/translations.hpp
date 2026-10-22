#pragma once

#include <qobject.h>
#include <qqmlintegration.h>
#include <qtranslator.h>
#include <qurl.h>
#include <qvariant.h>

namespace caelestia {

// Loads Qt translation catalogues (caelestia_<code>.qm) for the shell and
// retranslates the QML engine when the language changes, so switching
// languages takes effect without restarting the shell.
class Translations : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(QString language READ language WRITE setLanguage NOTIFY languageChanged)
    Q_PROPERTY(QString activeLanguage READ activeLanguage NOTIFY activeLanguageChanged)
    Q_PROPERTY(
        QStringList extraSearchPaths READ extraSearchPaths WRITE setExtraSearchPaths NOTIFY extraSearchPathsChanged)
    Q_PROPERTY(QVariantList available READ available NOTIFY availableChanged)

public:
    explicit Translations(QObject* parent = nullptr);

    [[nodiscard]] QString language() const;
    void setLanguage(const QString& language);

    [[nodiscard]] QString activeLanguage() const;

    [[nodiscard]] QStringList extraSearchPaths() const;
    void setExtraSearchPaths(const QStringList& paths);

    [[nodiscard]] QVariantList available() const;

    Q_INVOKABLE static QString nativeNameFor(const QString& code);
    Q_INVOKABLE static QString nameFor(const QString& code);

signals:
    void languageChanged();
    void activeLanguageChanged();
    void extraSearchPathsChanged();
    void availableChanged();

private:
    [[nodiscard]] QStringList searchPaths() const;
    [[nodiscard]] QStringList candidates() const;
    void reload();
    void setActiveLanguage(const QString& code);
    void refreshAvailable();

    QString m_language;
    QString m_activeLanguage;
    QStringList m_extraSearchPaths;
    QVariantList m_available;
    QTranslator* m_translator;
};

} // namespace caelestia
