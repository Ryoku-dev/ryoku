#pragma once

#include <qfile.h>
#include <qhash.h>
#include <qnetworkaccessmanager.h>
#include <qnetworkreply.h>
#include <qobject.h>
#include <qpointer.h>
#include <qqmlengine.h>
#include <qtimer.h>

namespace caelestia {

class Requests : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

public:
    explicit Requests(QObject* parent = nullptr);

    Q_INVOKABLE int get(
        const QUrl& url, QJSValue callback, QJSValue onError = {}, QJSValue headers = {}, int timeoutMs = 0);

    Q_INVOKABLE int post(const QUrl& url, const QByteArray& body, const QString& contentType, QJSValue callback,
        QJSValue onError = {}, QJSValue headers = {}, int timeoutMs = 0);

    Q_INVOKABLE int download(const QUrl& url, const QString& destPath, QJSValue onComplete, QJSValue onProgress = {},
        QJSValue onError = {}, QJSValue headers = {}, int timeoutMs = 0);

    Q_INVOKABLE void cancel(int requestId);

    Q_INVOKABLE QJSValue parseJson(const QString& text) const;

    Q_INVOKABLE QString toJson(const QJSValue& value) const;

    Q_INVOKABLE void resetCookies();

signals:
    /// Emitted for every active download so QML can drive a global progress bar.
    void downloadProgress(int requestId, qint64 bytesReceived, qint64 bytesTotal);

private:
    struct ActiveRequest {
        QPointer<QNetworkReply> reply;
        QTimer* timeoutTimer = nullptr;
        bool isDownload = false;
        QFile* destFile = nullptr;
        QString destPath;
        QJSValue onProgress;
        QJSValue onComplete;
        QJSValue onError;
    };

    int registerReply(QNetworkReply* reply, QJSValue callback, QJSValue onError, int timeoutMs);
    void cleanupRequest(int requestId);
    int nextRequestId();

    void abortAndFail(int requestId, const QString& errorMessage, bool removeDestFile);

    QNetworkAccessManager* m_manager;
    QHash<int, ActiveRequest> m_activeRequests;
    int m_nextRequestId = 1;
};

} // namespace caelestia
