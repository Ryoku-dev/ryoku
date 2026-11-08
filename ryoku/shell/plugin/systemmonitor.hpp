#pragma once

#include <QObject>
#include <QString>
#include <QVariantList>
#include <QVector>
#include <qqmlregistration.h>

#include <array>
#include <atomic>
#include <condition_variable>
#include <cstdint>
#include <limits>
#include <mutex>
#include <thread>

class SystemGraph;

class SystemMonitor : public QObject {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(bool active READ active WRITE setActive NOTIFY activeChanged)
    Q_PROPERTY(int samplePeriodMs READ samplePeriodMs WRITE setSamplePeriodMs NOTIFY samplePeriodMsChanged)
    Q_PROPERTY(int historyLimit READ historyLimit WRITE setHistoryLimit NOTIFY historyLimitChanged)
    Q_PROPERTY(bool includeVirtualNetwork READ includeVirtualNetwork WRITE setIncludeVirtualNetwork
                   NOTIFY includeVirtualNetworkChanged)
    Q_PROPERTY(bool gpu READ gpu WRITE setGpu NOTIFY gpuChanged)
    Q_PROPERTY(qreal cpuPercent READ cpuPercent NOTIFY samplesChanged)
    Q_PROPERTY(qreal memoryPercent READ memoryPercent NOTIFY samplesChanged)
    Q_PROPERTY(qreal gpuPercent READ gpuPercent NOTIFY samplesChanged)
    Q_PROPERTY(bool cpuAvailable READ cpuAvailable NOTIFY samplesChanged)
    Q_PROPERTY(bool memoryAvailable READ memoryAvailable NOTIFY samplesChanged)
    Q_PROPERTY(bool gpuAvailable READ gpuAvailable NOTIFY samplesChanged)
    Q_PROPERTY(qreal gpuPowerWatts READ gpuPowerWatts NOTIFY samplesChanged)
    Q_PROPERTY(qreal fanRpm READ fanRpm NOTIFY samplesChanged)
    Q_PROPERTY(QString cpuName READ cpuName NOTIFY samplesChanged)
    Q_PROPERTY(QString gpuName READ gpuName NOTIFY samplesChanged)
    Q_PROPERTY(QString userName READ userName NOTIFY samplesChanged)
    Q_PROPERTY(QString hostName READ hostName NOTIFY samplesChanged)
    Q_PROPERTY(qulonglong uptimeSeconds READ uptimeSeconds NOTIFY samplesChanged)
    Q_PROPERTY(qreal memoryUsedGiB READ memoryUsedGiB NOTIFY samplesChanged)
    Q_PROPERTY(qreal memoryTotalGiB READ memoryTotalGiB NOTIFY samplesChanged)
    Q_PROPERTY(qreal storageUsedGiB READ storageUsedGiB NOTIFY samplesChanged)
    Q_PROPERTY(qreal storageTotalGiB READ storageTotalGiB NOTIFY samplesChanged)
    Q_PROPERTY(int sampleCount READ sampleCount NOTIFY samplesChanged)
    Q_PROPERTY(QVariantList cpuHistory READ cpuHistory NOTIFY samplesChanged)
    Q_PROPERTY(QVariantList networkRxHistory READ networkRxHistory NOTIFY samplesChanged)
    Q_PROPERTY(QVariantList networkTxHistory READ networkTxHistory NOTIFY samplesChanged)
    Q_PROPERTY(QVariantList coreLoads READ coreLoads NOTIFY samplesChanged)
    Q_PROPERTY(int coreCount READ coreCount NOTIFY samplesChanged)
    Q_PROPERTY(double cpuFrequencyGhz READ cpuFrequencyGhz NOTIFY samplesChanged)
    Q_PROPERTY(double loadAverage READ loadAverage NOTIFY samplesChanged)
    Q_PROPERTY(int processCount READ processCount NOTIFY samplesChanged)
    Q_PROPERTY(double swapUsedGiB READ swapUsedGiB NOTIFY samplesChanged)
    Q_PROPERTY(double swapTotalGiB READ swapTotalGiB NOTIFY samplesChanged)
    Q_PROPERTY(double cpuTemp READ cpuTemp NOTIFY samplesChanged)
    Q_PROPERTY(bool cpuTempAvailable READ cpuTempAvailable NOTIFY samplesChanged)
    Q_PROPERTY(QString cpuTempLabel READ cpuTempLabel NOTIFY samplesChanged)
    Q_PROPERTY(double gpuTemp READ gpuTemp NOTIFY samplesChanged)
    Q_PROPERTY(bool gpuTempAvailable READ gpuTempAvailable NOTIFY samplesChanged)
    Q_PROPERTY(double gpuMemoryUsedGiB READ gpuMemoryUsedGiB NOTIFY samplesChanged)
    Q_PROPERTY(double gpuMemoryTotalGiB READ gpuMemoryTotalGiB NOTIFY samplesChanged)
    Q_PROPERTY(bool gpuMemoryAvailable READ gpuMemoryAvailable NOTIFY samplesChanged)
    Q_PROPERTY(double storageTemp READ storageTemp NOTIFY samplesChanged)
    Q_PROPERTY(bool storageTempAvailable READ storageTempAvailable NOTIFY samplesChanged)
    Q_PROPERTY(double networkRxBytesPerSec READ networkRxBytesPerSec NOTIFY samplesChanged)
    Q_PROPERTY(double networkTxBytesPerSec READ networkTxBytesPerSec NOTIFY samplesChanged)
    Q_PROPERTY(bool networkAvailable READ networkAvailable NOTIFY samplesChanged)
    Q_PROPERTY(QString networkInterface READ networkInterface NOTIFY samplesChanged)
    Q_PROPERTY(double diskReadBytesPerSec READ diskReadBytesPerSec NOTIFY samplesChanged)
    Q_PROPERTY(double diskWriteBytesPerSec READ diskWriteBytesPerSec NOTIFY samplesChanged)
    Q_PROPERTY(bool diskAvailable READ diskAvailable NOTIFY samplesChanged)
    Q_PROPERTY(double batteryPercent READ batteryPercent NOTIFY samplesChanged)
    Q_PROPERTY(bool batteryCharging READ batteryCharging NOTIFY samplesChanged)
    Q_PROPERTY(bool batteryAvailable READ batteryAvailable NOTIFY samplesChanged)

public:
    explicit SystemMonitor(QObject* parent = nullptr);
    ~SystemMonitor() override;
    // The default sidebar monitor samples four times a second. Lightweight
    // facades may choose a slower period; graph capacity stays sized for the
    // default monitor.
    static constexpr int SamplePeriodMs = 250;
    static constexpr int RateWindowSamples = 1000 / SamplePeriodMs;
    static constexpr int WindowSeconds = 60;
    static constexpr int HistoryCapacity = WindowSeconds * 1000 / SamplePeriodMs + RateWindowSamples;

