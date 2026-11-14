#include "insert.hpp"

#include "virtual-keyboard-unstable-v1-client-protocol.h"

#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-keysyms.h>

#include <cerrno>
#include <chrono>
#include <csignal>
#include <cstdio>
#include <cstring>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/wait.h>
#include <unistd.h>

#include <algorithm>
#include <map>
#include <string_view>

namespace ryoku {
namespace {

// Evdev codes for the printable block, minus Escape/Backspace/Tab/Enter and the
// modifiers: literal text is never put on a key that also means something.
constexpr unsigned textCodes[] = {
    2,  3,  4,  5,  6,  7,  8,  9,  10, 11, 12, 13, 16, 17, 18, 19, 20, 21, 22,
    23, 24, 25, 26, 27, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 43, 44,
    45, 46, 47, 48, 49, 50, 51, 52, 53, 57,
};

constexpr unsigned codeControl = 29;
constexpr unsigned codeV = 47;

} // namespace

std::vector<uint32_t> utf8Codepoints(const std::string& text) {
    std::vector<uint32_t> out;
    out.reserve(text.size());
    size_t i = 0;
    while (i < text.size()) {
        const auto lead = static_cast<unsigned char>(text[i]);
        uint32_t cp = 0;
        size_t extra = 0;
        if (lead < 0x80) {
            cp = lead;
        } else if ((lead & 0xE0) == 0xC0) {
            cp = lead & 0x1F;
            extra = 1;
        } else if ((lead & 0xF0) == 0xE0) {
            cp = lead & 0x0F;
            extra = 2;
        } else if ((lead & 0xF8) == 0xF0) {
            cp = lead & 0x07;
            extra = 3;
        } else {
            i++;
            continue;
        }
        if (i + extra >= text.size()) {
            // Truncated sequence: the tail of a cut-off transcription.
            break;
        }
        bool valid = true;
        for (size_t k = 1; k <= extra; k++) {
            const auto next = static_cast<unsigned char>(text[i + k]);
            if ((next & 0xC0) != 0x80) {
                valid = false;
                break;
            }
            cp = (cp << 6) | (next & 0x3F);
        }
        i += extra + 1;
        if (!valid)
            continue;
        // Control characters other than a newline are mistranscriptions, not
        // something to type into the reader's app.
        if (cp < 32 && cp != '\n' && cp != '\t')
            continue;
        out.push_back(cp);
    }
    return out;
}

std::string keysymName(uint32_t codepoint) {
    if (codepoint == '\n')
        return "Return";
    if (codepoint == '\t')
        return "Tab";
    if (codepoint == ' ')
        return "space";
    char name[16];
    std::snprintf(name, sizeof(name), "U%04X", codepoint);
    return name;
}

std::string buildKeymap(const std::vector<std::pair<unsigned, std::string>>& keys) {
    unsigned maximum = 255;
    for (const auto& [evdev, name] : keys) {
        (void)name;
        maximum = std::max(maximum, evdev + 8);
    }

    std::string out;
    out += "xkb_keymap {\n";
    // an identical map can stay cached after focus restores the physical layout
    const auto generation = std::chrono::steady_clock::now().time_since_epoch().count();
    out += "xkb_keycodes \"ryoku_" + std::to_string(generation) + "\" {\n";
    out += "  minimum = 8;\n";
    out += "  maximum = " + std::to_string(maximum) + ";\n";
    for (const auto& [evdev, name] : keys) {
        (void)name;
        out += "  <K" + std::to_string(evdev + 8) + "> = " + std::to_string(evdev + 8) + ";\n";
    }
    out += "};\n";
    out += "xkb_types \"ryoku\" { include \"complete\" };\n";
    out += "xkb_compatibility \"ryoku\" { include \"complete\" };\n";
    out += "xkb_symbols \"ryoku\" {\n";
    for (const auto& [evdev, name] : keys) {
        out += "  key <K" + std::to_string(evdev + 8) + "> {[ " + name + " ]};\n";
    }
    out += "};\n};\n";
    return out;
}

bool keyStrokeFor(xkb_keymap* keymap, uint32_t codepoint, KeyStroke& stroke) {
    if (!keymap)
        return false;
    const auto symbol = codepoint == '\n' ? XKB_KEY_Return
        : codepoint == '\t' ? XKB_KEY_Tab : xkb_utf32_to_keysym(codepoint);
    uint32_t allowed = 0;
    for (const auto* name : {XKB_MOD_NAME_SHIFT, "Mod5"}) {
        const auto index = xkb_keymap_mod_get_index(keymap, name);
        if (index != XKB_MOD_INVALID && index < 32)
            allowed |= 1u << index;
    }
    for (auto key = xkb_keymap_min_keycode(keymap); key <= xkb_keymap_max_keycode(keymap); key++) {
        if (key < 8)
            continue;
        for (uint32_t group = 0; group < xkb_keymap_num_layouts_for_key(keymap, key); group++) {
            for (uint32_t level = 0; level < xkb_keymap_num_levels_for_key(keymap, key, group); level++) {
                const xkb_keysym_t* symbols = nullptr;
                if (xkb_keymap_key_get_syms_by_level(keymap, key, group, level, &symbols) != 1
                    || symbols[0] != symbol)
                    continue;
                xkb_mod_mask_t masks[16];
                const auto count = xkb_keymap_key_get_mods_for_level(keymap, key, group, level,
                                                                    masks, 16);
                for (size_t i = 0; i < count; i++) {
                    if ((masks[i] & ~allowed) == 0) {
                        stroke = {key - 8, masks[i], group};
                        return true;
                    }
                }
            }
        }
    }
    return false;
}

Inserter::~Inserter() { close(); }

void Inserter::close() {
    if (keyboard) {
        zwp_virtual_keyboard_v1_destroy(keyboard);
        keyboard = nullptr;
    }
    if (manager) {
        zwp_virtual_keyboard_manager_v1_destroy(manager);
        manager = nullptr;
    }
    if (seatKeyboard) {
        wl_keyboard_release(seatKeyboard);
        seatKeyboard = nullptr;
    }
    xkb_keymap_unref(seatKeymap);
    seatKeymap = nullptr;
    xkb_context_unref(context);
    context = nullptr;
    if (seatKeymapFd >= 0) {
        ::close(seatKeymapFd);
        seatKeymapFd = -1;
        seatKeymapSize = 0;
    }
    if (seat) {
        wl_seat_release(seat);
        seat = nullptr;
    }
    if (registry) {
        wl_registry_destroy(registry);
        registry = nullptr;
    }
    if (display) {
        wl_display_disconnect(display);
        display = nullptr;
    }
}

namespace {

struct BindState {
    zwp_virtual_keyboard_manager_v1* manager = nullptr;
    wl_seat* seat = nullptr;
};

void bindManager(void* data, wl_registry* registry, uint32_t name, const char* interface,
                 uint32_t version) {
    auto* state = static_cast<BindState*>(data);
    if (std::strcmp(interface, zwp_virtual_keyboard_manager_v1_interface.name) == 0
        && !state->manager) {
        state->manager = static_cast<zwp_virtual_keyboard_manager_v1*>(
            wl_registry_bind(registry, name, &zwp_virtual_keyboard_manager_v1_interface,
                             std::min<uint32_t>(version, 1)));
    } else if (std::strcmp(interface, wl_seat_interface.name) == 0 && !state->seat) {
        // The protocol says the seat may be null, but a compositor that needs to
        // know which keyboard to fake (niri, and smithay-based ones generally)
        // refuses the request outright. Naming the seat costs nothing and works
        // everywhere.
        state->seat = static_cast<wl_seat*>(
            wl_registry_bind(registry, name, &wl_seat_interface, std::min<uint32_t>(version, 7)));
    }
}

const wl_registry_listener registryListener = {bindManager, nullptr};

} // namespace

bool Inserter::connect(std::string& err) {
    close();
    display = wl_display_connect(nullptr);
    if (!display) {
        err = "no Wayland display (is WAYLAND_DISPLAY set?)";
        return false;
    }
    registry = wl_display_get_registry(display);
    BindState state;
    wl_registry_add_listener(registry, &registryListener, &state);
    wl_display_roundtrip(display);
    if (!state.manager) {
        err = "the compositor does not offer the virtual-keyboard protocol";
        close();
        return false;
    }
    if (!state.seat) {
        err = "the compositor offered no seat to type through";
        close();
        return false;
    }
    manager = state.manager;
    seat = state.seat;
    keyboard = zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(manager, seat);
    if (!keyboard) {
        err = "cannot create the virtual keyboard";
        close();
        return false;
    }
    wl_display_roundtrip(display);
    seatKeyboard = wl_seat_get_keyboard(seat);
    static const wl_keyboard_listener listener = {
        keymapReceived,
        [](void*, wl_keyboard*, uint32_t, wl_surface*, wl_array*) {},
        [](void*, wl_keyboard*, uint32_t, wl_surface*) {},
        [](void*, wl_keyboard*, uint32_t, uint32_t, uint32_t, uint32_t) {},
        [](void*, wl_keyboard*, uint32_t, uint32_t, uint32_t, uint32_t, uint32_t) {},
        [](void*, wl_keyboard*, int32_t, int32_t) {},
    };
    wl_keyboard_add_listener(seatKeyboard, &listener, this);
    wl_display_roundtrip(display);
    return true;
}

void Inserter::keymapReceived(void* data, wl_keyboard*, uint32_t format, int32_t fd,
                             uint32_t size) {
    auto& inserter = *static_cast<Inserter*>(data);
    if (format != WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 || size == 0) {
        ::close(fd);
        return;
    }
    const auto* text = static_cast<const char*>(mmap(nullptr, size, PROT_READ, MAP_PRIVATE, fd, 0));
    if (text == MAP_FAILED) {
        ::close(fd);
        return;
    }
    // don't adopt the temporary Unicode map we uploaded ourselves
    if (std::string_view(text, size).find("xkb_keycodes \"ryoku_") != std::string_view::npos) {
        munmap(const_cast<char*>(text), size);
        ::close(fd);
        return;
    }
    if (!inserter.context)
        inserter.context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    auto* keymap = inserter.context
        ? xkb_keymap_new_from_buffer(inserter.context, text, size, XKB_KEYMAP_FORMAT_TEXT_V1,
                                    XKB_KEYMAP_COMPILE_NO_FLAGS)
        : nullptr;
    munmap(const_cast<char*>(text), size);
    if (!keymap) {
        ::close(fd);
        return;
    }
    xkb_keymap_unref(inserter.seatKeymap);
    if (inserter.seatKeymapFd >= 0)
        ::close(inserter.seatKeymapFd);
    inserter.seatKeymap = keymap;
    inserter.seatKeymapFd = fd;
    inserter.seatKeymapSize = size;
}

bool Inserter::upload(const std::vector<std::pair<unsigned, std::string>>& keys,
                      std::string& err) {
    const std::string keymap = buildKeymap(keys) + std::string(1, '\0');
    const int fd = memfd_create("ryoku-keymap", MFD_CLOEXEC);
    if (fd < 0) {
        err = std::string("memfd_create failed: ") + std::strerror(errno);
        return false;
    }
    if (write(fd, keymap.data(), keymap.size()) != static_cast<ssize_t>(keymap.size())
        || lseek(fd, 0, SEEK_SET) != 0) {
        err = "cannot write the keymap";
        ::close(fd);
        return false;
    }
    zwp_virtual_keyboard_v1_keymap(keyboard, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1, fd,
                                   static_cast<uint32_t>(keymap.size()));
    wl_display_roundtrip(display);
    ::close(fd);
    return true;
}

bool Inserter::sendKey(unsigned evdev, bool down) {
    if (!keyboard || !display)
        return false;
    zwp_virtual_keyboard_v1_key(keyboard, ++serial, evdev,
                                down ? WL_KEYBOARD_KEY_STATE_PRESSED
                                     : WL_KEYBOARD_KEY_STATE_RELEASED);
    return true;
}

void Inserter::sync() { wl_display_roundtrip(display); }

namespace {

// A rejected request (a bad keymap, a seat the compositor will not accept)
// shows up as a display error rather than a reply, and libwayland only whispers
// it to stderr. Surface it, or a dictation that "succeeded" typed nothing.
bool displayHealthy(wl_display* display, std::string& err) {
    if (display && wl_display_get_error(display) == 0)
        return true;
    err = "the compositor rejected the virtual keyboard";
    return false;
}

} // namespace

bool Inserter::type(const std::string& utf8, int delayMs, std::string& err) {
    if (!ready()) {
        err = "no virtual keyboard";
        return false;
    }
    const auto codepoints = utf8Codepoints(utf8);
    if (codepoints.empty())
        return true;

    std::map<uint32_t, KeyStroke> strokes;
    bool mapped = seatKeymapFd >= 0;
    for (const auto cp : codepoints) {
        if (!mapped)
            break;
        if (strokes.count(cp))
            continue;
        KeyStroke stroke;
        mapped = keyStrokeFor(seatKeymap, cp, stroke);
        if (mapped)
            strokes.emplace(cp, stroke);
    }
    if (mapped) {
        zwp_virtual_keyboard_v1_keymap(keyboard, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1,
                                      seatKeymapFd, seatKeymapSize);
        sync();
        uint32_t modifiers = UINT32_MAX, group = UINT32_MAX;
        for (const auto cp : codepoints) {
            const auto& stroke = strokes.at(cp);
            if (modifiers != stroke.modifiers || group != stroke.group) {
                modifiers = stroke.modifiers;
                group = stroke.group;
                zwp_virtual_keyboard_v1_modifiers(keyboard, modifiers, 0, 0, group);
            }
            sendKey(stroke.evdev, true);
            sync();
            if (delayMs > 0)
                usleep(static_cast<useconds_t>(delayMs) * 1000);
            sendKey(stroke.evdev, false);
            sync();
            usleep(1500);
        }
        zwp_virtual_keyboard_v1_modifiers(keyboard, 0, 0, 0, 0);
        sync();
        return displayHealthy(display, err);
    }

    constexpr size_t slots = sizeof(textCodes) / sizeof(textCodes[0]);
    size_t at = 0;
    while (at < codepoints.size()) {
        // One keymap per distinct-symbol batch: a layout that can carry every
        // character in front of us costs one upload, not one per character.
        std::map<uint32_t, unsigned> assigned;
        std::vector<std::pair<unsigned, std::string>> keys;
        size_t end = at;
        while (end < codepoints.size() && assigned.size() < slots) {
            const auto cp = codepoints[end];
            if (!assigned.count(cp)) {
                const unsigned code = textCodes[assigned.size()];
                assigned[cp] = code;
                keys.emplace_back(code, keysymName(cp));
            }
            end++;
        }
        if (!upload(keys, err))
            return false;

        for (; at < end; at++) {
            const unsigned code = assigned[codepoints[at]];
            sendKey(code, true);
            sync();
            if (delayMs > 0)
                usleep(static_cast<useconds_t>(delayMs) * 1000);
            sendKey(code, false);
            sync();
            usleep(1500);
        }
    }
    return displayHealthy(display, err);
}

bool Inserter::pasteShortcut(std::string& err) {
    if (!ready()) {
        err = "no virtual keyboard";
        return false;
    }
    KeyStroke stroke{codeV, 0, 0};
    const auto control = seatKeymap ? xkb_keymap_mod_get_index(seatKeymap, XKB_MOD_NAME_CTRL)
                                   : XKB_MOD_INVALID;
    uint32_t modifiers = 4;
    if (seatKeymapFd >= 0 && control != XKB_MOD_INVALID && control < 32
        && keyStrokeFor(seatKeymap, 'v', stroke)) {
        modifiers = (1u << control) | stroke.modifiers;
        zwp_virtual_keyboard_v1_keymap(keyboard, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1,
                                      seatKeymapFd, seatKeymapSize);
    } else if (!upload({{codeControl, "Control_L"}, {codeV, "v"}}, err)) {
        return false;
    }

    zwp_virtual_keyboard_v1_modifiers(keyboard, modifiers, 0, 0, stroke.group);
    sync();
    sendKey(stroke.evdev, true);
    sync();
    usleep(2000);
    sendKey(stroke.evdev, false);
    sync();
    zwp_virtual_keyboard_v1_modifiers(keyboard, 0, 0, 0, 0);
    sync();
    return displayHealthy(display, err);
}

bool setClipboard(const std::string& utf8, std::string& err) {
    int fds[2];
    if (pipe(fds) != 0) {
        err = "cannot create a pipe";
        return false;
    }
    const pid_t pid = fork();
    if (pid < 0) {
        ::close(fds[0]);
        ::close(fds[1]);
        err = "cannot fork";
        return false;
    }
    if (pid == 0) {
        dup2(fds[0], STDIN_FILENO);
        ::close(fds[0]);
        ::close(fds[1]);
        execlp("wl-copy", "wl-copy", "--type", "text/plain;charset=utf-8", nullptr);
        _exit(127);
    }
    ::close(fds[0]);
    size_t written = 0;
    while (written < utf8.size()) {
        const ssize_t n = write(fds[1], utf8.data() + written, utf8.size() - written);
        if (n <= 0)
            break;
        written += static_cast<size_t>(n);
    }
    ::close(fds[1]);
    int status = 0;
    waitpid(pid, &status, 0);
    if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
        err = "wl-copy is not installed";
        return false;
    }
    return true;
}

} // namespace ryoku
