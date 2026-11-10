#!/usr/bin/env python3
import os
import pty
import select
import shutil
import signal
import sys
import tempfile
import time
from pathlib import Path

REPO = Path(os.environ.get("RYOKU_PATH", Path(__file__).resolve().parents[1]))


def write_stub(path: Path, body: str = "exit 0\n") -> None:
    path.write_text("#!/usr/bin/env bash\n" + body)
    path.chmod(0o755)


def read_until(fd: int, needle: bytes, timeout: float = 8) -> bytes:
    data = bytearray()
    end = time.time() + timeout
    while time.time() < end:
        ready, _, _ = select.select([fd], [], [], 0.1)
        if ready:
            try:
                data.extend(os.read(fd, 4096))
            except OSError:
                break
            if needle in data:
                return bytes(data)
    raise AssertionError(f"prompt {needle!r} not seen; output={bytes(data)!r}")


def exercise(shell: str) -> None:
    with tempfile.TemporaryDirectory() as raw:
        home = Path(raw)
        cfg = home / ".config"
        target = cfg / shell
        target.mkdir(parents=True)
        shutil.copy(REPO / f"ryoku/apps/{shell}/ryoku.{ 'bash' if shell == 'bash' else 'zsh'}", target)
        shutil.copy(REPO / f"ryoku/apps/{shell}/rashin.{ 'bash' if shell == 'bash' else 'zsh'}", target)
        (cfg / "ryoku-terminal").mkdir(parents=True)
        shutil.copy(REPO / "ryoku/apps/terminal-shell/env.sh", cfg / "ryoku-terminal/env.sh")
        if shell == "bash":
            (target / "user.bash").write_text("PS1='PTY> '\n")
        else:
            (target / "user.zsh").write_text("PROMPT='PTY> '\n")
            (target / ".zshrc").write_text(f"source '{target / 'ryoku.zsh'}'\n")

        bindir = home / "bin"
        bindir.mkdir()
        for name in ("ryoku-fastfetch", "starship", "zoxide", "mise", "fd", "eza"):
            body = "printf ':\\n'\n" if name in ("starship", "zoxide", "mise", "fzf") else "exit 0\n"
            write_stub(bindir / name, body)
        write_stub(
            bindir / "rashin",
            "case \" $* \" in\n"
            "  *\" ask --buffer -- \"*) printf \"touch '%s/ran'\\n\" \"$HOME\" ;;\n"
            "esac\n",
        )

        env = os.environ.copy()
        env.update({
            "HOME": str(home),
            "XDG_CONFIG_HOME": str(cfg),
            "PATH": f"{bindir}:/usr/bin",
            "TERM": "xterm-256color",
        })
        argv = ["bash", "--noprofile", "--rcfile", str(target / "ryoku.bash"), "-i"] if shell == "bash" else ["zsh", "-di"]
        pid, fd = pty.fork()
        if shell == "zsh":
            env["ZDOTDIR"] = str(target)
        if pid == 0:
            os.execvpe(argv[0], argv, env)
        try:
            startup = read_until(fd, b"PTY> ")
            if b"unsupported readline function" in startup:
                raise AssertionError(f"{shell} startup emitted unsupported Readline warnings")
            os.write(fd, b"describe marker")
            os.write(fd, b"\x1br")
            time.sleep(0.5)
            if (home / "ran").exists():
                raise AssertionError(f"{shell} Alt+R executed without Enter")
            os.write(fd, b"\r")
            end = time.time() + 5
            while time.time() < end and not (home / "ran").exists():
                time.sleep(0.1)
            if not (home / "ran").exists():
                raise AssertionError(f"{shell} Alt+R did not replace the buffer")
            read_until(fd, b"PTY> ")
            os.write(fd, b"exit\r")
            os.waitpid(pid, 0)
        finally:
            try:
                os.kill(pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            os.close(fd)


for name in ("bash", "zsh"):
    exercise(name)
print("terminal shell PTY: bash zsh passed")
