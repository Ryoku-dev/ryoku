.pragma library

var title = "Sign-in & security";
var lead = "How your saved passwords open, your fingerprints and security keys, and where each one can stand in for your password.";

var sections = [
    { id: "keyring", title: "Keyring" },
    { id: "fingerprint", title: "Fingerprint" },
    { id: "security-keys", title: "Security keys" },
    { id: "where-used", title: "Where each is used" }
];

function b(key) {
    return { store: "signin", key: key };
}

function when(key) {
    return { bind: b(key), is: [true] };
}

// template rows feed search only; the page draws them itself
var rows = [
    { id: "keyring-mode", section: "keyring", label: "Saved passwords open",
      detail: "Your keyring holds the passwords apps save. At sign-in opens it with your login password; Never ask leaves it without a password.",
      keywords: ["keyring", "gnome keyring", "secrets", "secret service", "seahorse", "password prompt", "unlock", "pam"],
      control: "picker", options: [
          { label: "At sign-in", value: "unlock-on-login" },
          { label: "Never ask", value: "never-ask" },
          { label: "Ask each time", value: "ask" }
      ], bind: b("keyring.mode"), changeLabel: "Keyring" },
    { id: "keyring-password", section: "keyring", label: "Remove the keyring's password", template: true,
      detail: "Type the keyring's password to make it blank, or start fresh with a backup",
      keywords: ["convert", "reset", "start fresh", "blank keyring", "backup"] },

    { id: "fingerprints", section: "fingerprint", label: "Fingerprints", template: true,
      detail: "Add, name, test and remove your fingerprints",
      keywords: ["fprintd", "enroll", "scan", "sensor", "reader", "finger", "biometric", "verify", "test"] },
    { id: "remove-fingerprints", section: "fingerprint", label: "Remove all fingerprints", template: true,
      detail: "Forget every fingerprint on this computer",
      keywords: ["delete", "clear", "erase", "fingerprints"] },

    { id: "security-key-list", section: "security-keys", label: "Your security keys", template: true,
      detail: "Add a key or remove one you no longer use",
      keywords: ["yubikey", "fido", "fido2", "u2f", "passkey", "pam_u2f", "token", "hardware key", "enroll"] },
    { id: "key-mode", section: "security-keys", label: "A key signs you in",
      detail: "On its own instead of your password, or together with it",
      keywords: ["two factor", "2fa", "mfa", "second factor", "either"],
      control: "segmented", options: [
          { label: "Instead of password", value: "either" },
          { label: "With password", value: "mfa" }
      ], bind: b("key.mode"), when: when("key.supported"), changeLabel: "Security key rule" },
    { id: "key-touch", section: "security-keys", label: "Touch the key",
      detail: "Ask you to touch the key each time it is used",
      keywords: ["presence", "user presence", "tap"],
      control: "toggle", bind: b("key.touch"), when: when("key.supported"), changeLabel: "Touch the key" },
    { id: "key-pin", section: "security-keys", label: "Ask for the key's PIN",
      detail: "When the key has a PIN; the lock screen asks only with designs that can",
      keywords: ["pin", "pin verification"],
      control: "toggle", bind: b("key.pin"), when: when("key.supported"), changeLabel: "Key PIN" },
    { id: "key-uv", section: "security-keys", label: "The key's own check",
      detail: "Use the key's built-in fingerprint or other check when it has one",
      keywords: ["user verification", "uv", "biometric key"],
      control: "toggle", bind: b("key.uv"), when: when("key.supported"), more: true, changeLabel: "Key's own check" },

    { id: "fp-login", section: "where-used", label: "Fingerprint at sign-in",
      detail: "Touch the sensor on the sign-in screen instead of typing your password",
      keywords: ["greeter", "sddm", "login screen", "pam"],
      control: "toggle", bind: b("fingerprint.sddm"), when: when("fingerprint.pam") },
    { id: "key-login", section: "where-used", label: "Security key at sign-in",
      detail: "Use your key on the sign-in screen",
      keywords: ["greeter", "sddm", "login screen", "yubikey"],
      control: "toggle", bind: b("key.login"), when: when("key.enrolled") },
    { id: "fp-lock", section: "where-used", label: "Fingerprint on the lock screen",
      detail: "The lock screen listens for a touch beside your password",
      keywords: ["unlock", "lockscreen", "qylock"],
      control: "toggle", bind: b("fingerprint.lock"), "default": true },
    { id: "key-lock", section: "where-used", label: "Security key on the lock screen",
      detail: "Unlock with your key beside your password, which always keeps working",
      keywords: ["unlock", "lockscreen", "yubikey"],
      control: "toggle", bind: b("key.lock"), when: when("key.lockReady") },
    { id: "fp-sudo", section: "where-used", label: "Fingerprint for sudo",
      detail: "Touch instead of typing your password for admin commands in a terminal",
      keywords: ["sudo", "terminal", "admin", "root", "pam"],
      control: "toggle", bind: b("fingerprint.sudo"), when: when("fingerprint.pam") },
    { id: "key-sudo", section: "where-used", label: "Security key for sudo",
      detail: "Use your key for admin commands in a terminal",
      keywords: ["sudo", "terminal", "admin", "root", "yubikey"],
      control: "toggle", bind: b("key.sudo"), when: when("key.enrolled") },
    { id: "fp-admin", section: "where-used", label: "Fingerprint for admin prompts",
      detail: "Touch instead of typing your password when an app asks for admin rights",
      keywords: ["polkit", "admin", "authentication", "pkexec"],
      control: "toggle", bind: b("fingerprint.polkit"), when: when("fingerprint.pam") },
    { id: "key-admin", section: "where-used", label: "Security key for admin prompts",
      detail: "Use your key when an app asks for admin rights",
      keywords: ["polkit", "admin", "authentication", "pkexec", "yubikey"],
      control: "toggle", bind: b("key.polkit"), when: when("key.enrolled") }
];

var related = ["lock-screen", "users", "power"];
