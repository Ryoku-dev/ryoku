pragma Singleton

import QtQuick
import Quickshell
import caelestia.utils

Singleton {
    id: root

    // The tool-call text protocol. These tags travel through both the prompt
    // and the parsers, so they live in exactly one place. The concatenation
    // is deliberate: an over-eager formatter once stripped the literal tags
    // out of the assistant, and empty tags here would make every scan spin
    // forever on each streamed chunk.
    readonly property string toolCallStart: "<" + "tool_call" + ">"
    readonly property string toolCallEnd: "</" + "tool_call" + ">"

    // Bookkeeping for one tool batch: how many dispatched tools are still
    // outstanding and what has come back so far. The assistant resets this
    // before dispatching and receives the batch via onAllToolsFinished.
    property int runningToolsCount: 0
    property string accumulatedToolResults: ""
    property string accumulatedToolImage: ""

    // Assigned by the assistant: function(results, imageB64OrNull).
    // Fired once every dispatched tool has reported back.
    property var onAllToolsFinished: null

    function reset() {
        runningToolsCount = 0;
        accumulatedToolResults = "";
        accumulatedToolImage = "";
    }

    function recordResult(chunk) {
        accumulatedToolResults += chunk;
    }

    function recordImage(b64) {
        accumulatedToolImage = b64;
    }

    function finishOneTool() {
        runningToolsCount--;
        checkToolsFinished();
    }

    function checkToolsFinished() {
        if (runningToolsCount === 0 && onAllToolsFinished) {
            var b64 = accumulatedToolImage ? accumulatedToolImage : null;
            onAllToolsFinished(accumulatedToolResults.trim(), b64);
        }
    }

    function parseTextToolCalls(text) {
        var calls = [];
        if (!root.toolCallStart || !root.toolCallEnd)
            return calls;
        var pos = 0;
        while (pos <= text.length) {
            var start = text.indexOf(root.toolCallStart, pos);
            if (start === -1) break;
            var bodyStart = start + root.toolCallStart.length;
            var end = text.indexOf(root.toolCallEnd, bodyStart);
            if (end === -1) break;
            var jsonStr = text.substring(bodyStart, end).trim();
            jsonStr = jsonStr.replace(/^```[a-zA-Z]*\n?/, "");
            jsonStr = jsonStr.replace(/```$/, "");
            jsonStr = jsonStr.trim();

            try {
                var parsed = JSON.parse(jsonStr);
                if (parsed && parsed.name) calls.push(parsed);
            } catch(e) { Logger.log("[AI] Bad tool_call JSON: " + jsonStr); }
            pos = end + root.toolCallEnd.length;
        }
        return calls;
    }

    function stripToolCalls(text) {
        if (!root.toolCallStart || !root.toolCallEnd || !text)
            return text || "";
        var result = text;
        while (true) {
            var s = result.indexOf(root.toolCallStart);
            if (s === -1) break;
            var e = result.indexOf(root.toolCallEnd, s + root.toolCallStart.length);
            if (e === -1) { result = result.substring(0, s); break; }
            result = result.substring(0, s) + result.substring(e + root.toolCallEnd.length);
        }
        return result.replace(/\s+$/, '');
    }

    function toolsPrompt() {
        return `\n\nYou have access to the following tools. To call a tool, output a ${root.toolCallStart} block containing ONLY valid JSON. Do not output any text inside the block other than the JSON object.\n\nFORMAT:\n${root.toolCallStart}\n{"name": "TOOL_NAME", "args": {ARGUMENTS}}\n${root.toolCallEnd}\n\nAVAILABLE TOOLS:\n- take_screenshot: Captures the user's screen for visual analysis. Args: none.\n  Example: ${root.toolCallStart}\n{"name": "take_screenshot", "args": {}}\n${root.toolCallEnd}\n\n- web_search: Searches the web. Args: query (string, required), page (number, optional).\n  Example: ${root.toolCallStart}\n{"name": "web_search", "args": {"query": "latest news"}}\n${root.toolCallEnd}\n\n- read_webpage: Fetches and reads the text of a URL. Args: url (string, required).\n  Example: ${root.toolCallStart}\n{"name": "read_webpage", "args": {"url": "https://example.com"}}\n${root.toolCallEnd}\n\n- open_app: Launches an installed desktop application. Args: app_name (string, required).\n  Example: ${root.toolCallStart}\n{"name": "open_app", "args": {"app_name": "dolphin"}}\n${root.toolCallEnd}\n\n- set_timer: Sets a countdown timer that fires a desktop notification. Args: seconds (number, required), message (string, required).\n  Example: ${root.toolCallStart}\n{"name": "set_timer", "args": {"seconds": 300, "message": "Break time!"}}\n${root.toolCallEnd}\n\n- get_weather: Gets the current local weather from the system dashboard. Args: none.\n  Example: ${root.toolCallStart}\n{"name": "get_weather", "args": {}}\n${root.toolCallEnd}\n\n- caelestia_command: Runs a caelestia CLI command. Valid subcommands: shell, toggle, scheme, search, screenshot, record, clipboard, emoji, wallpaper, resizer, install, update. Args: subcommand (string, required), args (string, optional extra flags).\n  Example: ${root.toolCallStart}\n{"name": "caelestia_command", "args": {"subcommand": "wallpaper", "args": "--random"}}\n${root.toolCallEnd}\n\nCRITICAL RULES:\n1. ALWAYS use a ${root.toolCallStart} block to call a tool. NEVER pretend to perform actions in plain text.\n2. You may output a brief acknowledgment before the ${root.toolCallStart} block (e.g. 'Opening Dolphin for you!') but you MUST include the block.\n3. You can include multiple ${root.toolCallStart} blocks in one response.\n4. After receiving tool results, respond naturally to the user based on what the tool returned.`;
    }
}
