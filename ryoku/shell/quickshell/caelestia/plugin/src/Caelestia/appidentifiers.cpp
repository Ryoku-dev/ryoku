#include <QCoreApplication>
#include <QString>
#include <QtGlobal>

namespace {

void setApplicationIdentifiers() {
    QCoreApplication::setOrganizationName(QStringLiteral("Caelestia"));
    QCoreApplication::setOrganizationDomain(QStringLiteral("caelestia.dots"));
    QCoreApplication::setApplicationName(QStringLiteral("caelestia-shell"));
}

} // namespace

// Q_CONSTRUCTOR_FUNCTION rather than a plain static: the compiler cannot drop it
// as unused, and the call is visible as what it is.
Q_CONSTRUCTOR_FUNCTION(setApplicationIdentifiers)
