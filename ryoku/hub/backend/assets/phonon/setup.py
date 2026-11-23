#!/usr/bin/env python3
"""Transactional installation of the Hub's dedicated Phonon-2 environment."""
import argparse
from contextlib import ExitStack, contextmanager
import ctypes
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys
import tempfile

OWNER = 'ryoku-phonon-v1'
FERMION_VERSION = '0.2.13'
TORCH_VERSION = '2.14.1+cpu'
MODEL_SHA = '4b6bfa3a12cc3c4e0a54f2ab3ec4ca7a842b09e5c7ecfc8e7ca0ac6cc8c11468'
MODEL_BYTES = 177438361
MODEL_RELATIVE = Path('.cache/fermion/speech/FermionResearch__Phonon-2/model_phonon2_c4c_int6')


def layout():
    home = Path.home()
    data = Path(os.environ.get('XDG_DATA_HOME', home / '.local/share')).absolute()
    config = Path(os.environ.get('XDG_CONFIG_HOME', home / '.config')).absolute()
    manager = os.environ.get('RYOKU_PHONON_SERVICE_MANAGER', 'systemd')
    if manager not in ('systemd', 'runit'):
        raise RuntimeError(f'Unsupported Phonon service manager: {manager}')
    root = data / 'phonon-2'
    paths = {'manager': manager, 'runtime': root / 'venv', 'model': home / MODEL_RELATIVE.parent,
             'unit': config / 'systemd/user/phonon-2.service' if manager == 'systemd' else None,
             'service': Path(os.environ.get('RYOKU_USER_SERVICE_DIR', home / '.config/service')).absolute() / 'phonon-2',
             'receipt': root / 'hub-install.json'}
    return paths


def managed_paths(paths):
    return [paths[name] for name in ('runtime', 'model', 'unit') if paths[name] is not None]


def require_runit_service(paths):
    if paths['manager'] != 'runit':
        return
    service = paths['service']
    no_symlink_ancestors(service)
    run = service / 'run'
    if not service.is_dir() or run.is_symlink() or not run.is_file() or not os.access(run, os.X_OK):
        raise RuntimeError(f'Phonon-2 runit service is not provisioned: {service}')


def no_symlink_ancestors(path):
    for candidate in reversed((path, *path.parents)):
        if candidate.is_symlink():
            raise RuntimeError(f'Refusing symlink in Phonon installation path: {candidate}')


