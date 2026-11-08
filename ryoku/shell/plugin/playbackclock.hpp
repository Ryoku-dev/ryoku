#pragma once

#include <QElapsedTimer>
#include <QList>
#include <QObject>
#include <QTimer>
#include <qqmlregistration.h>

class PlaybackClock : public QObject {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(bool running READ running WRITE setRunning NOTIFY runningChanged)
    Q_PROPERTY(int interval READ interval WRITE setInterval NOTIFY intervalChanged)
    Q_PROPERTY(double anchorPosition READ anchorPosition WRITE setAnchorPosition NOTIFY anchorPositionChanged)
    Q_PROPERTY(double rate READ rate WRITE setRate NOTIFY rateChanged)
    Q_PROPERTY(double length READ length WRITE setLength NOTIFY lengthChanged)
    Q_PROPERTY(double elapsed READ elapsed NOTIFY elapsedChanged)
    Q_PROPERTY(QList<double> lineTimes READ lineTimes WRITE setLineTimes NOTIFY lineTimesChanged)
    Q_PROPERTY(int lineIndex READ lineIndex NOTIFY lineIndexChanged)

public:
    explicit PlaybackClock(QObject* parent = nullptr);

    bool running() const { return m_running; }
    void setRunning(bool running);

    int interval() const { return m_timer.interval(); }
    void setInterval(int interval);

    double anchorPosition() const { return m_anchorPosition; }
    void setAnchorPosition(double position);
    Q_INVOKABLE void reanchor(double position);

    double rate() const { return m_rate; }
    void setRate(double rate);

    double length() const { return m_length; }
    void setLength(double length);

    double elapsed() const { return m_elapsed; }

    const QList<double>& lineTimes() const { return m_lineTimes; }
    void setLineTimes(const QList<double>& lineTimes);

    int lineIndex() const { return m_lineIndex; }

signals:
    void runningChanged();
    void intervalChanged();
    void anchorPositionChanged();
    void rateChanged();
    void lengthChanged();
    void elapsedChanged();
    void lineTimesChanged();
    void lineIndexChanged();

private:
    double currentElapsed() const;
    double boundedPosition(double position) const;
    int findLineIndex(double elapsed) const;
    void publish(double elapsed);
    void updateElapsed();

    QTimer m_timer;
    QElapsedTimer m_anchorClock;
    QList<double> m_lineTimes;
    double m_anchorPosition = 0.0;
    double m_rate = 1.0;
    double m_length = 0.0;
    double m_elapsed = 0.0;
    int m_lineIndex = -1;
    bool m_running = false;
};
