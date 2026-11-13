#pragma once

#include <cstdint>
#include <string>
#include <utility>
#include <vector>

struct wl_display;
struct wl_registry;
struct wl_seat;
struct zwp_virtual_keyboard_manager_v1;
struct zwp_virtual_keyboard_v1;

namespace ryoku {

// Typing into the focused window through the Wayland virtual-keyboard protocol.
//
// This is what wtype does, done in-process so the package needs no helper: one
// keymap carrying the characters of the batch, one press/release per character.
// The app keeps its own layout; nothing here touches XKB state outside the
// keymap we upload and the keys we send.
class Inserter {
public:
    Inserter() = default;
    ~Inserter();
    Inserter(const Inserter&) = delete;
    Inserter& operator=(const Inserter&) = delete;

    bool connect(std::string& err);
    void close();
    bool ready() const { return keyboard != nullptr; }

    bool type(const std::string& utf8, int delayMs, std::string& err);
    // Sends Ctrl+V. The caller puts the text on the clipboard first.
    bool pasteShortcut(std::string& err);

private:
    bool upload(const std::vector<std::pair<unsigned, std::string>>& keys, std::string& err);
    bool sendKey(unsigned evdev, bool down);
    void sync();

    wl_display* display = nullptr;
    wl_registry* registry = nullptr;
    wl_seat* seat = nullptr;
    zwp_virtual_keyboard_manager_v1* manager = nullptr;
    zwp_virtual_keyboard_v1* keyboard = nullptr;
    uint32_t serial = 0;
};

// UTF-8 to code points; invalid bytes are dropped rather than typed as noise.
std::vector<uint32_t> utf8Codepoints(const std::string& text);
// The XKB keysym name for one code point, e.g. "U00E9", "Return".
std::string keysymName(uint32_t codepoint);
// A complete xkb_keymap for the given (evdev code, keysym name) pairs.
std::string buildKeymap(const std::vector<std::pair<unsigned, std::string>>& keys);

// Puts text on the Wayland clipboard by way of wl-copy. False when wl-clipboard
// is missing, which the daemon reports instead of silently losing the words.
bool setClipboard(const std::string& utf8, std::string& err);

} // namespace ryoku