@contextmanager
def installation_lock():
    state = Path(os.environ.get('XDG_STATE_HOME', Path.home() / '.local/state'))
    path = state / 'ryoku/locks/phonon-2.lock'
    no_symlink_ancestors(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid():
            raise RuntimeError('Phonon installation lock belongs to another owner')
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise RuntimeError('Another Phonon installation operation is already running') from None
        yield
    finally:
        os.close(fd)


def digest(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as stream:
        value = hashlib.file_digest(stream, 'sha256').hexdigest()
    return value


def manifest(root):
    """Inventory links themselves, including bin/python, without following them."""
    no_symlink_ancestors(root)
    result = {}

    def walk(fd, relative):
        for name in sorted(os.listdir(fd)):
            info = os.stat(name, dir_fd=fd, follow_symlinks=False)
            if info.st_uid != os.getuid():
                raise RuntimeError(f'Phonon contains a file owned by another user: {root / relative / name}')
            key = str(relative / name)
            entry = {'mode': stat.S_IMODE(info.st_mode)}
            if stat.S_ISLNK(info.st_mode):
                entry.update(kind='link', target=os.readlink(name, dir_fd=fd))
            elif stat.S_ISDIR(info.st_mode):
                entry['kind'] = 'directory'
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                try:
                    walk(child, relative / name)
                finally:
                    os.close(child)
            elif stat.S_ISREG(info.st_mode):
                item = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
                with os.fdopen(item, 'rb') as stream:
                    entry.update(kind='file', sha256=hashlib.file_digest(stream, 'sha256').hexdigest())
            else:
                raise RuntimeError(f'Unsupported special file in Phonon environment: {root / key}')
            result[key] = entry

    fd = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        info = os.fstat(fd)
        if info.st_uid != os.getuid():
            raise RuntimeError(f'Phonon directory belongs to another user: {root}')
        result['.'] = {'kind': 'directory', 'mode': stat.S_IMODE(info.st_mode)}
        walk(fd, Path())
    finally:
        os.close(fd)
    return result


def unit_text(paths, bytecode=True, quoted=True):
    command = str(paths['runtime'] / 'bin/phonon')
    if quoted:
        command = json.dumps(command.replace('%', '%%').replace('$', '$$'), ensure_ascii=False)
    template = Path(__file__).with_name('phonon-2.service').read_text()
    template = template.replace('%h/.local/share/phonon-2/venv/bin/phonon', command)
    if not bytecode:
        template = template.replace('Environment=PYTHONDONTWRITEBYTECODE=1\n', '')
    return template


def run(*args, env=None, capture=False):
    result = subprocess.run(list(map(str, args)), check=True, env=env,
                            stdout=subprocess.PIPE if capture else sys.stderr,
                            stderr=subprocess.PIPE if capture else sys.stderr, text=capture)
    return result.stdout if capture else None


def validate_runtime(root):
    env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
    script = ('import importlib.metadata as m,json,sys; '
              'print(json.dumps({"fermion":m.version("fermion-research"),'
              '"torch":m.version("torch"),"prefix":sys.prefix,'
              '"cuda":[d.metadata["Name"] for d in m.distributions() '
              'if d.metadata["Name"].lower().startswith(("nvidia-","cuda-","triton"))]}))')
    doc = json.loads(run(root / 'bin/python', '-I', '-B', '-c', script, env=env, capture=True))
    if doc.get('fermion') != FERMION_VERSION or doc.get('torch') != TORCH_VERSION or doc.get('cuda'):
        raise RuntimeError('Phonon needs the pinned Fermion runtime and CPU-only PyTorch, without CUDA packages')
    if Path(doc.get('prefix', '')) != root or not (root / 'bin/phonon').is_file():
        raise RuntimeError('Phonon virtual environment is not complete or relocatable')
    run(root / 'bin/phonon', '--help', env=env, capture=True)


def validate_model(root):
    no_symlink_ancestors(root)
    name = MODEL_RELATIVE.name
    if any(child.name not in (name, name + '.lock') for child in root.iterdir()):
        raise RuntimeError('Phonon model repository contains unrelated files or profiles')
    lock = root / (name + '.lock')
    if lock.is_symlink() or (lock.exists() and (not lock.is_file() or lock.stat().st_size != 0)):
        raise RuntimeError('Phonon model repository contains an unrecognized lock')
    root = root / name
    no_symlink_ancestors(root)
    for name in ('model.fermion', 'config.json', 'packed_manifest.json'):
        file = root / name
        if file.is_symlink() or not file.is_file():
            raise RuntimeError(f'Incomplete or linked Phonon model: {file}')
    packed = json.loads((root / 'packed_manifest.json').read_text())
    config = json.loads((root / 'config.json').read_text())
    if (packed.get('container_sha256') != MODEL_SHA or
            (root / 'model.fermion').stat().st_size != MODEL_BYTES or
            digest(root / 'model.fermion') != MODEL_SHA or
            config.get('sample_rate') != 16000):
        raise RuntimeError('Phonon model integrity check failed')


def receipt_document(paths, adopted=False):
    roots = {name: str(paths[name]) for name in ('runtime', 'model')}
    if paths['unit'] is not None:
        roots['unit'] = str(paths['unit'])
    doc = {'owner': OWNER, 'version': 1 if paths['manager'] == 'systemd' else 2,
           'adopted': adopted, 'roots': roots,
           'trees': {name: manifest(paths[name]) for name in ('runtime', 'model')}}
    if paths['manager'] == 'runit':
        doc['manager'] = 'runit'
    else:
        doc['unit'] = {'sha256': digest(paths['unit']),
                       'mode': stat.S_IMODE(paths['unit'].stat().st_mode)}
    return doc


def read_receipt(paths):
    no_symlink_ancestors(paths['receipt'])
    if not paths['receipt'].is_file():
        raise RuntimeError('Phonon has no managed installation receipt; adopt it first')
    doc = json.loads(paths['receipt'].read_text())
    roots = {name: str(paths[name]) for name in ('runtime', 'model')}
    if paths['unit'] is not None:
        roots['unit'] = str(paths['unit'])
    version = 1 if paths['manager'] == 'systemd' else 2
    if (doc.get('owner') != OWNER or doc.get('version') != version or doc.get('roots') != roots or
            (paths['manager'] == 'runit' and doc.get('manager') != 'runit')):
        raise RuntimeError('Unrecognized Phonon receipt or paths outside the managed installation')
    if set(doc.get('trees', {})) != {'runtime', 'model'}:
        raise RuntimeError('Incomplete Phonon ownership receipt')
    if paths['unit'] is not None and not isinstance(doc.get('unit'), dict):
        raise RuntimeError('Incomplete Phonon ownership receipt')
    if paths['unit'] is None and 'unit' in doc:
        raise RuntimeError('Runit receipt must not claim ownership of a systemd unit')
    for tree in doc['trees'].values():
        if not isinstance(tree, dict) or '.' not in tree:
            raise RuntimeError('Incomplete Phonon ownership manifest')
        for name in tree:
            if Path(name).is_absolute() or '..' in Path(name).parts:
                raise RuntimeError('Phonon receipt contains paths outside its managed directory')
    return doc


def validate_remove(paths):
    doc = read_receipt(paths)
    for name in ('runtime', 'model'):
        no_symlink_ancestors(paths[name])
        if paths[name].exists():
            current = manifest(paths[name])
            expected = doc['trees'][name]
            if any(key not in expected or value != expected[key] for key, value in current.items()):
                raise RuntimeError(f'Phonon files were changed; leaving installation intact: {paths[name]}')
    if paths['unit'] is not None:
        no_symlink_ancestors(paths['unit'])
    if paths['unit'] is not None and paths['unit'].exists():
        current = {'sha256': digest(paths['unit']), 'mode': stat.S_IMODE(paths['unit'].stat().st_mode)}
        if current != doc['unit']:
            raise RuntimeError(f'Phonon unit was changed; leaving installation intact: {paths["unit"]}')
    return doc


def publish_file(source, target):
    no_symlink_ancestors(target)
    os.link(source, target, follow_symlinks=False)


def publish_directory(source, target):
    """Linux no-replace rename also refuses pre-existing empty directories."""
    no_symlink_ancestors(target)
    libc = ctypes.CDLL(None, use_errno=True)
    rename = libc.renameat2
    rename.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
    rename.restype = ctypes.c_int
    if rename(-100, os.fsencode(source), -100, os.fsencode(target), 1) != 0:
        code = ctypes.get_errno()
        raise OSError(code, os.strerror(code), str(target))


def write_receipt(paths, adopted=False, published=None):
    doc = receipt_document(paths, adopted)
    fd, name = tempfile.mkstemp(prefix='.phonon-receipt-', dir=paths['receipt'].parent)
    tmp = Path(name)
    try:
        with os.fdopen(fd, 'w') as stream:
            json.dump(doc, stream, indent=2)
            stream.write('\n')
        previous = signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGTERM, signal.SIGINT})
        try:
            publish_file(tmp, paths['receipt'])
            if published is not None:
                published.append(('file', paths['receipt']))
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, previous)
    finally:
        tmp.unlink(missing_ok=True)
    return doc