    int samplePeriodMs() const { return m_samplePeriodMs.load(std::memory_order_relaxed); }
    void setSamplePeriodMs(int periodMs);
    int historyLimit() const { return m_historyLimit; }
    void setHistoryLimit(int limit);
    bool includeVirtualNetwork() const { return m_includeVirtualNetwork.load(std::memory_order_relaxed); }
    void setIncludeVirtualNetwork(bool include);
    bool gpu() const { return m_gpu.load(std::memory_order_relaxed); }
    void setGpu(bool enabled);

    bool active() const { return m_active; }
    void setActive(bool active);

    qreal cpuPercent() const { return m_cpuPercent; }
    qreal memoryPercent() const { return m_memoryPercent; }
    qreal gpuPercent() const { return m_gpuPercent; }
    bool cpuAvailable() const { return m_cpuAvailable; }
    bool memoryAvailable() const { return m_memoryAvailable; }
    bool gpuAvailable() const { return m_gpuAvailable; }
    qreal gpuPowerWatts() const { return m_gpuPowerWatts; }
    qreal fanRpm() const { return m_fanRpm; }
    const QString& cpuName() const { return m_cpuName; }
    const QString& gpuName() const { return m_gpuName; }
    const QString& userName() const { return m_userName; }
    const QString& hostName() const { return m_hostName; }
    qulonglong uptimeSeconds() const { return m_uptimeSeconds; }
    qreal memoryUsedGiB() const { return m_memoryUsedGiB; }
    qreal memoryTotalGiB() const { return m_memoryTotalGiB; }
    qreal storageUsedGiB() const { return m_storageUsedGiB; }
    qreal storageTotalGiB() const { return m_storageTotalGiB; }
    int sampleCount() const { return m_historyCount; }
    const QVariantList& cpuHistory() const { return m_cpuHistory; }
    const QVariantList& networkRxHistory() const { return m_networkRxHistory; }
    const QVariantList& networkTxHistory() const { return m_networkTxHistory; }
    const QVariantList& coreLoads() const { return m_coreLoads; }
    int coreCount() const { return m_coreCount; }
    double cpuFrequencyGhz() const { return m_cpuFrequencyGhz; }
    double loadAverage() const { return m_loadAverage; }
    int processCount() const { return m_processCount; }
    double swapUsedGiB() const { return m_swapUsedGiB; }
    double swapTotalGiB() const { return m_swapTotalGiB; }
    double cpuTemp() const { return m_cpuTemp; }
    bool cpuTempAvailable() const { return m_cpuTempAvailable; }
    const QString& cpuTempLabel() const { return m_cpuTempLabel; }
    double gpuTemp() const { return m_gpuTemp; }
    bool gpuTempAvailable() const { return m_gpuTempAvailable; }
    double gpuMemoryUsedGiB() const { return m_gpuMemoryUsedGiB; }
    double gpuMemoryTotalGiB() const { return m_gpuMemoryTotalGiB; }
    bool gpuMemoryAvailable() const { return m_gpuMemoryAvailable; }
    double storageTemp() const { return m_storageTemp; }
    bool storageTempAvailable() const { return m_storageTempAvailable; }
    double networkRxBytesPerSec() const { return m_networkRxBytesPerSec; }
    double networkTxBytesPerSec() const { return m_networkTxBytesPerSec; }
    bool networkAvailable() const { return m_networkAvailable; }
    const QString& networkInterface() const { return m_networkInterface; }
    double diskReadBytesPerSec() const { return m_diskReadBytesPerSec; }
    double diskWriteBytesPerSec() const { return m_diskWriteBytesPerSec; }
    bool diskAvailable() const { return m_diskAvailable; }
    double batteryPercent() const { return m_batteryPercent; }
    bool batteryCharging() const { return m_batteryCharging; }
    bool batteryAvailable() const { return m_batteryAvailable; }

signals:
    void activeChanged();
    void samplePeriodMsChanged();
    void historyLimitChanged();
    void includeVirtualNetworkChanged();
    void gpuChanged();
    void samplesChanged();

private:
    friend class SystemGraph;

