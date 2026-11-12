.pragma library

var title = "Users";
var lead = "The people who use this computer, what each of them may do, and accounts set up from a profile.";

var sections = [
    { id: "people", title: "People on this computer" },
    { id: "this-profile", title: "This profile account" },
    { id: "profiles", title: "Account profiles" }
];

// a profile account sees only its own panel, so the other rows are hidden there (and in search);
// its panel's rows show only there
var inProfile = { bind: { store: "users", key: "inProfileAccount" } };
var users = { hostCap: "users", unless: inProfile };
var profileOnly = { hostCap: "users", unless: { bind: inProfile.bind, is: [false] } };

// the page draws people and profiles itself; these rows feed search and land on what it draws.
// A section's search result lands on its first row here, so each section starts with the row
// drawn nearest its top.
var rows = [
    { id: "role", section: "people", label: "Role", template: true, needs: users,
      detail: "Administrators can change the system and other accounts; standard accounts change only their own",
      keywords: ["administrator", "admin", "standard", "sudo", "wheel", "root", "privileges"] },
    { id: "picture", section: "people", label: "Your picture", template: true, needs: users,
      detail: "The picture beside your name at sign-in and on the lock screen",
      keywords: ["avatar", "profile picture", "face", "photo", "icon", "account picture"] },
    { id: "permissions", section: "people", label: "Permissions", template: true, needs: users,
      detail: "Devices, sound, virtual machines and containers each person may use",
      keywords: ["groups", "input", "video", "audio", "kvm", "libvirt", "docker", "virtualization", "containers", "access"] },
    { id: "storage", section: "people", label: "Home folder storage", template: true, needs: users,
      detail: "How much a person's home folder holds, and a limit on it",
      keywords: ["storage limit", "quota", "disk space", "home size", "btrfs", "qgroup", "subvolume"] },
    { id: "password", section: "people", label: "Password", template: true, needs: users,
      detail: "Set a new sign-in password for a person",
      keywords: ["change password", "reset password", "forgot password", "passwd", "sign in"] },
    { id: "remove-person", section: "people", label: "Remove a person", template: true, needs: users,
      detail: "Delete an account and archive or erase its home folder",
      keywords: ["delete user", "delete account", "userdel", "remove account", "erase"] },
    { id: "add-person", section: "people", label: "Add a person", template: true, needs: users,
      detail: "Give someone their own sign-in, home folder and desktop",
      keywords: ["add user", "new user", "new account", "create account", "useradd", "family", "guest"] },

    { id: "this-reset", section: "this-profile", label: "Reset settings", template: true, needs: profileOnly,
      detail: "Put back every file the profile manages; your projects and notes stay",
      keywords: ["reset", "restore defaults", "managed files", "factory", "profile"] },
    { id: "this-checkpoint", section: "this-profile", label: "Save a checkpoint", template: true, needs: profileOnly,
      detail: "Keep this account's settings as they are now, before a lab or a big change",
      keywords: ["snapshot", "checkpoint", "save point", "backup", "profile"] },

    { id: "profile-tools", section: "profiles", label: "Profile tools", template: true, needs: users,
      detail: "Choose the tools a profile account has, after reviewing what changes",
      keywords: ["packages", "components", "install tools", "edit tools", "plan"] },
    { id: "checkpoints", section: "profiles", label: "Checkpoints", template: true, needs: users,
      detail: "Save a profile account's settings and go back to them later",
      keywords: ["snapshot", "restore", "revert", "backup", "save point"] },
    { id: "profile-reset", section: "profiles", label: "Reset profile settings", template: true, needs: users,
      detail: "Put back every file the profile manages; your projects and notes stay",
      keywords: ["reset", "restore defaults", "managed files", "factory"] },
    { id: "remove-profile", section: "profiles", label: "Remove a profile account", template: true, needs: users,
      detail: "Take away the profile's tools and its account, keeping or erasing its home",
      keywords: ["delete profile", "uninstall profile", "remove tools"] },
    { id: "create-profile", section: "profiles", label: "Create a profile account", template: true, needs: users,
      detail: "A separate standard account set up from a profile installed from RyoStore, with its own tools",
      keywords: ["profile", "account profile", "pentest", "security", "blackarch", "kali", "lab", "separate account"] },
    { id: "browse-profiles", section: "profiles", label: "Find profiles in RyoStore",
      detail: "Install a profile here, then create its account on this page",
      keywords: ["store", "ryostore", "download", "more profiles"], needs: users,
      control: "handoff", command: ["ryostore", "open", "profiles"], actionText: "Open" }
];

// the words for each permission ryoku-host offers; others keep the backend's label
var permissions = [
    { id: "devices", label: "Devices and input",
      detail: "Game controllers, drawing tablets, cameras and key remapping" },
    { id: "audio", label: "Sound devices",
      detail: "Use sound cards directly, for music and recording software" },
    { id: "virtualization", label: "Virtual machines",
      detail: "Run and manage virtual machines" },
    { id: "containers", label: "Containers",
      detail: "Run containers. This is close to administrator access." }
];

var related = ["sign-in", "lock-screen", "backup-import"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}

function permission(id) {
    for (var i = 0; i < permissions.length; i++)
        if (permissions[i].id === id)
            return permissions[i];
    return null;
}
