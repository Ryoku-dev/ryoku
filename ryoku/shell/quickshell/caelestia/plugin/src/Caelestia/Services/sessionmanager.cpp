#include "sessionmanager.hpp"

#include <QtDBus/qdbusconnection.h>
#include <QtDBus/qdbuserror.h>
#include <QtDBus/qdbusmessage.h>
#include <QtDBus/qdbuspendingcall.h>
#include <QtDBus/qdbuspendingreply.h>
#include <QtDBus/qdbusreply.h>
#include <qloggingcategory.h>

#include "../toaster.hpp"

Q_LOGGING_CATEGORY(lcSessionManager, "caelestia.services.sessionmanager", QtInfoMsg)

namespace caelestia::services {

namespace {

// QString rather than const char* so that the DBus calls below do not need a
// conversion at every use site (the implicit one is disabled shell wide).
const QString LOGIN_SERVICE = QStringLiteral("org.freedesktop.login1");
const QString LOGIN_PATH = QStringLiteral("/org/freedesktop/login1");
const QString LOGIN_IFACE = QStringLiteral("org.freedesktop.login1.Manager");
const QString SESSION_IFACE = QStringLiteral("org.freedesktop.login1.Session");

} // namespace

SessionManager::SessionManager(QObject* parent)
    : QObject(parent) {
    auto bus = getSystemBus();
    if (!bus)
        return;

    bool ok = bus->connect(LOGIN_SERVICE, LOGIN_PATH, LOGIN_IFACE, QStringLiteral("PrepareForSleep"), this,
        SLOT(handlePrepareForSleep(bool)));
    if (!ok)
        qCWarning(lcSessionManager) << "Failed to connect to PrepareForSleep signal:" << bus->lastError().message();

    auto sessionMsg =
        QDBusMessage::createMethodCall(LOGIN_SERVICE, LOGIN_PATH, LOGIN_IFACE, QStringLiteral("GetSession"));
    sessionMsg.setArguments({ QStringLiteral("auto") });
    const QDBusReply<QDBusObjectPath> sessionReply = bus->call(sessionMsg);
    if (!sessionReply.isValid()) {
        qCWarning(lcSessionManager) << "Failed to get session path:" << sessionReply.error().message();
        return;
    }
    m_sessionPath = sessionReply.value().path();

    ok = bus->connect(
        LOGIN_SERVICE, m_sessionPath, SESSION_IFACE, QStringLiteral("Lock"), this, SLOT(handleLockRequested()));
    if (!ok)
        qCWarning(lcSessionManager) << "Failed to connect to Lock signal:" << bus->lastError().message();

    ok = bus->connect(
        LOGIN_SERVICE, m_sessionPath, SESSION_IFACE, QStringLiteral("Unlock"), this, SLOT(handleUnlockRequested()));
    if (!ok)
        qCWarning(lcSessionManager) << "Failed to connect to Unlock signal:" << bus->lastError().message();
}

bool SessionManager::exec(const QStringList& command) {
    if (command.isEmpty()) {
        return false;
    }

    using Qt::StringLiterals::operator""_s;
    static const QHash<QString, void (SessionManager::*)()> cmds = {
        { u"logout"_s, &SessionManager::logout },
        { u"suspend"_s, &SessionManager::suspend },
        { u"suspendthenhibernate"_s, &SessionManager::suspendThenHibernate },
        { u"hibernate"_s, &SessionManager::hibernate },
        { u"poweroff"_s, &SessionManager::poweroff },
        { u"reboot"_s, &SessionManager::reboot },
    };

    auto cmd = command.first();
    if ((cmd == u"systemctl"_s || cmd == u"loginctl"_s) && command.size() == 2)
        cmd = command.at(1);
    if (cmd == u"loginctl"_s && command.size() == 3 && command.at(1) == u"terminate-user"_s && command.at(2).isEmpty())
        cmd = u"logout"_s;

    cmd = cmd.remove(QStringLiteral("-")).remove(QStringLiteral("_")).toLower();

    const auto methodPtr = cmds.value(cmd, nullptr);
    if (methodPtr) {
        (this->*methodPtr)();
        return true;
    }

    return false;
}

void SessionManager::logout() {
    callSession(QStringLiteral("Terminate"));
}

void SessionManager::suspend() {
    callManager(QStringLiteral("Suspend"));
}

void SessionManager::suspendThenHibernate() {
    if (queryHibernateAvailable()) {
        callManager(QStringLiteral("SuspendThenHibernate"));
    } else {
        qCInfo(lcSessionManager) << "SuspendThenHibernate unavailable, falling back to suspend";
        callManager(QStringLiteral("Suspend"));
    }
}

void SessionManager::hibernate() {
    if (queryHibernateAvailable()) {
        callManager(QStringLiteral("Hibernate"));
    } else {
        qCWarning(lcSessionManager) << "Hibernate unavailable, ignoring hibernate request";

        auto* const engine = qmlEngine(this);
        if (!engine)
            return;
        auto* const toaster = engine->singletonInstance<Toaster*>("Caelestia", "Toaster");
        if (!toaster)
            return;
        toaster->toast(tr("Hibernate failed"), tr("Enable hibernation to use this feature."), QStringLiteral("warning"),
            Toast::Type::Warning);
    }
}

void SessionManager::poweroff() {
    callManager(QStringLiteral("PowerOff"));
}

void SessionManager::reboot() {
    callManager(QStringLiteral("Reboot"));
}

std::optional<QDBusConnection> SessionManager::getSystemBus() const {
    auto bus = QDBusConnection::systemBus();
    if (!bus.isConnected()) {
        qCWarning(lcSessionManager) << "Failed to connect to system bus:" << bus.lastError().message();
        return std::nullopt;
    }
    return bus;
}

bool SessionManager::queryHibernateAvailable() const {
    auto bus = getSystemBus();
    if (!bus)
        return false;

    auto hibernateMsg =
        QDBusMessage::createMethodCall(LOGIN_SERVICE, LOGIN_PATH, LOGIN_IFACE, QStringLiteral("CanHibernate"));
    const QDBusReply<QString> hibernateReply = bus->call(hibernateMsg);
    if (!hibernateReply.isValid()) {
        qCWarning(lcSessionManager) << "Failed to query hibernate support:" << hibernateReply.error().message();
    } else {
        const auto state = hibernateReply.value();
        return state == QStringLiteral("yes") || state == QStringLiteral("challenge");
    }

    return false;
}

void SessionManager::call(const QString& path, const QString& iface, const QString& method, const QVariantList& args) {
    auto bus = getSystemBus();
    if (!bus)
        return;

    auto msg = QDBusMessage::createMethodCall(LOGIN_SERVICE, path, iface, method);
    msg.setArguments(args);

    auto* watcher = new QDBusPendingCallWatcher(bus->asyncCall(msg), this);
    connect(watcher, &QDBusPendingCallWatcher::finished, this, [method](QDBusPendingCallWatcher* self) {
        const QDBusPendingReply<> reply = *self;
        if (reply.isError())
            qCWarning(lcSessionManager) << "Call to" << method << "failed:" << reply.error().message();
        self->deleteLater();
    });
}

void SessionManager::callManager(const QString& method) {
    call(LOGIN_PATH, LOGIN_IFACE, method, { /* interactive = */ true });
}

void SessionManager::callSession(const QString& method) {
    if (m_sessionPath.isEmpty()) {
        qCWarning(lcSessionManager) << "Cannot call" << method << "- no session path";
        return;
    }

    call(m_sessionPath, SESSION_IFACE, method);
}

void SessionManager::handlePrepareForSleep(bool sleep) {
    if (sleep) {
        emit aboutToSleep();
    } else {
        emit resumed();
    }
}

void SessionManager::handleLockRequested() {
    emit lockRequested();
}

void SessionManager::handleUnlockRequested() {
    emit unlockRequested();
}

} // namespace caelestia::services
