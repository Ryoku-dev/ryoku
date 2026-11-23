import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('redux_setup', Path(__file__).with_name('setup.py'))
m = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(m)


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.stage = self.root / 'stage'
        self.stage.mkdir()
        self.env = patch.dict(os.environ, {
            'HOME': str(self.root / 'home'), 'XDG_DATA_HOME': str(self.root / 'data'),
            'XDG_CONFIG_HOME': str(self.root / 'config'), 'XDG_STATE_HOME': str(self.root / 'state')})
        self.env.start()
        self.addCleanup(self.env.stop)
        self.paths = m.layout()
        self.out = io.StringIO()

    def fake_run(self, log, *args, on_line=None):
        if args[:2] == ('git', 'init'):
            native = self.stage / 'native'
            (native / 'third_party/ggml').mkdir(parents=True)
            (native / 'LICENSE').write_text('native license')
            (native / 'third_party/ggml/LICENSE').write_text('ggml license')
        if args[:2] == ('cmake', '--build'):
            for line in ['[ 10%] Building C object a.o\n', '[ 60%] Building CXX object b.o\n', '[100%] Linking\n']:
                log.write(line)
                on_line(line)
            library = self.stage / 'build/libparakeet.so'
            library.parent.mkdir(parents=True)
            library.write_bytes(b'library')

    def fake_download(self, url, target, sha, report):
        target.write_bytes(b'model')
        report(0.5)
        report(1.0)

    def install(self, download=None):
        with patch.object(m, 'run', self.fake_run), \
                patch.object(m, 'download', download or self.fake_download), \
                patch.object(m.shutil, 'which', return_value='/bin/tool'), \
                patch.object(m.subprocess, 'run') as host, \
                patch.object(m.ctypes, 'CDLL') as lib:
            host.return_value.stdout = ''
            lib.return_value.parakeet_capi_abi_version.return_value = 10
            m.install(self.paths, self.stage, m.Progress(self.out))

    def prepare_adoptable_install(self):
        model = b'pinned Redux model fixture'
        adapter = Path(__file__).with_name('adapter.py').read_bytes()
        expected_license = b'\n'.join([
            Path(__file__).with_name('UPSTREAM.md').read_bytes(),
            Path(__file__).with_name('LICENSE').read_bytes(),
        ])
        contents = {
            'model': model,
            'client': adapter,
            'worker': adapter,
            'library': b'native runtime fixture',
            'license': expected_license + b'\nnative license\n\nggml license\n',
            'unit': m.worker_unit(self.paths).encode(),
        }
        for key, value in contents.items():
            path = self.paths[key]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(value)
        runtime_licenses = contents['license'][len(expected_license):]
        return m.digest(self.paths['model']), m.hashlib.sha256(runtime_licenses).hexdigest()

    def test_install_remove_and_reinstall(self):
        unrelated = self.root / 'data/voxtype/models/keep.gguf'
        unrelated.parent.mkdir(parents=True)
        unrelated.write_text('keep')
        self.install()
        self.assertTrue(all(p.is_file() for p in self.paths.values()))
        self.assertIn('--pool-threads 4 --chunk-seconds 30', self.paths['unit'].read_text())
        m.remove(self.paths)
        self.assertFalse(any(p.exists() for p in self.paths.values()))
        self.assertTrue(unrelated.exists())
        m.shutil.rmtree(self.stage / 'build')
        m.shutil.rmtree(self.stage / 'native')
        self.install()
        self.assertTrue(self.paths['receipt'].exists())

    def test_progress_walks_every_step_once_and_never_goes_back(self):
        self.install()
        lines = self.out.getvalue().splitlines()
        stages = [line.removeprefix('stage: ') for line in lines if line.startswith('stage: ')]
        self.assertEqual(stages, list(m.STEPS))
        percents = [int(line[:-1]) for line in lines if line.endswith('%')]
        self.assertEqual(percents, sorted(set(percents)))
        self.assertEqual(percents[-1], 100)
        self.assertIn('[ 60%] Building CXX object b.o', m.log_path().read_text())

    def test_build_status_lines(self):
        self.assertEqual(m.build_fraction('[ 45%] Building CXX object x.o'), 0.45)
        self.assertEqual(m.build_fraction('[3/12] Building CXX object x.o'), 0.25)
        self.assertIsNone(m.build_fraction('-- Configuring done'))

    def test_missing_tools_are_named_with_the_install_command(self):
        with patch.object(m.shutil, 'which', side_effect=lambda tool: None if tool in ('cmake', 'c++') else '/bin/' + tool), \
                patch.object(m.subprocess, 'run') as host:
            host.return_value.stdout = 'sudo pacman -S base-devel cmake git\n'
            message = m.missing_tools()
        self.assertIn('cmake, a C++ compiler', message)
        self.assertIn('sudo pacman -S base-devel cmake git', message)

    def test_modified_files_are_not_deleted(self):
        self.install()
        self.paths['client'].write_text('user edited')
        with self.assertRaisesRegex(RuntimeError, 'changed'):
            m.remove(self.paths)
        self.assertTrue(self.paths['model'].exists())
        self.assertTrue(self.paths['receipt'].exists())

    def test_interrupted_download_publishes_nothing(self):
        def cancel(*args):
            raise KeyboardInterrupt()
        with self.assertRaises(KeyboardInterrupt):
            self.install(download=cancel)
        self.assertFalse(any(p.exists() for p in self.paths.values()))

    def test_publish_failure_rolls_back(self):
        real_link = os.link
        count = 0

        def fail(src, dst):
            nonlocal count
            count += 1
            if count == 3:
                raise OSError('disk full')
            return real_link(src, dst)
        with patch.object(m.os, 'link', fail):
            with self.assertRaisesRegex(OSError, 'disk full'):
                self.install()
        self.assertFalse(any(p.exists() for p in self.paths.values()))
        self.assertFalse(list(self.root.rglob('*.installing')))

    def test_unrecognized_receipt_cannot_delete_other_files(self):
        self.paths['receipt'].parent.mkdir(parents=True)
        external = self.root / 'keep'
        external.write_text('keep')
        self.paths['receipt'].write_text(json.dumps({'owner': m.OWNER, 'files': {str(external): m.digest(external)}}))
        with self.assertRaisesRegex(RuntimeError, 'outside'):
            m.remove(self.paths)
        self.assertTrue(external.exists())

    def test_removal_without_a_receipt_is_refused(self):
        with self.assertRaisesRegex(RuntimeError, 'not installed from Ryoku Settings'):
            m.remove(self.paths)

    def test_adopt_records_recognized_files_and_allows_safe_removal(self):
        model_sha, licenses_sha = self.prepare_adoptable_install()
        previous_handler = m.signal.getsignal(m.signal.SIGTERM)
        try:
            with patch.object(m, 'MODEL_SHA', model_sha), \
                    patch.object(m, 'RUNTIME_LICENSES_SHA', licenses_sha), \
                    patch.object(m.ctypes, 'CDLL') as lib, \
                    patch.object(m.sys, 'argv', ['setup.py', 'adopt']):
                lib.return_value.parakeet_capi_abi_version.return_value = 10
                m.main()
        finally:
            m.signal.signal(m.signal.SIGTERM, previous_handler)
        receipt = m.read_receipt(self.paths)
        self.assertTrue(receipt['adopted'])
        self.assertEqual(len(receipt['files']), 6)
        m.validate_remove(self.paths)
        m.remove(self.paths)
        self.assertFalse(any(path.exists() for path in self.paths.values()))

    def test_adoption_rejects_an_unrecognized_model_without_claiming_files(self):
        self.prepare_adoptable_install()
        with self.assertRaisesRegex(RuntimeError, 'model does not match'):
            m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())
        self.assertTrue(self.paths['model'].exists())

    def test_adoption_rejects_a_truncated_runtime_license_bundle(self):
        model_sha, _ = self.prepare_adoptable_install()
        self.paths['license'].write_bytes(self.paths['license'].read_bytes().split(b'\nnative license', 1)[0])
        with patch.object(m, 'MODEL_SHA', model_sha):
            with self.assertRaisesRegex(RuntimeError, 'runtime license bundle is incomplete'):
                m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())
        self.assertTrue(self.paths['license'].exists())

    def test_adoption_rejects_changed_service_unit(self):
        model_sha, licenses_sha = self.prepare_adoptable_install()
        self.paths['unit'].write_text('[Service]\nExecStart=/bin/other\n')
        with patch.object(m, 'MODEL_SHA', model_sha), patch.object(m, 'RUNTIME_LICENSES_SHA', licenses_sha):
            with self.assertRaisesRegex(RuntimeError, 'service unit differs'):
                m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())
        self.assertTrue(self.paths['unit'].exists())

    def test_adoption_rejects_unusable_native_runtime(self):
        model_sha, licenses_sha = self.prepare_adoptable_install()
        with patch.object(m, 'MODEL_SHA', model_sha), \
                patch.object(m, 'RUNTIME_LICENSES_SHA', licenses_sha), \
                patch.object(m.ctypes, 'CDLL', side_effect=OSError('bad ABI')):
            with self.assertRaisesRegex(RuntimeError, 'native runtime is not usable'):
                m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())

    def test_adoption_rejects_symlinked_install_file(self):
        self.prepare_adoptable_install()
        outside = self.root / 'outside'
        outside.write_text('keep')
        self.paths['client'].unlink()
        self.paths['client'].symlink_to(outside)
        with self.assertRaisesRegex(RuntimeError, 'regular installed file'):
            m.adopt(self.paths)
        self.assertFalse(self.paths['receipt'].exists())
        self.assertEqual(outside.read_text(), 'keep')


if __name__ == '__main__':
    unittest.main()
