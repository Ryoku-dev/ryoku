#include "cavaprovider.hpp"

#include <qloggingcategory.h>

#include <cava/cavacore.h>

#include <algorithm>
#include <cmath>
#include <cstddef>

#include "audiocollector.hpp"
#include "audioprovider.hpp"

Q_LOGGING_CATEGORY(lcCava, "caelestia.services.cava", QtInfoMsg)
Q_LOGGING_CATEGORY(lcCavaProcessor, "caelestia.services.cava.processor", QtInfoMsg)

namespace caelestia::services {

CavaProcessor::CavaProcessor(QObject* parent)
    : AudioProcessor(parent)
    , m_plan(nullptr)
    , m_in(new double[ac::CHUNK_SIZE])
    , m_out(nullptr)
    , m_bars(0) {};

CavaProcessor::~CavaProcessor() {
    cleanup();
    delete[] m_in;
}

void CavaProcessor::process() {
    if (!m_plan || m_bars == 0 || !m_out) {
        return;
    }

    if (m_frameValues.size() != m_bars) {
        m_frameValues.resize(m_bars);
    }

    const int count = static_cast<int>(AudioCollector::instance().readChunk(m_in));

    if (isSilent(m_in, static_cast<std::size_t>(count))) {
        if (std::any_of(m_values.cbegin(), m_values.cend(), [](double value) {
                return value != 0.0;
            })) {
            m_frameValues.fill(0.0);
            m_values.fill(0.0);
            emit valuesChanged(m_values);
        }
        return;
    }

    cava_execute(m_in, count, m_out, m_plan);

    const double inv = 1.0 / 1.5;
    double carry = 0.0;
    for (int i = 0; i < m_bars; ++i) {
        carry = std::max(m_out[i], carry * inv);
        m_frameValues[i] = carry;
    }

    carry = 0.0;
    for (int i = m_bars - 1; i >= 0; --i) {
        carry = std::max(m_out[i], carry * inv);
        m_frameValues[i] = std::max(m_frameValues[i], carry);
    }

    bool changed = m_values.size() != m_frameValues.size();
    if (!changed) {
        constexpr double epsilon = 0.0005;
        for (int i = 0; i < m_frameValues.size(); ++i) {
            if (std::abs(m_frameValues[i] - m_values[i]) > epsilon) {
                changed = true;
                break;
            }
        }
    }

    if (changed) {
        m_values = m_frameValues;
        emit valuesChanged(m_values);
    }
}

void CavaProcessor::setBars(int bars) {
    if (bars < 0) {
        qCWarning(lcCavaProcessor) << "setBars: bars must be greater than 0. Setting to 0.";
        bars = 0;
    }

    if (m_bars != bars) {
        m_bars = bars;
        m_values.resize(m_bars);
        m_frameValues.resize(m_bars);
        reload();
    }
}

void CavaProcessor::reload() {
    cleanup();
    initCava();
}

void CavaProcessor::cleanup() {
    if (m_plan) {
        cava_destroy(m_plan);
        m_plan = nullptr;
    }

    if (m_out) {
        delete[] m_out;
        m_out = nullptr;
    }
}

void CavaProcessor::initCava() {
    if (m_plan || m_bars == 0) {
        return;
    }

    constexpr int channels = 1;
    constexpr int autosens = 1;
    constexpr double noiseReduction = 0.85;
    constexpr int lowCutoff = 50;
    constexpr int highCutoff = 10000;

#ifdef CAVA_SCALING_LINEAR
    m_plan = cava_init(
        m_bars, ac::SAMPLE_RATE, channels, autosens, noiseReduction, lowCutoff, highCutoff, CAVA_SCALING_LINEAR);
#else
    m_plan = cava_init(m_bars, ac::SAMPLE_RATE, channels, autosens, noiseReduction, lowCutoff, highCutoff);
#endif
    m_out = new double[static_cast<size_t>(m_bars)];
}

CavaProvider::CavaProvider(QObject* parent)
    : AudioProvider(parent)
    , m_bars(0)
    , m_values(m_bars, 0.0) {
    m_processor = new CavaProcessor();
    init();

    connect(static_cast<CavaProcessor*>(m_processor), &CavaProcessor::valuesChanged, this, &CavaProvider::updateValues);
}

int CavaProvider::bars() const {
    return m_bars;
}

void CavaProvider::setBars(int bars) {
    if (bars < 0) {
        qCWarning(lcCava) << "setBars: bars must be greater than 0. Setting to 0.";
        bars = 0;
    }

    if (m_bars == bars) {
        return;
    }

    m_values.resize(bars, 0.0);
    m_bars = bars;
    emit barsChanged();
    emit valuesChanged();

    QMetaObject::invokeMethod(
        static_cast<CavaProcessor*>(m_processor), &CavaProcessor::setBars, Qt::QueuedConnection, bars);
}

QVector<double> CavaProvider::values() const {
    return m_values;
}

void CavaProvider::updateValues(const QVector<double>& values) {
    if (values != m_values) {
        m_values = values;
        emit valuesChanged();
    }
}

} // namespace caelestia::services
