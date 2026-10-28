#include "systemmonitor.hpp"

#include <QByteArray>
#include <QByteArrayView>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QHash>
#include <QLibrary>
#include <QMetaObject>
#include <QSet>
#include <QStringList>
#include <QVector>

#include <algorithm>
#include <array>
#include <charconv>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <limits>
#include <mutex>
#include <utility>
#include <pwd.h>
#include <sys/statvfs.h>
#include <unistd.h>

namespace {

using Clock = std::chrono::steady_clock;
constexpr double BytesPerGiB = 1024.0 * 1024.0 * 1024.0;

qint64 monotonicMilliseconds() {
    return std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now().time_since_epoch()).count();
}

QByteArray readSmallFile(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.read(16384);
}

bool readUnsignedFile(const QString& path, quint64& value) {
    const QByteArray bytes = readSmallFile(path).trimmed();
    if (bytes.isEmpty())
        return false;
    bool ok = false;
    value = bytes.toULongLong(&ok);
    return ok;
}

quint64 readUnsignedFile(const QString& path) {
    quint64 value = 0;
    readUnsignedFile(path, value);
    return value;
}

QString readTextFile(const QString& path) {
    return QString::fromUtf8(readSmallFile(path)).trimmed();
}

QString ueventField(const QString& devicePath, const QByteArray& key) {
    const QByteArray contents = readSmallFile(devicePath + QStringLiteral("/uevent"));
    const QByteArray prefix = key + '=';
    qsizetype offset = 0;
    while (offset < contents.size()) {
        const qsizetype end = contents.indexOf('\n', offset);
        const qsizetype length = (end < 0 ? contents.size() : end) - offset;
        const QByteArrayView line(contents.constData() + offset, length);
        if (line.startsWith(prefix))
            return QString::fromUtf8(line.sliced(prefix.size()));
        if (end < 0)
            break;
        offset = end + 1;
    }
    return {};
}

QString cpuModelName() {
    QFile file(QStringLiteral("/proc/cpuinfo"));
    if (!file.open(QIODevice::ReadOnly))
        return {};
    QByteArray line;
    while (!(line = file.readLine()).isEmpty()) {
        const qsizetype colon = line.indexOf(':');
        if (colon < 0)
            continue;
        const QByteArray key = line.first(colon).trimmed();
        if (key == "model name" || key == "Hardware" || key == "Processor")
            return QString::fromUtf8(line.sliced(colon + 1)).trimmed();
    }
    return {};
}

QString currentUserName() {
    passwd entry{};
    passwd* result = nullptr;
    std::array<char, 16384> buffer{};
    if (getpwuid_r(geteuid(), &entry, buffer.data(), buffer.size(), &result) == 0 && result && result->pw_name)
        return QString::fromLocal8Bit(result->pw_name);
    return qEnvironmentVariable("USER");
}

QString currentHostName() {
    std::array<char, 256> name{};
    if (gethostname(name.data(), name.size() - 1) != 0)
        return {};
    name.back() = '\0';
    return QString::fromLocal8Bit(name.data());
}

struct CpuTotals {
    quint64 total = 0;
    quint64 idle = 0;
    bool valid = false;
};

bool parseCpuTotals(QByteArrayView line, CpuTotals& totals) {
    const char* first = line.data();
    const char* last = first + line.size();
    while (first < last && *first != ' ' && *first != '\t')
        ++first;

    std::array<quint64, 8> values{};
    int count = 0;
    while (first < last && count < static_cast<int>(values.size())) {
        while (first < last && (*first == ' ' || *first == '\t'))
            ++first;
        if (first == last)
            break;
        const auto parsed = std::from_chars(first, last, values[count]);
        if (parsed.ec != std::errc{})
            return false;
        first = parsed.ptr;
        ++count;
    }
    if (count < 4)
        return false;

    for (int i = 0; i < count; ++i)
        totals.total += values[i];
    totals.idle = values[3] + (count > 4 ? values[4] : 0);
    totals.valid = true;
    return true;
}

struct CpuSnapshot {
    CpuTotals aggregate;
    QVector<CpuTotals> cores;
};

CpuSnapshot readCpuSnapshot() {
    QFile file(QStringLiteral("/proc/stat"));
    if (!file.open(QIODevice::ReadOnly))
        return {};

    CpuSnapshot snapshot;
    QVector<std::pair<int, CpuTotals>> indexed;
    QByteArray line;
    while (!(line = file.readLine()).isEmpty()) {
        const qsizetype separator = line.indexOf(' ');
        if (separator < 0)
            continue;
        const QByteArray label = line.first(separator);
        if (label == "cpu") {
            parseCpuTotals(line, snapshot.aggregate);
            continue;
        }
        if (!label.startsWith("cpu"))
            break;
        bool indexOk = false;
        const int index = label.sliced(3).toInt(&indexOk);
        CpuTotals totals;
        if (indexOk && parseCpuTotals(line, totals))
            indexed.append({index, totals});
    }
    std::sort(indexed.begin(), indexed.end(), [](const auto& left, const auto& right) {
        return left.first < right.first;
    });
    snapshot.cores.reserve(indexed.size());
    for (const auto& entry : std::as_const(indexed))
        snapshot.cores.append(entry.second);
    return snapshot;
}

double cpuLoad(const CpuTotals& current, const CpuTotals& previous, bool& available) {
    available = current.valid && previous.valid && current.total > previous.total;
    if (!available)
        return 0.0;
    const quint64 totalDelta = current.total - previous.total;
    const quint64 idleDelta = current.idle >= previous.idle ? current.idle - previous.idle : 0;
    return std::clamp(100.0 * static_cast<double>(totalDelta - std::min(totalDelta, idleDelta))
                          / static_cast<double>(totalDelta),
                      0.0, 100.0);
}

struct MemoryReading {
    double usedGiB = 0.0;
    double totalGiB = 0.0;
    double percent = 0.0;
    double swapUsedGiB = 0.0;
    double swapTotalGiB = 0.0;
    bool available = false;
};

MemoryReading readMemory() {
    const QByteArray bytes = readSmallFile(QStringLiteral("/proc/meminfo"));
    quint64 totalKiB = 0;
    quint64 availableKiB = 0;
    quint64 freeKiB = 0;
    quint64 buffersKiB = 0;
    quint64 cachedKiB = 0;
    quint64 swapTotalKiB = 0;
    quint64 swapFreeKiB = 0;
    bool availableKnown = false;
    qsizetype offset = 0;
    while (offset < bytes.size()) {
        const qsizetype newline = bytes.indexOf('\n', offset);
        const qsizetype end = newline < 0 ? bytes.size() : newline;
        const qsizetype colon = bytes.indexOf(':', offset);
        if (colon >= offset && colon < end) {
            const QByteArrayView key(bytes.constData() + offset, colon - offset);
            const char* first = bytes.constData() + colon + 1;
            const char* last = bytes.constData() + end;
            while (first < last && (*first == ' ' || *first == '\t'))
                ++first;
            quint64 value = 0;
            if (std::from_chars(first, last, value).ec == std::errc{}) {
                if (key == "MemTotal")
                    totalKiB = value;
                else if (key == "MemAvailable") {
                    availableKiB = value;
                    availableKnown = true;
                } else if (key == "MemFree")
                    freeKiB = value;
                else if (key == "Buffers")
                    buffersKiB = value;
                else if (key == "Cached")
                    cachedKiB = value;
                else if (key == "SwapTotal")
                    swapTotalKiB = value;
                else if (key == "SwapFree")
                    swapFreeKiB = value;
            }
        }
        offset = end + 1;
    }
    if (totalKiB == 0)
        return {};
    if (!availableKnown)
        availableKiB = std::min(totalKiB, freeKiB + buffersKiB + cachedKiB);

    const quint64 usedKiB = totalKiB - std::min(totalKiB, availableKiB);
    const quint64 swapUsedKiB = swapTotalKiB - std::min(swapTotalKiB, swapFreeKiB);
    MemoryReading out;
    out.usedGiB = static_cast<double>(usedKiB) * 1024.0 / BytesPerGiB;
    out.totalGiB = static_cast<double>(totalKiB) * 1024.0 / BytesPerGiB;
    out.percent = 100.0 * static_cast<double>(usedKiB) / static_cast<double>(totalKiB);
    out.swapUsedGiB = static_cast<double>(swapUsedKiB) * 1024.0 / BytesPerGiB;
    out.swapTotalGiB = static_cast<double>(swapTotalKiB) * 1024.0 / BytesPerGiB;
    out.available = true;
    return out;
}

