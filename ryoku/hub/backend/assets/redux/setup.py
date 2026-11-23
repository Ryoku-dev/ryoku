#!/usr/bin/env python3
"""Transactional user installation for the Hub's Redux adapter option.

Progress goes to stdout for the Hub's model card: a `stage: <step>` line when
the step changes (the Hub words it) and an `NN%` line when the overall percent
moves. Everything the build tools print goes to the install log instead, so the
Hub never has to hold a compiler's output.
"""
import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import urllib.request

NATIVE_REV = '5c9b0fbbd8e611d88e47d7e1ebae9bca9b447e8f'
MODEL_REV = '68f808a5903a0bf12657bca3a2a669d573abfb58'
MODEL_SHA = '574614b9a4d9f72ab202877a7ad6a1f4bf819dd1d27b9d42dfe8cd429fcebdd5'
MODEL_URL = f'https://huggingface.co/mudler/parakeet-cpp-gguf/resolve/{MODEL_REV}/redux-packed.gguf'
# SHA-256 of the native and ggml license files at the pinned revisions above.
RUNTIME_LICENSES_SHA = 'e3e44ee64d99bb4c436e3843cedf9e71268649dfc7bac418c598130fac6bdd0d'
OWNER = 'ryoku-redux-v1'

# Where each step sits on the card's single progress bar, as (start, end).
STEPS = {'fetch': (0, 8), 'configure': (8, 12), 'build': (12, 72), 'download': (72, 97), 'install': (97, 100)}
REQUIRED_TOOLS = {'git': 'git', 'cmake': 'cmake', 'c++': 'a C++ compiler', 'make': 'make'}
MAKE_PROGRESS = re.compile(r'^\[\s*(\d+)%\]')
NINJA_PROGRESS = re.compile(r'^\[(\d+)/(\d+)\]')


class Progress:
    """Prints the step and the overall percent, each only when it changes."""

    def __init__(self, out=sys.stdout):
        self.out, self.step, self.percent = out, None, -1

    def begin(self, step):
        self.step = step
        print('stage: ' + step, file=self.out, flush=True)
        self.advance(0)

    def advance(self, fraction):
        start, end = STEPS[self.step]
        percent = int(start + (end - start) * max(0.0, min(1.0, fraction)))
        if percent > self.percent:
            self.percent = percent
            print(f'{percent}%', file=self.out, flush=True)


def build_fraction(line):
    """The share of the build a make or ninja status line reports, or None."""
    if match := MAKE_PROGRESS.match(line):
        return int(match.group(1)) / 100
    if match := NINJA_PROGRESS.match(line):
        done, total = int(match.group(1)), int(match.group(2))
        return done / total if total else None
    return None


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def layout():
    home = Path.home()
    config = Path(os.environ.get('XDG_CONFIG_HOME', home / '.config'))
    data = Path(os.environ.get('XDG_DATA_HOME', home / '.local/share'))
    root = data / 'voxtype-redux'
    return {'model': data / 'voxtype/models/redux-packed.gguf',
            'client': home / '.local/bin/voxtype-redux',
            'worker': root / 'adapter.py', 'library': root / 'runtime/libparakeet.so',
            'license': root / 'NOTICE.md',
            'unit': config / 'systemd/user/voxtype-redux.service',
            'receipt': root / 'hub-install.json'}


def log_path():
    state = Path(os.environ.get('XDG_STATE_HOME', Path.home() / '.local/state'))
    return state / 'ryoku/redux-install.log'


def read_receipt(paths):
    doc = json.loads(paths['receipt'].read_text())
    if doc.get('owner') != OWNER or not isinstance(doc.get('files'), dict):
        raise RuntimeError('Unrecognized Redux installation receipt')
    allowed = {str(p) for key, p in paths.items() if key != 'receipt'}
    if not set(doc['files']).issubset(allowed):
        raise RuntimeError('Receipt contains files outside the Redux installation')
    return doc


def validate_remove(paths):
    if not paths['receipt'].exists():
        raise RuntimeError('Redux was not installed from Ryoku Settings, so there is nothing to remove')
    doc = read_receipt(paths)
    for name, expected in doc['files'].items():
        path = Path(name)
        if path.is_symlink() or (path.exists() and digest(path) != expected):
            raise RuntimeError(f'Redux file was changed; leaving installation intact: {path}')
    return doc


def remove(paths):
    doc = validate_remove(paths)
    for name in doc['files']:
        Path(name).unlink(missing_ok=True)
    paths['receipt'].unlink(missing_ok=True)
    for key in ['library', 'worker']:
        try:
            paths[key].parent.rmdir()
        except OSError:
            pass


def lower_priority():
    os.nice(10)


def run(log, *args, on_line=None):
    """Runs one build command at low priority with its output in the log."""
    print('$ ' + ' '.join(map(str, args)), file=log, flush=True)
    with subprocess.Popen(list(map(str, args)), stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                          text=True, errors='replace', preexec_fn=lower_priority) as proc:
        for line in proc.stdout:
            log.write(line)
            if on_line:
                on_line(line)
    if proc.returncode:
        log.flush()
        raise subprocess.CalledProcessError(proc.returncode, args)


