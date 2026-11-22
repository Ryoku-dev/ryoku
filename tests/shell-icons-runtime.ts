import { resolve } from 'node:path';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';

// Integration check: needs Quickshell, ryoku-shell icons and an installed icon theme.
const root = resolve(import.meta.dir, '..');
const checkout = resolve(root, 'ryoku/shell/quickshell');

// A desktop entry whose id differs from its icon, laid where the shell's
// DesktopEntries scan reads it, so the check can assert the entry step of
// appIcon without leaning on anything installed on the machine.
const dataHome = mkdtempSync(resolve(tmpdir(), 'ryoku-icon-probe-'));
mkdirSync(resolve(dataHome, 'applications'), { recursive: true });
writeFileSync(resolve(dataHome, 'applications', 'ryoku-icon-probe.desktop'), [
    '[Desktop Entry]',
    'Name=Ryoku Icon Probe',
    'Exec=probe',
    `Icon=${resolve(root, 'tests/fixtures/shell-icons/assets/probe-icon.svg')}`,
    'Type=Application',
    '',
].join('\n'));

try {
    for (let attempt = 1; attempt <= 3; attempt++) {
        const proc = Bun.spawn(['qs', '-p', resolve(import.meta.dir, 'fixtures/shell-icons'), '--no-color'], {
            env: {
                ...process.env,
                QT_QPA_PLATFORM: process.env.QT_QPA_PLATFORM || 'offscreen',
                RYOKU_SHELL_DIR: '',
                XDG_DATA_HOME: dataHome,
                // The checkout is the only QML root: a Ryoku box exports QML2_IMPORT_PATH
                // with its deployed ~/.config/quickshell first, which binds the
                // installed shell and leaves the code under review unexercised.
                QML_IMPORT_PATH: checkout,
                QML2_IMPORT_PATH: checkout,
            },
            stdout: 'pipe', stderr: 'pipe',
        });
        const timer = setTimeout(() => proc.kill(), 20000);
        const [stdout, stderr, code] = await Promise.all([
            new Response(proc.stdout).text(), new Response(proc.stderr).text(), proc.exited,
        ]);
        clearTimeout(timer);
        const log = stdout + stderr;
        if (code !== 0 || /ICONS_SMOKE_(FAIL|TIMEOUT)/.test(log)
            || !log.includes('ICONS_SMOKE_PASS') || /decodeErrors=[1-9]/.test(log)) {
            console.error(log);
            throw new Error(`Icon runtime check failed on attempt ${attempt}`);
        }
        console.log(`Attempt ${attempt}: ${log.split('\n').find(line => line.includes('ICONS_SMOKE_PASS'))}`);
    }
} finally {
    rmSync(dataHome, { recursive: true, force: true });
}