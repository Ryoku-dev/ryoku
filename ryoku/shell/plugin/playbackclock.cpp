#include "playbackclock.hpp"

#include <algorithm>
#include <cmath>

PlaybackClock::PlaybackClock(QObject* parent) : QObject(parent) {
    m_timer.setInterval(60);
    m_timer.setTimerType(Qt::PreciseTimer);
    connect(&m_timer, &QTimer::timeout, this, &PlaybackClock::updateElapsed);
    m_anchorClock.start();
}

void PlaybackClock::setRunning(bool running) {
    if (m_running == running)
        return;

    const double now = currentElapsed();
    const bool anchorChanged = m_anchorPosition != now;
    m_running = running;
    m_anchorPosition = now;
    m_anchorClock.restart();
    publish(now);

    if (m_running)
        m_timer.start();
    else
        m_timer.stop();

    if (anchorChanged)
        emit anchorPositionChanged();
    emit runningChanged();
}

void PlaybackClock::setInterval(int interval) {
    const int bounded = std::max(1, interval);
    if (m_timer.interval() == bounded)
        return;
    m_timer.setInterval(bounded);
    emit intervalChanged();
}

void PlaybackClock::setAnchorPosition(double position) {
    reanchor(position);
}

void PlaybackClock::reanchor(double position) {
    const double bounded = boundedPosition(position);
    const bool anchorChanged = m_anchorPosition != bounded;
    m_anchorPosition = bounded;
    m_anchorClock.restart();
    publish(bounded);
    if (anchorChanged)
        emit anchorPositionChanged();
}

void PlaybackClock::setRate(double rate) {
    const double bounded = std::isfinite(rate) ? std::max(0.0, rate) : 1.0;
    if (m_rate == bounded)
        return;

    const double now = currentElapsed();
    const bool anchorChanged = m_anchorPosition != now;
    m_anchorPosition = now;
    m_anchorClock.restart();
    m_rate = bounded;
    publish(now);
    if (anchorChanged)
        emit anchorPositionChanged();
    emit rateChanged();
}

void PlaybackClock::setLength(double length) {
    const double bounded = std::isfinite(length) ? std::max(0.0, length) : 0.0;
    if (m_length == bounded)
        return;

    const double now = currentElapsed();
    m_length = bounded;
    const double clamped = boundedPosition(now);
    const bool anchorChanged = m_anchorPosition != clamped;
    m_anchorPosition = clamped;
    m_anchorClock.restart();
    publish(clamped);
    emit lengthChanged();
    if (anchorChanged)
        emit anchorPositionChanged();
}

void PlaybackClock::setLineTimes(const QList<double>& lineTimes) {
    if (m_lineTimes == lineTimes)
        return;
    m_lineTimes = lineTimes;
    const int nextIndex = findLineIndex(m_elapsed);
    const bool indexChanged = m_lineIndex != nextIndex;
    m_lineIndex = nextIndex;
    emit lineTimesChanged();
    if (indexChanged)
        emit lineIndexChanged();
}

double PlaybackClock::currentElapsed() const {
    if (!m_running)
        return m_elapsed;
    const double seconds = static_cast<double>(m_anchorClock.elapsed()) / 1000.0;
    return boundedPosition(m_anchorPosition + seconds * m_rate);
}

double PlaybackClock::boundedPosition(double position) const {
    if (!std::isfinite(position))
        return 0.0;
    const double nonnegative = std::max(0.0, position);
    return m_length > 0.0 ? std::min(nonnegative, m_length) : nonnegative;
}

int PlaybackClock::findLineIndex(double elapsed) const {
    const auto it = std::upper_bound(m_lineTimes.cbegin(), m_lineTimes.cend(), elapsed);
    return it == m_lineTimes.cbegin() ? -1 : static_cast<int>(it - m_lineTimes.cbegin() - 1);
}

void PlaybackClock::publish(double elapsed) {
    const int nextIndex = findLineIndex(elapsed);
    const bool moved = m_elapsed != elapsed;
    const bool indexChanged = m_lineIndex != nextIndex;
    m_elapsed = elapsed;
    m_lineIndex = nextIndex;
    if (moved)
        emit elapsedChanged();
    if (indexChanged)
        emit lineIndexChanged();
}

void PlaybackClock::updateElapsed() {
    publish(currentElapsed());
}
