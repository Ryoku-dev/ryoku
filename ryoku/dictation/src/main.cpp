// ryoku-dictate: the voice typing daemon.
//
// One process holds the microphone stream, the speech model and the keyboard,
// so a dictation costs no helper processes and no second capture. It speaks a
// line protocol on $XDG_RUNTIME_DIR/ryoku-dictate.sock; ryoku-shell relays the
// state to the desktop and the Hub edits the model and device.
//
// The daemon is started on demand and is expected to be idle when dictation is
// off. The model is unmapped after `idle_unload_secs`, so an idle machine holds
// no weights at all.

#include "capture.hpp"
#include "config.hpp"
#include "engine.hpp"
#include "insert.hpp"
#include "wav.hpp"

#include <poll.h>
#include <signal.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/un.h>
#include <unistd.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <cerrno>
#include <cstring>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace {

using Clock = std::chrono::steady_clock;

const char* const version = "1.0.0";

std::mutex clientsMutex;
std::vector<int> clients;
std::atomic<bool> running{true};

void broadcast(const std::string& line) {
    const std::string frame = line + "\n";
    std::lock_guard guard(clientsMutex);
    for (auto it = clients.begin(); it != clients.end();) {
        const ssize_t n = write(*it, frame.data(), frame.size());
        if (n < 0 && std::getenv("RYOKU_DICTATE_DEBUG"))
            std::fprintf(stderr, "ryoku-dictate: broadcast to %d failed: %s\n", *it,
                         std::strerror(errno));
        if (n < 0) {
            ::close(*it);
            it = clients.erase(it);
        } else {
            ++it;
        }
    }
}

std::string escapeText(const std::string& text) {
    std::string out;
    out.reserve(text.size());
    for (char c : text) {
        switch (c) {
        case '\\': out += "\\\\"; break;
        case '\n': out += "\\n"; break;
        case '\r': out += "\\r"; break;
        default: out += c;
        }
    }
    return out;
}

struct Session {
    ryoku::Config config;
    ryoku::Engine engine;
    ryoku::Capture capture;
    ryoku::Inserter inserter;
    std::mutex mutex;
    std::string state = "idle";
    std::string error;
    std::string model;
    std::string backend;
    std::thread worker;
    Clock::time_point lastUse = Clock::now();
    Clock::time_point recordingAt{};
    bool hadSpeech = false;
    Clock::time_point lastSpeech = Clock::now();

    void publish(const std::string& next) {
        {
            std::lock_guard guard(mutex);
            state = next;
        }
        broadcast("state " + next);
    }

    std::string currentState() {
        std::lock_guard guard(mutex);
        return state;
    }

    bool ensureModel() {
        if (engine.ready())
            return true;
        const std::string path = ryoku::Config::resolveModel(config);
        if (path.empty()) {
            fail("no speech model installed");
            return false;
        }
        publish("loading");
        std::string errorText;
        if (!engine.load(path, config.device, 0, errorText)) {
            fail(errorText);
            publish("error");
            return false;
        }
        {
            std::lock_guard guard(mutex);
            model = path;
            backend = engine.backend();
        }
        lastUse = Clock::now();
        broadcast("model " + escapeText(path));
        broadcast("backend " + backend);
        broadcast("state idle");
        {
            std::lock_guard guard(mutex);
            state = "idle";
        }
        return true;
    }

    void fail(const std::string& text) {
        {
            std::lock_guard guard(mutex);
            error = text;
        }
        broadcast("error " + escapeText(text));
        std::fprintf(stderr, "ryoku-dictate: %s\n", text.c_str());
    }

    void start() {
        if (!config.enabled) {
            fail("dictation is turned off");
            return;
        }
        if (currentState() == "recording")
            return;
        if (!ensureModel())
            return;

        std::string errorText;
        if (!capture.start(config.source, errorText)) {
            fail(errorText);
            publish("error");
            return;
        }
        recordingAt = Clock::now();
        lastSpeech = recordingAt;
        hadSpeech = false;
        publish("recording");
        broadcast("level 0 0");
    }

    void stop();

    void cancel() {
        engine.abort();
        if (capture.running()) {
            capture.stop();
            capture.take();
        }
        if (currentState() == "recording" || currentState() == "transcribing")
            publish("idle");
    }

