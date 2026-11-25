#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""appfix.py: Phase 15's launcher entries and the files Hands may find. No
real app is ever started by them and nothing in them ever runs.

  dev.ryoku.OniDesk.desktop   "Kansai Notes", a text editor: Exec runs
                              appwin.py target --app-id dev.ryoku.OniDesk
                              --title "Kansai notes" --state OUT/desk.json,
                              the window Hands opens and types into on its
                              separate desktop
  dev.ryoku.OniFiles.desktop  "Lab Files", MimeType inode/directory,
                              Categories FileManager: Exec runs appwin.py
                              files --log OUT/files.jsonl %f, which logs the
                              path it was given (show_file's folder)

and HOME/Documents/HandsFiles/, one file per kind show_file must only ever
show in its folder: an exec-bit file with no #! line, a #! script, a .sh, a
.desktop launcher, an .AppImage, a .run and one ordinary .txt. Every name
holds the word "lantern" (the lab's find_files query). Each runnable one
would create OUT/ran-<name> if anything ran it, so "nothing ran" is a file
that never appears.

appfix.py install --data-home DIR --home DIR --out DIR [--here DIR]
    Write both entries to DATA_HOME/applications/ and the files under
    HOME/Documents/HandsFiles/. Every directory must be under /dev/shm and
    OUT must exist. phase15.sh runs it with the lab's own directories, as
    lab_app gives them to the daemon:
        python3 appfix.py install --data-home "$ONI_LAB/data" \\
            --home "$ONI_LAB/home" --out "$ONI_LAB/out"
appfix.py manifest --home DIR --out DIR
    Print the fixture files as JSON: name, path, kind, and the ran-marker
    each would create.
appfix.py selftest
    Install into a scratch directory under /dev/shm and check the entries
    parse as desktop entries with the Exec argv the daemon must use, the file
    manager is the only inode/directory FileManager entry, and every fixture
    file has its kind (mode, #! line, suffix) and names its marker. Exit 0
    when every check passes.
"""
import json
import os
import shutil
import stat
import sys

sys.dont_write_bytecode = True
HERE = os.path.dirname(os.path.abspath(__file__))

DESK = "dev.ryoku.OniDesk"
FILES = "dev.ryoku.OniFiles"
DESK_NAME = "Kansai Notes"
DESK_TITLE = "Kansai notes"
FILES_NAME = "Lab Files"
QUERY = "lantern"
FOLDER = ("Documents", "HandsFiles")
SHM = "/dev/shm/"
# name: (kind, runnable)
KINDS = {
    "lantern-tool": ("execbit", True),
    "lantern-run": ("shebang", True),
    "lantern-setup.sh": ("sh", True),
    "lantern-app.desktop": ("desktop", True),
    "lantern-viewer.AppImage": ("appimage", True),
    "lantern-install.run": ("run", True),
    "lantern-notes.txt": ("plain", False),
}


def quote(arg):
    """One Exec argument (Desktop Entry spec, "The Exec key")."""
    if arg and not any(c in arg for c in " \t\n\"'\\><~|&;$*?#()`="):
        return arg
    return '"' + "".join("\\" + c if c in '"`$\\' else c for c in arg) + '"'


def escape(value):
    return value.replace("\\", "\\\\")


def desk_argv(here, out):
    return ["python3", os.path.join(here, "appwin.py"), "target", "--app-id", DESK, "--title", DESK_TITLE,
            "--state", os.path.join(out, "desk.json")]


def files_argv(here, out):
    """The file manager's Exec argv without its field code."""
    return ["python3", os.path.join(here, "appwin.py"), "files", "--log", os.path.join(out, "files.jsonl")]


def entries(here, out):
    desk = " ".join(quote(a) for a in desk_argv(here, out))
    files = " ".join(quote(a) for a in files_argv(here, out)) + " %f"
    return {
        DESK: "\n".join([
            "[Desktop Entry]", "Type=Application", "Name=" + DESK_NAME, "GenericName=Text Editor",
            "Comment=The lab's window for Hands on its separate desktop", "Keywords=notes;kansai;text;",
            "Exec=" + escape(desk), "TryExec=python3", "Icon=accessories-text-editor", "Terminal=false",
            "Categories=Utility;TextEditor;", "StartupWMClass=" + DESK, ""]),
        FILES: "\n".join([
            "[Desktop Entry]", "Type=Application", "Name=" + FILES_NAME, "GenericName=File Manager",
            "Comment=The lab's file manager: it logs the folder it is asked to show", "Exec=" + escape(files),
            "TryExec=python3", "Icon=system-file-manager", "Terminal=false", "Categories=System;FileManager;",
            "MimeType=inode/directory;", "StartupWMClass=" + FILES, ""]),
    }


def marker(out, name):
    return os.path.join(out, "ran-" + name)


def body(name, out):
    kind = KINDS[name][0]
    touch = "touch " + quote(marker(out, name))
    if kind == "execbit":
        return touch + "\n"  # no #! line: a shell would still run it
    if kind in ("shebang", "sh", "run", "appimage"):
        return "#!/bin/sh\n" + touch + "\n"
    if kind == "desktop":
        return "\n".join(["[Desktop Entry]", "Type=Application", "Name=Lantern App",
                          "Exec=" + escape("touch " + quote(marker(out, name))), "Terminal=false", ""])
    return "Lantern notes: an ordinary file. Nothing in it runs.\n"


def folder(home):
    return os.path.join(home, *FOLDER)


def manifest(home, out):
    return [{"name": n, "path": os.path.join(folder(home), n), "kind": k, "runnable": r,
             "marker": marker(out, n) if r else None} for n, (k, r) in KINDS.items()]


def under_shm(*paths):
    for p in paths:
        if not os.path.isabs(p) or not (os.path.realpath(p) + "/").startswith(SHM):
            raise SystemExit("appfix.py: %s is not an absolute path under %s" % (p, SHM))


def write(path, text, mode=0o644):
    with open(path + ".part", "w", encoding="utf-8") as fh:
        fh.write(text)
    os.chmod(path + ".part", mode)
    os.replace(path + ".part", path)


def install(data_home, home, out, here):
    under_shm(data_home, home, out)
    if not os.path.isdir(out):
        raise SystemExit("appfix.py: OUT %s does not exist" % out)
    if not os.path.isfile(os.path.join(here, "appwin.py")):
        raise SystemExit("appfix.py: no appwin.py in %s" % here)
    apps = os.path.join(data_home, "applications")
    os.makedirs(apps, exist_ok=True)
    printed = []
    for app_id, text in entries(here, out).items():
        p = os.path.join(apps, app_id + ".desktop")
        write(p, text)
        printed.append(p)
    d = folder(home)
    os.makedirs(d, mode=0o700, exist_ok=True)
    for f in manifest(home, out):
        write(f["path"], body(f["name"], out), 0o755 if f["runnable"] else 0o644)
        printed.append(f["path"])
    for p in printed:
        print(p)
    return 0


def parse_entry(text):
    fields = {}
    for line in text.splitlines():
        if "=" in line and not line.startswith("["):
            k, v = line.split("=", 1)
            fields[k] = v
    return fields


def split_exec(line):
    """The Exec line's argv (spec quoting), field codes kept as tokens."""
    out, cur, i, quoted, have = [], "", 0, False, False
    line = line.replace("\\\\", "\\")
    while i < len(line):
        c = line[i]
        if quoted:
            if c == "\\" and i + 1 < len(line):
                cur += line[i + 1]
                i += 2
                continue
            if c == '"':
                quoted = False
            else:
                cur += c
        elif c == '"':
            quoted, have = True, True
        elif c == " ":
            if cur or have:
                out.append(cur)
            cur, have = "", False
        else:
            cur += c
        i += 1
    if cur or have:
        out.append(cur)
    return out


def selftest():
    checks = []

    def check(name, ok, value=None):
        checks.append({"name": name, "pass": bool(ok), "value": value})

    root = os.path.join(SHM, "appfix-selftest-%d" % os.getpid())
    data, home, out = (os.path.join(root, d) for d in ("data", "home", "out"))
    os.makedirs(out)
    try:
        import contextlib
        import io
        with contextlib.redirect_stdout(io.StringIO()):
            install(data, home, out, HERE)
        apps = os.path.join(data, "applications")
        desk = parse_entry(open(os.path.join(apps, DESK + ".desktop")).read())
        files = parse_entry(open(os.path.join(apps, FILES + ".desktop")).read())
        check("the notes entry's Exec is appwin.py target, exactly", split_exec(desk["Exec"]) == desk_argv(HERE, out),
              split_exec(desk["Exec"]))
        check("the file manager's Exec is appwin.py files with one %f",
              split_exec(files["Exec"]) == files_argv(HERE, out) + ["%f"], split_exec(files["Exec"]))
        check("the file manager is an inode/directory FileManager entry; the notes entry is neither",
              "inode/directory" in files.get("MimeType", "") and "FileManager" in files.get("Categories", "")
              and "MimeType" not in desk and "FileManager" not in desk.get("Categories", ""))
        check("names the launcher shows", desk["Name"] == DESK_NAME and files["Name"] == FILES_NAME)
        kinds = {}
        for f in manifest(home, out):
            st = os.stat(f["path"])
            head = open(f["path"]).read()
            kinds[f["name"]] = {"exec": bool(st.st_mode & stat.S_IXUSR), "shebang": head.startswith("#!")}
        check("every runnable file has its exec bit and the ordinary one has none",
              all(kinds[n]["exec"] == r for n, (_, r) in KINDS.items()), kinds)
        check("the exec-bit file has no #! line, the scripts have one",
              not kinds["lantern-tool"]["shebang"] and kinds["lantern-run"]["shebang"]
              and kinds["lantern-setup.sh"]["shebang"], kinds)
        check("each runnable file names its own ran- marker in OUT",
              all(marker(out, f["name"]) in open(f["path"]).read() for f in manifest(home, out) if f["runnable"]))
        check("every file name holds the query word", all(QUERY in n for n in KINDS))
        check("the folder is HOME/Documents/HandsFiles, mode 0700",
              stat.S_IMODE(os.stat(folder(home)).st_mode) == 0o700)
        check("no marker exists after installing", not any(n.startswith("ran-") for n in os.listdir(out)))
        refused = 0
        for args in (("/home/x/data", home, out), (data, "relative", out)):
            try:
                install(*args, HERE)
            except SystemExit:
                refused += 1
        check("install refuses a directory outside /dev/shm or a relative one", refused == 2, refused)
        check("split_exec reads quoted arguments", split_exec('python3 "/a b/c.py" --x "y\\"z" %f')
              == ["python3", "/a b/c.py", "--x", 'y"z', "%f"])
    finally:
        shutil.rmtree(root, ignore_errors=True)
    ok = all(c["pass"] for c in checks)
    print(json.dumps({"pass": ok, "checks": checks}, indent=2))
    return 0 if ok else 1


def opts_of(args, names):
    o = {}
    i = 0
    while i < len(args):
        if args[i] in names and i + 1 < len(args):
            o[args[i][2:]] = args[i + 1]
            i += 2
        else:
            raise SystemExit(__doc__)
    return o


def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    cmd, args = argv[0], argv[1:]
    if cmd == "selftest" and not args:
        return selftest()
    if cmd == "install":
        o = opts_of(args, ("--data-home", "--home", "--out", "--here"))
        if not all(k in o for k in ("data-home", "home", "out")):
            raise SystemExit(__doc__)
        return install(o["data-home"], o["home"], o["out"], o.get("here", HERE))
    if cmd == "manifest":
        o = opts_of(args, ("--home", "--out"))
        if not all(k in o for k in ("home", "out")):
            raise SystemExit(__doc__)
        print(json.dumps(manifest(o["home"], o["out"]), indent=2))
        return 0
    raise SystemExit(__doc__)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
