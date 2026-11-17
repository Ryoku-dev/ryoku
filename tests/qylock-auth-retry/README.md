# qylock authentication retry tests

`run.py` exercises the production retry components with Qt Quick Test. It uses
real Qt key, pointer, focus, and timer events. The Quickshell and PAM objects are
schema-compatible isolated stubs: no production shell, PAM service, D-Bus,
socket, camera, or biometric provider is loaded.

Run with either a Qt installation that provides `qmltestrunner`:

    python3 tests/qylock-auth-retry/run.py -v1

or a Python environment containing PySide6. The runner derives all repository
paths, creates an isolated HOME, XDG config and runtime directory, disables
session and system bus access, and selects offscreen software rendering.
Both runner backends clean up the temporary HOME after completion.

Runner cleanup regressions need only the Python standard library:

    python3 -m unittest discover -s tests/qylock-auth-retry -p test_runner.py -v

The suite covers default-off empty submission and active descendant controls,
the production host's dedicated retry gate, explicit opt-in restart,
malformed and unreadable flag outcomes, typed and hidden password priority,
focus transfer, pending and submitted password guards, real Return and keypad
Enter events, background clicks, control exclusion, settled motion, first-entry
suppression, cooldown, proof and foreground guards, recognition overlap, and
security-key ownership including activation during a pending retry.