def download(url, target, expected, report=lambda fraction: None):
    with urllib.request.urlopen(url, timeout=30) as response, target.open('wb') as stream:
        total = int(response.headers.get('Content-Length', 0))
        count = 0
        while block := response.read(1024 * 1024):
            stream.write(block)
            count += len(block)
            if total:
                report(count / total)
    if digest(target) != expected:
        raise RuntimeError('Redux model checksum mismatch; nothing was installed')


def missing_tools():
    missing = [label for tool, label in REQUIRED_TOOLS.items() if shutil.which(tool) is None]
    if not missing:
        return None
    message = f"Building Redux needs {', '.join(missing)}."
    if shutil.which('ryoku-host'):
        advice = subprocess.run(['ryoku-host', 'pkg', 'advice', 'base-devel', 'cmake', 'git'],
                                capture_output=True, text=True).stdout.strip()
        if advice:
            message += f' Install them with: {advice}'
    return message


def build_runtime(stage, log, progress):
    source, build = stage / 'native', stage / 'build'
    progress.begin('fetch')
    run(log, 'git', 'init', '-q', source)
    progress.advance(0.2)
    run(log, 'git', '-C', source, 'fetch', '--depth', '1', 'https://github.com/mudler/parakeet.cpp.git', NATIVE_REV)
    progress.advance(0.6)
    run(log, 'git', '-C', source, 'checkout', '--detach', 'FETCH_HEAD')
    run(log, 'git', '-C', source, 'submodule', 'update', '--init', '--depth', '1', 'third_party/ggml')
    progress.begin('configure')
    run(log, 'cmake', '-S', source, '-B', build, '-DCMAKE_BUILD_TYPE=Release',
        '-DPARAKEET_SHARED=ON', '-DBUILD_SHARED_LIBS=OFF', '-DCMAKE_POSITION_INDEPENDENT_CODE=ON',
        '-DPARAKEET_BUILD_CLI=OFF', '-DPARAKEET_BUILD_SERVER=OFF',
        '-DPARAKEET_WITH_CED=OFF', '-DPARAKEET_WITH_VOICEDETECT=OFF')
    progress.begin('build')
    # Half the cores at most, at low priority, so the desktop stays responsive.
    jobs = max(1, min(4, (os.cpu_count() or 2) // 2))

    def on_line(line):
        fraction = build_fraction(line)
        if fraction is not None:
            progress.advance(fraction)
    run(log, 'cmake', '--build', build, '--target', 'parakeet', '-j', jobs, on_line=on_line)
    library = build / 'libparakeet.so'
    native = ctypes.CDLL(str(library))
    native.parakeet_capi_abi_version.restype = ctypes.c_int
    if native.parakeet_capi_abi_version() < 10 or not hasattr(native, 'parakeet_capi_set_concurrency'):
        raise RuntimeError('Native runtime does not provide the required adapter ABI')
    return source, library


def worker_unit(paths):
    # systemd expands percent and dollar even within quoted arguments.
    def q(x):
        return json.dumps(str(x).replace('%', '%%').replace('$', '$$'), ensure_ascii=False)
    return f'''[Unit]
Description=Resident Redux adapter for stock Voxtype
[Service]
Type=notify
NotifyAccess=main
ExecStart={q(sys.executable)} {q(paths['worker'])} serve --backend cpp --model {q(paths['model'])} --library {q(paths['library'])} --pool-threads 4 --chunk-seconds 30
RuntimeDirectory=voxtype-redux
RuntimeDirectoryMode=0700
UMask=0077
Restart=on-failure
RestartSec=2
TimeoutStartSec=180
TimeoutStopSec=15
[Install]
WantedBy=default.target
'''


def publish(paths, contents):
    installed, prepared = [], []
    receipt_tmp = paths['receipt'].with_suffix('.tmp')
    try:
        hashes = {}
        for key, value in contents.items():
            dest = paths[key]
            dest.parent.mkdir(parents=True, exist_ok=True)
            tmp = dest.with_name(dest.name + '.installing')
            # Exclusive creation never overwrites user files or follows symlinks.
            with tmp.open('xb') as stream:
                prepared.append(tmp)
                if isinstance(value, Path):
                    with value.open('rb') as src:
                        shutil.copyfileobj(src, stream)
                else:
                    stream.write(value)
            tmp.chmod(0o755 if key in ['client', 'worker', 'library'] else 0o600)
            hashes[str(dest)] = digest(tmp)
        receipt_tmp.write_text(json.dumps({'owner': OWNER, 'files': hashes}, indent=2) + '\n')
        receipt_tmp.chmod(0o600)
        # Publish only complete files. Blocking termination here protects the
        # few renames; all slow work above remains cancelable.
        signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM, signal.SIGINT})
        try:
            for key, tmp in zip(contents, prepared):
                dest = paths[key]
                os.link(tmp, dest)  # atomic, exclusive; never replace user files
                installed.append(dest)
            os.link(receipt_tmp, paths['receipt'])
        except BaseException:
            for path in reversed(installed):
                path.unlink(missing_ok=True)
            raise
        finally:
            signal.pthread_sigmask(signal.SIG_UNBLOCK, {signal.SIGTERM, signal.SIGINT})
    finally:
        for tmp in prepared:
            tmp.unlink(missing_ok=True)
        receipt_tmp.unlink(missing_ok=True)