struct StorageReading {
    double usedGiB = 0.0;
    double totalGiB = 0.0;
};

StorageReading readRootStorage() {
    struct statvfs stats {};
    if (statvfs("/", &stats) != 0 || stats.f_blocks == 0)
        return {};
    const long double blockSize = stats.f_frsize ? stats.f_frsize : stats.f_bsize;
    const long double total = static_cast<long double>(stats.f_blocks) * blockSize;
    const long double available = static_cast<long double>(stats.f_bavail) * blockSize;
    StorageReading out;
    out.totalGiB = static_cast<double>(total / BytesPerGiB);
    out.usedGiB = static_cast<double>((total - std::min(total, available)) / BytesPerGiB);
    return out;
}

qulonglong readUptimeSeconds() {
    QFile file(QStringLiteral("/proc/uptime"));
    if (!file.open(QIODevice::ReadOnly))
        return 0;
    bool ok = false;
    const double seconds = file.readLine().split(' ').value(0).toDouble(&ok);
    return ok && seconds > 0.0 ? static_cast<qulonglong>(seconds) : 0;
}

struct LoadReading {
    double average = 0.0;
    int processCount = 0;
};

LoadReading readLoad() {
    const QList<QByteArray> fields = readSmallFile(QStringLiteral("/proc/loadavg")).simplified().split(' ');
    if (fields.size() < 4)
        return {};
    bool averageOk = false;
    const double average = fields[0].toDouble(&averageOk);
    const qsizetype slash = fields[3].indexOf('/');
    bool countOk = false;
    const int processCount = slash >= 0 ? fields[3].sliced(slash + 1).toInt(&countOk) : 0;
    return {averageOk ? average : 0.0, countOk ? processCount : 0};
}

struct FrequencySource {
    QString frequencyPath;
    QString onlinePath;
};

QVector<FrequencySource> discoverFrequencySources() {
    QVector<std::pair<int, FrequencySource>> indexed;
    const QDir cpus(QStringLiteral("/sys/devices/system/cpu"));
    const QFileInfoList entries = cpus.entryInfoList(QStringList{QStringLiteral("cpu*")}, QDir::Dirs | QDir::NoDotAndDotDot,
                                                     QDir::Name);
    for (const QFileInfo& entry : entries) {
        const QString name = entry.fileName();
        bool ok = false;
        const int index = name.sliced(3).toInt(&ok);
        if (!ok)
            continue;
        const QString frequencyPath = entry.absoluteFilePath() + QStringLiteral("/cpufreq/scaling_cur_freq");
        if (!QFileInfo::exists(frequencyPath))
            continue;
        indexed.append({index, {frequencyPath, entry.absoluteFilePath() + QStringLiteral("/online")}});
    }
    std::sort(indexed.begin(), indexed.end(), [](const auto& left, const auto& right) {
        return left.first < right.first;
    });
    QVector<FrequencySource> sources;
    sources.reserve(indexed.size());
    for (auto& entry : indexed)
        sources.append(std::move(entry.second));
    return sources;
}

double readMeanFrequencyGhz(const QVector<FrequencySource>& sources) {
    long double sumKhz = 0.0;
    int count = 0;
    for (const FrequencySource& source : sources) {
        if (QFileInfo::exists(source.onlinePath) && readSmallFile(source.onlinePath).trimmed() != "1")
            continue;
        quint64 frequencyKhz = 0;
        if (!readUnsignedFile(source.frequencyPath, frequencyKhz))
            continue;
        sumKhz += frequencyKhz;
        ++count;
    }
    return count > 0 ? static_cast<double>(sumKhz / static_cast<long double>(count) / 1000000.0L) : 0.0;
}

struct TemperatureSensor {
    QString inputPath;
    QString label;
};

struct HwmonTemperature {
    QString name;
    QString label;
    QString inputPath;
};

QVector<HwmonTemperature> discoverHwmonTemperatures() {
    QVector<HwmonTemperature> sensors;
    const QDir hwmon(QStringLiteral("/sys/class/hwmon"));
    const QFileInfoList devices = hwmon.entryInfoList(QStringList{QStringLiteral("hwmon*")},
                                                      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& device : devices) {
        const QString base = device.absoluteFilePath();
        const QString name = readTextFile(base + QStringLiteral("/name"));
        const QDir directory(base);
        const QFileInfoList inputs = directory.entryInfoList(QStringList{QStringLiteral("temp*_input")}, QDir::Files,
                                                             QDir::Name);
        for (const QFileInfo& input : inputs) {
            const QString fileName = input.fileName();
            const QString stem = fileName.first(fileName.size() - 6);
            bool indexOk = false;
            stem.sliced(4).toInt(&indexOk);
            if (!indexOk)
                continue;
            sensors.append({name, readTextFile(base + '/' + stem + QStringLiteral("_label")), input.absoluteFilePath()});
        }
    }
    return sensors;
}

