pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import M3Shapes
import Caelestia.Blobs
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.components.effects
import caelestia.components.filedialog
import caelestia.services
import caelestia.utils
import "ai"
import "ai/attachments.js" as AttachmentPaths
import "ai/claudecode.js" as ClaudeCode

Item {
    id: root

    ChatStore {
        id: chatStore

        provider: root.provider
        onTitleNeeded: (chatId, firstMessage) => root.generateChatTitleAsync(chatId, firstMessage)
    }

    property bool isHistoryTab: false

    readonly property string currentChatId: chatStore.currentChatId

    property var currentRequest: null

    // The reply the current request writes to: { chatId, msgId }.
    property var activeReply: null

    // Replying (in any chat), or running the tools of a reply; the sidebar stays
    // loaded meanwhile.
    readonly property bool busy: isTyping || inAgentLoop || Object.keys(claudeCodeRuns).length > 0

    property real savedContentY: -1

    onProviderChanged: {
        cancelRateLimitRetry();
        if (isOpenaiCompat)
            fetchOpenaiCompatModels(provider);
        else if (isClaude)
            fetchClaudeModels();
    }

    onVisibleChanged: {
        if (visible) {
            refreshAllModels();
            if (savedContentY >= 0) {
                Qt.callLater(function() { listView.contentY = savedContentY; });
            }
        } else {
            savedContentY = listView.contentY;
        }
    }

    // Ask every enabled provider what it offers, rather than shipping lists that
    // go stale each time a vendor releases a model.
    function refreshAllModels() {
        fetchOllamaModels();
        fetchClaudeCodeModels();
        fetchClaudeModels();
        const compat = root.openaiCompatProviders;
        for (var i = 0; i < compat.length; i++) {
            if (providerList.indexOf(compat[i]) !== -1)
                fetchOpenaiCompatModels(compat[i]);
        }
    }

    Component.onCompleted: {
        loadAllKeys();
        refreshAllModels();
        loadHistory();
        AiToolCalls.onAllToolsFinished = (results, imageB64) => {
            sendPrompt(results, true, imageB64, "multi_tool");
        };
    }

    function logFetchError(provider) {
        Logger.log("[AI] Network error fetching models from " + (provider || "unknown"));
    }

    function handleSendError(reply) {
        isTyping = false;
        isThinking = false;
        inAgentLoop = false;
        currentActionText = "";
        const m = chatStore.message(reply.chatId, reply.msgId);
        if (m && !m.isFinished) {
            chatStore.update(reply.chatId, reply.msgId, {
                "isFinished": true,
                "text": m.text || "⚠️ Network error - check your connection and try again."
            });
        }
    }

    property var ollamaModelsList: []

    property var claudeModelsList: []

    function fetchClaudeModels() {
        const key = root.getApiKeyFor("claude");
        if (key === "")
            return;
        const base = GlobalConfig.ai.anthropicUrl || "https://api.anthropic.com";
        var xhr = new XMLHttpRequest();
        xhr.open("GET", base + "/v1/models?limit=100", true);
        xhr.setRequestHeader("x-api-key", key);
        xhr.setRequestHeader("anthropic-version", "2023-06-01");
        xhr.setRequestHeader("anthropic-dangerous-direct-browser-access", "true");
        xhr.onreadystatechange = () => {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return;
            if (xhr.status !== 200) {
                Logger.log("[AI] Claude model list failed (status " + xhr.status + ")");
                return;
            }
            try {
                const parsed = JSON.parse(xhr.responseText);
                var list = [];
                for (var i = 0; i < (parsed.data || []).length; i++) {
                    if (parsed.data[i].id)
                        list.push(parsed.data[i].id);
                }
                if (list.length === 0)
                    return;
                list.reverse();
                root.claudeModelsList = list;
                if (list.indexOf(GlobalConfig.ai.defaultClaudeModel) === -1)
                    GlobalConfig.ai.defaultClaudeModel = list[0];
            } catch (e) {
                Logger.log("[AI] Error parsing Claude models: " + e.message);
            }
        };
        xhr.onerror = () => { root.logFetchError("Claude"); };
        xhr.send();
    }

    property var claudeCodeModelsList: ["default"]

    readonly property var claudeCodeEffortOptions: {
        var lv = ClaudeCode.effortLevelsFor(activeModel());
        return lv.length > 0 ? ["default"].concat(lv) : [];
    }

    function fetchClaudeCodeModels() {
        var bin = claudeCodeBinPath();
        var script =
            "t=\"$(readlink -f " + JSON.stringify(bin) + " 2>/dev/null)\"; [ -z \"$t\" ] && t=" + JSON.stringify(bin) + "; " +
            "strings \"$t\" 2>/dev/null | grep -oE 'claude-(opus|sonnet|haiku|fable)-[0-9]+(-[0-9]+)?' | sort -u";
        var commandStr = JSON.stringify(["sh", "-c", script]);
        var qml =
            "import QtQuick\n" +
            "import Quickshell.Io\n" +
            "Process {\n" +
            "    id: mp\n" +
            "    command: " + commandStr + "\n" +
            "    stdout: StdioCollector { onStreamFinished: root.applyClaudeCodeModels(text || \"\"); }\n" +
            "    onExited: code => mp.destroy()\n" +
            "}";
        try {
            var o = Qt.createQmlObject(qml, root, "ccModelsProc");
            o.running = true;
        } catch (e) {
            Logger.log("[AI] claude-code model fetch error: " + e.message);
        }
    }

    function applyClaudeCodeModels(text) {
        claudeCodeModelsList = ClaudeCode.parseModelList(text);
    }

    readonly property string provider: GlobalConfig.ai.defaultProvider || "ollama"

    readonly property bool isClaude: provider === "claude"

    readonly property bool isClaudeCode: provider === "claude-code"

    // Running Claude Code replies by chat id. They keep going while another chat
    // is open.
    property var claudeCodeRuns: ({})

    readonly property var currentClaudeCodeProc: claudeCodeRuns[currentChatId] ?? null

    // Working directory and permission mode of the current Claude Code chat. Both are
    // stored per chat (a CLI session only resumes from the directory it was created
    // in); a new chat inherits whatever was selected last.
    readonly property string claudeCodeChatCwd: chatStore.currentProp("claudeCodeCwd") || Quickshell.env("HOME") || "."

    readonly property string claudeCodePermissionMode: chatStore.currentProp("claudeCodePermissionMode") || defaultClaudeCodePermissionMode()

    // Bypass is only offered once it has been allowed in the AI settings.
    readonly property var claudeCodePermissionModes: GlobalConfig.ai.claudeCodeSkipPermissions
        ? ["bypassPermissions", "acceptEdits", "auto", "plan", "default"]
        : ["acceptEdits", "auto", "plan", "default"]

    readonly property var claudeCodeSidebarAllowedTools: ["Bash", "WebFetch", "WebSearch", "Read"]

    function defaultClaudeCodePermissionMode() {
        return GlobalConfig.ai.claudeCodeSkipPermissions ? "bypassPermissions" : "default";
    }

    function permissionModeLabel(mode) {
        switch (mode) {
        case "bypassPermissions": return qsTr("Bypass");
        case "acceptEdits": return qsTr("Accept edits");
        case "auto": return qsTr("Auto");
        case "plan": return qsTr("Plan");
        case "default": return qsTr("Default");
        }
        return mode;
    }

    function setClaudeCodeCwd(dir) {
        dir = (dir || "").replace(/\/+$/, "") || "/";
        if (dir === claudeCodeChatCwd)
            return;
        // The old session lives under the previous directory's project, so --resume
        // would fail; the next send seeds a fresh session with the transcript instead.
        chatStore.setChatProps(currentChatId, { "claudeCodeCwd": dir, "claudeCodeSessionId": "" });
    }

    function setClaudeCodePermissionMode(mode) {
        chatStore.setChatProps(currentChatId, { "claudeCodePermissionMode": mode });
    }

    function shortPath(p) {
        var home = Quickshell.env("HOME") || "";
        if (home !== "" && (p === home || p.indexOf(home + "/") === 0))
            return "~" + p.substring(home.length);
        return p;
    }

    // Every path under the home directory written as ~/..., for tool summaries
    // (file paths, shell commands).
    function shortPaths(text) {
        return ClaudeCode.shortPaths(text, Quickshell.env("HOME") || "");
    }

    Attachments {
        id: attachments

        onPasteText: inputArea.paste()
    }

    readonly property bool canSend: inputArea.text.length > 0 || attachments.pending.length > 0

    // Open the current Claude Code session in a terminal (`claude --resume`).
    function openClaudeCodeInTerminal() {
        var sid = claudeCodeSessionFor(currentChatId);
        var dir = activeClaudeConfigDir();
        var inner = "cd " + shellQuote(claudeCodeCwd()) + " && ";
        if (dir && dir !== "")
            inner += "CLAUDE_CONFIG_DIR=" + shellQuote(dir) + " ";
        inner += "exec " + shellQuote(claudeCodeBinPath());
        if (sid !== "")
            inner += " --resume " + shellQuote(sid);
        var term = GlobalConfig.ai.loginTerminal || "konsole";
        Launch.exec([term, "-e", "sh", "-lc", inner]);
    }

    // Live status for a running Claude Code reply, shown under it the way the CLI
    // does: a rotating verb, elapsed time and output tokens.
    readonly property real claudeCodeStartedAt: currentClaudeCodeProc?.startedAt ?? 0

    property int claudeCodeElapsed: 0

    readonly property int claudeCodeOutTokens: currentClaudeCodeProc?.outputTokens ?? 0

    readonly property bool claudeCodeToolRunning: currentClaudeCodeProc?.toolRunning ?? false

    readonly property var thinkingVerbs: [
        "Thinking", "Pondering", "Musing", "Mulling", "Cogitating", "Ruminating",
        "Brewing", "Simmering", "Percolating", "Noodling", "Tinkering", "Puzzling",
        "Untangling", "Connecting dots", "Crunching", "Sketching", "Weaving",
        "Conjuring", "Scheming", "Distilling", "Assembling", "Wrangling",
        "Spelunking", "Unravelling", "Contemplating", "Deliberating", "Figuring"
    ]

    function randomThinkingVerb() {
        var v;
        do {
            v = thinkingVerbs[Math.floor(Math.random() * thinkingVerbs.length)] + "…";
        } while (v === currentActionText && thinkingVerbs.length > 1);
        return v;
    }

    function formatElapsed(sec) {
        if (sec < 60)
            return sec + "s";
        return Math.floor(sec / 60) + "m " + (sec % 60) + "s";
    }

    Timer {
        interval: 1000
        repeat: true
        running: root.isClaudeCode && root.isTyping && root.claudeCodeStartedAt > 0
        onTriggered: root.claudeCodeElapsed = Math.floor((Date.now() - root.claudeCodeStartedAt) / 1000)
    }

    // A new verb every few seconds while nothing more specific (a tool) is shown.
    Timer {
        interval: 4000
        repeat: true
        running: root.isClaudeCode && root.isTyping && !root.claudeCodeToolRunning
        onTriggered: root.currentActionText = root.randomThinkingVerb()
    }

    property var promptSuggestions: []

    property bool loadingSuggestions: false

    function fetchPromptSuggestions() {
        if (!isClaudeCode || loadingSuggestions)
            return;
        loadingSuggestions = true;
        promptSuggestions = [];

        var lines = [];
        var msgs = chatStore.current().messages;
        for (var li = 0; li < msgs.length; li++) {
            var lm = msgs[li];
            if (!lm.isUser && !lm.isFinished)
                continue;
            var lt = (lm.text || "").trim();
            if (lt === "")
                continue;
            lines.push((lm.isUser ? "User" : "Assistant") + ": " + lt);
        }
        if (lines.length > 6)
            lines = lines.slice(lines.length - 6);
        var context = lines.join("\n");
        var draft = "";
        try {
            draft = (inputArea.text || "").trim();
        } catch (e) {}

        var prompt;
        if (context === "" && draft === "") {
            prompt = "Suggest exactly 4 short, varied example prompts a user might ask a helpful AI desktop assistant. Reply with ONLY a JSON array of 4 short strings, nothing else.";
        } else {
            prompt = "You are suggesting what the user might type NEXT in this chat. Based only on the context below, propose exactly 4 short, specific follow-up prompts they are likely to want to send next. Reply with ONLY a JSON array of 4 short strings, nothing else.\n\n";
            if (context !== "")
                prompt += "Conversation so far:\n" + context + "\n\n";
            if (draft !== "")
                prompt += "The user has started typing: \"" + draft + "\"\n\n";
        }

        var cmd = [claudeCodeBinPath(), "-p", prompt, "--output-format", "json"];
        var qml =
            "import QtQuick\n" +
            "import Quickshell.Io\n" +
            "Process {\n" +
            "    id: sp\n" +
            "    command: " + JSON.stringify(cmd) + "\n" +
            "    workingDirectory: " + JSON.stringify(claudeCodeCwd()) + "\n" +
            claudeCodeEnvSnippet() +
            "    stdout: StdioCollector { onStreamFinished: root.applyPromptSuggestions(text || \"\"); }\n" +
            "    onExited: code => { root.loadingSuggestions = false; sp.destroy(); }\n" +
            "}";
        try {
            var o = Qt.createQmlObject(qml, root, "promptSugProc");
            o.running = true;
        } catch (e) {
            loadingSuggestions = false;
            Logger.log("[AI] suggestion process error: " + e.message);
        }
    }

    function applyPromptSuggestions(text) {
        loadingSuggestions = false;
        var resultStr = "";
        try {
            resultStr = JSON.parse(text).result || "";
        } catch (e) {
            return;
        }
        var arr = null;
        try {
            arr = JSON.parse(resultStr);
        } catch (e) {
            var m = resultStr.match(/\[[\s\S]*\]/);
            if (m)
                try { arr = JSON.parse(m[0]); } catch (e2) {}
        }
        if (Array.isArray(arr)) {
            var list = [];
            for (var i = 0; i < arr.length && i < 6; i++)
                list.push(String(arr[i]));
            promptSuggestions = list;
        }
    }

    // The CLI session a chat resumes, if it was started with the active account.
    function claudeCodeSessionFor(chatId) {
        var s = chatStore.session(chatId);
        if (!s || (s.claudeCodeSessionAccount || "") !== (GlobalConfig.ai.activeClaudeAccount || ""))
            return "";
        return s.claudeCodeSessionId || "";
    }

    function setClaudeCodeSession(chatId, sid) {
        if (sid)
            chatStore.setChatProps(chatId, {
                "claudeCodeSessionId": sid,
                "claudeCodeSessionAccount": GlobalConfig.ai.activeClaudeAccount || ""
            });
    }

    readonly property bool isOpenaiCompat: root.openaiCompatProviders.indexOf(provider) !== -1

    // opencode is the exception: it is in the list above because it shares all of
    // that, but only for part of its catalogue — the rest needs Anthropic Messages,
    // and it authenticates with x-api-key. See opencodeWire() and setAuthHeader().
    readonly property bool isOpencode: provider === "opencode" || provider === "opencode-go"

    readonly property var openaiCompatProviders: ["openai", "gemini", "openrouter", "opencode", "opencode-go"]

    function openaiCompatBase(p) {
        const which = p || provider;
        if (which === "gemini")
            return GlobalConfig.ai.geminiUrl || "https://generativelanguage.googleapis.com/v1beta/openai";
        if (which === "openrouter")
            return GlobalConfig.ai.openrouterUrl || "https://openrouter.ai/api/v1";
        if (which === "opencode")
            return GlobalConfig.ai.opencodeUrl || "https://opencode.ai/zen/v1";
        if (which === "opencode-go")
            return GlobalConfig.ai.opencodeGoUrl || "https://opencode.ai/zen/go/v1";
        return GlobalConfig.ai.openaiUrl || "https://api.openai.com/v1";
    }

    readonly property var opencodeAnthropicPrefixes: ({
        "opencode": ["claude-", "qwen"],
        "opencode-go": ["minimax-", "qwen"]
    })

    readonly property var opencodeUnsupportedPrefixes: ({
        "opencode": ["gpt-", "gemini-"],
        "opencode-go": []
    })

    function opencodeWire(p, model) {
        const prefixes = root.opencodeAnthropicPrefixes[p] || [];
        for (var i = 0; i < prefixes.length; i++)
            if ((model || "").indexOf(prefixes[i]) === 0)
                return "anthropic";
        return "openai";
    }

    function opencodeSupports(p, model) {
        const prefixes = root.opencodeUnsupportedPrefixes[p] || [];
        for (var i = 0; i < prefixes.length; i++)
            if ((model || "").indexOf(prefixes[i]) === 0)
                return false;
        return true;
    }

    readonly property bool anthropicWire: isClaude || (isOpencode && opencodeWire(provider, activeModel()) === "anthropic")

    function setAuthHeader(xhr, p) {
        const which = p || provider;
        const key = root.getApiKeyFor(which);
        if (which === "opencode" || which === "opencode-go")
            xhr.setRequestHeader("x-api-key", key);
        else
            xhr.setRequestHeader("Authorization", "Bearer " + key);
    }

    property var keyringKeys: ({})

    function keyringOwner(p) {
        const which = p || provider;
        return which === "opencode-go" ? "opencode" : which;
    }

    function keyringAttr(p) {
        return "caelestia-ai-" + root.keyringOwner(p);
    }

    function loadKeyring(p) {
        const which = p || provider;
        const cmd = ["secret-tool", "lookup", "service", "caelestia", "key", root.keyringAttr(which)];
        const qml = 'import QtQuick\nimport Quickshell.Io\n' +
            'Process {\n    id: kp\n    command: ' + JSON.stringify(cmd) + '\n' +
            '    stdout: StdioCollector { onStreamFinished: root.onKeyringKey(' + JSON.stringify(which) + ', (text || "").trim(), kp); }\n' +
            '    onExited: code => { if (code !== 0) kp.destroy(); }\n}';
        try {
            const o = Qt.createQmlObject(qml, root, "keyringProc");
            o.running = true;
        } catch (e) {}
    }

    function fetchModelsForKey(p) {
        if (root.openaiCompatProviders.indexOf(p) !== -1) {
            if (!root.modelsFetched[p])
                root.fetchOpenaiCompatModels(p);
        } else if (p === "claude" && root.claudeModelsList.length === 0) {
            root.fetchClaudeModels();
        }
    }

    function onKeyringKey(p, key, proc) {
        if (key !== "") {
            const m = root.keyringKeys;
            m[p] = key;
            root.keyringKeys = Object.assign({}, m);
            root.fetchModelsForKey(p);
        }
        if (proc)
            proc.destroy();
    }

    function storeKeyring(p, key) {
        const which = root.keyringOwner(p || provider);
        const m = root.keyringKeys;
        m[which] = key;
        root.keyringKeys = Object.assign({}, m);
        if (key !== "")
            root.fetchModelsForKey(which);

        const attr = root.keyringAttr(which);
        const script = key === ""
            ? "secret-tool clear service caelestia key " + JSON.stringify(attr)
            : "printf %s \"$CAELESTIA_AI_KEY\" | secret-tool store --label=" + JSON.stringify("Caelestia " + which + " API key") +
              " service caelestia key " + JSON.stringify(attr);
        try {
            const o = Qt.createQmlObject('import QtQuick\nimport Quickshell.Io\nProcess { id: sp; command: ' +
                JSON.stringify(["sh", "-c", script]) +
                '\n environment: ({ CAELESTIA_AI_KEY: ' + JSON.stringify(key) + ' })\n' +
                ' onExited: code => sp.destroy() }', root, "keyringStore");
            o.running = true;
        } catch (e) {}
    }

    function migratePlaintextKey(p, configKey) {
        const existing = (GlobalConfig.ai[configKey] || "").trim();
        if (existing === "")
            return;
        root.storeKeyring(p, existing);
        GlobalConfig.ai[configKey] = "";
        Logger.log("[AI] moved " + p + " API key from shell.json into the keyring");
    }

    function getApiKeyFor(p) {
        const which = p || provider;
        var envName = "ANTHROPIC_API_KEY";
        var configured = GlobalConfig.ai.anthropicApiKey;
        if (which === "openai") {
            envName = "OPENAI_API_KEY";
            configured = GlobalConfig.ai.openaiApiKey;
        } else if (which === "gemini") {
            envName = "GEMINI_API_KEY";
            configured = GlobalConfig.ai.geminiApiKey;
        } else if (which === "openrouter") {
            envName = "OPENROUTER_API_KEY";
            configured = GlobalConfig.ai.openrouterApiKey;
        } else if (which === "opencode" || which === "opencode-go") {
            envName = "OPENCODE_API_KEY";
            configured = GlobalConfig.ai.opencodeApiKey;
        }
        const envKey = Quickshell.env(envName);
        if (envKey && envKey.trim() !== "")
            return envKey.trim();

        const stored = root.keyringKeys[root.keyringOwner(which)];
        if (stored && stored !== "")
            return stored;
        return (configured || "").trim();
    }

    readonly property var legacyKeyFields: ({
        "claude": "anthropicApiKey",
        "openai": "openaiApiKey",
        "gemini": "geminiApiKey",
        "openrouter": "openrouterApiKey",
        "opencode": "opencodeApiKey"
    })

    function loadAllKeys() {
        for (const p in root.legacyKeyFields) {
            root.loadKeyring(p);
            root.migratePlaintextKey(p, root.legacyKeyFields[p]);
        }
    }

    function getApiKey() {
        return root.getApiKeyFor(root.provider);
    }

    readonly property bool needsApiKey: isClaude || isOpenaiCompat

    function activeModel() {
        if (isClaudeCode)
            return GlobalConfig.ai.defaultClaudeCodeModel || "default";
        if (isClaude)
            return GlobalConfig.ai.defaultClaudeModel || root.claudeModelsList[0] || "";
        if (isOpenaiCompat)
            return GlobalConfig.ai[root.defaultModelField(provider)] || root.openaiCompatModelList()[0] || "";
        return GlobalConfig.ai.defaultOllamaModel || root.ollamaModelsList[0] || "";
    }

    function defaultModelField(p) {
        const which = p || provider;
        if (which === "gemini")
            return "defaultGeminiModel";
        if (which === "openrouter")
            return "defaultOpenrouterModel";
        if (which === "opencode")
            return "defaultOpencodeModel";
        if (which === "opencode-go")
            return "defaultOpencodeGoModel";
        return "defaultOpenaiModel";
    }

    readonly property var providerList: {
        var l = [];
        if (GlobalConfig.ai.enableOllama)
            l.push("ollama");
        if (GlobalConfig.ai.enableClaudeCode)
            l.push("claude-code");
        if (GlobalConfig.ai.enableClaude)
            l.push("claude");
        if (GlobalConfig.ai.enableOpenai)
            l.push("openai");
        if (GlobalConfig.ai.enableGemini)
            l.push("gemini");
        if (GlobalConfig.ai.enableOpenrouter)
            l.push("openrouter");
        if (GlobalConfig.ai.enableOpencode)
            l.push("opencode");
        if (GlobalConfig.ai.enableOpencodeGo)
            l.push("opencode-go");
        if (l.length === 0)
            l.push("ollama");
        return l;
    }

    function providerLabel(p) {
        if (p === "claude-code")
            return "Claude Code";
        if (p === "claude")
            return "Claude API";
        if (p === "openai")
            return "ChatGPT";
        if (p === "gemini")
            return "Gemini";
        if (p === "openrouter")
            return "OpenRouter";
        if (p === "opencode")
            return "opencode Zen";
        if (p === "opencode-go")
            return "opencode Go";
        return "Ollama";
    }

    property bool isTyping: false

    property bool isThinking: false

    property string currentThoughtText: ""

    property bool isThoughtExpanded: false

    onIsTypingChanged: {
        if (isTyping) listView.positionViewAtEnd();
    }

    property bool inAgentLoop: false

    property int rateLimitRetries: 0

    readonly property int maxRateLimitRetries: 3

    property bool onFreeTier: false

    property int rateLimitSecondsLeft: 0

    function cancelRateLimitRetry(): void {
        rateLimitRetryTimer.stop();
        rateLimitRetryTimer.retryFn = null;
        rateLimitSecondsLeft = 0;
        rateLimitRetries = 0;
    }

    Timer {
        id: rateLimitRetryTimer

        interval: 1000
        repeat: true

        property var retryFn: null

        property string forChat: ""

        property string forModel: ""
        onTriggered: {
            root.rateLimitSecondsLeft--;
            if (root.rateLimitSecondsLeft > 0) {
                root.currentActionText = qsTr("Rate limited - retrying in %1s…").arg(root.rateLimitSecondsLeft);
                return;
            }
            stop();
            if (forChat !== root.currentChatId || forModel !== root.activeModel()) {
                retryFn = null;
                root.currentActionText = "";
                root.isTyping = false;
                root.isThinking = false;
                root.inAgentLoop = false;
                return;
            }
            if (retryFn) { const f = retryFn; retryFn = null; f(); }
        }
    }

    function rateLimitDelayMs(xhr) {
        const header = xhr.getResponseHeader("Retry-After");
        if (header && !isNaN(parseFloat(header)))
            return Math.ceil(parseFloat(header) * 1000) + 500;
        const m = /retry in ([0-9.]+)\s*s/i.exec(xhr.responseText || "");
        if (m)
            return Math.ceil(parseFloat(m[1]) * 1000) + 500;
        return 15000;
    }

    function shellQuote(str) {
        if (str === null || str === undefined) return "''";
        return "'" + String(str).replace(/'/g, "'\\''") + "'";
    }

    function runAgentCommand(cmd, type) {
        var commandStr = Array.isArray(cmd) ? JSON.stringify(cmd) : '["sh", "-c", ' + JSON.stringify("exec </dev/null; " + cmd) + ']';
        var processQml = "import QtQuick\n" +
                         "import Quickshell.Io\n" +
                         "Process {\n" +
                         "    id: proc\n" +
                         "    command: " + commandStr + "\n" +
                         "    property string outStr: \"\"\n" +
                         "    property string errStr: \"\"\n" +
                         "    property bool hasExited: false\n" +
                         "    property bool outFinished: false\n" +
                         "    property bool errFinished: false\n" +
                         "    function checkDone() {\n" +
                         "        if (hasExited && outFinished && errFinished) {\n" +
                         "            root.handleAgentProcessResult(" + JSON.stringify(type) + ", proc.outStr, proc.errStr, " + JSON.stringify(cmd) + ");\n" +
                         "            proc.destroy();\n" +
                         "        }\n" +
                         "    }\n" +
                         "    stdout: StdioCollector { onStreamFinished: { proc.outStr = text || \"\"; proc.outFinished = true; proc.checkDone(); } }\n" +
                         "    stderr: StdioCollector { onStreamFinished: { proc.errStr = text || \"\"; proc.errFinished = true; proc.checkDone(); } }\n" +
                         "    onExited: code => { proc.hasExited = true; proc.checkDone(); }\n" +
                         "}";
        try {
            var obj = Qt.createQmlObject(processQml, root, "agentProcess");
            obj.running = true;
        } catch(e) {
            console.error("AGENT PROCESS ERROR: " + e.message);
            console.error("FAILED QML: \n" + processQml);
        }
    }

    function handleAgentProcessResult(type, stdout, stderr, cmd) {
        if (type === "screenshot_take") {
            var convertCmd = `magick ${Paths.runtimeTemp("orion_screenshot.png")} -resize '1024x1024>' -quality 85 ${Paths.runtimeTemp("orion_screenshot.jpg")} && base64 ${Paths.runtimeTemp("orion_screenshot.jpg")}`;
            runAgentCommand(convertCmd, "screenshot_encode");
        } else if (type === "screenshot_encode") {
            var b64 = stdout.replace(/\n/g, "").trim();
            AiToolCalls.recordImage(b64);
            AiToolCalls.recordResult("Result of take_screenshot:\nScreenshot taken. Analyze the attached image.\n\n");
            AiToolCalls.finishOneTool();
        } else if (type.startsWith("exec_")) {
            var toolName = type.substring(5);
            var outText = stdout.trim();
            var errText = stderr.trim();
            if (!outText && !errText) {
                outText = "(Command completed with no output. If it was a background task, it has been launched successfully.)";
            }
            AiToolCalls.recordResult("Result of " + toolName + ":\n" + outText + (errText ? "\n\nErrors reported:\n" + errText : "") + "\n\n");
            AiToolCalls.finishOneTool();
        }
    }

    function claudeCodeCwd() {
        return claudeCodeChatCwd || Quickshell.env("HOME") || ".";
    }

    function claudeCodeBinPath() {
        var b = (GlobalConfig.ai.claudeCodeBin || "claude").trim();
        if (b === "" || b === "claude") {
            var home = Quickshell.env("HOME") || "";
            if (home !== "")
                return home + "/.local/bin/claude";
            return "claude";
        }
        return b;
    }

    function claudeAccounts() {
        var list = [{ "id": "", "name": "Default", "dir": "" }];
        try {
            var parsed = JSON.parse(GlobalConfig.ai.claudeAccountsJson || "[]");
            if (Array.isArray(parsed)) {
                var home = Quickshell.env("HOME") || "";
                for (var i = 0; i < parsed.length; i++) {
                    var a = parsed[i];
                    if (a && a.id)
                        list.push({
                            "id": String(a.id),
                            "name": String(a.name || a.id),
                            "dir": home + "/.config/caelestia/claude/" + String(a.id)
                        });
                }
            }
        } catch (e) {}
        return list;
    }

    function activeClaudeAccountObj() {
        var id = GlobalConfig.ai.activeClaudeAccount || "";
        var list = claudeAccounts();
        for (var i = 0; i < list.length; i++)
            if (list[i].id === id)
                return list[i];
        return list[0];
    }

    function activeClaudeConfigDir() {
        return activeClaudeAccountObj().dir || "";
    }

    property var resolvedAccountNames: ({})

    function accountJsonPath(id) {
        var home = Quickshell.env("HOME") || "";
        if (!id || id === "")
            return home + "/.claude.json";
        return home + "/.config/caelestia/claude/" + id + "/.claude.json";
    }

    function accountLabel(id) {
        if (resolvedAccountNames[id])
            return resolvedAccountNames[id];
        var list = claudeAccounts();
        for (var i = 0; i < list.length; i++)
            if (list[i].id === id)
                return list[i].name;
        return "Default";
    }

    Instantiator {
        model: root.claudeAccountIds
        delegate: FileView {
            required property string modelData

            path: root.accountJsonPath(modelData)
            printErrors: false
            watchChanges: false
            onLoaded: {
                try {
                    var d = JSON.parse(text());
                    var oa = d.oauthAccount || {};
                    var nm = oa.displayName || oa.emailAddress || "";
                    if (nm) {
                        var map = root.resolvedAccountNames;
                        map[modelData] = nm;
                        root.resolvedAccountNames = Object.assign({}, map);
                    }
                } catch (e) {}
            }
        }
    }

    readonly property var claudeAccountIds: {
        var l = [];
        var a = claudeAccounts();
        for (var i = 0; i < a.length; i++)
            l.push(a[i].id);
        return l;
    }

    function claudeCodeEnvSnippet() {
        var dir = activeClaudeConfigDir();
        if (dir && dir !== "")
            return "    environment: ({ \"CLAUDE_CONFIG_DIR\": " + JSON.stringify(dir) + " })\n";
        return "";
    }

    function generateClaudeCodeTitleAsync(chatId, firstMessage) {
        if (!firstMessage)
            return;
        var safeMsg = firstMessage.substring(0, 200);
        var prompt = "Output ONLY a concise 2-4 word title for the following message. No quotes, no trailing punctuation, no explanation.\n\nMessage: " + safeMsg;

        var cmd = [claudeCodeBinPath(), "-p", prompt, "--output-format", "json"];
        var commandStr = JSON.stringify(cmd);
        var cwdStr = JSON.stringify(claudeCodeCwd());
        var chatIdStr = JSON.stringify(chatId);
        var qml =
            "import QtQuick\n" +
            "import Quickshell.Io\n" +
            "Process {\n" +
            "    id: tproc\n" +
            "    command: " + commandStr + "\n" +
            "    workingDirectory: " + cwdStr + "\n" +
            claudeCodeEnvSnippet() +
            "    stdout: StdioCollector { onStreamFinished: root.handleClaudeCodeTitle(" + chatIdStr + ", text || \"\", tproc); }\n" +
            "    onExited: code => { if (code !== 0) tproc.destroy(); }\n" +
            "}";
        try {
            var obj = Qt.createQmlObject(qml, root, "claudeCodeTitleProc");
            obj.running = true;
        } catch (e) {
            Logger.log("[AI] claude-code title process error: " + e.message);
        }
    }

    function handleClaudeCodeTitle(chatId, text, proc) {
        try {
            var parsed = JSON.parse(text);
            if (parsed && parsed.result && !parsed.is_error)
                applyGeneratedTitle(chatId, String(parsed.result));
        } catch (e) {}
        if (proc)
            proc.destroy();
    }

    // Stops the Claude Code replies running in the given chats.
    function stopClaudeCode(chatIds) {
        for (var i = 0; i < chatIds.length; i++)
            if (claudeCodeRuns[chatIds[i]])
                claudeCodeRuns[chatIds[i]].stop();
    }

    function setClaudeCodeRun(chatId, proc) {
        var runs = Object.assign({}, claudeCodeRuns);
        if (proc)
            runs[chatId] = proc;
        else
            delete runs[chatId];
        claudeCodeRuns = runs;
    }

    function sendClaudeCode(promptText, attachments) {
        claudeCodeElapsed = 0;
        currentActionText = randomThinkingVerb();
        const reply = startReply();

        var sid = claudeCodeSessionFor(reply.chatId);

        // Fresh session (new chat, the active account changed, or the working
        // directory changed) → seed it with the prior transcript so the conversation
        // carries over.
        var promptToSend = promptText;
        if (sid === "") {
            var transcript = ClaudeCode.transcript(chatStore.session(reply.chatId).messages);
            if (transcript !== "")
                promptToSend = transcript;
        }

        var props = {
            "chatId": reply.chatId,
            "msgId": reply.msgId,
            "workingDirectory": claudeCodeCwd(),
            "command": ClaudeCode.replyCommand({
                "bin": claudeCodeBinPath(),
                "prompt": promptToSend,
                "attachments": attachments,
                "permissionMode": claudeCodePermissionMode,
                "bypassAllowed": GlobalConfig.ai.claudeCodeSkipPermissions,
                "allowedTools": claudeCodeSidebarAllowedTools,
                "model": GlobalConfig.ai.defaultClaudeCodeModel,
                "effort": GlobalConfig.ai.claudeCodeEffort,
                "sessionId": sid
            })
        };
        var configDir = activeClaudeConfigDir();
        if (configDir !== "")
            props.environment = { "CLAUDE_CONFIG_DIR": configDir };

        var proc = claudeCodeSessionComponent.createObject(root, props);
        if (!proc) {
            chatStore.update(reply.chatId, reply.msgId, {
                "text": "⚠️ Failed to launch Claude Code: " + claudeCodeSessionComponent.errorString(),
                "isFinished": true
            });
            isTyping = false;
            isThinking = false;
            inAgentLoop = false;
            return;
        }
        proc.startedAt = Date.now();
        setClaudeCodeRun(reply.chatId, proc);
        proc.running = true;
    }

    // Updates from a running Claude Code session. They go to the session's own
    // message; the live status line only follows the chat that is open.
    function onClaudeCodeText(proc, text) {
        chatStore.update(proc.chatId, proc.msgId, { "text": text });
        if (proc.chatId !== currentChatId)
            return;
        isThinking = false;
        listView.positionViewAtEnd();
    }

    function onClaudeCodeThought(proc, text) {
        chatStore.update(proc.chatId, proc.msgId, { "thoughtText": text });
        if (proc.chatId === currentChatId)
            currentThoughtText = text;
    }

    function onClaudeCodeTools(proc, tools) {
        chatStore.update(proc.chatId, proc.msgId, { "toolsJson": tools.length > 0 ? JSON.stringify(tools) : "" });
        if (proc.chatId === currentChatId)
            listView.positionViewAtEnd();
    }

    function onClaudeCodeToolStarted(proc, name) {
        proc.toolRunning = true;
        if (proc.chatId !== currentChatId)
            return;
        currentActionText = "Running " + name + "…";
        isThinking = true;
    }

    function onClaudeCodeToolsDone(proc) {
        proc.toolRunning = false;
        if (proc.chatId === currentChatId)
            currentActionText = randomThinkingVerb();
    }

    function onClaudeCodeProgress(proc, text) {
        if (proc.chatId !== currentChatId)
            return;
        var shown = shortPaths(text);
        currentActionText = shown.length > 40 ? shown.substring(0, 40) + "…" : shown;
    }

    function onClaudeCodeTokens(proc, count) {
        proc.outputTokens = count;
    }

    function onClaudeCodeUsage(proc, usage) {
        chatStore.update(proc.chatId, proc.msgId, { "usageText": usage });
    }

    function onClaudeCodeCompleted(proc, reply) {
        var text = reply.text;
        if (text === "" && reply.tools.length === 0)
            text = proc.stopped ? qsTr("(stopped)") : "(no output)";
        setClaudeCodeSession(proc.chatId, reply.sessionId);
        chatStore.update(proc.chatId, proc.msgId, {
            "text": text,
            "toolsJson": reply.tools.length > 0 ? JSON.stringify(reply.tools) : "",
            "usageText": reply.usage,
            "isFinished": true
        });
        proc.toolRunning = false;
        if (proc.chatId === currentChatId) {
            isTyping = false;
            isThinking = false;
            inAgentLoop = false;
            currentActionText = "Thinking...";
            listView.positionViewAtEnd();
        }
    }

    function onClaudeCodeEnded(proc) {
        if (claudeCodeRuns[proc.chatId] === proc)
            setClaudeCodeRun(proc.chatId, null);
        proc.destroy();
    }

    property string currentActionText: "Thinking..."

    function fetchOllamaModels() {
        var ollamaUrl = GlobalConfig.ai.ollamaUrl || "http://localhost:11434";
        var xhr = new XMLHttpRequest();
        xhr.open("GET", ollamaUrl + "/api/tags", true);
        xhr.onreadystatechange = () => {
            if (xhr.readyState === XMLHttpRequest.DONE) {
                if (xhr.status === 200) {
                    try {
                        var response = JSON.parse(xhr.responseText);
                        var list = [];
                        if (response.models) {
                            for (var i = 0; i < response.models.length; i++) {
                                list.push(response.models[i].name);
                            }
                        }
                        ollamaModelsList = list;
                        if (list.length > 0 && list.indexOf(GlobalConfig.ai.defaultOllamaModel) === -1)
                            GlobalConfig.ai.defaultOllamaModel = list[0];
                    } catch (e) {
                        Logger.log("Error parsing Ollama models: " + e.message);
                    }
                } else {
                    Logger.log("Ollama tags request failed (status " + xhr.status + ")");
                }
            }
        };
        xhr.onerror = () => { root.logFetchError("Ollama"); };
        xhr.send();
    }

    property var openaiCompatModels: ({})

    function openaiCompatModelList(p) {
        return root.openaiCompatModels[p || provider] || [];
    }

    property var modelsFetched: ({})

    function fetchOpenaiCompatModels(p, force = false) {
        const which = p || provider;
        if (!force && root.modelsFetched[which])
            return;
        const key = root.getApiKeyFor(which);
        const publicCatalogue = which === "openrouter" || which === "opencode" || which === "opencode-go";
        if (key === "" && !publicCatalogue)
            return;

        var xhr = new XMLHttpRequest();
        xhr.open("GET", root.openaiCompatBase(which) + "/models", true);
        if (key !== "")
            root.setAuthHeader(xhr, which);
        xhr.onreadystatechange = () => {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return;
            if (xhr.status !== 200) {
                Logger.log("[AI] " + root.providerLabel(which) + " model list failed (status " + xhr.status + ")");
                return;
            }
            try {
                const parsed = JSON.parse(xhr.responseText);
                var list = [];
                for (var i = 0; i < (parsed.data || []).length; i++) {
                    const id = parsed.data[i].id;
                    if (!id)
                        continue;
                    list.push(id.indexOf("models/") === 0 ? id.substring(7) : id);
                }
                list = list.filter(m => !/embed|whisper|tts|audio|image|vision-preview|moderation|rerank|dall-e/i.test(m));
                if (which === "opencode" || which === "opencode-go")
                    list = list.filter(m => root.opencodeSupports(which, m));
                list.sort();
                if (list.length === 0)
                    return;
                var next = {};
                for (var k in root.openaiCompatModels)
                    next[k] = root.openaiCompatModels[k];
                next[which] = list;
                root.openaiCompatModels = next;

                const seen = root.modelsFetched;
                seen[which] = true;
                root.modelsFetched = seen;

                const cfgKey = root.defaultModelField(which);
                if (list.indexOf(GlobalConfig.ai[cfgKey]) === -1)
                    GlobalConfig.ai[cfgKey] = list[0];
            } catch (e) {
                Logger.log("[AI] Error parsing " + root.providerLabel(which) + " models: " + e.message);
            }
        };
        xhr.onerror = () => { root.logFetchError(which); };
        xhr.send();
    }

    function createNewChat() {
        leaveChat();
        chatStore.newChat({
            "claudeCodeCwd": claudeCodeChatCwd,
            "claudeCodePermissionMode": claudeCodePermissionMode
        });
        enterChat();
        attachments.take();
        isHistoryTab = false;
    }

    function loadChat(id) {
        leaveChat();
        if (!chatStore.open(id)) {
            createNewChat();
            return;
        }
        enterChat();
        savedContentY = -1;
        Qt.callLater(function() { listView.positionViewAtEnd(); });
        isHistoryTab = false;
    }

    // Before another chat is opened. A Claude Code reply keeps running in the
    // background; API replies, their tool loop and rate limit retries belong to
    // the open chat and end with it.
    function leaveChat() {
        cancelRateLimitRetry();
        if (currentRequest) {
            const xhr = currentRequest;
            currentRequest = null;
            xhr.abort();
        }
        isTyping = false;
        isThinking = false;
        inAgentLoop = false;
        currentThoughtText = "";
    }

    // After a chat is opened: pick up the status of a reply still running in it.
    function enterChat() {
        const proc = currentClaudeCodeProc;
        if (!proc)
            return;
        activeReply = { "chatId": proc.chatId, "msgId": proc.msgId };
        isTyping = true;
        inAgentLoop = true;
        claudeCodeElapsed = Math.floor((Date.now() - proc.startedAt) / 1000);
        currentActionText = randomThinkingVerb();
    }

    function loadHistory() {
        chatStore.load();
        if (chatStore.sessions.length > 0)
            loadChat(chatStore.sessions[0].id);
        else
            createNewChat();
    }

    // Ends whatever the current chat is waiting for: an API request, a rate
    // limit retry or a Claude Code process.
    function stopReply() {
        stopClaudeCode([currentChatId]);
        leaveChat();
    }

    // Starts an empty reply in the current chat for a request to write to.
    function startReply() {
        const msgs = chatStore.current().messages;
        const empty = msgs.filter(m => !m.isUser && !m.isFinished && m.text === "" && m.toolsJson === "");
        for (var i = 0; i < empty.length; i++)
            chatStore.remove(currentChatId, empty[i].msgId);
        activeReply = {
            "chatId": currentChatId,
            "msgId": chatStore.append(currentChatId, { "isFinished": false })
        };
        listView.positionViewAtEnd();
        return activeReply;
    }

    function applyGeneratedTitle(chatId, raw) {
        if (!raw)
            return;
        var title = raw.trim().replace(/^"|"$/g, '').replace(/\n/g, ' ');
        if (title.length > 40)
            title = title.substring(0, 40) + "...";
        if (title.length > 0)
            updateChatTitle(chatId, title);
    }

    function generateChatTitleAsync(chatId, firstMessage) {
        if (!firstMessage) return;

        if (root.isClaudeCode) {
            root.generateClaudeCodeTitleAsync(chatId, firstMessage);
            return;
        }

        var safeMsg = firstMessage.substring(0, 200);
        var titleSystem = "You are a title generator. Output ONLY a 2-4 word title representing the user's message. NO quotes, NO explanation.";
        var xhr = new XMLHttpRequest();

        if (root.isClaude) {
            if (root.getApiKey() === "")
                return;
            var claudeBase = GlobalConfig.ai.anthropicUrl || "https://api.anthropic.com";
            xhr.open("POST", claudeBase + "/v1/messages", true);
            xhr.setRequestHeader("Content-Type", "application/json");
            xhr.setRequestHeader("x-api-key", root.getApiKey());
            xhr.setRequestHeader("anthropic-version", "2023-06-01");
            xhr.setRequestHeader("anthropic-dangerous-direct-browser-access", "true");
            xhr.onreadystatechange = () => {
                if (xhr.readyState === XMLHttpRequest.DONE && xhr.status === 200) {
                    try {
                        var parsed = JSON.parse(xhr.responseText);
                        if (parsed.content && parsed.content.length > 0 && parsed.content[0].text)
                            root.applyGeneratedTitle(chatId, parsed.content[0].text);
                    } catch (e) {}
                }
            };
            xhr.onerror = () => {};
            xhr.send(JSON.stringify({
                model: root.activeModel(),
                max_tokens: 32,
                system: titleSystem,
                messages: [{ role: "user", content: "Message: " + safeMsg + "\nTitle:" }]
            }));
            return;
        }

        if (root.isOpenaiCompat) {
            if (root.getApiKey() === "")
                return;
            const useAnthropic = root.anthropicWire;
            xhr.open("POST", root.openaiCompatBase() + (useAnthropic ? "/messages" : "/chat/completions"), true);
            xhr.setRequestHeader("Content-Type", "application/json");
            root.setAuthHeader(xhr);
            if (useAnthropic)
                xhr.setRequestHeader("anthropic-version", "2023-06-01");
            xhr.onreadystatechange = () => {
                if (xhr.readyState === XMLHttpRequest.DONE && xhr.status === 200) {
                    try {
                        var oaiParsed = JSON.parse(xhr.responseText);
                        if (useAnthropic) {
                            if (oaiParsed.content && oaiParsed.content.length > 0 && oaiParsed.content[0].text)
                                root.applyGeneratedTitle(chatId, oaiParsed.content[0].text);
                        } else if (oaiParsed.choices && oaiParsed.choices.length > 0 && oaiParsed.choices[0].message) {
                            root.applyGeneratedTitle(chatId, oaiParsed.choices[0].message.content || "");
                        }
                    } catch (e) {}
                }
            };
            xhr.onerror = () => {};
            xhr.send(JSON.stringify(useAnthropic ? {
                model: root.activeModel(),
                max_tokens: 32,
                system: titleSystem,
                messages: [{ role: "user", content: "Message: " + safeMsg + "\nTitle:" }]
            } : {
                model: root.activeModel(),
                messages: [
                    { role: "system", content: titleSystem },
                    { role: "user", content: "Message: " + safeMsg + "\nTitle:" }
                ],
                stream: false
            }));
            return;
        }

        var url = (GlobalConfig.ai.ollamaUrl || "http://localhost:11434") + "/api/generate";
        xhr.open("POST", url, true);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onreadystatechange = () => {
            if (xhr.readyState === XMLHttpRequest.DONE && xhr.status === 200) {
                try {
                    var parsed = JSON.parse(xhr.responseText);
                    if (parsed.response)
                        root.applyGeneratedTitle(chatId, parsed.response);
                } catch (e) {}
            }
        };
        xhr.onerror = () => {};
        xhr.send(JSON.stringify({
            model: GlobalConfig.ai.defaultOllamaModel || "llama3",
            system: titleSystem,
            prompt: "Message: " + safeMsg + "\nTitle:",
            stream: false
        }));
    }

    function updateChatTitle(chatId, title) {
        if (title && chatId)
            chatStore.setTitle(chatId, title);
    }

    function addAiMessage(message) {
        chatStore.append(currentChatId, { "text": message || "" });
        listView.positionViewAtEnd();
    }

    function sendPrompt(promptText, isSystemToolResult = false, base64Image = null, toolName = "", isRetry = false) {
        var attachmentPaths = (!isSystemToolResult && !isRetry && root.isClaudeCode) ? attachments.pending.map(a => a.path) : [];
        if (!promptText.trim() && !base64Image && attachmentPaths.length === 0) return;
        attachments.take();

        promptSuggestions = [];

        if (!isRetry)
            cancelRateLimitRetry();

        if (!isSystemToolResult && !isRetry) {
            chatStore.append(currentChatId, {
                "isUser": true,
                "text": promptText || "",
                "attachments": attachmentPaths.join("\n")
            });
            listView.positionViewAtEnd();
        }

        if (root.needsApiKey && root.getApiKey() === "") {
            const envNames = {
                "claude": "ANTHROPIC_API_KEY",
                "openai": "OPENAI_API_KEY",
                "gemini": "GEMINI_API_KEY",
                "openrouter": "OPENROUTER_API_KEY",
                "opencode": "OPENCODE_API_KEY",
                "opencode-go": "OPENCODE_API_KEY"
            };
            addAiMessage("⚠️ No " + root.providerLabel(root.provider) + " API key configured. Set the "
                + (envNames[root.provider] || "API") + " environment variable, or add a key in the AI settings.");
            return;
        }

        isTyping = true;
        isThinking = true;
        inAgentLoop = true;
        currentThoughtText = "";
        isThoughtExpanded = false;
        
        if (isSystemToolResult) {
            if (toolName === "web_search" || toolName === "read_webpage") {
                currentActionText = "Reading results...";
            } else if (toolName === "take_screenshot") {
                currentActionText = "Analyzing screen...";
            } else if (toolName === "get_weather") {
                currentActionText = "Analyzing weather...";
            } else {
                currentActionText = "Thinking...";
            }
        } else {
            currentActionText = "Thinking...";
        }

        if (root.isClaudeCode) {
            root.sendClaudeCode(ClaudeCode.withAttachmentList(promptText, attachmentPaths), attachmentPaths);
            return;
        }

        var xhr = new XMLHttpRequest();
        root.currentRequest = xhr;

        var model = root.activeModel();
        const useAnthropic = root.anthropicWire;
        if (root.isClaude) {
            var claudeBase = GlobalConfig.ai.anthropicUrl || "https://api.anthropic.com";
            xhr.open("POST", claudeBase + "/v1/messages", true);
            xhr.setRequestHeader("Content-Type", "application/json");
            xhr.setRequestHeader("x-api-key", root.getApiKey());
            xhr.setRequestHeader("anthropic-version", "2023-06-01");
            xhr.setRequestHeader("anthropic-dangerous-direct-browser-access", "true");
        } else if (root.isOpenaiCompat) {
            xhr.open("POST", root.openaiCompatBase() + (useAnthropic ? "/messages" : "/chat/completions"), true);
            xhr.setRequestHeader("Content-Type", "application/json");
            root.setAuthHeader(xhr);
            if (useAnthropic)
                xhr.setRequestHeader("anthropic-version", "2023-06-01");
            if (root.provider === "openrouter") {
                xhr.setRequestHeader("HTTP-Referer", "https://github.com/ladybug-me/caelestia-kde");
                xhr.setRequestHeader("X-Title", "Caelestia Shell");
            }
        } else {
            var ollamaUrl = GlobalConfig.ai.ollamaUrl || "http://localhost:11434";
            xhr.open("POST", ollamaUrl + "/api/chat", true);
            xhr.setRequestHeader("Content-Type", "application/json");
        }
        
        var processedTextLength = 0;
        var accumulatedThoughtText = "";
        var accumulatedContentText = "";
        var rawAccumulatedContentText = "";
        var finalToolCalls = null;

        const reply = startReply();

        xhr.onerror = () => { root.handleSendError(reply); };
        xhr.onreadystatechange = () => {
            if (xhr.readyState === 3 || xhr.readyState === XMLHttpRequest.DONE) {
                if (xhr.status === 200) {
                    var currentText = xhr.responseText;
                    var unparsed = currentText.substring(processedTextLength);
                    var lines = unparsed.split('\n');
                    
                    var linesToProcess = (xhr.readyState === XMLHttpRequest.DONE) ? lines.length : lines.length - 1;
                    
                    for (var i = 0; i < linesToProcess; i++) {
                        var rawLine = lines[i];
                        var line = rawLine.trim();
                        if (line === "") {
                            processedTextLength += rawLine.length + 1;
                            continue;
                        }

                        var chunkContent = "";
                        var chunkReasoning = "";

                        if (useAnthropic) {
                            if (line.indexOf("event:") === 0) {
                                processedTextLength += rawLine.length + 1;
                                continue;
                            }
                            if (line.indexOf("data:") !== 0) {
                                processedTextLength += rawLine.length + 1;
                                continue;
                            }
                            var jsonStr = line.substring(5).trim();
                            if (jsonStr === "" || jsonStr === "[DONE]") {
                                processedTextLength += rawLine.length + 1;
                                continue;
                            }
                            try {
                                var evt = JSON.parse(jsonStr);
                                processedTextLength += rawLine.length + 1;
                                if (evt.type === "content_block_delta" && evt.delta) {
                                    if (evt.delta.type === "text_delta")
                                        chunkContent = evt.delta.text || "";
                                    else if (evt.delta.type === "thinking_delta")
                                        chunkReasoning = evt.delta.thinking || "";
                                } else if (evt.type === "error") {
                                    Logger.log("[AI] Claude stream error: " + JSON.stringify(evt.error || {}));
                                }
                            } catch (e) {
                                break;
                            }
                        } else if (root.isOpenaiCompat) {
                            if (line.indexOf("data:") !== 0) {
                                processedTextLength += rawLine.length + 1;
                                continue;
                            }
                            var oaiJson = line.substring(5).trim();
                            if (oaiJson === "" || oaiJson === "[DONE]") {
                                processedTextLength += rawLine.length + 1;
                                continue;
                            }
                            try {
                                var oaiEvt = JSON.parse(oaiJson);
                                processedTextLength += rawLine.length + 1;
                                if (oaiEvt.error) {
                                    Logger.log("[AI] " + root.providerLabel(root.provider) + " stream error: " + JSON.stringify(oaiEvt.error));
                                } else if (oaiEvt.choices && oaiEvt.choices.length > 0) {
                                    var delta = oaiEvt.choices[0].delta || {};
                                    chunkContent = delta.content || "";
                                    chunkReasoning = delta.reasoning_content || delta.reasoning || "";
                                }
                            } catch (e) {
                                break;
                            }
                        } else {
                            try {
                                var parsed = JSON.parse(line);
                                processedTextLength += rawLine.length + 1;
                                if (parsed.message) {
                                    chunkReasoning = parsed.message.thinking || parsed.message.reasoning || parsed.message.reasoning_content || "";
                                    chunkContent = parsed.message.content || "";
                                }
                            } catch (e) {
                                break;
                            }
                        }

                        if (chunkReasoning)
                            accumulatedThoughtText += chunkReasoning;
                        if (chunkContent)
                            rawAccumulatedContentText += chunkContent;

                        if (chunkContent === "" && chunkReasoning === "")
                            continue;

                        var displayContent = AiToolCalls.stripToolCalls(rawAccumulatedContentText);
                        var displayThought = accumulatedThoughtText;

                        if (accumulatedThoughtText === "") {
                            var openThinkIdx = displayContent.indexOf("<think>");
                            var closeThinkIdx = displayContent.indexOf("</think>");

                            if (openThinkIdx !== -1) {
                                if (closeThinkIdx !== -1) {
                                    displayThought = displayContent.substring(openThinkIdx + 7, closeThinkIdx).trim();
                                    displayContent = displayContent.substring(0, openThinkIdx) + displayContent.substring(closeThinkIdx + 8);
                                } else {
                                    displayThought = displayContent.substring(openThinkIdx + 7).trim();
                                    displayContent = displayContent.substring(0, openThinkIdx);
                                }
                            }
                        }

                        root.currentThoughtText = displayThought.trim();

                        if (displayContent.trim() !== "") {
                            if (isThinking) isThinking = false;
                        }

                        chatStore.update(reply.chatId, reply.msgId, {
                            "thoughtText": displayThought.trim(),
                            "text": displayContent.trim()
                        });
                        listView.positionViewAtEnd();
                    }
                }
                
                if (xhr.readyState === XMLHttpRequest.DONE) {
                    if (root.currentRequest === xhr)
                        root.currentRequest = null;
                    if (xhr.status === 200) {
                        root.rateLimitRetries = 0;
                        chatStore.update(reply.chatId, reply.msgId, { "isFinished": true });
                        
                        var enableTools = GlobalConfig.ai.enableCelestialMode;
                        var textToolCalls = enableTools ? AiToolCalls.parseTextToolCalls(rawAccumulatedContentText) : [];

                        if (textToolCalls.length > 0) {
                            if (enableTools) {
                                currentActionText = "Using tools...";
                                AiToolCalls.reset();

                                for (var t = 0; t < textToolCalls.length; t++) {
                                    var toolCall = textToolCalls[t];
                                    var toolName = toolCall.name;
                                    var args = toolCall.args || {};

                                    if (toolName === "take_screenshot" || toolName === "web_search" || toolName === "read_webpage" || toolName === "open_app" || toolName === "caelestia_command") {
                                        AiToolCalls.runningToolsCount++;

                                    }

                                    if (toolName === "take_screenshot") {
                                        currentActionText = "Analyzing screen...";
                                        var screenCmd = `spectacle -b -m -n -o ${Paths.runtimeTemp("orion_screenshot.png")}`;
                                        runAgentCommand(screenCmd, "screenshot_take");

                                    } else if (toolName === "web_search") {
                                        currentActionText = "Searching the web...";
                                        var query = String(args.query || "");
                                        var page = args.page || 1;
                                        runAgentCommand(["env", "PYTHONIOENCODING=utf8", "python3", Quickshell.shellDir + "/scripts/orion_search.py", "--mode", "search", "--query", query, "--page", String(page)], "exec_" + toolName);

                                    } else if (toolName === "read_webpage") {
                                        currentActionText = "Reading webpage...";
                                        var url = String(args.url || "");
                                        runAgentCommand(["env", "PYTHONIOENCODING=utf8", "python3", Quickshell.shellDir + "/scripts/orion_search.py", "--mode", "read", "--url", url], "exec_" + toolName);

                                    } else if (toolName === "open_app") {
                                        currentActionText = "Opening app...";
                                        var app = String(args.app_name || "");
                                        var safeApp = shellQuote("Name=.*" + app);
                                        runAgentCommand(["sh", "-c", 'grep -i -m 1 "^Exec=" $(find /usr/share/applications ~/.local/share/applications -name "*.desktop" -exec grep -il "$1" {} + 2>/dev/null) | cut -d "=" -f 2- | sed "s/ %[a-zA-Z]//g" | xargs -I {} sh -c "setsid {} >/dev/null 2>&1 &"', "--", safeApp], "exec_" + toolName);

                                    } else if (toolName === "set_timer") {
                                        currentActionText = "Setting timer...";
                                        var secs = Number(args.seconds) || 5;
                                        var msg = String(args.message || "Timer finished");
                                        var timerQml = "import QtQuick; Timer { interval: " + (secs * 1000) + "; running: true; onTriggered: { root.runAgentCommand(['notify-send', 'Orion Timer', " + JSON.stringify(msg) + "], 'timer_trigger'); destroy(); } }";
                                        Qt.createQmlObject(timerQml, root, "timer_" + Date.now());
                                        AiToolCalls.recordResult("Tool: set_timer\nResult: Timer set for " + secs + " seconds with message: " + msg + "\n\n");

                                    } else if (toolName === "get_weather") {
                                        currentActionText = "Checking weather...";
                                        var weatherStr = Weather.city + ": " + Weather.temp + " (" + Weather.description + "). Humidity: " + Weather.humidity + "%, Wind: " + Weather.windSpeed + " km/h";
                                        AiToolCalls.recordResult("Tool: get_weather\nResult: Local weather from system dashboard: " + weatherStr + "\n\n");

                                    } else if (toolName === "caelestia_command") {
                                        currentActionText = "Running caelestia...";
                                        var subcmd = String(args.subcommand || "");
                                        var subargs = String(args.args || "").trim();
                                        var cmdArr = ["caelestia", subcmd];
                                        if (subargs) cmdArr = cmdArr.concat(subargs.split(/\s+/));
                                        runAgentCommand(cmdArr, "exec_" + toolName);

                                    } else {
                                        Logger.log("[AI] Unknown tool: " + toolName);
                                        AiToolCalls.recordResult("Tool: " + toolName + "\nResult: unknown tool; nothing was executed.\n\n");

                                    }
                                }

                                if (AiToolCalls.runningToolsCount === 0) {
                                    if (AiToolCalls.accumulatedToolResults !== "") {
                                        AiToolCalls.checkToolsFinished();

                                    } else {
                                        currentActionText = "Thinking...";
                                        isTyping = false;
                                        isThinking = false;
                                        inAgentLoop = false;
                                    }
                                }
                            } else {
                                currentActionText = "Thinking...";
                                isTyping = false;
                                isThinking = false;
                                inAgentLoop = false;
                            }
                        } else {
                            currentActionText = "Thinking...";
                            isTyping = false;
                            isThinking = false;
                            inAgentLoop = false;
                        }
                    } else {
                        var providerName = root.providerLabel(root.provider);
                        var apiDetail = "";
                        try {
                            const errBody = JSON.parse(xhr.responseText);
                            const e = Array.isArray(errBody) ? (errBody[0] || {}).error : errBody.error;
                            if (e) {
                                const raw = e.metadata && e.metadata.raw ? String(e.metadata.raw) : "";
                                const provider = e.metadata && e.metadata.provider_name ? String(e.metadata.provider_name) : "";
                                if (raw)
                                    apiDetail = " " + (provider ? provider + ": " : "") + raw.split("\n")[0];
                                else if (e.message)
                                    apiDetail = " " + String(e.message).split("\n")[0];
                            }
                        } catch (e) {}
                        const perDayQuota = /PerDay|per day/i.test(xhr.responseText || "");
                        if (xhr.status === 429 && perDayQuota)
                            root.cancelRateLimitRetry();

                        if (xhr.status === 429 && !perDayQuota && root.rateLimitRetries < root.maxRateLimitRetries) {
                            if ((xhr.responseText || "").indexOf("free_tier") !== -1)
                                root.onFreeTier = true;
                            const waitMs = root.rateLimitDelayMs(xhr);
                            root.rateLimitRetries++;
                            root.rateLimitSecondsLeft = Math.max(1, Math.round(waitMs / 1000));
                            root.currentActionText = qsTr("Rate limited - retrying in %1s…").arg(root.rateLimitSecondsLeft);
                            root.isTyping = true;
                            root.isThinking = true;
                            rateLimitRetryTimer.forChat = root.currentChatId;
                            rateLimitRetryTimer.forModel = root.activeModel();
                            rateLimitRetryTimer.retryFn = () => root.sendPrompt(promptText, isSystemToolResult, base64Image, toolName, true);
                            rateLimitRetryTimer.restart();
                            return;
                        }

                        var hint = "";
                        if (xhr.status === 429 && perDayQuota)
                            hint = " This model's daily free quota is used up - it resets tomorrow. Pick another model, or use Claude Code, which is not on this quota.";
                        else if (xhr.status === 429)
                            hint = " Rate limit reached and still limited after " + root.maxRateLimitRetries + " retries - wait a minute and try again.";
                        else if (root.needsApiKey && (xhr.status === 401 || xhr.status === 403))
                            hint = " Check your API key.";
                        var errMsg = (xhr.status === 0) ? "Generation canceled" : (providerName + " request failed (status " + xhr.status + ")." + hint + apiDetail);
                        var currentText = (chatStore.message(reply.chatId, reply.msgId) || {}).text || "";
                        chatStore.update(reply.chatId, reply.msgId, {
                            "text": currentText.trim() === "" ? errMsg : currentText + "\n\n*[" + errMsg + "]*",
                            "isFinished": true
                        });
                        isTyping = false;
                        isThinking = false;
                        inAgentLoop = false;
                    }
                }
            }
        };

        var enableTools = GlobalConfig.ai.enableCelestialMode;
        var sysPrompt = "You are a helpful AI assistant integrated into the user's desktop OS shell (Caelestia, running on KDE Plasma/Wayland).";
        if (enableTools) {
            sysPrompt += AiToolCalls.toolsPrompt();
        }
        
        var requestBody;
        if (useAnthropic) {
            var claudeMessages = [];
            var history = chatStore.session(reply.chatId).messages;
            for (var i = 0; i < history.length; i++) {
                var msg = history[i];
                if (!msg.isUser && !msg.isFinished && (msg.text || "") === "")
                    continue;
                if ((msg.text || "") === "")
                    continue;
                claudeMessages.push({
                    "role": msg.isUser ? "user" : "assistant",
                    "content": msg.text
                });
            }

            if (isSystemToolResult) {
                var claudeContent;
                if (base64Image) {
                    claudeContent = [
                        { "type": "text", "text": promptText },
                        { "type": "image", "source": { "type": "base64", "media_type": "image/jpeg", "data": base64Image } }
                    ];
                } else {
                    claudeContent = promptText;
                }
                claudeMessages.push({ "role": "user", "content": claudeContent });
            }

            requestBody = {
                "model": model,
                "max_tokens": 4096,
                "system": sysPrompt,
                "messages": claudeMessages,
                "stream": true
            };
        } else {
            var messages = [];
            messages.push({
                "role": "system",
                "content": sysPrompt
            });

            var chatMsgs = chatStore.session(reply.chatId).messages;
            for (var j = 0; j < chatMsgs.length; j++) {
                var m = chatMsgs[j];
                messages.push({
                    "role": m.isUser ? "user" : "assistant",
                    "content": m.text || ""
                });
            }

            if (isSystemToolResult) {
                var toolMsg = {
                    "role": "user",
                    "content": promptText
                };
                if (base64Image) {
                    if (root.isOpenaiCompat) {
                        toolMsg["content"] = [
                            { "type": "text", "text": promptText },
                            { "type": "image_url", "image_url": { "url": "data:image/jpeg;base64," + base64Image } }
                        ];
                    } else {
                        toolMsg["images"] = [base64Image];
                    }
                }
                messages.push(toolMsg);
            }

            requestBody = {
                "model": model,
                "messages": messages,
                "stream": true
            };
        }

        xhr.send(JSON.stringify(requestBody));
    }

    Item {
        id: mainWrapper

        anchors.fill: parent
        anchors.margins: Tokens.padding.medium

         RowLayout {
             id: modeSwitcherRow

             anchors.top: parent.top
             anchors.left: parent.left
             anchors.right: parent.right
             anchors.rightMargin: 0
             z: 10
             spacing: Tokens.spacing.small

             StyledRect {
                 id: modeSwitcherBg

                 implicitWidth: modeRow.width
                 implicitHeight: 32
                 radius: Tokens.rounding.full
                 color: Colours.tPalette.m3surfaceContainer

                 StyledClippingRect {
                     z: -1
                     anchors.fill: parent
                     radius: Tokens.rounding.full

                     ShaderEffectSource {
                         id: switcherBlurSource

                         sourceItem: contentStack
                         sourceRect: {
                             var p = parent.mapToItem(contentStack, 0, 0);
                             return Qt.rect(p.x, p.y, parent.width, parent.height);
                         }
                     }
                     MultiEffect {
                         anchors.fill: parent
                         source: switcherBlurSource
                         blurEnabled: true
                         blurMax: 32
                     }
                 }

                 StyledRect {
                     width: isHistoryTab ? historyTab.width : chatTab.width
                     height: parent.height
                     radius: Tokens.rounding.full
                     color: Colours.palette.m3primary
                     x: isHistoryTab ? historyTab.x : chatTab.x
                     
                     Behavior on x { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                     Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                 }

                 Row {
                     id: modeRow

                     height: parent.height

                     Item {
                         id: chatTab

                         height: parent.height
                         width: !isHistoryTab ? 40 : chatContent.implicitWidth + Tokens.padding.medium * 2
                         

                         Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }

                         StateLayer {
                             radius: Tokens.rounding.full
                             onClicked: isHistoryTab = false
                         }

                         Row {
                             id: chatContent

                             anchors.centerIn: parent
                             spacing: Tokens.spacing.small

                             MaterialIcon {
                                 anchors.verticalCenter: parent.verticalCenter
                                 text: "chat"
                                 color: !isHistoryTab ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                             }
                             Text {
                                 anchors.verticalCenter: parent.verticalCenter
                                 text: qsTr("Chat")
                                 color: !isHistoryTab ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.body.small
                                 visible: isHistoryTab
                             }
                         }
                     }

                     Item {
                         id: historyTab

                         height: parent.height
                         width: isHistoryTab ? 40 : historyContent.implicitWidth + Tokens.padding.medium * 2
                         

                         Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }

                         StateLayer {
                             radius: Tokens.rounding.full
                             onClicked: isHistoryTab = true
                         }

                         Row {
                             id: historyContent

                             anchors.centerIn: parent
                             spacing: Tokens.spacing.small

                             MaterialIcon {
                                 anchors.verticalCenter: parent.verticalCenter
                                 text: "history"
                                 color: isHistoryTab ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                             }
                             Text {
                                 anchors.verticalCenter: parent.verticalCenter
                                 text: qsTr("History")
                                 color: isHistoryTab ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.body.small
                                 visible: !isHistoryTab
                             }
                         }
                     }
                 }
             }

         }

         Flow {
             id: selectorRow

             anchors.top: modeSwitcherRow.bottom
             anchors.left: parent.left
             anchors.right: parent.right
             anchors.topMargin: Tokens.spacing.small
             z: 10
             spacing: Tokens.spacing.small

             SplitButton {
                 id: providerSelector

                 type: SplitButton.Tonal
                 verticalPadding: 4
                 visible: root.providerList.length > 1
                 Layout.preferredWidth: implicitWidth

                 active: menuItems.find(m => m.modelData === root.provider) ?? menuItems[0] ?? null
                 menu.onItemSelected: item => {
                     GlobalConfig.ai.defaultProvider = item.modelData;
                 }

                 menuItems: providerVariants.instances

                 fallbackIcon: "cloud"
                 fallbackText: qsTr("Provider")
                 stateLayer.disabled: true

                 Variants {
                     id: providerVariants

                     model: root.providerList

                     delegate: MenuItem {
                         required property string modelData

                         text: root.providerLabel(modelData)
                     }
                 }
             }

             SplitButton {
                 id: modelSelector

                 type: SplitButton.Tonal
                 verticalPadding: 4
                 Layout.preferredWidth: implicitWidth

                 active: menuItems.find(m => m.modelData === root.activeModel()) ?? menuItems[0] ?? null
                 menu.onItemSelected: item => {
                     if (root.isClaudeCode) {
                         GlobalConfig.ai.defaultClaudeCodeModel = item.modelData;
                         GlobalConfig.ai.claudeCodeEffort = "default";
                     } else if (root.isClaude)
                         GlobalConfig.ai.defaultClaudeModel = item.modelData;
                     else if (root.isOpenaiCompat)
                         GlobalConfig.ai[root.defaultModelField(root.provider)] = item.modelData;
                     else
                         GlobalConfig.ai.defaultOllamaModel = item.modelData;
                 }

                 menuItems: modelVariants.instances

                 fallbackIcon: "smart_toy"
                 fallbackText: qsTr("Select Model")
                 stateLayer.disabled: true

                 Variants {
                     id: modelVariants

                     model: {
                         if (root.isClaudeCode)
                             return root.claudeCodeModelsList;
                         if (root.isClaude)
                             return root.claudeModelsList;
                         if (root.isOpenaiCompat)
                             return root.openaiCompatModelList();
                         return root.ollamaModelsList;
                     }

                     delegate: MenuItem {
                         required property string modelData

                         text: modelData
                     }
                 }
             }

             SplitButton {
                 id: effortSelector

                 type: SplitButton.Tonal
                 verticalPadding: 4
                 visible: root.isClaudeCode && root.claudeCodeEffortOptions.length > 0

                 active: menuItems.find(m => m.modelData === (GlobalConfig.ai.claudeCodeEffort || "default")) ?? menuItems[0] ?? null
                 menu.onItemSelected: item => {
                     GlobalConfig.ai.claudeCodeEffort = item.modelData;
                 }

                 menuItems: effortVariants.instances

                 fallbackIcon: "neurology"
                 fallbackText: qsTr("Effort")
                 stateLayer.disabled: true

                 Variants {
                     id: effortVariants

                     model: root.claudeCodeEffortOptions

                     delegate: MenuItem {
                         required property string modelData

                         text: modelData
                     }
                 }
             }

             SplitButton {
                 id: accountSelector

                 type: SplitButton.Tonal
                 verticalPadding: 4
                 visible: root.isClaudeCode && root.claudeAccountIds.length > 1

                 active: menuItems.find(m => m.modelData === (GlobalConfig.ai.activeClaudeAccount || "")) ?? menuItems[0] ?? null
                 menu.onItemSelected: item => {
                     GlobalConfig.ai.activeClaudeAccount = item.modelData;
                 }

                 menuItems: accountVariants.instances

                 fallbackIcon: "person"
                 fallbackText: qsTr("Account")
                 stateLayer.disabled: true

                 Variants {
                     id: accountVariants

                     model: root.claudeAccountIds

                     delegate: MenuItem {
                         required property string modelData

                         text: root.accountLabel(modelData)
                     }
                 }
             }

             // Permission mode for this chat (Claude Code).
             SplitButton {
                 id: permissionSelector

                 type: SplitButton.Tonal
                 verticalPadding: 4
                 visible: root.isClaudeCode

                 active: menuItems.find(m => m.modelData === ClaudeCode.effectivePermissionMode(root.claudeCodePermissionMode, GlobalConfig.ai.claudeCodeSkipPermissions)) ?? menuItems[0] ?? null
                 menu.onItemSelected: item => root.setClaudeCodePermissionMode(item.modelData)

                 menuItems: permissionVariants.instances

                 fallbackIcon: "shield"
                 fallbackText: qsTr("Permissions")
                 stateLayer.disabled: true

                 Variants {
                     id: permissionVariants

                     model: root.claudeCodePermissionModes

                     delegate: MenuItem {
                         required property string modelData

                         text: root.permissionModeLabel(modelData)
                     }
                 }
             }

             // Working directory for this chat (Claude Code).
             StyledRect {
                 id: cwdButton

                 visible: root.isClaudeCode
                 implicitWidth: Math.min(cwdRow.implicitWidth + Tokens.padding.medium * 2, 220)
                 implicitHeight: permissionSelector.height
                 radius: Tokens.rounding.full
                 color: Colours.tPalette.m3secondaryContainer

                 StateLayer {
                     radius: Tokens.rounding.full
                     color: Colours.palette.m3onSecondaryContainer
                     onClicked: cwdDialog.open()
                 }

                 RowLayout {
                     id: cwdRow

                     anchors.fill: parent
                     anchors.leftMargin: Tokens.padding.medium
                     anchors.rightMargin: Tokens.padding.medium
                     spacing: Tokens.spacing.small

                     MaterialIcon {
                         text: "folder"
                         color: Colours.palette.m3onSecondaryContainer
                         font: Tokens.font.icon.small
                     }

                     StyledText {
                         Layout.fillWidth: true
                         text: root.shortPath(root.claudeCodeChatCwd)
                         color: Colours.palette.m3onSecondaryContainer
                         font: Tokens.font.label.medium
                         elide: Text.ElideMiddle
                     }
                 }

                 FileDialog {
                     id: cwdDialog

                     selectFolder: true
                     title: qsTr("Claude Code working directory")
                     onAccepted: path => root.setClaudeCodeCwd(path)
                 }
             }

             // Continue this chat's session in a terminal (claude --resume).
             StyledRect {
                 visible: root.isClaudeCode
                 implicitWidth: permissionSelector.height
                 implicitHeight: permissionSelector.height
                 radius: Tokens.rounding.full
                 color: Colours.tPalette.m3secondaryContainer

                 StateLayer {
                     radius: Tokens.rounding.full
                     color: Colours.palette.m3onSecondaryContainer
                     onClicked: root.openClaudeCodeInTerminal()
                 }

                 MaterialIcon {
                     anchors.centerIn: parent
                     text: "terminal"
                     color: Colours.palette.m3onSecondaryContainer
                     font: Tokens.font.icon.small
                 }
             }

         }
         
         Item {
             id: contentStack

             anchors.top: selectorRow.bottom
             anchors.bottom: parent.bottom
             anchors.left: parent.left
             anchors.right: parent.right
             anchors.topMargin: Tokens.spacing.medium

             Item {
                 anchors.fill: parent
                 opacity: !isHistoryTab ? 1 : 0
                 visible: opacity > 0

                 Behavior on opacity { NumberAnimation { duration: 250; easing.type: Easing.InOutQuad } }

                 VerticalFadeFlickable {
                     id: listView

                     // Follows the end of the chat (new messages, a streaming reply)
                     // until the user scrolls away from it.
                     property bool pinnedToEnd: false
                     property bool settingY: false

                     function positionViewAtEnd(): void {
                         pinnedToEnd = true;
                         Qt.callLater(snapToEnd);
                     }

                     function snapToEnd(): void {
                         if (!pinnedToEnd)
                             return;
                         settingY = true;
                         contentY = Math.max(0, contentHeight - height);
                         settingY = false;
                     }

                     anchors.top: parent.top
                     anchors.bottom: attachmentStrip.visible ? attachmentStrip.top : inputBoxRow.top
                     anchors.left: parent.left
                     anchors.right: parent.right
                     anchors.bottomMargin: Tokens.spacing.medium
                     // Every message is laid out, not just the ones in view: with
                     // messages of very different heights a ListView can only estimate
                     // the ones it has not created, and the estimate (the scrollbar's
                     // size and position) changes as they are created while scrolling.
                     contentWidth: width
                     contentHeight: messageColumn.implicitHeight
                     boundsBehavior: Flickable.StopAtBounds
                     onContentHeightChanged: snapToEnd()
                     onHeightChanged: snapToEnd()
                     onContentYChanged: {
                         if (!settingY && contentY < contentHeight - height - 2)
                             pinnedToEnd = false;
                     }

                     ScrollBar.vertical: StyledScrollBar {
                         flickable: listView
                     }

                     Column {
                         id: messageColumn

                         width: listView.width

                         Column {
                             width: parent.width
                             spacing: Tokens.spacing.medium

                             Repeater {
                                 model: chatStore.messages

                                 ChatMessage {
                                     width: listView.width - Tokens.padding.large
                                     thinking: root.isThinking
                                     onViewStateEdited: patch => chatStore.update(root.currentChatId, msgId, patch)
                                 }
                             }
                         }

                         // Claude Code keeps its status line up for the whole reply,
                         // below the text as it streams.
                         Item {
                             id: statusFooter

                             readonly property bool shown: isThinking || (root.isClaudeCode && root.isTyping)
                             readonly property real maxBubbleWidth: listView.width * 0.85
                             readonly property real naturalWidth: footerCol.implicitWidth + Tokens.padding.medium * 2 + 8
                             // Widest the bubble has been during this reply. The verb, the
                             // counters and the thoughts all change width as they update;
                             // only growing keeps the bubble from wobbling.
                             property real stableWidth: 0

                             onNaturalWidthChanged: if (shown) stableWidth = Math.max(stableWidth, naturalWidth)
                             onShownChanged: stableWidth = shown ? naturalWidth : 0

                             width: listView.width
                             height: shown ? bubbleBg.height + Tokens.spacing.medium : 0
                             visible: opacity > 0
                             opacity: shown ? 1 : 0
                             
                             Behavior on height { Anim { type: Anim.DefaultSpatial } }
                             Behavior on opacity { Anim { type: Anim.DefaultSpatial } }

                             StyledRect {
                                 id: bubbleBg

                                 y: Tokens.spacing.medium / 2
                                 width: Math.min(statusFooter.maxBubbleWidth, Math.max(statusFooter.stableWidth, statusFooter.naturalWidth))
                                 height: footerCol.implicitHeight + Tokens.padding.medium * 2
                                 radius: Tokens.rounding.large
                                 color: Colours.tPalette.m3surfaceContainer

                                 topLeftRadius: Tokens.rounding.large
                                 topRightRadius: Tokens.rounding.large
                                 bottomLeftRadius: 4
                                 bottomRightRadius: Tokens.rounding.large

                                 Column {
                                     id: footerCol

                                     anchors.fill: parent
                                     anchors.margins: Tokens.padding.medium
                                     spacing: Tokens.spacing.small
                                     
                                     Row {
                                         spacing: Tokens.spacing.small
                                         
                                         LoadingIndicator {
                                             visible: !root.isClaudeCode
                                             width: 20
                                             height: 20
                                             color: Colours.palette.m3primary
                                         }

                                         // Claude Code: the CLI's twinkling star.
                                         StyledText {
                                             id: starGlyph

                                             readonly property var frames: ["·", "✢", "✳", "✶", "✻", "✽", "✻", "✶", "✳", "✢"]
                                             property int frame: 0

                                             visible: root.isClaudeCode
                                             // The frames come from different fallback fonts with
                                             // different line heights; a fixed box keeps the row
                                             // (and the bubble) from bouncing with every frame.
                                             width: 20
                                             height: 20
                                             horizontalAlignment: Text.AlignHCenter
                                             verticalAlignment: Text.AlignVCenter
                                             anchors.verticalCenter: parent.verticalCenter
                                             text: frames[frame]
                                             color: root.claudeCodeToolRunning ? Colours.palette.m3tertiary : Colours.palette.m3primary
                                             font.pointSize: Tokens.font.body.small.pointSize * 1.2
                                             font.family: Tokens.font.body.small.family

                                             Behavior on color { CAnim {} }

                                             Timer {
                                                 interval: 120
                                                 repeat: true
                                                 running: starGlyph.visible && root.isTyping
                                                 onTriggered: starGlyph.frame = (starGlyph.frame + 1) % starGlyph.frames.length
                                             }
                                         }
                                         
                                         Item {
                                             width: mainText.implicitWidth
                                             height: mainText.implicitHeight

                                             Behavior on width { NumberAnimation { duration: 300; easing.type: Easing.OutCubic } }
                                             
                                             StyledText {
                                                 id: mainText

                                                 text: displayedText
                                                 color: Colours.palette.m3onSurfaceVariant
                                                 font: Tokens.font.body.small
                                                 
                                                 property string displayedText: root.currentActionText

                                                 property string nextText: ""

                                                 transform: Translate { id: textTrans; y: 0 }
                                                 opacity: 1.0

                                                 Connections {
                                                     target: root

                                                     function onCurrentActionTextChanged() {
                                                         if (root.currentActionText !== mainText.displayedText) {
                                                             mainText.nextText = root.currentActionText;
                                                             switchAnim.restart();
                                                         }
                                                     }
                                                 }

                                                 SequentialAnimation {
                                                     id: switchAnim

                                                     ParallelAnimation {
                                                         NumberAnimation { target: textTrans; property: "y"; to: -8; duration: 150; easing.type: Easing.InCubic }
                                                         NumberAnimation { target: mainText; property: "opacity"; to: 0.0; duration: 150; easing.type: Easing.InCubic }
                                                     }
                                                     PropertyAction { target: mainText; property: "displayedText"; value: mainText.nextText }
                                                     PropertyAction { target: textTrans; property: "y"; value: 8 }
                                                     ParallelAnimation {
                                                         NumberAnimation { target: textTrans; property: "y"; to: 0; duration: 400; easing.type: Easing.OutBack; easing.overshoot: 1.5 }
                                                         NumberAnimation { target: mainText; property: "opacity"; to: 1.0; duration: 250; easing.type: Easing.OutQuad }
                                                     }
                                                 }

                                                 SequentialAnimation {
                                                     running: isThinking && !switchAnim.running
                                                     loops: Animation.Infinite

                                                     NumberAnimation { target: mainText; property: "opacity"; from: 1.0; to: 0.4; duration: 800; easing.type: Easing.InOutSine }
                                                     NumberAnimation { target: mainText; property: "opacity"; from: 0.4; to: 1.0; duration: 800; easing.type: Easing.InOutSine }
                                                 }
                                             }
                                         }
                                         
                                         Item {
                                             visible: root.currentThoughtText !== ""
                                             width: Tokens.spacing.medium
                                             height: 1
                                         }
                                         
                                         Item {
                                             visible: root.currentThoughtText !== ""
                                             width: thoughtRowFooter.implicitWidth
                                             height: thoughtRowFooter.implicitHeight

                                             Row {
                                                 id: thoughtRowFooter

                                                 spacing: Tokens.spacing.small

                                                 MaterialIcon {
                                                     text: "expand_more"
                                                     color: Colours.palette.m3onSurfaceVariant
                                                     font: Tokens.font.icon.small
                                                     rotation: root.isThoughtExpanded ? 180 : 0

                                                     Behavior on rotation { NumberAnimation { duration: 150; easing.type: Easing.OutQuad } }
                                                 }
                                             }
                                             MouseArea {
                                                 anchors.fill: parent
                                                 anchors.margins: -10
                                                 cursorShape: Qt.PointingHandCursor
                                                 onClicked: root.isThoughtExpanded = !root.isThoughtExpanded
                                             }
                                         }
                                     }
                                     // Claude Code: elapsed time and output tokens on their own line.
                                     StyledText {
                                         visible: root.isClaudeCode && root.isTyping
                                         width: Math.min(implicitWidth, statusFooter.maxBubbleWidth - Tokens.padding.medium * 2 - 8)
                                         leftPadding: 20 + Tokens.spacing.small
                                         text: root.formatElapsed(root.claudeCodeElapsed)
                                             + (root.claudeCodeOutTokens > 0 ? " · ↓ " + ClaudeCode.formatTokens(root.claudeCodeOutTokens) + " tokens" : "")
                                         color: Colours.palette.m3outline
                                         font.family: Tokens.font.mono.small.family
                                         font.pointSize: Tokens.font.label.small.pointSize
                                         elide: Text.ElideRight
                                     }

                                     Item {
                                         id: footerThoughtContentWrapper

                                         width: root.isThoughtExpanded ? footerThoughtContent.width : 0
                                         height: root.isThoughtExpanded ? footerThoughtContent.implicitHeight : 0
                                         clip: true
                                         
                                         Behavior on height { NumberAnimation { duration: 200; easing.type: Easing.InOutQuad } }

                                         TextEdit {
                                             id: footerThoughtContent

                                             width: Math.min(implicitWidth, listView.width * 0.85 - Tokens.padding.medium * 2)
                                             textFormat: Text.MarkdownText
                                             text: root.currentThoughtText
                                             color: Colours.palette.m3onSurfaceVariant
                                             font: Tokens.font.body.small
                                             wrapMode: Text.Wrap
                                             readOnly: true
                                             selectByMouse: true
                                             selectionColor: Colours.palette.m3primary
                                             selectedTextColor: Colours.palette.m3onPrimary
                                             opacity: root.isThoughtExpanded ? 1.0 : 0.0
                                             
                                             Behavior on opacity {
                                                 SequentialAnimation {
                                                     PauseAnimation { duration: root.isThoughtExpanded ? 100 : 0 }
                                                     NumberAnimation { duration: 150; easing.type: Easing.InOutQuad }
                                                 }
                                             }
                                         }
                                     }
                                 }
                             }
                         }
                     }
                 }

                 ColumnLayout {
                     anchors.centerIn: listView
                     opacity: chatStore.messages.count === 0 && !isTyping && !isThinking ? 1.0 : 0.0
                     visible: opacity > 0

                     Behavior on opacity { NumberAnimation { duration: 250; easing.type: Easing.InOutQuad } }

                     spacing: Tokens.spacing.large

                     Item {
                         Layout.alignment: Qt.AlignHCenter
                         implicitWidth: 72
                         implicitHeight: 72

                         Logo {
                             id: emptyStateLogo

                             anchors.fill: parent
                             visible: false
                         }

                         MultiEffect {
                             anchors.fill: parent
                             source: emptyStateLogo
                             colorization: 1.0
                             colorizationColor: Colours.palette.m3primary
                         }
                     }

                     StyledText {
                         id: greetingText
                         Layout.alignment: Qt.AlignHCenter
                         Layout.maximumWidth: listView.width - (Tokens.padding.large * 2)

                         horizontalAlignment: Text.AlignHCenter
                         wrapMode: Text.Wrap
                         font: Tokens.font.title.medium
                         color: Colours.palette.m3onSurfaceVariant

                         property var phrases: [
                             "Ask away, %1!",
                             "How can I help you today, %1?",
                             "What's on your mind, %1?",
                             "Ready when you are, %1!",
                             "Let's get started, %1.",
                             "What shall we explore today, %1?",
                             "I'm all ears, %1!"
                         ]

                         Component.onCompleted: {
                             var user = Quickshell.env("USER") || "user";
                             var userCapitalized = user.charAt(0).toUpperCase() + user.slice(1);
                             var phrase = phrases[Math.floor(Math.random() * phrases.length)];
                             text = phrase.replace("%1", userCapitalized);
                         }
                     }
                 }

                 Item {
                     id: scrollBtnWrapper

                     anchors.bottom: inputBoxRow.top
                     anchors.bottomMargin: Tokens.spacing.large
                     anchors.right: parent.right
                     anchors.rightMargin: Tokens.padding.large
                     width: 36
                     height: 36
                     z: 20
                     opacity: (!listView.atYEnd && chatStore.messages.count > 0) ? 1.0 : 0.0
                     visible: opacity > 0

                     Behavior on opacity { NumberAnimation { duration: 200; easing.type: Easing.InOutQuad } }

                     StyledRect {
                         id: scrollBtnBg

                         anchors.fill: parent
                         radius: 18
                         color: Colours.tPalette.m3surfaceContainerHigh
                     }

                     MultiEffect {
                         anchors.fill: scrollBtnBg
                         source: scrollBtnBg
                         shadowEnabled: true
                         shadowOpacity: 0.3
                         shadowBlur: 0.5
                         shadowVerticalOffset: 2
                     }

                     MaterialIcon {
                         anchors.centerIn: parent
                         text: "arrow_downward"
                         font: Tokens.font.icon.small
                         color: Colours.palette.m3onSurface
                     }

                     MouseArea {
                         anchors.fill: parent
                         cursorShape: Qt.PointingHandCursor
                         onClicked: listView.positionViewAtEnd()
                     }
                 }

                 ColumnLayout {
                     id: suggestionBox

                     anchors.bottom: attachmentStrip.visible ? attachmentStrip.top : inputBoxRow.top
                     anchors.bottomMargin: Tokens.spacing.small
                     anchors.left: parent.left
                     anchors.right: parent.right
                     z: 11
                     spacing: Tokens.spacing.small
                     visible: root.isClaudeCode && root.promptSuggestions.length > 0

                     RowLayout {
                         Layout.fillWidth: true
                         spacing: Tokens.spacing.small

                         StyledText {
                             Layout.fillWidth: true
                             text: qsTr("Suggestions")
                             color: Colours.palette.m3onSurfaceVariant
                             font: Tokens.font.label.small
                         }

                         Item {
                             Layout.preferredWidth: 24
                             Layout.preferredHeight: 24

                             MaterialIcon {
                                 anchors.centerIn: parent
                                 text: "close"
                                 color: closeSugMouse.containsMouse ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                             }

                             MouseArea {
                                 id: closeSugMouse

                                 anchors.fill: parent
                                 hoverEnabled: true
                                 cursorShape: Qt.PointingHandCursor
                                 onClicked: root.promptSuggestions = []
                             }
                         }
                     }

                     Repeater {
                         model: root.promptSuggestions

                         StyledRect {
                             required property string modelData
                             Layout.fillWidth: true

                             implicitHeight: sugChipText.implicitHeight + Tokens.padding.medium * 2
                             radius: Tokens.rounding.large
                             color: Colours.tPalette.m3surfaceContainerHigh

                             StateLayer {
                                 radius: Tokens.rounding.large
                                 onClicked: {
                                     inputArea.text = modelData;
                                     root.promptSuggestions = [];
                                     inputArea.forceActiveFocus();
                                 }
                             }

                             StyledText {
                                 id: sugChipText

                                 anchors.left: parent.left
                                 anchors.right: parent.right
                                 anchors.top: parent.top
                                 anchors.leftMargin: Tokens.padding.medium
                                 anchors.rightMargin: Tokens.padding.medium
                                 anchors.topMargin: Tokens.padding.medium
                                 text: parent.modelData
                                 color: Colours.palette.m3onSurface
                                 font: Tokens.font.body.small
                                 wrapMode: Text.Wrap
                             }
                         }
                     }
                 }

                 // Files attached to the next message (Claude Code).
                 Flow {
                     id: attachmentStrip

                     anchors.bottom: inputBoxRow.top
                     anchors.bottomMargin: Tokens.spacing.small
                     anchors.left: parent.left
                     anchors.right: parent.right
                     z: 10
                     spacing: Tokens.spacing.small
                     visible: root.isClaudeCode && attachments.pending.length > 0

                     Repeater {
                         model: attachments.pending

                         StyledRect {
                             id: attachChip

                             required property var modelData

                             implicitWidth: Math.min(attachChipRow.implicitWidth + Tokens.padding.medium * 2, attachmentStrip.width)
                             implicitHeight: attachChipRow.implicitHeight + Tokens.padding.small * 2
                             radius: Tokens.rounding.full
                             color: Colours.tPalette.m3secondaryContainer

                             RowLayout {
                                 id: attachChipRow

                                 anchors.fill: parent
                                 anchors.leftMargin: Tokens.padding.medium
                                 anchors.rightMargin: Tokens.padding.small
                                 spacing: Tokens.spacing.small

                                 MaterialIcon {
                                     text: attachChip.modelData.isImage ? "image" : "attach_file"
                                     color: Colours.palette.m3onSecondaryContainer
                                     font: Tokens.font.icon.small
                                 }

                                 StyledText {
                                     Layout.fillWidth: true
                                     Layout.maximumWidth: 180
                                     text: attachChip.modelData.path.replace(/^.*\//, "")
                                     color: Colours.palette.m3onSecondaryContainer
                                     font: Tokens.font.label.medium
                                     elide: Text.ElideMiddle
                                 }

                                 Item {
                                     Layout.preferredWidth: 20
                                     Layout.preferredHeight: 20

                                     MaterialIcon {
                                         anchors.centerIn: parent
                                         text: "close"
                                         color: Colours.palette.m3onSecondaryContainer
                                         font: Tokens.font.icon.small
                                     }

                                     MouseArea {
                                         anchors.fill: parent
                                         cursorShape: Qt.PointingHandCursor
                                         onClicked: attachments.remove(attachChip.modelData.path)
                                     }
                                 }
                             }
                         }
                     }
                 }

                 FileDialog {
                     id: attachDialog

                     title: qsTr("Attach a file")
                     onAccepted: path => attachments.add(path)
                 }

                 StyledRect {
                     id: inputBoxRow

                     anchors.bottom: parent.bottom
                     anchors.left: parent.left
                     anchors.right: parent.right
                     z: 10
                     implicitHeight: Math.max(48, inputArea.implicitHeight + Tokens.padding.medium * 2)
                     color: Colours.tPalette.m3surfaceContainer
                     radius: 24

                     StyledClippingRect {
                         z: -1
                         anchors.fill: parent
                         radius: 24

                         ShaderEffectSource {
                             id: inputBlurSource

                             sourceItem: contentStack
                             sourceRect: {
                                 var p = parent.mapToItem(contentStack, 0, 0);
                                 return Qt.rect(p.x, p.y, parent.width, parent.height);
                             }
                         }
                         MultiEffect {
                             anchors.fill: parent
                             source: inputBlurSource
                             blurEnabled: true
                             blurMax: 32
                         }
                     }

                     StateLayer {
                         id: inputStateLayer

                         anchors.fill: parent
                         radius: 24
                         hoverEnabled: false
                         cursorShape: Qt.IBeamCursor
                         onClicked: inputArea.forceActiveFocus()
                     }

                     DropArea {
                         anchors.fill: parent
                         enabled: root.isClaudeCode
                         onDropped: drop => {
                             if (!drop.hasUrls)
                                 return;
                             for (var i = 0; i < drop.urls.length; i++)
                                 attachments.add(drop.urls[i].toString());
                             drop.acceptProposedAction();
                         }
                     }

                     RowLayout {
                         anchors.fill: parent
                         anchors.leftMargin: Tokens.padding.large
                         anchors.rightMargin: Tokens.padding.small
                         spacing: Tokens.spacing.small

                         ScrollView {
                             id: inputScroll
                             Layout.fillWidth: true
                             Layout.fillHeight: true
                             
                             TextArea {
                                 id: inputArea

                                 verticalAlignment: TextInput.AlignVCenter
                                 placeholderText: qsTr("Ask assistant...")
                                 color: Colours.palette.m3onSurface
                                 placeholderTextColor: Colours.palette.m3outline
                                 font: Tokens.font.body.small
                                 wrapMode: Text.Wrap
                                 selectByMouse: true
                                 background: null

                                 MouseArea {
                                     anchors.fill: parent
                                     hoverEnabled: true
                                     cursorShape: Qt.IBeamCursor
                                     propagateComposedEvents: true
                                     onPressed: mouse => {
                                          var mapped = mapToItem(inputStateLayer, mouse.x, mouse.y);
                                          inputStateLayer.press(mapped.x, mapped.y);
                                          mouse.accepted = false;
                                      }
                                 }

                                 Keys.onPressed: event => {
                                     if (event.key === Qt.Key_Return && !(event.modifiers & Qt.ShiftModifier)) {
                                         event.accepted = true;
                                         if (!root.isTyping) {
                                             root.sendPrompt(inputArea.text);
                                             inputArea.clear();
                                         }
                                     } else if (root.isClaudeCode && event.matches(StandardKey.Paste)) {
                                         // An image on the clipboard becomes an attachment;
                                         // anything else is pasted as text as usual.
                                         event.accepted = true;
                                         attachments.pasteImage();
                                     }
                                 }
                             }
                         }

                         // Attach a file (Claude Code).
                         Item {
                             visible: root.isClaudeCode
                             Layout.preferredWidth: visible ? 32 : 0
                             Layout.preferredHeight: 32

                             MaterialIcon {
                                 anchors.centerIn: parent
                                 text: "attach_file"
                                 color: attachMouse.containsMouse ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                             }

                             MouseArea {
                                 id: attachMouse

                                 anchors.fill: parent
                                 hoverEnabled: true
                                 cursorShape: Qt.PointingHandCursor
                                 onClicked: attachDialog.open()
                             }
                         }

                         Item {
                             visible: root.isClaudeCode
                             Layout.preferredWidth: visible ? 32 : 0
                             Layout.preferredHeight: 32

                             MaterialIcon {
                                 anchors.centerIn: parent
                                 text: root.loadingSuggestions ? "hourglass_empty" : "lightbulb"
                                 color: sugMouse.containsMouse ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                             }

                             MouseArea {
                                 id: sugMouse

                                 anchors.fill: parent
                                 hoverEnabled: true
                                 cursorShape: Qt.PointingHandCursor
                                 onClicked: root.fetchPromptSuggestions()
                             }
                         }

                         Item {
                             Layout.preferredWidth: 36
                             Layout.preferredHeight: 36

                             MaterialShape {
                                 anchors.fill: parent
                                 color: root.isTyping ? Colours.palette.m3error : (root.canSend ? Colours.palette.m3primary : Colours.layer(Colours.tPalette.m3surfaceContainerHigh, 2))
                                 shape: root.isTyping ? MaterialShape.Cookie4Sided : (root.canSend ? MaterialShape.Arrow : MaterialShape.Circle)
                                 scale: (!root.canSend && !root.isTyping) ? 1 : sendMouse.pressed ? 0.6 : sendMouse.containsMouse ? 0.8 : 0.7
                                 rotation: 0
                                 
                                 Behavior on scale { Anim { type: Anim.FastSpatial } }
                                 Behavior on color { CAnim {} }

                                 MouseArea {
                                     id: sendMouse

                                     anchors.fill: parent
                                     hoverEnabled: true
                                     cursorShape: (root.canSend || root.isTyping) ? Qt.PointingHandCursor : Qt.ArrowCursor
                                     onClicked: {
                                         if (root.isTyping) {
                                             root.stopReply();
                                             if (root.activeReply)
                                                 chatStore.update(root.activeReply.chatId, root.activeReply.msgId, { "isFinished": true });
                                         } else if (root.canSend) {
                                             root.sendPrompt(inputArea.text);
                                             inputArea.clear();
                                         }
                                     }
                                 }
                             }

                             MaterialIcon {
                                 anchors.centerIn: parent
                                 text: "arrow_upward"
                                 color: Colours.palette.m3onSurfaceVariant
                                 font: Tokens.font.icon.small
                                 opacity: (root.canSend || root.isTyping) ? 0 : 1

                                 Behavior on opacity { Anim { type: Anim.DefaultEffects } }
                             }
                         }
                     }
                 }
             }

             HistoryPane {
                 id: historyPane

                 anchors.fill: parent
                 store: chatStore
                 active: root.isHistoryTab
                 busy: root.isTyping

                 onChatOpened: chatId => root.loadChat(chatId)
                 onNewChatRequested: root.createNewChat()
                 onRemoving: chatIds => {
                     root.cancelRateLimitRetry();
                     root.stopClaudeCode(chatIds);
                 }
                 onSwitchChat: chatId => {
                     if (chatId)
                         root.loadChat(chatId);
                     else
                         root.createNewChat();
                     root.isHistoryTab = true;
                 }
             }
         }
    }

    Component {
        id: claudeCodeSessionComponent

        ClaudeCodeSession {
            id: session

            onTextUpdated: text => root.onClaudeCodeText(session, text)
            onThoughtUpdated: text => root.onClaudeCodeThought(session, text)
            onToolsUpdated: tools => root.onClaudeCodeTools(session, tools)
            onToolStarted: name => root.onClaudeCodeToolStarted(session, name)
            onToolsDone: root.onClaudeCodeToolsDone(session)
            onProgressed: text => root.onClaudeCodeProgress(session, text)
            onOutputTokensUpdated: count => root.onClaudeCodeTokens(session, count)
            onUsageUpdated: usage => root.onClaudeCodeUsage(session, usage)
            onCompleted: reply => root.onClaudeCodeCompleted(session, reply)
            onEnded: root.onClaudeCodeEnded(session)
        }
    }
}