    void insert(const std::string& text) {
        if (std::getenv("RYOKU_DICTATE_NO_INSERT")) {
            broadcast("inserted skipped");
            return;
        }
        if (text.empty())
            return;
        std::string errorText;
        if (!inserter.ready() && !inserter.connect(errorText)) {
            fail(errorText);
            return;
        }
        if (config.mode == "type") {
            if (!inserter.type(text, config.typeDelayMs, errorText))
                fail(errorText);
        } else {
            if (!ryoku::setClipboard(text, errorText)) {
                fail(errorText);
                return;
            }
            if (!inserter.pasteShortcut(errorText))
                fail(errorText);
        }
    }

    void transcribe(std::vector<float> pcm) {
        publish("transcribing");
        const auto began = Clock::now();
        std::string text;
        std::string errorText;
        if (!engine.run(pcm, config.language, text, errorText)) {
            if (!engine.aborted())
                fail(errorText);
            publish("idle");
            return;
        }
        const auto spent = std::chrono::duration_cast<std::chrono::milliseconds>(
                               Clock::now() - began)
                               .count();
        // Trim: the models pad a sentence with edge whitespace.
        while (!text.empty() && (text.front() == ' ' || text.front() == '\n'))
            text.erase(text.begin());
        while (!text.empty() && (text.back() == ' ' || text.back() == '\n'))
            text.pop_back();

        broadcast("final " + escapeText(text));
        broadcast("took " + std::to_string(spent));
        if (!text.empty()) {
            publish("inserting");
            insert(text);
        }
        publish("idle");
        lastUse = Clock::now();
    }

    void tick() {
        const std::string now = currentState();
        if (now == "recording") {
            float rms = 0;
            float peak = 0;
            capture.levels(rms, peak);
            broadcast("level " + std::to_string(rms) + " " + std::to_string(peak));

            const float floorRms = std::pow(10.0f, config.silenceDb / 20.0f);
            if (rms > floorRms) {
                hadSpeech = true;
                lastSpeech = Clock::now();
            }
            const auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                                     Clock::now() - recordingAt)
                                     .count();
            const auto silent = std::chrono::duration_cast<std::chrono::milliseconds>(
                                    Clock::now() - lastSpeech)
                                    .count();
            if (config.maxDurationSecs > 0 && elapsed >= config.maxDurationSecs * 1000LL) {
                broadcast("note max duration reached");
                stop();
            } else if (config.silenceMs > 0 && hadSpeech && silent >= config.silenceMs) {
                stop();
            }
        }

        if (config.idleUnloadSecs > 0 && engine.ready() && now == "idle") {
            const auto idle = std::chrono::duration_cast<std::chrono::seconds>(
                                  Clock::now() - lastUse)
                                  .count();
            if (idle >= config.idleUnloadSecs) {
                engine.unload();
                {
                    std::lock_guard guard(mutex);
                    backend.clear();
                }
                broadcast("backend ");
            }
        }
    }
};

void Session::stop() {
    if (currentState() != "recording")
        return;
    capture.stop();
    auto pcm = capture.take();
    if (pcm.size() < 16000 / 8) {
        publish("idle");
        return;
    }
    if (worker.joinable())
        worker.join();
    worker = std::thread([this, pcm = std::move(pcm)] { transcribe(pcm); });
}

// ── command handling ────────────────────────────────────────────────────────

void reply(int fd, const std::string& line) {
    const std::string frame = line + "\n";
    (void)write(fd, frame.data(), frame.size());
}

void handleSet(Session& session, const std::string& args) {
    const auto space = args.find(' ');
    if (space == std::string::npos)
        return;
    const std::string key = args.substr(0, space);
    const std::string value = args.substr(space + 1);
    if (key == "device" && (value == "auto" || value == "cpu" || value == "gpu")) {
        session.config.device = value;
        // A device change parks the loaded model; the next start reloads it.
        session.engine.unload();
        broadcast("device " + value);
    } else if (key == "mode" && (value == "paste" || value == "type")) {
        session.config.mode = value;
        broadcast("mode " + value);
    } else if (key == "language") {
        session.config.language = value == "auto" ? "" : value;
        broadcast("language " + (value.empty() ? "auto" : value));
    } else if (key == "model") {
        session.config.model = value;
        session.engine.unload();
        broadcast("model " + escapeText(value));
    }
}

