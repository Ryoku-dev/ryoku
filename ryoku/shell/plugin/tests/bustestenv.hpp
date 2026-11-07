#pragma once

#include "busclient.hpp"

#include <QQmlComponent>
#include <QQmlEngine>
#include <QTemporaryDir>
#include <QTest>

// A private runtime dir for the bus socket and an engine with Ryoku.Bus
// loaded from the build tree, so tests never reach a live session's daemon.
struct BusTestEnv {
    QTemporaryDir dir;

    BusTestEnv()
    {
        qputenv("XDG_RUNTIME_DIR", dir.path().toUtf8());
        qputenv("RYOKU_BUS_CLIENT", "test");
    }

    static void prepare(QQmlEngine &engine)
    {
        engine.addImportPath(QStringLiteral(RYOKU_BUS_QML_DIR));
    }

    // Creates a QML object from source (the import line is added).
    static QObject *create(QQmlEngine &engine, const QByteArray &body)
    {
        QQmlComponent c(&engine);
        c.setData("import QtQml\nimport Ryoku.Bus\n" + body, QUrl(QStringLiteral("file:///test.qml")));
        QObject *obj = c.create();
        if (!obj)
            qWarning().noquote() << c.errorString();
        return obj;
    }

    static BusClient *client(QQmlEngine &engine)
    {
        delete create(engine, "QtObject {}");
        return engine.singletonInstance<BusClient *>("Ryoku.Bus", "BusClient");
    }
};
