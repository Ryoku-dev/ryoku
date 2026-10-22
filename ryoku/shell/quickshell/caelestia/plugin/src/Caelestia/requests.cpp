#include "requests.hpp"

#include <qdir.h>
#include <qfile.h>
#include <qfileinfo.h>
#include <qjsengine.h>
#include <qjsondocument.h>
#include <qjsvalueiterator.h>
#include <qloggingcategory.h>
#include <qnetworkcookiejar.h>
#include <qnetworkreply.h>
#include <qnetworkrequest.h>

Q_LOGGING_CATEGORY(lcRequests, "caelestia.requests", QtInfoMsg)

namespace caelestia {

using Qt::StringLiterals::operator""_ba;

static void applyHeaders(QNetworkRequest& request, const QJSValue& headers) {
    if (!headers.isObject()) {
        return;
    }
    QJSValueIterator it(headers);
    while (it.hasNext()) {
        it.next();
        request.setRawHeader(it.name().toUtf8(), it.value().toString().toUtf8());
    }
}

static QNetworkRequest buildRequest(const QUrl& url, const QJSValue& headers) {
    QNetworkRequest request(url);
    request.setAttribute(QNetworkRequest::CacheLoadControlAttribute, QNetworkRequest::AlwaysNetwork);
    request.setAttribute(QNetworkRequest::CookieSaveControlAttribute, QNetworkRequest::Manual);
    request.setRawHeader("Cache-Control"_ba, "no-cache, no-store"_ba);
    request.setRawHeader("Pragma"_ba, "no-cache"_ba);
    request.setRawHeader("Connection"_ba, "close"_ba);
    applyHeaders(request, headers);
    return request;
}

Requests::Requests(QObject* parent)
    : QObject(parent)
    , m_manager(new QNetworkAccessManager(this)) {}

// ── internal wiring ───────────────────────────────────────────────

int Requests::nextRequestId() {
    int id = m_nextRequestId;
    for (int attempts = 0; attempts < 100000; ++attempts) {
        if (id == 0) {
            id = 1;
        }
        if (!m_activeRequests.contains(id)) {
            m_nextRequestId = id + 1;
            return id;
        }
        ++id;
    }
    qCCritical(lcRequests) << "Exhausted request IDs!";
    return -1;
}

int Requests::registerReply(QNetworkReply* reply, QJSValue callback, QJSValue onError, int timeoutMs) {
    const int reqId = nextRequestId();
    if (reqId < 0) {
        reply->abort();
        reply->deleteLater();
        return -1;
    }

    auto& ar = m_activeRequests[reqId];
    ar.reply = reply;
    ar.onComplete = callback;
    ar.onError = onError;

    if (timeoutMs > 0) {
        auto* timer = new QTimer(this);
        timer->setSingleShot(true);
        ar.timeoutTimer = timer;
        QObject::connect(timer, &QTimer::timeout, this, [this, reqId]() {
            abortAndFail(reqId, QStringLiteral("Request timed out"), /*removeDestFile=*/false);
        });
        timer->start(timeoutMs);
    }

    QObject::connect(reply, &QNetworkReply::finished, this, [this, reqId, reply]() {
        auto it = m_activeRequests.find(reqId);
        if (it == m_activeRequests.end()) {
            reply->deleteLater();
            return;
        }

        const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        const bool httpError = status > 0 && (status < 200 || status >= 300);
        const bool isError = reply->error() != QNetworkReply::NoError || httpError;
        const QString error = httpError ? QStringLiteral("HTTP status %1").arg(status) : reply->errorString();

        if (isError && !it->isDownload) {
            if (it->onError.isCallable()) {
                it->onError.call({ error, status });
            } else {
                qCWarning(lcRequests) << "request" << reqId << "failed:" << error;
            }
            cleanupRequest(reqId);
            return;
        }

        if (it->isDownload) {
            if (it->destFile) {
                if (it->destFile->isOpen()) {
                    it->destFile->close();
                }
                if (isError) {
                    it->destFile->remove();
                }
            }

            if (isError) {
                if (it->onError.isCallable()) {
                    it->onError.call({ error, status });
                } else {
                    qCWarning(lcRequests) << "download" << reqId << "failed:" << error;
                }
            } else if (it->onComplete.isCallable()) {
                it->onComplete.call({ it->destPath, status });
            }

            cleanupRequest(reqId);
            return;
        }

        if (it->onComplete.isCallable()) {
            it->onComplete.call({ QString::fromUtf8(reply->readAll()), status });
        }

        cleanupRequest(reqId);
    });

    return reqId;
}

void Requests::cleanupRequest(int requestId) {
    auto it = m_activeRequests.find(requestId);
    if (it == m_activeRequests.end()) {
        return;
    }

    if (it->timeoutTimer) {
        it->timeoutTimer->stop();
        it->timeoutTimer->deleteLater();
    }

    if (it->destFile) {
        if (it->destFile->isOpen()) {
            it->destFile->close();
        }
        delete it->destFile;
    }

    if (it->reply) {
        it->reply->deleteLater();
    }

    m_activeRequests.erase(it);
}

void Requests::abortAndFail(int requestId, const QString& errorMessage, bool removeDestFile) {
    auto it = m_activeRequests.find(requestId);
    if (it == m_activeRequests.end()) {
        return;
    }

    ActiveRequest ar = m_activeRequests.take(requestId);

    if (ar.timeoutTimer) {
        ar.timeoutTimer->stop();
        ar.timeoutTimer->deleteLater();
    }

    if (ar.reply) {
        ar.reply->abort();
        ar.reply->deleteLater();
    }

    if (ar.destFile) {
        if (ar.destFile->isOpen()) {
            ar.destFile->close();
        }
        if (removeDestFile) {
            ar.destFile->remove();
        }
        delete ar.destFile;
    }

    if (ar.onError.isCallable()) {
        ar.onError.call({ errorMessage, 0 });
    } else {
        qCWarning(lcRequests) << "request" << requestId << "failed:" << errorMessage;
    }
}

int Requests::get(const QUrl& url, QJSValue callback, QJSValue onError, QJSValue headers, int timeoutMs) {
    if (!callback.isCallable()) {
        qCWarning(lcRequests) << "get: callback is not callable";
        return -1;
    }

    auto* reply = m_manager->get(buildRequest(url, headers));
    return registerReply(reply, callback, onError, timeoutMs);
}

int Requests::post(const QUrl& url, const QByteArray& body, const QString& contentType, QJSValue callback,
    QJSValue onError, QJSValue headers, int timeoutMs) {
    if (!callback.isCallable()) {
        qCWarning(lcRequests) << "post: callback is not callable";
        return -1;
    }

    QNetworkRequest req = buildRequest(url, headers);
    req.setHeader(QNetworkRequest::ContentTypeHeader, contentType);
    auto* reply = m_manager->post(req, body);
    return registerReply(reply, callback, onError, timeoutMs);
}

int Requests::download(const QUrl& url, const QString& destPath, QJSValue onComplete, QJSValue onProgress,
    QJSValue onError, QJSValue headers, int timeoutMs) {
    if (!onComplete.isCallable()) {
        qCWarning(lcRequests) << "download: onComplete callback is not callable";
        return -1;
    }

    auto* reply = m_manager->get(buildRequest(url, headers));
    const int reqId = nextRequestId();
    if (reqId < 0) {
        reply->abort();
        reply->deleteLater();
        return -1;
    }

    const QFileInfo destination(destPath);
    if (!QDir().mkpath(destination.absolutePath())) {
        const QString error = QStringLiteral("Cannot create destination directory: ") + destination.absolutePath();
        reply->abort();
        reply->deleteLater();
        if (onError.isCallable()) {
            onError.call({ error, 0 });
        }
        return -1;
    }

    auto* file = new QFile(destPath);
    if (!file->open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        const QString fileError = file->errorString();
        qCWarning(lcRequests) << "download: cannot open" << destPath << fileError;
        delete file;
        reply->abort();
        reply->deleteLater();
        if (onError.isCallable()) {
            onError.call({ QStringLiteral("Cannot open destination file: ") + fileError, 0 });
        }
        return -1;
    }

    auto& ar = m_activeRequests[reqId];
    ar.reply = reply;
    ar.isDownload = true;
    ar.destFile = file;
    ar.destPath = destPath;
    ar.onProgress = onProgress;
    ar.onComplete = onComplete;
    ar.onError = onError;

    if (timeoutMs > 0) {
        auto* timer = new QTimer(this);
        timer->setSingleShot(true);
        ar.timeoutTimer = timer;
        QObject::connect(timer, &QTimer::timeout, this, [this, reqId]() {
            abortAndFail(reqId, QStringLiteral("Download timed out"), /*removeDestFile=*/true);
        });
        timer->start(timeoutMs);
    }

    QObject::connect(reply, &QNetworkReply::readyRead, this, [this, reqId, reply]() {
        auto it = m_activeRequests.find(reqId);
        if (it == m_activeRequests.end() || !it->destFile) {
            return;
        }
        const QByteArray data = reply->readAll();
        if (it->destFile->write(data) != data.size()) {
            const QString writeError = it->destFile->errorString();
            qCWarning(lcRequests) << "download" << reqId << "write failed:" << writeError;
            abortAndFail(reqId, QStringLiteral("Write failed: ") + writeError, /*removeDestFile=*/true);
        }
    });

    QObject::connect(reply, &QNetworkReply::downloadProgress, this, [this, reqId](qint64 received, qint64 total) {
        emit downloadProgress(reqId, received, total);

        auto it = m_activeRequests.find(reqId);
        if (it != m_activeRequests.end() && it->onProgress.isCallable()) {
            it->onProgress.call({ static_cast<double>(received), static_cast<double>(total) });
        }
    });

    QObject::connect(reply, &QNetworkReply::finished, this, [this, reqId, reply]() {
        auto it = m_activeRequests.find(reqId);
        if (it == m_activeRequests.end()) {
            reply->deleteLater();
            return;
        }

        const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        const bool httpError = status > 0 && (status < 200 || status >= 300);
        bool isError = reply->error() != QNetworkReply::NoError || httpError;
        QString error = httpError ? QStringLiteral("HTTP status %1").arg(status) : reply->errorString();

        if (it->destFile && it->destFile->isOpen()) {
            const QByteArray data = reply->readAll();
            if (!isError && it->destFile->write(data) != data.size()) {
                isError = true;
                error = QStringLiteral("Write failed: ") + it->destFile->errorString();
            }
            it->destFile->close();
        }
        if (it->destFile && isError) {
            it->destFile->remove();
        }

        if (isError) {
            if (it->onError.isCallable()) {
                it->onError.call({ error, status });
            } else {
                qCWarning(lcRequests) << "download" << reqId << "failed:" << error;
            }
        } else if (it->onComplete.isCallable()) {
            it->onComplete.call({ it->destPath, status });
        }

        cleanupRequest(reqId);
    });

    return reqId;
}

void Requests::cancel(int requestId) {
    if (!m_activeRequests.contains(requestId)) {
        return;
    }
    ActiveRequest ar = m_activeRequests.take(requestId);
    if (ar.timeoutTimer) {
        ar.timeoutTimer->stop();
        ar.timeoutTimer->deleteLater();
    }
    if (ar.reply) {
        ar.reply->abort();
        ar.reply->deleteLater();
    }
    if (ar.destFile) {
        if (ar.destFile->isOpen()) {
            ar.destFile->close();
        }
        ar.destFile->remove();
        delete ar.destFile;
    }
}

QJSValue Requests::parseJson(const QString& text) const {
    QJsonParseError error;
    const QJsonDocument doc = QJsonDocument::fromJson(text.toUtf8(), &error);
    if (error.error != QJsonParseError::NoError) {
        qCWarning(lcRequests) << "parseJson:" << error.errorString();
        return QJSValue::NullValue;
    }
    auto* engine = qmlEngine(this);
    if (!engine) {
        return QJSValue::NullValue;
    }
    return engine->toScriptValue(doc.toVariant());
}

QString Requests::toJson(const QJSValue& value) const {
    if (value.isUndefined() || value.isNull()) {
        return QStringLiteral("null");
    }
    auto* engine = qmlEngine(this);
    if (!engine) {
        return {};
    }
    QJSValue json = engine->globalObject().property(QStringLiteral("JSON"));
    QJSValue stringify = json.property(QStringLiteral("stringify"));
    QJSValue result = stringify.callWithInstance(json, { value });
    return result.isUndefined() ? QStringLiteral("null") : result.toString();
}

void Requests::resetCookies() {
    m_manager->setCookieJar(new QNetworkCookieJar(m_manager));
}

} // namespace caelestia