void handleCommand(Session& session, int fd, const std::string& line) {
    std::string command = line;
    std::string args;
    const auto space = line.find(' ');
    if (space != std::string::npos) {
        command = line.substr(0, space);
        args = line.substr(space + 1);
    }
    if (command == "start") {
        session.start();
        reply(fd, "ok");
    } else if (command == "stop") {
        session.stop();
        reply(fd, "ok");
    } else if (command == "cancel") {
        session.cancel();
        reply(fd, "ok");
    } else if (command == "toggle") {
        if (session.currentState() == "recording")
            session.stop();
        else
            session.start();
        reply(fd, "ok");
    } else if (command == "status") {
        std::lock_guard guard(session.mutex);
        reply(fd, "state " + session.state);
        reply(fd, "model " + escapeText(session.model));
        reply(fd, "backend " + session.backend);
        reply(fd, "device " + session.config.device);
        reply(fd, std::string("enabled ") + (session.config.enabled ? "true" : "false"));
        reply(fd, "mode " + session.config.mode);
        reply(fd, "language " + (session.config.language.empty() ? "auto" : session.config.language));
        // the card measures the recording against the automatic stop
        reply(fd, "limit " + std::to_string(session.config.maxDurationSecs));
        std::string which;
        reply(fd, std::string("gpu ") + (ryoku::gpuAvailable(which) ? which : "none"));
        reply(fd, "end");
    } else if (command == "set") {
        handleSet(session, args);
        reply(fd, "ok");
    } else if (command == "quit") {
        reply(fd, "ok");
        running = false;
    } else {
        reply(fd, "error unknown command");
    }
}

// ── entry points ────────────────────────────────────────────────────────────

int selftest(const std::string& wav, const std::string& model, const std::string& device,
             const std::string& language) {
    std::vector<float> pcm;
    std::string errorText;
    if (!ryoku::readWav16k(wav, pcm, errorText)) {
        std::fprintf(stderr, "ryoku-dictate: %s\n", errorText.c_str());
        return 1;
    }
    ryoku::Engine engine;
    if (!engine.load(model, device, 0, errorText)) {
        std::fprintf(stderr, "ryoku-dictate: %s\n", errorText.c_str());
        return 1;
    }
    const auto began = Clock::now();
    std::string text;
    if (!engine.run(pcm, language, text, errorText)) {
        std::fprintf(stderr, "ryoku-dictate: %s\n", errorText.c_str());
        return 1;
    }
    const auto spent =
        std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now() - began).count();
    std::fprintf(stderr, "backend=%s audio=%.2fs took=%lldms\n", engine.backend().c_str(),
                 double(pcm.size()) / 16000.0, static_cast<long long>(spent));
    std::printf("%s\n", text.c_str());
    return 0;
}

void onSignal(int) { running = false; }

} // namespace