def remove(paths):
    validate_remove(paths)
    if not shutil.rmtree.avoids_symlink_attacks:
        raise RuntimeError('This Python cannot safely remove an environment containing interpreter symlinks')
    for name in ('runtime', 'model'):
        if paths[name].exists():
            shutil.rmtree(paths[name])
    if paths['unit'] is not None:
        paths['unit'].unlink(missing_ok=True)
    paths['receipt'].unlink()
    parents = [paths['receipt'].parent, paths['model'].parent]
    if paths['unit'] is not None:
        parents.append(paths['unit'].parent)
    for parent in parents:
        try:
            parent.rmdir()
        except OSError:
            pass


def adopt(paths):
    if paths['receipt'].exists() or paths['receipt'].is_symlink():
        return validate_remove(paths)
    for path in managed_paths(paths):
        no_symlink_ancestors(path)
    require_runit_service(paths)
    if paths['unit'] is None:
        validate_runtime(paths['runtime'])
        validate_model(paths['model'])
        return write_receipt(paths, adopted=True)
    if not paths['unit'].is_file():
        raise RuntimeError('Existing Phonon installation has no recognizable service')
    original = paths['unit'].read_bytes()
    normalized = lambda text: [line.strip() for line in text.splitlines() if line.strip()]
    known = [unit_text(paths, bytecode, quoted) for bytecode in (False, True) for quoted in (False, True)]
    if normalized(original.decode()) not in [normalized(text) for text in known]:
        raise RuntimeError('Existing Phonon service was changed or is not recognizable')
    validate_runtime(paths['runtime'])
    validate_model(paths['model'])
    # Upgrade only the recognized session-created unit, before its final hash
    # is captured, so later imports cannot mutate the owned runtime with pyc.
    replacement = unit_text(paths).encode()
    published = []
    try:
        if original != replacement:
            fd, name = tempfile.mkstemp(prefix='.phonon-adopt-', dir=paths['unit'].parent)
            tmp = Path(name)
            try:
                with os.fdopen(fd, 'wb') as stream:
                    stream.write(replacement)
                tmp.chmod(stat.S_IMODE(paths['unit'].stat().st_mode))
                if paths['unit'].is_symlink() or paths['unit'].read_bytes() != original:
                    raise RuntimeError('Phonon service changed during adoption')
                os.replace(tmp, paths['unit'])
            finally:
                tmp.unlink(missing_ok=True)
        return write_receipt(paths, adopted=True, published=published)
    except BaseException:
        if published:
            paths['receipt'].unlink(missing_ok=True)
        if (original != replacement and not paths['unit'].is_symlink() and
                paths['unit'].is_file() and paths['unit'].read_bytes() == replacement):
            no_symlink_ancestors(paths['unit'])
            fd, name = tempfile.mkstemp(prefix='.phonon-restore-', dir=paths['unit'].parent)
            tmp = Path(name)
            try:
                with os.fdopen(fd, 'wb') as stream:
                    stream.write(original)
                tmp.chmod(stat.S_IMODE(paths['unit'].stat().st_mode))
                os.replace(tmp, paths['unit'])
            finally:
                tmp.unlink(missing_ok=True)
        raise


