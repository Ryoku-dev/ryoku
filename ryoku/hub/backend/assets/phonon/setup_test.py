import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('phonon_setup', Path(__file__).with_name('setup.py'))
m = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(m)
MODEL = b'owned-model-fixture'
MODEL_SHA = hashlib.sha256(MODEL).hexdigest()


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.env = patch.dict(os.environ, {'HOME': str(self.root / 'home'),
                                          'XDG_DATA_HOME': str(self.root / 'data'),
                                          'XDG_CONFIG_HOME': str(self.root / 'config'),
                                          'XDG_STATE_HOME': str(self.root / 'state'),
                                          'RYOKU_PHONON_SERVICE_MANAGER': 'systemd'})
        self.env.start()
        self.addCleanup(self.env.stop)
        self.paths = m.layout()
        self.interpreter = self.root / 'shared-python'
        self.interpreter.write_text('shared interpreter must never be removed')
        self.calls = []
        self.run_patch = patch.object(m, 'run', self.fake_run)
        self.run_patch.start()
        self.addCleanup(self.run_patch.stop)
        for target, value in [('MODEL_SHA', MODEL_SHA), ('MODEL_BYTES', len(MODEL))]:
            setting = patch.object(m, target, value)
            setting.start()
            self.addCleanup(setting.stop)
        tool = patch.object(m.shutil, 'which', return_value='/usr/bin/uv')
        tool.start()
        self.addCleanup(tool.stop)

    def fake_run(self, *args, env=None, capture=False):
        self.calls.append((args, env))
        if capture:
            return json.dumps({'fermion': m.FERMION_VERSION, 'torch': m.TORCH_VERSION,
                               'prefix': str(Path(args[0]).parent.parent), 'cuda': []})
        if args[:2] == ('uv', 'venv'):
            runtime = Path(args[-1])
            (runtime / 'bin').mkdir(parents=True)
            (runtime / 'bin/python').symlink_to(self.interpreter)
            (runtime / 'bin/python3').symlink_to('python')
            (runtime / 'bin/phonon').write_text('relocatable phonon command')
            (runtime / 'bin/phonon').chmod(0o755)
            (runtime / 'package.py').write_text('owned runtime package')
        elif 'import fermion; fermion.load_speech("phonon-2")' in args:
            model = Path(env['FERMION_CACHE_DIR']) / m.MODEL_RELATIVE.relative_to('.cache/fermion')
            model.mkdir(parents=True)
            (model / 'model.fermion').write_bytes(MODEL)
            (model / 'config.json').write_text(json.dumps({'sample_rate': 16000}))
            (model / 'packed_manifest.json').write_text(json.dumps({'container_sha256': MODEL_SHA}))
            model.with_name(model.name + '.lock').touch()

    def install(self):
        m.install(self.paths)

    def assert_unpublished(self):
        owned = (*m.managed_paths(self.paths), self.paths['receipt'])
        self.assertFalse(any(path.exists() or path.is_symlink() for path in owned))
        self.assertFalse(list(self.root.rglob('.phonon-*')))

    def test_install_remove_and_reinstall_preserves_shared_files(self):
        shared_cache = self.root / 'home/.cache/uv/keep'
        other_model = self.root / 'home/.cache/fermion/speech/other/model'
        for file in (shared_cache, other_model):
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text('unrelated')
        self.install()
        owned = (*m.managed_paths(self.paths), self.paths['receipt'])
        self.assertTrue(all(path.exists() for path in owned))
        self.assertEqual(stat.S_IMODE(self.paths['receipt'].stat().st_mode), 0o600)
        self.assertIn('Environment=PYTHONDONTWRITEBYTECODE=1', self.paths['unit'].read_text())
        self.assertIn('--served-model-name phonon-2', self.paths['unit'].read_text())
        self.assertEqual(self.paths['runtime'].joinpath('bin/python').readlink(), self.interpreter)
        cpu = next(args for args, _ in self.calls if f'torch=={m.TORCH_VERSION}' in args)
        self.assertIn('--no-cache', cpu)
        self.assertIn('https://download.pytorch.org/whl/cpu', cpu)
        self.assertTrue(any('--relocatable' in args for args, _ in self.calls))
        download = next(env for args, env in self.calls if 'import fermion; fermion.load_speech("phonon-2")' in args)
        self.assertNotEqual(download['HOME'], str(Path.home()))
        self.assertTrue(download['HF_HUB_CACHE'].startswith(download['HOME']))
        self.assertEqual(Path(download['FERMION_CACHE_DIR']), Path(download['HOME']) / '.cache/fermion')
        self.assertNotEqual(download['FERMION_CACHE_DIR'], download['HF_HUB_CACHE'])
        m.validate_remove(self.paths)
        m.remove(self.paths)
        self.assert_unpublished()
        self.assertEqual(self.interpreter.read_text(), 'shared interpreter must never be removed')
        self.assertTrue(shared_cache.exists())
        self.assertTrue(other_model.exists())
        self.install()
        m.remove(self.paths)

    def test_runit_install_receipt_leaves_packaged_service_owned_by_ryoku(self):
        with patch.dict(os.environ, {'RYOKU_PHONON_SERVICE_MANAGER': 'runit',
                                    'RYOKU_USER_SERVICE_DIR': str(self.root / 'services')}):
            paths = m.layout()
            service_run = paths['service'] / 'run'
            service_run.parent.mkdir(parents=True)
            service_run.write_text('#!/bin/sh\n')
            service_run.chmod(0o755)
            (service_run.parent / 'down').write_text('')

            m.install(paths)
            receipt = m.read_receipt(paths)
            self.assertEqual(receipt['manager'], 'runit')
            self.assertNotIn('unit', receipt)
            service_before = service_run.read_bytes()

            m.remove(paths)

            self.assertEqual(service_run.read_bytes(), service_before)
            self.assertTrue((service_run.parent / 'down').exists())
            self.assertFalse(paths['runtime'].exists())
            self.assertFalse(paths['model'].exists())

    def test_adopt_normalizes_known_unit_and_removes_owned_trees(self):
        self.install()
        self.paths['receipt'].unlink()
        self.paths['unit'].write_text(m.unit_text(self.paths, bytecode=False, quoted=False))
        doc = m.adopt(self.paths)
        self.assertTrue(doc['adopted'])
        self.assertIn('PYTHONDONTWRITEBYTECODE=1', self.paths['unit'].read_text())
        m.validate_remove(self.paths)
        m.remove(self.paths)
        self.assert_unpublished()
        self.assertTrue(self.interpreter.exists())

    def test_changed_runtime_refuses_all_removal(self):
        self.install()
        (self.paths['runtime'] / 'package.py').write_text('user edited')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.paths['model'].exists())
        self.assertTrue(self.paths['unit'].exists())
        self.assertTrue(self.paths['receipt'].exists())

    def test_foreign_runtime_file_refuses_removal(self):
        self.install()
        (self.paths['runtime'] / 'foreign-user-file').write_text('keep')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.paths['runtime'].exists())

    def test_changed_model_refuses_removal(self):
        self.install()
        (self.paths['model'] / m.MODEL_RELATIVE.name / 'model.fermion').write_text('changed')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.paths['runtime'].exists())

    def test_changed_unit_refuses_removal(self):
        self.install()
        self.paths['unit'].write_text('user service')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.paths['runtime'].exists())

    def test_foreign_receipt_roots_cannot_target_another_directory(self):
        self.install()
        doc = json.loads(self.paths['receipt'].read_text())
        doc['roots']['runtime'] = str(self.interpreter.parent)
        self.paths['receipt'].write_text(json.dumps(doc))
        with self.assertRaisesRegex(RuntimeError, 'outside'):
            m.remove(self.paths)
        self.assertTrue(self.interpreter.exists())

    def test_changed_interpreter_symlink_refuses_removal(self):
        self.install()
        python = self.paths['runtime'] / 'bin/python'
        python.unlink()
        python.symlink_to('/bin/sh')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.interpreter.exists())

    def test_unit_symlink_never_deletes_target(self):
        self.install()
        self.paths['unit'].unlink()
        self.paths['unit'].symlink_to(self.interpreter)
        with self.assertRaisesRegex(RuntimeError, 'symlink'):
            m.remove(self.paths)
        self.assertTrue(self.interpreter.exists())

    def test_symlinked_destination_parent_refuses_install(self):
        self.paths['runtime'].parent.parent.mkdir(parents=True)
        foreign = self.root / 'foreign'
        foreign.mkdir()
        self.paths['runtime'].parent.symlink_to(foreign, target_is_directory=True)
        with self.assertRaisesRegex(RuntimeError, 'symlink'):
            self.install()
        self.assertEqual(list(foreign.iterdir()), [])

    def test_unknown_adoption_unit_and_cuda_runtime_are_refused(self):
        self.install()
        self.paths['receipt'].unlink()
        self.paths['unit'].write_text('foreign service')
        with self.assertRaisesRegex(RuntimeError, 'recognizable'):
            m.adopt(self.paths)
        self.paths['unit'].write_text(m.unit_text(self.paths))
        def cuda(*args, **kwargs):
            return json.dumps({'fermion': m.FERMION_VERSION, 'torch': '2.14.1',
                               'prefix': str(self.paths['runtime']), 'cuda': ['nvidia-cublas']})
        with patch.object(m, 'run', cuda):
            with self.assertRaisesRegex(RuntimeError, 'CPU-only'):
                m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())

    def test_cancel_during_download_publishes_nothing(self):
        original = self.fake_run
        def cancel(*args, **kwargs):
            if 'import fermion; fermion.load_speech("phonon-2")' in args:
                raise KeyboardInterrupt()
            return original(*args, **kwargs)
        with patch.object(m, 'run', cancel):
            with self.assertRaises(KeyboardInterrupt):
                self.install()
        self.assert_unpublished()

    def test_cancel_after_publication_rolls_back(self):
        original = m.validate_runtime
        def cancel(root):
            if root == self.paths['runtime']:
                raise KeyboardInterrupt()
            return original(root)
        with patch.object(m, 'validate_runtime', cancel):
            with self.assertRaises(KeyboardInterrupt):
                self.install()
        self.assert_unpublished()

    def test_publication_failure_rolls_back_created_trees(self):
        original = m.publish_file
        def fail(source, target):
            if target == self.paths['unit']:
                raise OSError('disk full')
            return original(source, target)
        with patch.object(m, 'publish_file', fail):
            with self.assertRaisesRegex(OSError, 'disk full'):
                self.install()
        self.assert_unpublished()

    def test_racing_foreign_directory_is_not_replaced(self):
        original = m.publish_directory
        def race(source, target):
            if target == self.paths['model']:
                target.mkdir()
            return original(source, target)
        with patch.object(m, 'publish_directory', race):
            with self.assertRaises(OSError):
                self.install()
        self.assertTrue(self.paths['model'].is_dir())
        self.assertFalse(self.paths['runtime'].exists())
        self.assertFalse(self.paths['unit'].exists())
        self.assertFalse(self.paths['receipt'].exists())

    def test_missing_owned_files_can_still_be_removed(self):
        self.install()
        (self.paths['runtime'] / 'package.py').unlink()
        m.remove(self.paths)
        self.assert_unpublished()

    def test_help_has_all_modes(self):
        result = subprocess.run([sys.executable, str(Path(m.__file__)), '--help'],
                                check=True, capture_output=True, text=True)
        self.assertIn('check-remove', result.stdout)
        self.assertIn('validate-remove', result.stdout)

    def test_concurrent_operation_lock_refuses_second_owner(self):
        with m.installation_lock():
            with self.assertRaisesRegex(RuntimeError, 'already running'):
                with m.installation_lock():
                    self.fail('Acquired a second lock')

    def test_cancel_at_receipt_publication_rolls_back(self):
        original = m.publish_file
        def cancel(signum, frame):
            raise KeyboardInterrupt()
        previous = signal.signal(signal.SIGTERM, cancel)
        self.addCleanup(signal.signal, signal.SIGTERM, previous)
        def interrupt(source, target):
            original(source, target)
            if target == self.paths['receipt']:
                signal.raise_signal(signal.SIGTERM)
        with patch.object(m, 'publish_file', interrupt):
            with self.assertRaises(KeyboardInterrupt):
                self.install()
        self.assert_unpublished()

    def test_real_process_group_cancellation_cleans_staging_promptly(self):
        ready = self.root / 'ready'
        code = f'''import importlib.util,pathlib,subprocess,sys
spec=importlib.util.spec_from_file_location("phonon_setup", {str(Path(m.__file__))!r})
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
def waiting(*args, **kwargs):
    pathlib.Path({str(ready)!r}).write_text("ready")
    subprocess.run([sys.executable,"-c","import time; time.sleep(30)"],check=True)
module.shutil.which=lambda name: "/usr/bin/uv" if name == "uv" else None
module.run=waiting
sys.argv=["setup.py","install"]
try: module.main()
except (Exception, KeyboardInterrupt): sys.exit(1)
'''
        process = subprocess.Popen([sys.executable, '-B', '-c', code],
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   start_new_session=True)
        try:
            deadline = time.monotonic() + 5
            while not ready.exists() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.01)
            self.assertTrue(ready.exists(), process.communicate(timeout=1) if process.poll() is not None else '')
            started = time.monotonic()
            os.killpg(process.pid, signal.SIGTERM)
            process.communicate(timeout=2)
            self.assertLess(time.monotonic() - started, 2)
            self.assertEqual(process.returncode, 1)
            self.assert_unpublished()
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
                process.communicate()

    def test_adoption_receipt_failure_restores_original_unit(self):
        self.install()
        self.paths['receipt'].unlink()
        original = m.unit_text(self.paths, bytecode=False, quoted=False)
        self.paths['unit'].write_text(original)
        with patch.object(m, 'write_receipt', side_effect=OSError('disk full')):
            with self.assertRaisesRegex(OSError, 'disk full'):
                m.adopt(self.paths)
        self.assertEqual(self.paths['unit'].read_text(), original)
        self.assertFalse(self.paths['receipt'].exists())

    def test_adoption_failure_never_restores_through_a_foreign_symlink(self):
        self.install()
        self.paths['receipt'].unlink()
        self.paths['unit'].write_text(m.unit_text(self.paths, bytecode=False))
        def fail(*args, **kwargs):
            self.paths['unit'].unlink()
            self.paths['unit'].symlink_to(self.interpreter)
            raise OSError('unit changed concurrently')
        with patch.object(m, 'write_receipt', fail):
            with self.assertRaisesRegex(OSError, 'concurrently'):
                m.adopt(self.paths)
        self.assertEqual(self.interpreter.read_text(), 'shared interpreter must never be removed')


if __name__ == '__main__':
    unittest.main()
