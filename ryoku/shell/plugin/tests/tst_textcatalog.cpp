#include "textcatalog.hpp"

#include <QFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

class TestTextCatalog : public QObject
{
    Q_OBJECT

    static bool writeFile(const QString &path, const QByteArray &contents)
    {
        QFile file(path);
        return file.open(QIODevice::WriteOnly | QIODevice::Truncate)
            && file.write(contents) == contents.size();
    }

private slots:
    void lookupAndTypes()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("catalog.json");
        QVERIFY(writeFile(path,
            R"({"hello":"bonjour","empty":"","number":7,"flag":true,"nested":{"value":"no"}})"));

        TextCatalog catalog;
        catalog.setPath(path);

        QVERIFY(catalog.loaded());
        QVERIFY(!catalog.failed());
        QCOMPARE(catalog.count(), 2);
        QVERIFY(catalog.contains(QStringLiteral("hello")));
        QCOMPARE(catalog.value(QStringLiteral("hello")), QStringLiteral("bonjour"));
        QVERIFY(catalog.contains(QStringLiteral("empty")));
        QCOMPARE(catalog.value(QStringLiteral("empty")), QStringLiteral(""));
        QVERIFY(!catalog.contains(QStringLiteral("number")));
        QVERIFY(!catalog.contains(QStringLiteral("nested")));
        QVERIFY(catalog.value(QStringLiteral("missing")).isNull());
    }

    void emptyPathClearsCatalog()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("catalog.json");
        QVERIFY(writeFile(path, R"({"key":"value"})"));

        TextCatalog catalog;
        catalog.setPath(path);
        const int revision = catalog.revision();
        catalog.setPath(QString());

        QCOMPARE(catalog.count(), 0);
        QVERIFY(catalog.loaded());
        QVERIFY(!catalog.failed());
        QCOMPARE(catalog.revision(), revision + 1);
        QVERIFY(catalog.value(QStringLiteral("key")).isNull());
    }

    void reloadsChangedFile()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("catalog.json");
        QVERIFY(writeFile(path, R"({"key":"first"})"));

        TextCatalog catalog;
        catalog.setPath(path);
        const int revision = catalog.revision();
        QSignalSpy revisionSpy(&catalog, &TextCatalog::revisionChanged);

        QVERIFY(writeFile(path, R"({"key":"second value"})"));

        QTRY_VERIFY(catalog.revision() > revision);
        QVERIFY(!revisionSpy.isEmpty());
        QCOMPARE(catalog.value(QStringLiteral("key")), QStringLiteral("second value"));
        QVERIFY(catalog.loaded());
        QVERIFY(!catalog.failed());
    }

    void missingTargetIgnoresUnrelatedFiles()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString target = directory.filePath("catalog.json");

        TextCatalog catalog;
        catalog.setPath(target);
        QVERIFY(!catalog.loaded());
        QVERIFY(catalog.failed());
        const int revision = catalog.revision();
        QSignalSpy revisionSpy(&catalog, &TextCatalog::revisionChanged);

        QVERIFY(writeFile(directory.filePath("unrelated.json"), R"({"other":"value"})"));
        QVERIFY(!revisionSpy.wait(200));
        QCOMPARE(catalog.revision(), revision);

        QVERIFY(writeFile(target, R"({"key":"created"})"));
        QTRY_COMPARE(catalog.revision(), revision + 1);
        QCOMPARE(revisionSpy.count(), 1);
        QCOMPARE(catalog.value(QStringLiteral("key")), QStringLiteral("created"));
        QVERIFY(catalog.loaded());
        QVERIFY(!catalog.failed());
    }
};

QTEST_GUILESS_MAIN(TestTextCatalog)
#include "tst_textcatalog.moc"
