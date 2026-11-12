.pragma library
.import "window-style.js" as WindowStyle
.import "bar.js" as Bar
.import "displays.js" as Displays
.import "shortcuts.js" as Shortcuts
.import "time-language.js" as TimeLanguage
.import "backup-import.js" as BackupImport
.import "about.js" as About
.import "clipboard.js" as Clipboard
.import "quick-settings.js" as QuickSettings
.import "desktop-dock.js" as DesktopDock
.import "updates.js" as Updates
.import "fonts-icons.js" as FontsIcons
.import "animations.js" as Animations
.import "keyboard.js" as Keyboard
.import "mouse-touchpad.js" as MouseTouchpad
.import "phone.js" as Phone
.import "window-behaviour.js" as WindowBehaviour
.import "window-manager.js" as WindowManager
.import "wifi.js" as Wifi
.import "bluetooth.js" as Bluetooth
.import "add-ons.js" as AddOns
.import "lock-screen.js" as LockScreen
.import "sign-in.js" as SignIn
.import "screen-capture.js" as ScreenCapture
.import "voice-typing.js" as VoiceTyping
.import "rashin.js" as Rashin
.import "notifications.js" as Notifications
.import "sounds.js" as Sounds
.import "launcher.js" as Launcher
.import "app-rules.js" as AppRules
.import "users.js" as Users
.import "default-apps.js" as DefaultApps
.import "startup-apps.js" as StartupApps
.import "terminal.js" as Terminal
.import "power.js" as Power
.import "graphics.js" as Graphics
.import "performance.js" as Performance
.import "game-bar.js" as GameBar

// each migrated page's schema by its registry `file`, so search reaches its rows
var byFile = {
    "DesktopDock": DesktopDock,
    "Power": Power,
    "Graphics": Graphics,
    "Performance": Performance,
    "GameBar": GameBar,
    "DefaultApps": DefaultApps,
    "StartupApps": StartupApps,
    "Terminal": Terminal,
    "AppRules": AppRules,
    "Users": Users,
    "QuickSettings": QuickSettings,
    "Notifications": Notifications,
    "Sounds": Sounds,
    "Launcher": Launcher,
    "LockScreen": LockScreen,
    "SignIn": SignIn,
    "ScreenCapture": ScreenCapture,
    "VoiceTyping": VoiceTyping,
    "Rashin": Rashin,
    "AddOns": AddOns,
    "Wifi": Wifi,
    "Bluetooth": Bluetooth,
    "WindowBehaviour": WindowBehaviour,
    "WindowManager": WindowManager,
    "Keyboard": Keyboard,
    "Phone": Phone,
    "MouseTouchpad": MouseTouchpad,
    "Animations": Animations,
    "FontsIcons": FontsIcons,
    "Clipboard": Clipboard,
    "WindowStyle": WindowStyle,
    "TimeLanguage": TimeLanguage,
    "BackupImport": BackupImport,
    "About": About,
    "Bar": Bar,
    "Displays": Displays,
    "Updates": Updates,
    "Shortcuts": Shortcuts
};

function forFile(file) {
    return file && byFile[file] ? byFile[file] : null;
}
