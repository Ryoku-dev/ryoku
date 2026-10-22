#include "circularbuffer.hpp"

#include <algorithm>

namespace caelestia {

CircularBuffer::CircularBuffer(QObject* parent)
    : QObject(parent) {}

int CircularBuffer::capacity() const {
    return m_capacity;
}

void CircularBuffer::setCapacity(int capacity) {
    if (capacity < 0)
        capacity = 0;
    if (m_capacity == capacity)
        return;

    const auto old = values();

    m_capacity = capacity;
    m_data.resize(capacity);
    m_data.fill(0.0);
    m_head = 0;
    m_count = 0;
    m_cachedMaximum = 0.0;
    m_maxDirty = false;

    if (m_capacity == 0) {
        emit capacityChanged();
        emit countChanged();
        emit valuesChanged();
        return;
    }

    const auto start = old.size() > capacity ? old.size() - capacity : 0;
    bool hasRestoredValue = false;
    for (auto i = start; i < old.size(); ++i) {
        m_data[m_head] = old[i];
        m_head = (m_head + 1) % m_capacity;
        m_count++;
        if (!hasRestoredValue) {
            m_cachedMaximum = old[i];
            hasRestoredValue = true;
        } else {
            m_cachedMaximum = std::max(m_cachedMaximum, old[i]);
        }
    }

    emit capacityChanged();
    emit countChanged();
    emit valuesChanged();
}

int CircularBuffer::count() const {
    return m_count;
}

QList<qreal> CircularBuffer::values() const {
    QList<qreal> result;
    result.reserve(m_count);
    for (int i = 0; i < m_count; ++i)
        result.append(at(i));
    return result;
}

qreal CircularBuffer::maximum() const {
    if (m_count == 0)
        return 0.0;

    if (m_maxDirty)
        const_cast<CircularBuffer*>(this)->recomputeMaximum();

    return m_cachedMaximum;
}

void CircularBuffer::push(qreal value) {
    if (m_capacity <= 0)
        return;

    const bool overwriting = m_count == m_capacity;
    const qreal overwritten = overwriting ? m_data[m_head] : 0.0;

    m_data[m_head] = value;
    m_head = (m_head + 1) % m_capacity;
    if (m_count < m_capacity) {
        m_count++;
        emit countChanged();
    }

    if (m_count == 1) {
        m_cachedMaximum = value;
        m_maxDirty = false;
    } else if (!overwriting) {
        m_cachedMaximum = std::max(m_cachedMaximum, value);
    } else if (value >= m_cachedMaximum) {
        m_cachedMaximum = value;
        m_maxDirty = false;
    } else if (!m_maxDirty && qFuzzyCompare(overwritten + 1.0, m_cachedMaximum + 1.0)) {
        m_maxDirty = true;
    }

    emit valuesChanged();
}

void CircularBuffer::clear() {
    if (m_count == 0)
        return;

    m_head = 0;
    m_count = 0;
    m_cachedMaximum = 0.0;
    m_maxDirty = false;
    emit countChanged();
    emit valuesChanged();
}

qreal CircularBuffer::at(int index) const {
    if (index < 0 || index >= m_count)
        return 0.0;

    const int actualIndex = (m_head - m_count + index + m_capacity) % m_capacity;
    return m_data[actualIndex];
}

void CircularBuffer::recomputeMaximum() {
    if (m_count == 0) {
        m_cachedMaximum = 0.0;
        m_maxDirty = false;
        return;
    }

    qreal maxVal = at(0);
    for (int i = 1; i < m_count; ++i)
        maxVal = std::max(maxVal, at(i));

    m_cachedMaximum = maxVal;
    m_maxDirty = false;
}

} // namespace caelestia
