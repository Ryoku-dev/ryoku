pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

QtObject {
    id: root

    readonly property var tools: [
        {
            id: "password",
            name: I18n.tr("Password generator"),
            type: "generator",
            description: I18n.tr("Create a secure password with the character sets you choose."),
            sampleInput: "",
            defaultOptions: { length: 20, uppercase: true, lowercase: true, numbers: true, symbols: true, avoidAmbiguous: true },
            options: [
                {
                    id: "length",
                    type: "choice",
                    label: I18n.tr("Length"),
                    default: 20,
                    choices: [
                        { value: 12, label: "12" },
                        { value: 16, label: "16" },
                        { value: 20, label: "20" },
                        { value: 32, label: "32" },
                        { value: 64, label: "64" }
                    ]
                },
                { id: "uppercase", type: "toggle", label: I18n.tr("A-Z"), default: true },
                { id: "lowercase", type: "toggle", label: I18n.tr("a-z"), default: true },
                { id: "numbers", type: "toggle", label: I18n.tr("0-9"), default: true },
                { id: "symbols", type: "toggle", label: I18n.tr("!@#$"), default: true },
                { id: "avoidAmbiguous", type: "toggle", label: I18n.tr("Avoid l, 1, O, and 0"), default: true }
            ]
        },
        {
            id: "uuid",
            name: I18n.tr("UUID generator"),
            type: "generator",
            description: I18n.tr("Create a random UUID v4 or time-ordered UUID v7."),
            sampleInput: "",
            defaultOptions: { version: "4", uppercase: false, hyphens: true, quantity: 1 },
            options: [
                {
                    id: "version",
                    type: "choice",
                    label: I18n.tr("Version"),
                    default: "4",
                    choices: [
                        { value: "4", label: I18n.tr("v4 · random") },
                        { value: "7", label: I18n.tr("v7 · time-ordered") }
                    ]
                },
                { id: "uppercase", type: "toggle", label: I18n.tr("Uppercase"), default: false },
                { id: "hyphens", type: "toggle", label: I18n.tr("Hyphens"), default: true }
            ]
        },
        {
            id: "json_formatter",
            name: I18n.tr("JSON formatter & validator"),
            type: "transformer",
            description: I18n.tr("Format, minify, sort, and validate JSON data."),
            sampleInput: "{\"name\":\"Example\",\"items\":[\"alpha\",\"beta\"],\"count\":2,\"active\":true}",
            defaultOptions: { indent: "2", sortKeys: false },
            options: [
                {
                    id: "indent",
                    type: "choice",
                    label: I18n.tr("Format"),
                    default: "2",
                    choices: [
                        { value: "2", label: I18n.tr("2 spaces") },
                        { value: "4", label: I18n.tr("4 spaces") },
                        { value: "tab", label: I18n.tr("Tab") },
                        { value: "minified", label: I18n.tr("Minify") }
                    ]
                },
                { id: "sortKeys", type: "toggle", label: I18n.tr("Sort keys"), default: false }
            ]
        },
        {
            id: "base64",
            name: I18n.tr("Base64 encode/decode"),
            type: "transformer",
            description: I18n.tr("Encode UTF-8 text or decode Base64 data."),
            sampleInput: "Hello, world!",
            defaultOptions: { mode: "encode", urlSafe: false },
            options: [
                {
                    id: "mode",
                    type: "choice",
                    label: I18n.tr("Mode"),
                    default: "encode",
                    choices: [
                        { value: "encode", label: I18n.tr("Encode") },
                        { value: "decode", label: I18n.tr("Decode") }
                    ]
                },
                { id: "urlSafe", type: "toggle", label: I18n.tr("URL safe"), default: false }
            ]
        },
        {
            id: "url_encode",
            name: I18n.tr("URL encode/decode"),
            type: "transformer",
            description: I18n.tr("Encode or decode URL paths and query values."),
            sampleInput: "https://example.com/search?query=hello world & cat=1",
            defaultOptions: { mode: "encode", component: true },
            options: [
                {
                    id: "mode",
                    type: "choice",
                    label: I18n.tr("Mode"),
                    default: "encode",
                    choices: [
                        { value: "encode", label: I18n.tr("Encode") },
                        { value: "decode", label: I18n.tr("Decode") }
                    ]
                },
                { id: "component", type: "toggle", label: I18n.tr("Encode one component"), default: true }
            ]
        },
        {
            id: "jwt_decoder",
            name: I18n.tr("JWT decoder"),
            type: "analyzer",
            description: I18n.tr("Read a token's header and payload without verifying its signature."),
            sampleInput: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
            defaultOptions: {},
            options: []
        },
        {
            id: "unix_timestamp",
            name: I18n.tr("Unix timestamp"),
            type: "transformer",
            description: I18n.tr("Convert epoch timestamps and dates into common time formats."),
            sampleInput: "now",
            defaultOptions: {},
            options: []
        },
        {
            id: "color_converter",
            name: I18n.tr("Color converter"),
            type: "transformer",
            description: I18n.tr("Convert colours between hex, RGB, and HSL."),
            sampleInput: "#4A90E2",
            defaultOptions: {},
            options: []
        },
        {
            id: "regex_tester",
            name: I18n.tr("Regex tester"),
            type: "transformer",
            description: I18n.tr("Test a regular expression and inspect every match group."),
            sampleInput: "Write to info@example.com or support@example.org.",
            defaultOptions: { pattern: "([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})", flags: "g" },
            options: [
                { id: "pattern", type: "text", label: I18n.tr("Pattern"), default: "([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})" },
                { id: "flags", type: "text", label: I18n.tr("Flags"), default: "g" }
            ]
        },
        {
            id: "text_diff",
            name: I18n.tr("Text diff"),
            type: "transformer",
            description: I18n.tr("Compare original and modified text line by line."),
            sampleInput: "const greeting = 'Hello';\nconsole.log(greeting);\n\n===DIFF_SPLIT===\nconst greeting = 'Hello, World!';\nconsole.log(greeting);\nconsole.log('Done!');",
            defaultOptions: {},
            options: []
        },
        {
            id: "chmod_calculator",
            name: I18n.tr("chmod calculator"),
            type: "analyzer",
            description: I18n.tr("Translate Unix permissions between octal and symbolic forms."),
            sampleInput: "4755",
            defaultOptions: {},
            options: []
        }
    ]

    function byId(id: string): var {
        return root.tools.find(tool => tool.id === id) ?? null;
    }

}