TemperatureSensor discoverCpuTemperature(const QVector<HwmonTemperature>& sensors) {
    const auto match = [&sensors](const auto& predicate) -> TemperatureSensor {
        for (const HwmonTemperature& sensor : sensors) {
            if (predicate(sensor))
                return {sensor.inputPath, QStringLiteral("package")};
        }
        return {};
    };

    TemperatureSensor selected = match([](const HwmonTemperature& sensor) {
        return sensor.name.compare(QStringLiteral("coretemp"), Qt::CaseInsensitive) == 0
            && sensor.label.compare(QStringLiteral("Package id 0"), Qt::CaseInsensitive) == 0;
    });
    if (!selected.inputPath.isEmpty())
        return selected;
    selected = match([](const HwmonTemperature& sensor) {
        const QString name = sensor.name.toLower();
        return (name == QStringLiteral("k10temp") || name == QStringLiteral("zenpower"))
            && sensor.label.compare(QStringLiteral("Tdie"), Qt::CaseInsensitive) == 0;
    });
    if (!selected.inputPath.isEmpty())
        return selected;
    selected = match([](const HwmonTemperature& sensor) {
        const QString name = sensor.name.toLower();
        return (name == QStringLiteral("k10temp") || name == QStringLiteral("zenpower"))
            && sensor.label.compare(QStringLiteral("Tctl"), Qt::CaseInsensitive) == 0;
    });
    if (!selected.inputPath.isEmpty())
        return selected;
    selected = match([](const HwmonTemperature& sensor) {
        const QString label = sensor.label.toLower();
        const bool excluded = label.contains(QStringLiteral("vrm")) || label.contains(QStringLiteral("soc"))
            || label.contains(QStringLiteral("pch"));
        return !excluded && (label.contains(QStringLiteral("cpu")) || label.contains(QStringLiteral("package"))
                             || label.contains(QStringLiteral("processor")));
    });
    if (!selected.inputPath.isEmpty())
        return selected;

    const QDir thermal(QStringLiteral("/sys/class/thermal"));
    const QFileInfoList zones = thermal.entryInfoList(QStringList{QStringLiteral("thermal_zone*")},
                                                      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    const auto thermalMatch = [&zones](const auto& predicate) -> TemperatureSensor {
        for (const QFileInfo& zone : zones) {
            const QString type = readTextFile(zone.absoluteFilePath() + QStringLiteral("/type")).toLower();
            if (predicate(type))
                return {zone.absoluteFilePath() + QStringLiteral("/temp"), QStringLiteral("zone")};
        }
        return {};
    };
    selected = thermalMatch([](const QString& type) {
        return type == QStringLiteral("x86_pkg_temp") || type == QStringLiteral("cpu")
            || type.contains(QStringLiteral("cpu"));
    });
    if (!selected.inputPath.isEmpty())
        return selected;
    return thermalMatch([](const QString& type) { return type == QStringLiteral("acpitz"); });
}

TemperatureSensor discoverStorageTemperature(const QVector<HwmonTemperature>& sensors) {
    for (const HwmonTemperature& sensor : sensors) {
        if (sensor.name.compare(QStringLiteral("nvme"), Qt::CaseInsensitive) == 0
            && sensor.label.compare(QStringLiteral("Composite"), Qt::CaseInsensitive) == 0) {
            return {sensor.inputPath, {}};
        }
    }
    for (const HwmonTemperature& sensor : sensors) {
        if (sensor.name.compare(QStringLiteral("drivetemp"), Qt::CaseInsensitive) == 0)
            return {sensor.inputPath, {}};
    }
    return {};
}

TemperatureSensor discoverGpuTemperature(const QString& devicePath) {
    const QDir hwmon(devicePath + QStringLiteral("/hwmon"));
    const QFileInfoList devices = hwmon.entryInfoList(QStringList{QStringLiteral("hwmon*")},
                                                      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    TemperatureSensor fallback;
    for (const QFileInfo& device : devices) {
        const QString base = device.absoluteFilePath();
        const QDir directory(base);
        const QFileInfoList inputs = directory.entryInfoList(QStringList{QStringLiteral("temp*_input")}, QDir::Files,
                                                             QDir::Name);
        for (const QFileInfo& input : inputs) {
            const QString stem = input.fileName().first(input.fileName().size() - 6);
            bool indexOk = false;
            stem.sliced(4).toInt(&indexOk);
            if (!indexOk)
                continue;
            const QString label = readTextFile(base + '/' + stem + QStringLiteral("_label"));
            TemperatureSensor sensor{input.absoluteFilePath(), {}};
            if (label.compare(QStringLiteral("edge"), Qt::CaseInsensitive) == 0)
                return sensor;
            if (fallback.inputPath.isEmpty())
                fallback = std::move(sensor);
        }
    }
    return fallback;
}
QString discoverHwmonMetric(const QString& devicePath, const QStringList& names) {
    const QDir hwmon(devicePath + QStringLiteral("/hwmon"));
    const QFileInfoList devices = hwmon.entryInfoList(QStringList{QStringLiteral("hwmon*")},
                                                      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& device : devices) {
        for (const QString& name : names) {
            const QString path = device.absoluteFilePath() + '/' + name;
            if (QFileInfo::exists(path))
                return path;
        }
    }
    return {};
}

QString discoverFanPath() {
    const QDir hwmon(QStringLiteral("/sys/class/hwmon"));
    const QFileInfoList devices = hwmon.entryInfoList(QStringList{QStringLiteral("hwmon*")},
                                                      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& device : devices) {
        const QString path = device.absoluteFilePath() + QStringLiteral("/fan1_input");
        quint64 rpm = 0;
        if (readUnsignedFile(path, rpm))
            return path;
    }
    return {};
}


bool readTemperature(const TemperatureSensor& sensor, double& celsius) {
    if (sensor.inputPath.isEmpty())
        return false;
    const QByteArray bytes = readSmallFile(sensor.inputPath).trimmed();
    bool ok = false;
    const qint64 millidegrees = bytes.toLongLong(&ok);
    if (!ok)
        return false;
    celsius = static_cast<double>(millidegrees) / 1000.0;
    return std::isfinite(celsius) && celsius >= -273.15 && celsius < 1000.0;
}

bool isCardName(const QString& name) {
    if (!name.startsWith(QStringLiteral("card")) || name.size() == 4)
        return false;
    for (qsizetype i = 4; i < name.size(); ++i) {
        if (!name[i].isDigit())
            return false;
    }
    return true;
}

bool runtimeSuspended(const QString& statusPath) {
    return !statusPath.isEmpty() && readSmallFile(statusPath).trimmed() == "suspended";
}

struct GpuCandidate {
    QString driver;
    QString name;
    QString devicePath;
    QString utilizationPath;
    QString runtimeStatusPath;
    quint64 vram = 0;
    int kind = 0;
};

bool stronger(const GpuCandidate& a, const GpuCandidate& b) {
    if (a.kind != b.kind)
        return a.kind > b.kind;
    if (a.vram != b.vram)
        return a.vram > b.vram;
    return a.driver < b.driver;
}

QVector<GpuCandidate> discoverSysfsGpus() {
    QVector<GpuCandidate> found;
    QSet<QString> seenDevices;
    const QDir drm(QStringLiteral("/sys/class/drm"));
    const QFileInfoList entries = drm.entryInfoList(QStringList{QStringLiteral("card*")},
                                                    QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& entry : entries) {
        if (!isCardName(entry.fileName()))
            continue;
        const QString devicePath = entry.absoluteFilePath() + QStringLiteral("/device");
        QFileInfo device(devicePath);
        if (!device.exists())
            continue;
        const QString identity = device.canonicalFilePath();
        if (identity.isEmpty() || seenDevices.contains(identity))
            continue;
        seenDevices.insert(identity);

        GpuCandidate candidate;
        candidate.devicePath = devicePath;
        candidate.driver = ueventField(devicePath, "DRIVER");
        if (candidate.driver.isEmpty())
            candidate.driver = QFileInfo(devicePath + QStringLiteral("/driver")).symLinkTarget().section('/', -1);
        candidate.vram = readUnsignedFile(devicePath + QStringLiteral("/mem_info_vram_total"));
        const quint64 visibleVram = readUnsignedFile(devicePath + QStringLiteral("/mem_info_vis_vram_total"));
        const QString removable = readTextFile(devicePath + QStringLiteral("/removable"));
        const bool discrete = candidate.driver == QStringLiteral("nvidia") || candidate.driver == QStringLiteral("nouveau")
            || (visibleVram > 0 && (visibleVram != candidate.vram || candidate.vram > 8ULL * 1024ULL * 1024ULL * 1024ULL))
            || (visibleVram == 0 && candidate.vram >= 2ULL * 1024ULL * 1024ULL * 1024ULL);
        candidate.kind = (removable == QStringLiteral("removable") || removable == QStringLiteral("1")) ? 2
                                                                                                           : (discrete ? 1 : 0);
        candidate.runtimeStatusPath = devicePath + QStringLiteral("/power/runtime_status");
        const QStringList utilizationNames{QStringLiteral("gpu_busy_percent"), QStringLiteral("gt_busy_percent")};
        for (const QString& name : utilizationNames) {
            const QString path = devicePath + '/' + name;
            if (QFileInfo::exists(path)) {
                candidate.utilizationPath = path;
                break;
            }
        }
        candidate.name = readTextFile(devicePath + QStringLiteral("/product_name"));
        if (candidate.name.isEmpty())
            candidate.name = readTextFile(devicePath + QStringLiteral("/marketing_name"));
        if (candidate.name.isEmpty()) {
            if (candidate.driver == QStringLiteral("amdgpu"))
                candidate.name = QStringLiteral("AMD GPU");
            else if (candidate.driver == QStringLiteral("i915") || candidate.driver == QStringLiteral("xe"))
                candidate.name = QStringLiteral("Intel GPU");
            else if (candidate.driver == QStringLiteral("nvidia") || candidate.driver == QStringLiteral("nouveau"))
                candidate.name = QStringLiteral("NVIDIA GPU");
            else
                candidate.name = candidate.driver;
        }
        found.append(std::move(candidate));
    }
    return found;
}

using NvmlReturn = int;
using NvmlDevice = void*;
constexpr NvmlReturn NvmlSuccess = 0;
constexpr unsigned int NvmlTemperatureGpu = 0;

struct NvmlUtilization {
    unsigned int gpu;
    unsigned int memory;
};

struct NvmlMemory {
    unsigned long long total;
    unsigned long long free;
    unsigned long long used;
};

struct GpuReading {
    double percent = 0.0;
    double temperature = 0.0;
    double memoryUsedGiB = 0.0;
    double powerWatts = 0.0;
    double memoryTotalGiB = 0.0;
    bool available = false;
    bool temperatureAvailable = false;
    bool memoryAvailable = false;
};

class GpuSampler {
public:
    ~GpuSampler() { stopNvml(); }

    template<typename Cancelled>
    QString discover(Cancelled&& cancelled) {
        const QVector<GpuCandidate> candidates = discoverSysfsGpus();
        if (candidates.isEmpty() || cancelled())
            return {};

        GpuCandidate best = candidates.front();
        bool hasNvidia = false;
        bool nvidiaMayBeProbed = true;
        for (const GpuCandidate& candidate : candidates) {
            if (stronger(candidate, best))
                best = candidate;
            if (candidate.driver == QStringLiteral("nvidia")) {
                hasNvidia = true;
                m_nvidiaRuntimePaths.append(candidate.runtimeStatusPath);
                if (runtimeSuspended(candidate.runtimeStatusPath))
                    nvidiaMayBeProbed = false;
            }
        }
        m_devicePath = best.devicePath;
        m_selectedDriver = best.driver;

        NvmlChoice nvml;
        if (hasNvidia && nvidiaMayBeProbed && !cancelled()) {
            m_nvmlAttempted = true;
            nvml = startNvml();
        }
        if (nvml.valid) {
            GpuCandidate nvmlCandidate;
            nvmlCandidate.driver = QStringLiteral("nvidia");
            nvmlCandidate.name = nvml.name;
            nvmlCandidate.vram = nvml.memoryBytes;
            nvmlCandidate.kind = 1;
            if (stronger(nvmlCandidate, best) || best.driver == QStringLiteral("nvidia")) {
                m_backend = Backend::Nvml;
                m_nvmlDevice = nvml.device;
                m_name = nvml.name;
                return m_name;
            }
        }

        stopNvml();
        m_backend = Backend::Sysfs;
        m_runtimeStatusPath = best.runtimeStatusPath;
        m_utilizationPath = best.utilizationPath;
        m_memoryUsedPath = best.devicePath + QStringLiteral("/mem_info_vram_used");
        m_memoryTotalPath = best.devicePath + QStringLiteral("/mem_info_vram_total");
        if (!runtimeSuspended(m_runtimeStatusPath)) {
            m_temperature = discoverGpuTemperature(best.devicePath);
            m_powerPath = discoverHwmonMetric(
                best.devicePath, {QStringLiteral("power1_average"), QStringLiteral("power1_input")});
        }
        m_name = best.name;
        return m_name;
    }

    GpuReading sample(bool includePower) {
        GpuReading reading;
        if (m_backend == Backend::Sysfs) {
            if (runtimeSuspended(m_runtimeStatusPath))
                return reading;
            if (m_selectedDriver == QStringLiteral("nvidia") && !m_nvmlAttempted) {
                m_nvmlAttempted = true;
                const NvmlChoice nvml = startNvml();
                if (nvml.valid) {
                    m_backend = Backend::Nvml;
                    m_nvmlDevice = nvml.device;
                    m_name = nvml.name;
                    return sample(includePower);
                }
                stopNvml();
                m_backend = Backend::Sysfs;
            }
            if (m_temperature.inputPath.isEmpty())
                m_temperature = discoverGpuTemperature(m_devicePath);
            if (includePower && m_powerPath.isEmpty()) {
                m_powerPath = discoverHwmonMetric(
                    m_devicePath, {QStringLiteral("power1_average"), QStringLiteral("power1_input")});
            }
            if (!m_utilizationPath.isEmpty()) {
                bool ok = false;
                const double value = readSmallFile(m_utilizationPath).trimmed().toDouble(&ok);
                if (ok && std::isfinite(value)) {
                    reading.percent = std::clamp(value, 0.0, 100.0);
                    reading.available = true;
                }
            }
            reading.temperatureAvailable = readTemperature(m_temperature, reading.temperature);
            quint64 used = 0;
            quint64 total = 0;
            if (readUnsignedFile(m_memoryUsedPath, used) && readUnsignedFile(m_memoryTotalPath, total) && total > 0) {
                reading.memoryUsedGiB = static_cast<double>(std::min(used, total)) / BytesPerGiB;
                reading.memoryTotalGiB = static_cast<double>(total) / BytesPerGiB;
                reading.memoryAvailable = true;
            }
            if (includePower && !m_powerPath.isEmpty()) {
                quint64 microwatts = 0;
                if (readUnsignedFile(m_powerPath, microwatts))
                    reading.powerWatts = static_cast<double>(microwatts) / 1000000.0;
            }
            return reading;
        }
        if (m_backend != Backend::Nvml)
            return reading;
        for (const QString& path : std::as_const(m_nvidiaRuntimePaths)) {
            if (runtimeSuspended(path))
                return reading;
        }

        NvmlUtilization utilization{};
        if (m_getUtilization && m_getUtilization(m_nvmlDevice, &utilization) == NvmlSuccess) {
            reading.percent = std::min(100u, utilization.gpu);
            reading.available = true;
        }
        unsigned int temperature = 0;
        if (m_getTemperature && m_getTemperature(m_nvmlDevice, NvmlTemperatureGpu, &temperature) == NvmlSuccess) {
            reading.temperature = temperature;
            reading.temperatureAvailable = true;
        }
        NvmlMemory memory{};
        if (m_getMemory && m_getMemory(m_nvmlDevice, &memory) == NvmlSuccess && memory.total > 0) {
            reading.memoryUsedGiB = static_cast<double>(std::min(memory.used, memory.total)) / BytesPerGiB;
            reading.memoryTotalGiB = static_cast<double>(memory.total) / BytesPerGiB;
            reading.memoryAvailable = true;
        }
        if (includePower) {
            unsigned int powerMilliwatts = 0;
            if (m_getPowerUsage && m_getPowerUsage(m_nvmlDevice, &powerMilliwatts) == NvmlSuccess)
                reading.powerWatts = static_cast<double>(powerMilliwatts) / 1000.0;
        }
        return reading;
    }

private:
    enum class Backend { None, Sysfs, Nvml };
    using Init = NvmlReturn (*)();
    using Shutdown = NvmlReturn (*)();
    using GetCount = NvmlReturn (*)(unsigned int*);
    using GetHandle = NvmlReturn (*)(unsigned int, NvmlDevice*);
    using GetUtilization = NvmlReturn (*)(NvmlDevice, NvmlUtilization*);
    using GetName = NvmlReturn (*)(NvmlDevice, char*, unsigned int);
    using GetMemory = NvmlReturn (*)(NvmlDevice, NvmlMemory*);
    using GetTemperature = NvmlReturn (*)(NvmlDevice, unsigned int, unsigned int*);
    using GetPowerUsage = NvmlReturn (*)(NvmlDevice, unsigned int*);

    struct NvmlChoice {
        NvmlDevice device = nullptr;
        QString name;
        quint64 memoryBytes = 0;
        bool valid = false;
    };

    template<typename Function>
    Function resolve(const char* name) {
        return reinterpret_cast<Function>(m_nvml.resolve(name));
    }

    NvmlChoice startNvml() {
        m_nvml.setFileNameAndVersion(QStringLiteral("nvidia-ml"), 1);
        if (!m_nvml.load())
            return {};
        m_init = resolve<Init>("nvmlInit_v2");
        if (!m_init)
            m_init = resolve<Init>("nvmlInit");
        m_shutdown = resolve<Shutdown>("nvmlShutdown");
        m_getCount = resolve<GetCount>("nvmlDeviceGetCount_v2");
        if (!m_getCount)
            m_getCount = resolve<GetCount>("nvmlDeviceGetCount");
        m_getHandle = resolve<GetHandle>("nvmlDeviceGetHandleByIndex_v2");
        if (!m_getHandle)
            m_getHandle = resolve<GetHandle>("nvmlDeviceGetHandleByIndex");
        m_getUtilization = resolve<GetUtilization>("nvmlDeviceGetUtilizationRates");
        m_getName = resolve<GetName>("nvmlDeviceGetName");
        m_getMemory = resolve<GetMemory>("nvmlDeviceGetMemoryInfo");
        m_getTemperature = resolve<GetTemperature>("nvmlDeviceGetTemperature");
        m_getPowerUsage = resolve<GetPowerUsage>("nvmlDeviceGetPowerUsage");
        if (!m_init || !m_shutdown || !m_getCount || !m_getHandle || !m_getUtilization || m_init() != NvmlSuccess) {
            stopNvml();
            return {};
        }
        m_nvmlInitialized = true;

        unsigned int count = 0;
        if (m_getCount(&count) != NvmlSuccess)
            return {};
        NvmlChoice best;
        for (unsigned int index = 0; index < count; ++index) {
            NvmlDevice device = nullptr;
            if (m_getHandle(index, &device) != NvmlSuccess || !device)
                continue;
            NvmlMemory memory{};
            const quint64 total = m_getMemory && m_getMemory(device, &memory) == NvmlSuccess ? memory.total : 0;
            if (best.valid && total <= best.memoryBytes)
                continue;
            std::array<char, 128> name{};
            if (m_getName)
                m_getName(device, name.data(), name.size());
            best.device = device;
            best.memoryBytes = total;
            best.name = name[0] ? QString::fromUtf8(name.data()) : QStringLiteral("NVIDIA GPU");
            best.valid = true;
        }
        return best;
    }

    void stopNvml() {
        if (m_nvmlInitialized && m_shutdown)
            m_shutdown();
        m_backend = Backend::None;
        m_nvmlInitialized = false;
        m_nvmlDevice = nullptr;
        m_init = nullptr;
        m_shutdown = nullptr;
        m_getCount = nullptr;
        m_getHandle = nullptr;
        m_getUtilization = nullptr;
        m_getName = nullptr;
        m_getMemory = nullptr;
        m_getTemperature = nullptr;
        m_getPowerUsage = nullptr;
        if (m_nvml.isLoaded())
            m_nvml.unload();
    }

    Backend m_backend = Backend::None;
    QString m_name;
    QString m_devicePath;
    QString m_selectedDriver;
    QString m_utilizationPath;
    QString m_runtimeStatusPath;
    QString m_memoryUsedPath;
    QString m_memoryTotalPath;
    QString m_powerPath;
    QStringList m_nvidiaRuntimePaths;
    TemperatureSensor m_temperature;
    QLibrary m_nvml;
    NvmlDevice m_nvmlDevice = nullptr;
    Init m_init = nullptr;
    Shutdown m_shutdown = nullptr;
    GetCount m_getCount = nullptr;
    GetHandle m_getHandle = nullptr;
    GetUtilization m_getUtilization = nullptr;
    GetName m_getName = nullptr;
    GetMemory m_getMemory = nullptr;
    GetPowerUsage m_getPowerUsage = nullptr;
    GetTemperature m_getTemperature = nullptr;
    bool m_nvmlInitialized = false;
    bool m_nvmlAttempted = false;
};

struct NetworkCounters {
    quint64 received = 0;
    quint64 transmitted = 0;
};

struct NetworkReading {
    double receivedPerSecond = 0.0;
    double transmittedPerSecond = 0.0;
    QString busiestInterface;
    bool available = false;
};

bool ignoredNetworkInterface(const QString& name) {
    return name == QStringLiteral("lo") || name.startsWith(QStringLiteral("docker"))
        || name.startsWith(QStringLiteral("veth")) || name.startsWith(QStringLiteral("virbr"))
        || name.startsWith(QStringLiteral("br-"));
}

// Rates are measured over a one-second window that slides forward every sample
// period, so each reading is a full second of traffic re-evaluated four times a
// second: live without the jitter of a quarter-second window.
class NetworkSampler {
public:
    NetworkSampler(int windowSamples, bool includeVirtual)
        : m_windowSamples(std::max(1, windowSamples))
        , m_includeVirtual(includeVirtual) {}

    NetworkReading sample(qint64 monotonicMs) {
        QFile file(QStringLiteral("/proc/net/dev"));
        if (!file.open(QIODevice::ReadOnly)) {
            m_window.clear();
            return {};
        }

        Snapshot current;
        current.monotonicMs = monotonicMs;
        QByteArray line;
        while (!(line = file.readLine()).isEmpty()) {
            const qsizetype colon = line.indexOf(':');
            if (colon < 0)
                continue;
            const QString name = QString::fromLatin1(line.first(colon)).trimmed();
            if (name.isEmpty() || name == QStringLiteral("lo")
                || (!m_includeVirtual && ignoredNetworkInterface(name))) {
                continue;
            }
            const QList<QByteArray> fields = line.sliced(colon + 1).simplified().split(' ');
            if (fields.size() < 16)
                continue;
            bool rxOk = false;
            bool txOk = false;
            const quint64 received = fields[0].toULongLong(&rxOk);
            const quint64 transmitted = fields[8].toULongLong(&txOk);
            if (rxOk && txOk)
                current.counters.insert(name, {received, transmitted});
        }
        if (current.counters.isEmpty()) {
            m_window.clear();
            return {};
        }

        NetworkReading reading;
        if (!m_window.isEmpty()) {
            const Snapshot& oldest = m_window.first();
            const qint64 elapsedMs = monotonicMs - oldest.monotonicMs;
            quint64 receivedDelta = 0;
            quint64 transmittedDelta = 0;
            quint64 busiestDelta = 0;
            bool matchedPrevious = false;
            for (auto it = current.counters.constBegin(); it != current.counters.constEnd(); ++it) {
                const auto previous = oldest.counters.constFind(it.key());
                if (previous == oldest.counters.constEnd())
                    continue;
                matchedPrevious = true;
                const quint64 rx = it->received >= previous->received ? it->received - previous->received : 0;
                const quint64 tx = it->transmitted >= previous->transmitted ? it->transmitted - previous->transmitted : 0;
                receivedDelta += rx;
                transmittedDelta += tx;
                if (rx + tx > busiestDelta) {
                    busiestDelta = rx + tx;
                    reading.busiestInterface = it.key();
                }
            }
            if (matchedPrevious && elapsedMs > 0) {
                const double seconds = static_cast<double>(elapsedMs) / 1000.0;
                reading.receivedPerSecond = static_cast<double>(receivedDelta) / seconds;
                reading.transmittedPerSecond = static_cast<double>(transmittedDelta) / seconds;
                reading.available = true;
            }
        }
        m_window.append(std::move(current));
        while (m_window.size() > m_windowSamples + 1)
            m_window.removeFirst();
        return reading;
    }

private:
    struct Snapshot {
        qint64 monotonicMs = 0;
        QHash<QString, NetworkCounters> counters;
    };

    QVector<Snapshot> m_window;
    int m_windowSamples = 1;
    bool m_includeVirtual = false;
};

bool ignoredBlockDevice(const QString& name) {
    return name.startsWith(QStringLiteral("loop")) || name.startsWith(QStringLiteral("ram"))
        || name.startsWith(QStringLiteral("zram")) || name.startsWith(QStringLiteral("dm-"));
}

struct DiskReading {
    double readPerSecond = 0.0;
    double writePerSecond = 0.0;
    bool available = false;
};

class DiskSampler {
public:
    explicit DiskSampler(int windowSamples)
        : m_windowSamples(std::max(1, windowSamples)) {
        const QDir blocks(QStringLiteral("/sys/block"));
        const QFileInfoList entries = blocks.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
        for (const QFileInfo& entry : entries) {
            const QString name = entry.fileName();
            if (ignoredBlockDevice(name))
                continue;
            quint64 blockSize = 0;
            if (!readUnsignedFile(entry.absoluteFilePath() + QStringLiteral("/queue/logical_block_size"), blockSize)
                || blockSize == 0) {
                continue;
            }
            m_blockSizes.insert(name, blockSize);
        }
    }

    DiskReading sample(qint64 monotonicMs) {
        QFile file(QStringLiteral("/proc/diskstats"));
        if (!file.open(QIODevice::ReadOnly)) {
            m_window.clear();
            return {};
        }

        Snapshot current;
        current.monotonicMs = monotonicMs;
        QByteArray line;
        while (!(line = file.readLine()).isEmpty()) {
            const QList<QByteArray> fields = line.simplified().split(' ');
            if (fields.size() < 10)
                continue;
            const QString name = QString::fromLatin1(fields[2]);
            const auto blockSize = m_blockSizes.constFind(name);
            if (blockSize == m_blockSizes.constEnd())
                continue;
            bool readOk = false;
            bool writeOk = false;
            const quint64 readSectors = fields[5].toULongLong(&readOk);
            const quint64 writeSectors = fields[9].toULongLong(&writeOk);
            if (readOk && writeOk)
                current.bytes.insert(name, {readSectors * *blockSize, writeSectors * *blockSize});
        }
        if (current.bytes.isEmpty()) {
            m_window.clear();
            return {};
        }

        DiskReading reading;
        if (!m_window.isEmpty()) {
            const Snapshot& oldest = m_window.first();
            const qint64 elapsedMs = monotonicMs - oldest.monotonicMs;
            quint64 readBytes = 0;
            quint64 writeBytes = 0;
            bool matchedPrevious = false;
            for (auto it = current.bytes.constBegin(); it != current.bytes.constEnd(); ++it) {
                const auto previous = oldest.bytes.constFind(it.key());
                if (previous == oldest.bytes.constEnd())
                    continue;
                matchedPrevious = true;
                readBytes += it->read >= previous->read ? it->read - previous->read : 0;
                writeBytes += it->written >= previous->written ? it->written - previous->written : 0;
            }
            if (matchedPrevious && elapsedMs > 0) {
                const double seconds = static_cast<double>(elapsedMs) / 1000.0;
                reading.readPerSecond = static_cast<double>(readBytes) / seconds;
                reading.writePerSecond = static_cast<double>(writeBytes) / seconds;
                reading.available = true;
            }
        }
        m_window.append(std::move(current));
        while (m_window.size() > m_windowSamples + 1)
            m_window.removeFirst();
        return reading;
    }

private:
    struct DiskBytes {
        quint64 read = 0;
        quint64 written = 0;
    };

    struct Snapshot {
        qint64 monotonicMs = 0;
        QHash<QString, DiskBytes> bytes;
    };

    QHash<QString, quint64> m_blockSizes;
    QVector<Snapshot> m_window;
    int m_windowSamples = 1;
};

struct BatterySensor {
    QString capacityPath;
    QString statusPath;
};

BatterySensor discoverBattery() {
    const QDir supplies(QStringLiteral("/sys/class/power_supply"));
    const QFileInfoList entries = supplies.entryInfoList(QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& entry : entries) {
        const QString base = entry.absoluteFilePath();
        if (readTextFile(base + QStringLiteral("/type")).compare(QStringLiteral("Battery"), Qt::CaseInsensitive) == 0)
            return {base + QStringLiteral("/capacity"), base + QStringLiteral("/status")};
    }
    return {};
}

struct BatteryReading {
    double percent = 0.0;
    bool charging = false;
    bool available = false;
};

BatteryReading readBattery(const BatterySensor& sensor) {
    if (sensor.capacityPath.isEmpty())
        return {};
    const QByteArray capacityBytes = readSmallFile(sensor.capacityPath).trimmed();
    bool ok = false;
    const double capacity = capacityBytes.toDouble(&ok);
    if (!ok || !std::isfinite(capacity))
        return {};
    return {std::clamp(capacity, 0.0, 100.0),
            readTextFile(sensor.statusPath).compare(QStringLiteral("Charging"), Qt::CaseInsensitive) == 0, true};
}

} // namespace

SystemMonitor::SystemMonitor(QObject* parent)
    : QObject(parent) {
    startWorker();
}

SystemMonitor::~SystemMonitor() {
    m_workerActive.store(false, std::memory_order_release);
    m_generation.fetch_add(1, std::memory_order_relaxed);
    stopWorker();
}

void SystemMonitor::setActive(bool active) {
    if (m_active == active)
        return;

    m_active = active;
    m_generation.fetch_add(1, std::memory_order_relaxed);
    m_workerActive.store(active, std::memory_order_release);
    clearValues();
    emit samplesChanged();
    emit activeChanged();
    m_workerWake.notify_all();
}

void SystemMonitor::setSamplePeriodMs(int periodMs) {
    periodMs = std::clamp(periodMs, 100, 60000);
    if (m_samplePeriodMs.load(std::memory_order_relaxed) == periodMs)
        return;

    m_samplePeriodMs.store(periodMs, std::memory_order_relaxed);
    m_generation.fetch_add(1, std::memory_order_relaxed);
    clearValues();
    emit samplesChanged();
    emit samplePeriodMsChanged();
    m_workerWake.notify_all();
}

void SystemMonitor::setHistoryLimit(int limit) {
    limit = std::clamp(limit, 0, 4096);
    if (m_historyLimit == limit)
        return;
    m_historyLimit = limit;
    const auto trim = [limit](QVariantList& history) {
        while (history.size() > limit)
            history.removeFirst();
    };
    trim(m_cpuHistory);
    trim(m_networkRxHistory);
    trim(m_networkTxHistory);
    emit historyLimitChanged();
    emit samplesChanged();
}

void SystemMonitor::appendHistory(QVariantList& history, double value) {
    if (m_historyLimit <= 0)
        return;
    history.append(QVariant::fromValue(value));
    while (history.size() > m_historyLimit)
        history.removeFirst();
}

void SystemMonitor::setIncludeVirtualNetwork(bool include) {
    if (m_includeVirtualNetwork.load(std::memory_order_relaxed) == include)
        return;
    m_includeVirtualNetwork.store(include, std::memory_order_relaxed);
    m_generation.fetch_add(1, std::memory_order_relaxed);
    clearValues();
    emit samplesChanged();
    emit includeVirtualNetworkChanged();
    m_workerWake.notify_all();
}


void SystemMonitor::clearValues() {
    m_cpuPercent = 0.0;
    m_memoryPercent = 0.0;
    m_gpuPercent = 0.0;
    m_cpuAvailable = false;
    m_memoryAvailable = false;
    m_gpuAvailable = false;
    m_gpuPowerWatts = 0.0;
    m_fanRpm = 0.0;
    m_cpuName.clear();
    m_gpuName.clear();
    m_userName.clear();
    m_hostName.clear();
    m_uptimeSeconds = 0;
    m_memoryUsedGiB = 0.0;
    m_memoryTotalGiB = 0.0;
    m_storageUsedGiB = 0.0;
    m_storageTotalGiB = 0.0;
    m_coreLoads.clear();
    m_coreCount = 0;
    m_cpuFrequencyGhz = 0.0;
    m_loadAverage = 0.0;
    m_processCount = 0;
    m_swapUsedGiB = 0.0;
    m_swapTotalGiB = 0.0;
    m_cpuTemp = 0.0;
    m_cpuTempAvailable = false;
    m_cpuTempLabel.clear();
    m_gpuTemp = 0.0;
    m_gpuTempAvailable = false;
    m_gpuMemoryUsedGiB = 0.0;
    m_gpuMemoryTotalGiB = 0.0;
    m_gpuMemoryAvailable = false;
    m_storageTemp = 0.0;
    m_storageTempAvailable = false;
    m_networkRxBytesPerSec = 0.0;
    m_networkTxBytesPerSec = 0.0;
    m_networkAvailable = false;
    m_networkInterface.clear();
    m_diskReadBytesPerSec = 0.0;
    m_diskWriteBytesPerSec = 0.0;
    m_diskAvailable = false;
    m_batteryPercent = 0.0;
    m_batteryCharging = false;
    m_batteryAvailable = false;
    m_historyStart = 0;
    m_historyCount = 0;
}

void SystemMonitor::startWorker() {
    m_worker = std::jthread([this](std::stop_token stop) {
        while (!stop.stop_requested()) {
            std::unique_lock activationLock(m_workerMutex);
            m_workerWake.wait(activationLock, stop, [this] {
                return m_workerActive.load(std::memory_order_acquire);
            });
            if (stop.stop_requested())
                break;
            const quint64 generation = m_generation.load(std::memory_order_relaxed);
            activationLock.unlock();

            const auto cancelled = [this, stop, generation] {
                return stop.stop_requested() || !m_workerActive.load(std::memory_order_acquire)
                    || generation != m_generation.load(std::memory_order_relaxed);
            };
            const int periodMs = m_samplePeriodMs.load(std::memory_order_relaxed);
            const int rateWindowSamples = std::max(1, (1000 + periodMs - 1) / periodMs);
            const bool includeVirtualNetwork = m_includeVirtualNetwork.load(std::memory_order_relaxed);

            const QVector<HwmonTemperature> hwmonTemperatures = discoverHwmonTemperatures();
            const TemperatureSensor cpuTemperature = discoverCpuTemperature(hwmonTemperatures);
            const TemperatureSensor storageTemperature = discoverStorageTemperature(hwmonTemperatures);
            const QVector<FrequencySource> frequencySources = discoverFrequencySources();
            const BatterySensor batterySensor = discoverBattery();
            const QString fanPath = discoverFanPath();
            DiskSampler disk(rateWindowSamples);
            NetworkSampler network(rateWindowSamples, includeVirtualNetwork);
            GpuSampler gpu;
            if (cancelled())
                continue;

            IdentityResult identity;
            identity.cpuName = cpuModelName();
            identity.gpuName = gpu.discover(cancelled);
            identity.userName = currentUserName();
            identity.hostName = currentHostName();
            if (cancelled())
                continue;
            QMetaObject::invokeMethod(this, [this, generation, identity = std::move(identity)]() mutable {
                applyIdentity(generation, std::move(identity));
            }, Qt::QueuedConnection);

            // CPU load is the busy share over the last second, re-evaluated every
            // sample period: the snapshot window holds one second of history so a
            // quarter-second sample never shows a quarter-second spike.
            QVector<CpuSnapshot> cpuWindow;
            // Sensors that cost a device query (hwmon, battery, statvfs) and values
            // that only move per second are refreshed once a second and carried.
            struct SlowReadings {
                double cpuTemp = 0.0;
                bool cpuTempAvailable = false;
                double storageTemp = 0.0;
                bool storageTempAvailable = false;
                double gpuPowerWatts = 0.0;
                double fanRpm = 0.0;
                BatteryReading battery;
                LoadReading load;
                StorageReading storage;
                qulonglong uptimeSeconds = 0;
            } slow;
            int tick = 0;
            auto nextSample = Clock::now();
            while (!cancelled()) {
                SampleResult result;
                result.monotonicMs = monotonicMilliseconds();

                CpuSnapshot cpu = readCpuSnapshot();
                if (!cpuWindow.isEmpty()) {
                    const CpuSnapshot& oldest = cpuWindow.first();
                    result.cpuPercent = cpuLoad(cpu.aggregate, oldest.aggregate, result.cpuAvailable);
                    if (cpu.cores.size() == oldest.cores.size() && !cpu.cores.isEmpty()) {
                        result.coreLoads.reserve(cpu.cores.size());
                        bool allAvailable = true;
                        for (qsizetype index = 0; index < cpu.cores.size(); ++index) {
                            bool available = false;
                            const double load = cpuLoad(cpu.cores[index], oldest.cores[index], available);
                            if (!available) {
                                allAvailable = false;
                                break;
                            }
                            result.coreLoads.append(load);
                        }
                        if (!allAvailable)
                            result.coreLoads.clear();
                    }
                }
                result.coreCount = cpu.cores.size();
                cpuWindow.append(std::move(cpu));
                while (cpuWindow.size() > rateWindowSamples + 1)
                    cpuWindow.removeFirst();
                result.cpuFrequencyGhz = readMeanFrequencyGhz(frequencySources);

                const MemoryReading memory = readMemory();
                result.memoryPercent = memory.percent;
                result.memoryUsedGiB = memory.usedGiB;
                result.memoryTotalGiB = memory.totalGiB;
                result.swapUsedGiB = memory.swapUsedGiB;
                result.swapTotalGiB = memory.swapTotalGiB;
                result.memoryAvailable = memory.available;

                const bool refreshSlow = tick % rateWindowSamples == 0;
                if (refreshSlow) {
                    slow.load = readLoad();
                    slow.cpuTempAvailable = readTemperature(cpuTemperature, slow.cpuTemp);
                    slow.storageTempAvailable = readTemperature(storageTemperature, slow.storageTemp);
                    slow.battery = readBattery(batterySensor);
                    slow.uptimeSeconds = readUptimeSeconds();
                    slow.storage = readRootStorage();
                    if (!fanPath.isEmpty()) {
                        quint64 fanRpm = 0;
                        slow.fanRpm = readUnsignedFile(fanPath, fanRpm) ? static_cast<double>(fanRpm) : 0.0;
                    }
                }
                ++tick;
                result.loadAverage = slow.load.average;
                result.processCount = slow.load.processCount;
                result.cpuTempLabel = cpuTemperature.label;
                result.cpuTemp = slow.cpuTemp;
                result.cpuTempAvailable = slow.cpuTempAvailable;
                result.storageTemp = slow.storageTemp;
                result.storageTempAvailable = slow.storageTempAvailable;
                result.batteryPercent = slow.battery.percent;
                result.batteryCharging = slow.battery.charging;
                result.batteryAvailable = slow.battery.available;
                result.uptimeSeconds = slow.uptimeSeconds;
                result.storageUsedGiB = slow.storage.usedGiB;
                result.storageTotalGiB = slow.storage.totalGiB;

                const GpuReading gpuReading = gpu.sample(refreshSlow);
                result.gpuPercent = gpuReading.percent;
                result.gpuAvailable = gpuReading.available;
                result.gpuTemp = gpuReading.temperature;
                result.gpuTempAvailable = gpuReading.temperatureAvailable;
                result.gpuMemoryUsedGiB = gpuReading.memoryUsedGiB;
                result.gpuMemoryTotalGiB = gpuReading.memoryTotalGiB;
                result.gpuMemoryAvailable = gpuReading.memoryAvailable;
                if (refreshSlow)
                    slow.gpuPowerWatts = gpuReading.powerWatts;
                result.gpuPowerWatts = slow.gpuPowerWatts;
                result.fanRpm = slow.fanRpm;

                const NetworkReading networkReading = network.sample(result.monotonicMs);
                result.networkRxBytesPerSec = networkReading.receivedPerSecond;
                result.networkTxBytesPerSec = networkReading.transmittedPerSecond;
                result.networkInterface = networkReading.busiestInterface;
                result.networkAvailable = networkReading.available;

                const DiskReading diskReading = disk.sample(result.monotonicMs);
                result.diskReadBytesPerSec = diskReading.readPerSecond;
                result.diskWriteBytesPerSec = diskReading.writePerSecond;
                result.diskAvailable = diskReading.available;

                if (!cancelled()) {
                    QMetaObject::invokeMethod(
                        this, [this, generation, result] { applySample(generation, result); }, Qt::QueuedConnection);
                }

                nextSample += std::chrono::milliseconds(periodMs);
                const auto finished = Clock::now();
                if (nextSample <= finished)
                    nextSample = finished + std::chrono::milliseconds(periodMs);
                std::unique_lock sampleLock(m_workerMutex);
                m_workerWake.wait_until(sampleLock, stop, nextSample, [this, generation] {
                    return !m_workerActive.load(std::memory_order_acquire)
                        || generation != m_generation.load(std::memory_order_relaxed);
                });
            }
        }
    });
}

void SystemMonitor::stopWorker() {
    if (!m_worker.joinable())
        return;
    m_worker.request_stop();
    m_workerWake.notify_all();
    m_worker.join();
    m_worker = std::jthread{};
}

void SystemMonitor::applyIdentity(quint64 generation, IdentityResult result) {
    if (!m_active || generation != m_generation.load(std::memory_order_relaxed))
        return;
    m_cpuName = std::move(result.cpuName);
    m_gpuName = std::move(result.gpuName);
    m_userName = std::move(result.userName);
    m_hostName = std::move(result.hostName);
    emit samplesChanged();
}

void SystemMonitor::applySample(quint64 generation, const SampleResult& result) {
    if (!m_active || generation != m_generation.load(std::memory_order_relaxed))
        return;

    m_cpuPercent = result.cpuPercent;
    m_memoryPercent = result.memoryPercent;
    m_gpuPercent = result.gpuPercent;
    m_cpuAvailable = result.cpuAvailable;
    m_memoryAvailable = result.memoryAvailable;
    m_gpuAvailable = result.gpuAvailable;
    m_gpuPowerWatts = result.gpuPowerWatts;
    m_fanRpm = result.fanRpm;
    m_uptimeSeconds = result.uptimeSeconds;
    m_memoryUsedGiB = result.memoryUsedGiB;
    m_memoryTotalGiB = result.memoryTotalGiB;
    m_storageUsedGiB = result.storageUsedGiB;
    m_storageTotalGiB = result.storageTotalGiB;
    m_coreLoads.clear();
    m_coreLoads.reserve(result.coreLoads.size());
    for (double load : result.coreLoads)
        m_coreLoads.append(QVariant::fromValue(load));
    m_coreCount = result.coreCount;
    m_cpuFrequencyGhz = result.cpuFrequencyGhz;
    m_loadAverage = result.loadAverage;
    m_processCount = result.processCount;
    m_swapUsedGiB = result.swapUsedGiB;
    m_swapTotalGiB = result.swapTotalGiB;
    m_cpuTemp = result.cpuTemp;
    m_cpuTempAvailable = result.cpuTempAvailable;
    m_cpuTempLabel = result.cpuTempLabel;
    m_gpuTemp = result.gpuTemp;
    m_gpuTempAvailable = result.gpuTempAvailable;
    m_gpuMemoryUsedGiB = result.gpuMemoryUsedGiB;
    m_gpuMemoryTotalGiB = result.gpuMemoryTotalGiB;
    m_gpuMemoryAvailable = result.gpuMemoryAvailable;
    m_storageTemp = result.storageTemp;
    m_storageTempAvailable = result.storageTempAvailable;
    m_networkRxBytesPerSec = result.networkRxBytesPerSec;
    m_networkTxBytesPerSec = result.networkTxBytesPerSec;
    m_networkAvailable = result.networkAvailable;
    if (result.cpuAvailable)
        appendHistory(m_cpuHistory, result.cpuPercent / 100.0);
    if (result.networkAvailable) {
        appendHistory(m_networkRxHistory, result.networkRxBytesPerSec);
        appendHistory(m_networkTxHistory, result.networkTxBytesPerSec);
    }
    m_networkInterface = result.networkInterface;
    m_diskReadBytesPerSec = result.diskReadBytesPerSec;
    m_diskWriteBytesPerSec = result.diskWriteBytesPerSec;
    m_diskAvailable = result.diskAvailable;
    m_batteryPercent = result.batteryPercent;
    m_batteryCharging = result.batteryCharging;
    m_batteryAvailable = result.batteryAvailable;

    const int index = (m_historyStart + m_historyCount) % HistoryCapacity;
    HistorySample history;
    history.monotonicMs = result.monotonicMs;
    history.cpu = result.cpuAvailable ? static_cast<float>(result.cpuPercent) : std::numeric_limits<float>::quiet_NaN();
    history.memory = result.memoryAvailable ? static_cast<float>(result.memoryPercent)
                                            : std::numeric_limits<float>::quiet_NaN();
    history.gpu = result.gpuAvailable ? static_cast<float>(result.gpuPercent) : std::numeric_limits<float>::quiet_NaN();
    history.netRx = result.networkAvailable ? static_cast<float>(result.networkRxBytesPerSec)
                                            : std::numeric_limits<float>::quiet_NaN();
    history.netTx = result.networkAvailable ? static_cast<float>(result.networkTxBytesPerSec)
                                            : std::numeric_limits<float>::quiet_NaN();
    history.diskRead = result.diskAvailable ? static_cast<float>(result.diskReadBytesPerSec)
                                            : std::numeric_limits<float>::quiet_NaN();
    history.diskWrite = result.diskAvailable ? static_cast<float>(result.diskWriteBytesPerSec)
                                             : std::numeric_limits<float>::quiet_NaN();
    history.cpuTemp = result.cpuTempAvailable ? static_cast<float>(result.cpuTemp)
                                              : std::numeric_limits<float>::quiet_NaN();
    history.gpuTemp = result.gpuTempAvailable ? static_cast<float>(result.gpuTemp)
                                              : std::numeric_limits<float>::quiet_NaN();
    if (m_historyCount < HistoryCapacity) {
        m_history[index] = history;
        ++m_historyCount;
    } else {
        m_history[m_historyStart] = history;
        m_historyStart = (m_historyStart + 1) % HistoryCapacity;
    }
    emit samplesChanged();
}

const SystemMonitor::HistorySample& SystemMonitor::historyAtOldest(int index) const {
    Q_ASSERT(index >= 0 && index < m_historyCount);
    return m_history[(m_historyStart + index) % HistoryCapacity];
}
