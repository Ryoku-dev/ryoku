#include "wav.hpp"

#include <cstdint>
#include <cstring>
#include <fstream>
#include <vector>

namespace ryoku {
namespace {

uint32_t le32(const unsigned char* p) {
    return p[0] | (p[1] << 8) | (p[2] << 16) | (uint32_t(p[3]) << 24);
}
uint16_t le16(const unsigned char* p) {
    return uint16_t(p[0] | (p[1] << 8));
}

struct Format {
    uint16_t channels = 1;
    uint32_t rate = 16000;
    uint16_t bits = 16;
    uint16_t encoding = 1; // 1 = PCM, 3 = IEEE float
};

// One linear pass: the engine only cares that the result is 16 kHz mono, and a
// dictation clip is short enough that windowed sinc would be vanity.
void resample(const std::vector<float>& in, uint32_t from, std::vector<float>& out) {
    if (in.empty() || from == 0 || from == 16000) {
        out = in;
        return;
    }
    const double ratio = 16000.0 / double(from);
    const size_t count = size_t(in.size() * ratio);
    out.resize(count);
    for (size_t i = 0; i < count; i++) {
        const double at = double(i) / ratio;
        const size_t a = size_t(at);
        const double f = at - double(a);
        const float x = a < in.size() ? in[a] : in.back();
        const float y = a + 1 < in.size() ? in[a + 1] : x;
        out[i] = float(x + (y - x) * f);
    }
}

} // namespace

bool readWav16k(const std::string& path, std::vector<float>& out, std::string& err) {
    std::ifstream file(path, std::ios::binary);
    if (!file) {
        err = "cannot open " + path;
        return false;
    }
    std::vector<unsigned char> bytes((std::istreambuf_iterator<char>(file)),
                                     std::istreambuf_iterator<char>());
    if (bytes.size() < 44 || std::memcmp(bytes.data(), "RIFF", 4) != 0
        || std::memcmp(bytes.data() + 8, "WAVE", 4) != 0) {
        err = "not a WAV file";
        return false;
    }

    Format format;
    bool haveFormat = false;
    const unsigned char* data = nullptr;
    size_t dataSize = 0;
    size_t at = 12;
    while (at + 8 <= bytes.size()) {
        const unsigned char* chunk = bytes.data() + at;
        const uint32_t size = le32(chunk + 4);
        const size_t body = at + 8;
        if (std::memcmp(chunk, "fmt ", 4) == 0 && body + 16 <= bytes.size()) {
            format.encoding = le16(bytes.data() + body);
            format.channels = le16(bytes.data() + body + 2);
            format.rate = le32(bytes.data() + body + 4);
            format.bits = le16(bytes.data() + body + 14);
            haveFormat = true;
        } else if (std::memcmp(chunk, "data", 4) == 0) {
            data = bytes.data() + body;
            dataSize = std::min<size_t>(size, bytes.size() - body);
        }
        at = body + size + (size & 1);
    }
    if (!haveFormat || !data || dataSize == 0) {
        err = "WAV has no fmt/data chunk";
        return false;
    }
    if (format.channels == 0 || format.channels > 8) {
        err = "unsupported channel count";
        return false;
    }

    const size_t frames = dataSize / (format.channels * (format.bits / 8));
    std::vector<float> mono(frames);
    for (size_t i = 0; i < frames; i++) {
        float sum = 0;
        for (uint16_t ch = 0; ch < format.channels; ch++) {
            const size_t index = i * format.channels + ch;
            if (format.encoding == 3 && format.bits == 32) {
                float sample;
                std::memcpy(&sample, data + index * 4, 4);
                sum += sample;
            } else if (format.bits == 16) {
                int16_t sample;
                std::memcpy(&sample, data + index * 2, 2);
                sum += float(sample) / 32768.0f;
            } else if (format.bits == 32) {
                int32_t sample;
                std::memcpy(&sample, data + index * 4, 4);
                sum += float(double(sample) / 2147483648.0);
            } else if (format.bits == 24) {
                const unsigned char* p = data + index * 3;
                int32_t sample = (p[0] << 8) | (p[1] << 16) | (int32_t(p[2]) << 24);
                sum += float(double(sample >> 8) / 8388608.0);
            } else {
                err = "unsupported sample width";
                return false;
            }
        }
        mono[i] = sum / float(format.channels);
    }
    resample(mono, format.rate, out);
    return true;
}

} // namespace ryoku
