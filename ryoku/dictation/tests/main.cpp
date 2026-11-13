// Unit tests for the parts of ryoku-dictate that do not need a microphone, a
// display or a model: settings parsing, UTF-8, the keymap we upload, and WAV
// reading. The engine is proved by `--selftest` against a real clip, which the
// package's behaviour test drives.

#include "config.hpp"
#include "insert.hpp"
#include "wav.hpp"

#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <string>
#include <vector>

#include <xkbcommon/xkbcommon.h>

namespace {

int failures = 0;

void check(bool ok, const char* what) {
    if (ok) {
        std::printf("ok   %s\n", what);
        return;
    }
    std::printf("FAIL %s\n", what);
    failures++;
}

void writeFile(const std::string& path, const std::string& body) {
    std::ofstream file(path);
    file << body;
}

void testConfig() {
    const char* home = "/tmp/ryoku-dictate-test";
    ::setenv("XDG_CONFIG_HOME", home, 1);
    ::setenv("HOME", home, 1);
    ::system("mkdir -p /tmp/ryoku-dictate-test/ryoku");

    writeFile(std::string(home) + "/ryoku/dictation.conf",
              "# a comment\n"
              "enabled=false\n"
              "device=gpu\n"
              "mode=type\n"
              "language=de\n"
              "max_duration_secs=9999\n"
              "silence_ms=-4\n"
              "silence_db=-60\n"
              "type_delay_ms=7\n"
              "nonsense=ignored\n");

    const auto config = ryoku::Config::load();
    check(!config.enabled, "enabled=false is read");
    check(config.device == "gpu", "device=gpu is read");
    check(config.mode == "type", "mode=type is read");
    check(config.language == "de", "language is read");
    check(config.maxDurationSecs == 120, "out-of-range duration keeps the default");
    check(config.silenceMs == 1500, "negative silence keeps the default");
    check(config.silenceDb == -60, "silence_db is read");
    check(config.typeDelayMs == 7, "type_delay_ms is read");

    // An unknown device falls back rather than making the file unusable.
    writeFile(std::string(home) + "/ryoku/dictation.conf", "device=quantum\nmode=sing\n");
    const auto fallback = ryoku::Config::load();
    check(fallback.device == "auto", "an unknown device keeps auto");
    check(fallback.mode == "paste", "an unknown mode keeps paste");

    std::remove((std::string(home) + "/ryoku/dictation.conf").c_str());
    const auto defaults = ryoku::Config::load();
    check(defaults.enabled && defaults.device == "auto" && defaults.mode == "paste",
          "a missing file is the defaults");
}

void testUtf8() {
    const auto plain = ryoku::utf8Codepoints("Hello, world.");
    check(plain.size() == 13, "ascii has one code point per byte");
    check(plain[0] == 'H', "first ascii byte");

    const auto accents = ryoku::utf8Codepoints("café — ok");
    check(accents.size() == 9, "multi-byte code points count once");
    check(accents[3] == 0xE9, "é decodes to U+00E9");
    check(accents[5] == 0x2014, "em dash decodes");

    const auto control = ryoku::utf8Codepoints(std::string("ab\x01" "c\n"));
    check(control.size() == 4, "control bytes are dropped, newline kept");

    const auto truncated = ryoku::utf8Codepoints(std::string("ok\xE2\x82"));
    check(truncated.size() == 2, "a truncated sequence is dropped");

    check(ryoku::keysymName('a') == "U0061", "ascii keysym name");
    check(ryoku::keysymName(0x00E9) == "U00E9", "accented keysym name");
    check(ryoku::keysymName('\n') == "Return", "newline maps to Return");
    check(ryoku::keysymName(' ') == "space", "space keeps its name");
}

void testKeymap() {
    const auto keymap = ryoku::buildKeymap({{9, "U0061"}, {10, "U0062"}});
    check(keymap.find("xkb_keymap") == 0, "keymap starts with xkb_keymap");
    check(keymap.find("<K17> = 17;") != std::string::npos, "evdev 9 is xkb keycode 17");
    check(keymap.find("key <K17> {[ U0061 ]};") != std::string::npos, "symbol row for a");
    check(keymap.find("key <K18> {[ U0062 ]};") != std::string::npos, "symbol row for b");
    check(keymap.find("include \"complete\"") != std::string::npos, "types and compat included");

    // A keymap the engine can actually compile is the real contract.
    struct xkb_context* context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    struct xkb_keymap* compiled =
        xkb_keymap_new_from_string(context, keymap.c_str(), XKB_KEYMAP_FORMAT_TEXT_V1,
                                   XKB_KEYMAP_COMPILE_NO_FLAGS);
    check(compiled != nullptr, "xkbcommon compiles the keymap");
    xkb_keymap_unref(compiled);
    xkb_context_unref(context);
}

void writeTinyWav(const std::string& path, int rate, int channels, int frames) {
    std::ofstream file(path, std::ios::binary);
    const int dataBytes = frames * channels * 2;
    auto put32 = [&](int value) {
        char bytes[4] = {char(value & 0xFF), char((value >> 8) & 0xFF),
                         char((value >> 16) & 0xFF), char((value >> 24) & 0xFF)};
        file.write(bytes, 4);
    };
    auto put16 = [&](int value) {
        char bytes[2] = {char(value & 0xFF), char((value >> 8) & 0xFF)};
        file.write(bytes, 2);
    };
    file.write("RIFF", 4);
    put32(36 + dataBytes);
    file.write("WAVE", 4);
    file.write("fmt ", 4);
    put32(16);
    put16(1);
    put16(channels);
    put32(rate);
    put32(rate * channels * 2);
    put16(channels * 2);
    put16(16);
    file.write("data", 4);
    put32(dataBytes);
    for (int i = 0; i < frames; i++) {
        for (int ch = 0; ch < channels; ch++)
            put16(ch == 0 ? 16384 : 0);
    }
}

void testWav() {
    const std::string path = "/tmp/ryoku-dictate-test/stereo48k.wav";
    writeTinyWav(path, 48000, 2, 4800); // 0.1 s
    std::vector<float> pcm;
    std::string error;
    check(ryoku::readWav16k(path, pcm, error), "a 48 kHz stereo WAV reads");
    check(std::abs(int(pcm.size()) - 1600) <= 1, "it resamples to 16 kHz");
    check(!pcm.empty() && pcm[0] > 0.2f && pcm[0] < 0.3f, "it downmixes stereo to mono (left 0.5, right 0 averaged)");

    std::string bad;
    check(!ryoku::readWav16k("/tmp/ryoku-dictate-test/nope.wav", pcm, bad),
          "a missing file fails");
}

} // namespace

int main() {
    testConfig();
    testUtf8();
    testKeymap();
    testWav();
    std::printf("%s (%d failure%s)\n", failures == 0 ? "PASS" : "FAIL", failures,
                failures == 1 ? "" : "s");
    return failures == 0 ? 0 : 1;
}