    struct HistorySample {
        qint64 monotonicMs = 0;
        float cpu = std::numeric_limits<float>::quiet_NaN();
        float memory = std::numeric_limits<float>::quiet_NaN();
        float gpu = std::numeric_limits<float>::quiet_NaN();
        float netRx = std::numeric_limits<float>::quiet_NaN();
        float netTx = std::numeric_limits<float>::quiet_NaN();
        float diskRead = std::numeric_limits<float>::quiet_NaN();
        float diskWrite = std::numeric_limits<float>::quiet_NaN();
        float cpuTemp = std::numeric_limits<float>::quiet_NaN();
        float gpuTemp = std::numeric_limits<float>::quiet_NaN();
    };

    struct IdentityResult {
        QString cpuName;
        QString gpuName;
        QString userName;
        QString hostName;
    };

    struct SampleResult {
        qint64 monotonicMs = 0;
        double cpuPercent = 0.0;
        double memoryPercent = 0.0;
        double gpuPercent = 0.0;
        double gpuPowerWatts = 0.0;
        double fanRpm = 0.0;
        double memoryUsedGiB = 0.0;
        double memoryTotalGiB = 0.0;
        double storageUsedGiB = 0.0;
        double storageTotalGiB = 0.0;
        qulonglong uptimeSeconds = 0;
        QVector<double> coreLoads;
        int coreCount = 0;
        double cpuFrequencyGhz = 0.0;
        double loadAverage = 0.0;
        int processCount = 0;
        double swapUsedGiB = 0.0;
        double swapTotalGiB = 0.0;
        double cpuTemp = 0.0;
        QString cpuTempLabel;
        double gpuTemp = 0.0;
        double gpuMemoryUsedGiB = 0.0;
        double gpuMemoryTotalGiB = 0.0;
        double storageTemp = 0.0;
        double networkRxBytesPerSec = 0.0;
        double networkTxBytesPerSec = 0.0;
        QString networkInterface;
        double diskReadBytesPerSec = 0.0;
        double diskWriteBytesPerSec = 0.0;
        double batteryPercent = 0.0;
        bool cpuAvailable = false;
        bool memoryAvailable = false;
        bool gpuAvailable = false;
        bool cpuTempAvailable = false;
        bool gpuTempAvailable = false;
        bool gpuMemoryAvailable = false;
        bool storageTempAvailable = false;
        bool networkAvailable = false;
        bool diskAvailable = false;
        bool batteryCharging = false;
        bool batteryAvailable = false;
    };