def install(paths, stage=None):
    if shutil.which('uv') is None:
        raise RuntimeError('Phonon installation needs uv; install uv first')
    require_runit_service(paths)
    for path in (*managed_paths(paths), paths['receipt']):
        no_symlink_ancestors(path)
        if path.exists():
            raise RuntimeError(f'Phonon installation files already exist; adopt or remove them first: {path}')
    root = paths['runtime'].parent
    if root.exists() and any(root.iterdir()):
        raise RuntimeError(f'Phonon directory contains unrelated files: {root}')
    created_parents = []
    parents = [root, paths['model'].parent]
    if paths['unit'] is not None:
        parents.append(paths['unit'].parent)
    for parent in parents:
        missing = []
        cursor = parent
        while not cursor.exists():
            missing.append(cursor)
            cursor = cursor.parent
        parent.mkdir(parents=True, exist_ok=True)
        created_parents.extend(reversed(missing))
    published = []
    try:
        with ExitStack() as stack:
            runtime_tmp = stack.enter_context(tempfile.TemporaryDirectory(prefix='.phonon-install-', dir=root))
            model_tmp = stack.enter_context(tempfile.TemporaryDirectory(prefix='.phonon-model-', dir=paths['model'].parent))
            unit_tmp = None
            if paths['unit'] is not None:
                unit_tmp = stack.enter_context(tempfile.TemporaryDirectory(prefix='.phonon-unit-', dir=paths['unit'].parent))
            build = Path(runtime_tmp)
            venv = build / 'venv'
            model_home = Path(model_tmp)
            staged_model = model_home / MODEL_RELATIVE.parent
            env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
            print('5%', flush=True)
            run('uv', 'venv', '--relocatable', '--python', '3.13', venv, env=env)
            print('15%', flush=True)
            run('uv', 'pip', 'install', '--no-cache', '--link-mode', 'copy', '--python', venv / 'bin/python',
                f'torch=={TORCH_VERSION}', '--index-url', 'https://download.pytorch.org/whl/cpu', env=env)
            constraints = build / 'cpu-constraints.txt'
            constraints.write_text(f'torch=={TORCH_VERSION}\n')
            run('uv', 'pip', 'install', '--no-cache', '--link-mode', 'copy', '--python', venv / 'bin/python',
                '--constraint', constraints, f'fermion-research=={FERMION_VERSION}',
                'safetensors', 'soundfile', 'scipy', 'zstandard', env=env)
            validate_runtime(venv)
            print('50%', flush=True)
            download_env = dict(env, HOME=str(model_home), HF_HOME=str(model_home / 'hf'),
                                HF_HUB_CACHE=str(model_home / 'hf-cache'),
                                FERMION_CACHE_DIR=str(model_home / '.cache/fermion'),
                                TORCH_HOME=str(model_home / 'torch-cache'),
                                XDG_CACHE_HOME=str(model_home / '.cache'))
            run(venv / 'bin/python', '-I', '-B', '-c',
                'import fermion; fermion.load_speech("phonon-2")', env=download_env)
            validate_model(staged_model)
            print('90%', flush=True)
            staged_unit = None
            if unit_tmp is not None:
                staged_unit = Path(unit_tmp) / 'phonon-2.service'
                staged_unit.write_text(unit_text(paths))
                staged_unit.chmod(0o644)
            blocked = {signal.SIGTERM, signal.SIGINT}
            previous = signal.pthread_sigmask(signal.SIG_BLOCK, blocked)
            try:
                publish_directory(venv, paths['runtime'])
                published.append(('tree', paths['runtime']))
                publish_directory(staged_model, paths['model'])
                published.append(('tree', paths['model']))
                if staged_unit is not None:
                    publish_file(staged_unit, paths['unit'])
                    published.append(('file', paths['unit']))
            finally:
                signal.pthread_sigmask(signal.SIG_SETMASK, previous)
            validate_runtime(paths['runtime'])
            write_receipt(paths, published=published)
        print('100%', flush=True)
    except BaseException:
        for kind, path in reversed(published):
            if kind == 'tree':
                shutil.rmtree(path)
            else:
                path.unlink(missing_ok=True)
        raise
    finally:
        for parent in reversed(created_parents):
            try:
                parent.rmdir()
            except OSError:
                pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['install', 'adopt', 'validate-remove', 'check-remove', 'remove'])
    args = parser.parse_args()
    def cancel(signum, frame):
        raise KeyboardInterrupt('Installation canceled')
    signal.signal(signal.SIGTERM, cancel)
    signal.signal(signal.SIGINT, cancel)
    with installation_lock():
        paths = layout()
        if args.action == 'install':
            install(paths)
        elif args.action == 'adopt':
            adopt(paths)
        elif args.action in ('validate-remove', 'check-remove'):
            validate_remove(paths)
        else:
            remove(paths)


if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        if getattr(error, 'stderr', None):
            print(error.stderr, file=sys.stderr)
        print(str(error) or 'Installation canceled', file=sys.stderr)
        sys.exit(1)