int main(int argc, char** argv) {
    std::string mode = "serve";
    std::string wav;
    std::string model;
    std::string device = "cpu";
    std::string language;

    for (int i = 1; i < argc; i++) {
        const std::string arg = argv[i];
        auto next = [&](const char* name) -> std::string {
            if (i + 1 >= argc) {
                std::fprintf(stderr, "ryoku-dictate: %s needs a value\n", name);
                std::exit(2);
            }
            return argv[++i];
        };
        if (arg == "--serve")
            mode = "serve";
        else if (arg == "--selftest") {
            mode = "selftest";
            wav = next("--selftest");
        } else if (arg == "--model")
            model = next("--model");
        else if (arg == "--device")
            device = next("--device");
        else if (arg == "--language")
            language = next("--language");
        else if (arg == "--version") {
            std::printf("ryoku-dictate %s\n", version);
            return 0;
        } else if (arg == "--print-config") {
            const auto config = ryoku::Config::load();
            std::printf("path=%s\nmodel=%s\ndevice=%s\nmode=%s\nbackend=%s\n",
                        ryoku::Config::path().c_str(),
                        ryoku::Config::resolveModel(config).c_str(), config.device.c_str(),
                        config.mode.c_str(), ryoku::availableBackends().c_str());
            return 0;
        } else {
            std::fprintf(stderr,
                         "usage: ryoku-dictate [--serve]\n"
                         "       ryoku-dictate --selftest file.wav [--model m.gguf] "
                         "[--device auto|cpu|gpu] [--language en]\n"
                         "       ryoku-dictate --print-config | --version\n");
            return 2;
        }
    }

    if (mode == "selftest") {
        if (model.empty())
            model = ryoku::Config::resolveModel(ryoku::Config::load());
        return selftest(wav, model, device, language);
    }

    Session session;
    session.config = ryoku::Config::load();

    const std::string socketPath = ryoku::Config::socketPath();
    unlink(socketPath.c_str());
    const int server = socket(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0);
    if (server < 0) {
        std::perror("socket");
        return 1;
    }
    sockaddr_un address{};
    address.sun_family = AF_UNIX;
    std::snprintf(address.sun_path, sizeof(address.sun_path), "%s", socketPath.c_str());
    if (bind(server, reinterpret_cast<sockaddr*>(&address), sizeof(address)) != 0) {
        std::perror("bind");
        ::close(server);
        return 1;
    }
    chmod(socketPath.c_str(), 0600);
    listen(server, 8);

    struct sigaction action {};
    action.sa_handler = onSignal;
    sigaction(SIGINT, &action, nullptr);
    sigaction(SIGTERM, &action, nullptr);
    signal(SIGPIPE, SIG_IGN);

    std::fprintf(stderr, "ryoku-dictate: listening on %s\n", socketPath.c_str());

    auto lastTick = Clock::now();
    while (running) {
        pollfd fds[1] = {{server, POLLIN, 0}};
        if (poll(fds, 1, 40) > 0 && (fds[0].revents & POLLIN)) {
            const int client = accept4(server, nullptr, nullptr, SOCK_CLOEXEC | SOCK_NONBLOCK);
            if (client >= 0) {
                {
                    // A client that connects mid-session must learn the current
                    // state at once: the level stream is the only thing that
                    // keeps a late watcher honest otherwise.
                    std::lock_guard guard(session.mutex);
                    const std::string greet = "state " + session.state + "\n";
                    (void)write(client, greet.data(), greet.size());
                }
                std::lock_guard guard(clientsMutex);
                clients.push_back(client);
            }
        }

        // Commands are handled outside the client lock: handling one writes an
        // event to every client, and broadcast takes the same lock.
        std::vector<std::pair<int, std::string>> work;
        {
            std::lock_guard guard(clientsMutex);
            for (auto it = clients.begin(); it != clients.end();) {
                char buffer[1024];
                const ssize_t n = read(*it, buffer, sizeof(buffer) - 1);
                if (n == 0) {
                    if (std::getenv("RYOKU_DICTATE_DEBUG"))
                        std::fprintf(stderr, "ryoku-dictate: client %d EOF\n", *it);
                    ::close(*it);
                    it = clients.erase(it);
                    continue;
                }
                if (n < 0) {
                    // EAGAIN is the normal case on a non-blocking client;
                    // anything else means the peer is gone.
                    if (std::getenv("RYOKU_DICTATE_DEBUG") && errno != EAGAIN
                        && errno != EWOULDBLOCK)
                        std::fprintf(stderr, "ryoku-dictate: client %d read: %s\n", *it,
                                     std::strerror(errno));
                    if (errno != EAGAIN && errno != EWOULDBLOCK) {
                        ::close(*it);
                        it = clients.erase(it);
                        continue;
                    }
                    ++it;
                    continue;
                }
                buffer[n] = '\0';
                const std::string pending(buffer);
                std::string::size_type start = 0;
                std::string::size_type lineEnd = 0;
                while ((lineEnd = pending.find('\n', start)) != std::string::npos) {
                    const std::string line = pending.substr(start, lineEnd - start);
                    start = lineEnd + 1;
                    if (!line.empty())
                        work.emplace_back(*it, line);
                }
                ++it;
            }
        }
        for (const auto& [fd, line] : work)
            handleCommand(session, fd, line);

        if (Clock::now() - lastTick > std::chrono::milliseconds(40)) {
            lastTick = Clock::now();
            session.tick();
        }
    }

    session.cancel();
    if (session.worker.joinable())
        session.worker.join();
    session.capture.stop();
    {
        std::lock_guard guard(clientsMutex);
        for (int fd : clients)
            ::close(fd);
        clients.clear();
    }
    ::close(server);
    unlink(socketPath.c_str());
    return 0;
}