    const HistorySample& historyAtOldest(int index) const;
    void startWorker();
    void stopWorker();
    void applyIdentity(quint64 generation, IdentityResult result);
    void applySample(quint64 generation, const SampleResult& result);
    void clearValues();
    void appendHistory(QVariantList& history, double value);
    std::atomic_int m_samplePeriodMs{SamplePeriodMs};
    int m_historyLimit = 0;
    std::atomic_bool m_includeVirtualNetwork{false};
    std::atomic_bool m_gpu{true};

    bool m_active = false;
    std::atomic<quint64> m_generation{0};
    std::atomic_bool m_workerActive{false};
    std::mutex m_workerMutex;
    std::condition_variable_any m_workerWake;
    std::jthread m_worker;

    qreal m_cpuPercent = 0.0;
    qreal m_memoryPercent = 0.0;
    qreal m_gpuPercent = 0.0;
    bool m_cpuAvailable = false;
    bool m_memoryAvailable = false;
    bool m_gpuAvailable = false;
    qreal m_gpuPowerWatts = 0.0;
    qreal m_fanRpm = 0.0;
    QString m_cpuName;
    QString m_gpuName;
    QString m_userName;
    QString m_hostName;
    qulonglong m_uptimeSeconds = 0;
    qreal m_memoryUsedGiB = 0.0;
    qreal m_memoryTotalGiB = 0.0;
    qreal m_storageUsedGiB = 0.0;
    qreal m_storageTotalGiB = 0.0;
    QVariantList m_coreLoads;
    QVariantList m_cpuHistory;
    QVariantList m_networkRxHistory;
    QVariantList m_networkTxHistory;
    int m_coreCount = 0;
    double m_cpuFrequencyGhz = 0.0;
    double m_loadAverage = 0.0;
    int m_processCount = 0;
    double m_swapUsedGiB = 0.0;
    double m_swapTotalGiB = 0.0;
    double m_cpuTemp = 0.0;
    bool m_cpuTempAvailable = false;
    QString m_cpuTempLabel;
    double m_gpuTemp = 0.0;
    bool m_gpuTempAvailable = false;
    double m_gpuMemoryUsedGiB = 0.0;
    double m_gpuMemoryTotalGiB = 0.0;
    bool m_gpuMemoryAvailable = false;
    double m_storageTemp = 0.0;
    bool m_storageTempAvailable = false;
    double m_networkRxBytesPerSec = 0.0;
    double m_networkTxBytesPerSec = 0.0;
    bool m_networkAvailable = false;
    QString m_networkInterface;
    double m_diskReadBytesPerSec = 0.0;
    double m_diskWriteBytesPerSec = 0.0;
    bool m_diskAvailable = false;
    double m_batteryPercent = 0.0;
    bool m_batteryCharging = false;
    bool m_batteryAvailable = false;

    std::array<HistorySample, HistoryCapacity> m_history{};
    int m_historyStart = 0;
    int m_historyCount = 0;
};
