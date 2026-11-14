#pragma once

#include <cstdint>
#include <string>
#include <utility>
#include <vector>

struct wl_display;
struct wl_registry;
struct wl_seat;
struct wl_keyboard;
struct xkb_context;
struct xkb_keymap;
struct zwp_virtual_keyboard_manager_v1;
struct zwp_virtual_keyboard_v1;

namespace ryoku {

// Typing into the focused window through the Wayland virtual-keyboard protocol.
//
// Text uses the seat's layout when it can represent the requested characters;
// a temporary Unicode map handles characters outside that layout.
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
    static void keymapReceived(void*, wl_keyboard*, uint32_t, int32_t, uint32_t);

    wl_display* display = nullptr;
    wl_registry* registry = nullptr;
    wl_seat* seat = nullptr;
    wl_keyboard* seatKeyboard = nullptr;
    xkb_context* context = nullptr;
    xkb_keymap* seatKeymap = nullptr;
    int seatKeymapFd = -1;
    uint32_t seatKeymapSize = 0;
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

struct KeyStroke {
    unsigned evdev = 0;
    uint32_t modifiers = 0;
    uint32_t group = 0;
};

bool keyStrokeFor(xkb_keymap* keymap, uint32_t codepoint, KeyStroke& stroke);

// Puts text on the Wayland clipboard by way of wl-copy. False when wl-clipboard
// is missing, which the daemon reports instead of silently losing the words.
bool setClipboard(const std::string& utf8, std::string& err);

} // namespace ryoku