def install(paths, stage, progress=None):
    progress = progress or Progress()
    if message := missing_tools():
        raise RuntimeError(message)
    if sys.version_info < (3, 11):
        raise RuntimeError('Redux needs Python 3.11 or newer')
    if any(p.exists() or p.is_symlink() for p in paths.values()):
        raise RuntimeError('Redux files already exist; remove them before installing it from Ryoku Settings')
    log_file = log_path()
    log_file.parent.mkdir(parents=True, exist_ok=True)
    with log_file.open('w') as log:
        try:
            source, library = build_runtime(stage, log, progress)
        except subprocess.CalledProcessError as error:
            raise RuntimeError(f'The speech runtime did not build ({Path(str(error.cmd[0])).name} '
                               f'exited {error.returncode}). The full log is in {log_file}') from None
    progress.begin('download')
    model = stage / 'redux-packed.gguf'
    download(MODEL_URL, model, MODEL_SHA, progress.advance)
    progress.begin('install')
    here = Path(__file__)
    adapter = here.with_name('adapter.py').read_bytes()
    notice = b'\n'.join(path.read_bytes() for path in [
        here.with_name('UPSTREAM.md'), here.with_name('LICENSE'),
        source / 'LICENSE', source / 'third_party/ggml/LICENSE']) + b'\n'
    publish(paths, {'model': model, 'library': library, 'client': adapter, 'worker': adapter,
                    'unit': worker_unit(paths).encode(), 'license': notice})
    progress.advance(1)


def adopt(paths):
    if paths['receipt'].exists() or paths['receipt'].is_symlink():
        raise RuntimeError('Redux already has an ownership receipt')

    keys = ['model', 'client', 'worker', 'library', 'license', 'unit']
    for key in keys:
        path = paths[key]
        if path.is_symlink() or not path.is_file():
            raise RuntimeError(f'Cannot adopt Redux; expected a regular installed file: {path}')

    if digest(paths['model']) != MODEL_SHA:
        raise RuntimeError('Cannot adopt Redux; the model does not match the pinned download')
    adapter = Path(__file__).with_name('adapter.py').read_bytes()
    if paths['client'].read_bytes() != adapter or paths['worker'].read_bytes() != adapter:
        raise RuntimeError('Cannot adopt Redux; the installed adapter differs from this version')
    if paths['unit'].read_bytes() != worker_unit(paths).encode():
        raise RuntimeError('Cannot adopt Redux; the installed service unit differs from this version')
    expected_license = b'\n'.join([
        Path(__file__).with_name('UPSTREAM.md').read_bytes(),
        Path(__file__).with_name('LICENSE').read_bytes(),
    ])
    license_bundle = paths['license'].read_bytes()
    if not license_bundle.startswith(expected_license):
        raise RuntimeError('Cannot adopt Redux; the installed license bundle is not recognized')
    runtime_licenses = license_bundle[len(expected_license):]
    if hashlib.sha256(runtime_licenses).hexdigest() != RUNTIME_LICENSES_SHA:
        raise RuntimeError('Cannot adopt Redux; the runtime license bundle is incomplete or changed')
    try:
        native = ctypes.CDLL(str(paths['library']))
        native.parakeet_capi_abi_version.restype = ctypes.c_int
        if native.parakeet_capi_abi_version() < 10 or not hasattr(native, 'parakeet_capi_set_concurrency'):
            raise RuntimeError('Cannot adopt Redux; the native runtime has an unsupported ABI')
    except (OSError, AttributeError) as error:
        raise RuntimeError(f'Cannot adopt Redux; the native runtime is not usable: {error}') from None

    receipt = paths['receipt']
    receipt.parent.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(prefix='.redux-adopt-', dir=receipt.parent)
    temp = Path(temp_name)
    try:
        doc = {'owner': OWNER, 'adopted': True,
               'files': {str(paths[key]): digest(paths[key]) for key in keys}}
        with os.fdopen(fd, 'w') as stream:
            json.dump(doc, stream, indent=2)
            stream.write('\n')
        temp.chmod(0o600)
        os.link(temp, receipt)
    finally:
        temp.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument('action', choices=['install', 'adopt', 'check-remove', 'remove'])
    args = parser.parse_args()

    def cancel(signum, frame):
        raise KeyboardInterrupt('Installation canceled')
    signal.signal(signal.SIGTERM, cancel)
    paths = layout()
    if args.action == 'install':
        install(paths, Path(__file__).parent)
    elif args.action == 'adopt':
        adopt(paths)
    elif args.action == 'check-remove':
        validate_remove(paths)
    else:
        remove(paths)


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print(str(error) or 'Installation canceled', file=sys.stderr)
        sys.exit(1)
