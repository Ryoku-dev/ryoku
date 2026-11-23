import importlib.util
import argparse
from pathlib import Path
import shlex
import unittest

spec = importlib.util.spec_from_file_location("benchmark", Path(__file__).with_name("run.py"))
benchmark = importlib.util.module_from_spec(spec)
spec.loader.exec_module(benchmark)


class WordErrorTests(unittest.TestCase):
    def test_existing_normalization(self):
        self.assertEqual(benchmark.normalize("Mr. Jones, Mrs. Jones; Ruggedo's 25!"),
                         ["mister", "jones", "missus", "jones", "ruggedo's", "25"])

    def test_insert_delete_and_substitute(self):
        self.assertEqual(benchmark.score("one two three", "one four five three")["word_errors"], 2)
        self.assertEqual(benchmark.score("one two three", "one three")["word_errors"], 1)

    def test_punctuation_does_not_change_score(self):
        self.assertEqual(benchmark.score("Go to full globe view, and then turn on the radio.",
                                         "Go to full globe view and then turn on the radio")["wer"], 0)

    def test_empty_reference_is_rejected(self):
        with self.assertRaises(ValueError):
            benchmark.score("...", "text")

    def test_reproduction_command_preserves_every_effective_override(self):
        root = Path("/tmp/benchmark setup with spaces")
        args = argparse.Namespace(
            manifest=root / "audio manifest.json",
            socket=root / "redux.sock",
            redux_model=root / "redux model.gguf",
            rounds=5,
            endpoint="http://127.0.0.1:8010/",
        )
        cases = [{"id": "short clip"}, {"id": "long-clip"}]
        output = root / "results" / "reproduction-1234"

        command = shlex.split(benchmark.reproduction_command(args, cases, output))

        self.assertEqual(command[0], "python3")
        self.assertEqual(command[1], "benchmarks/phonon-redux/run.py")
        self.assertIn("--run", command)
        self.assertEqual(command[command.index("--manifest") + 1], str(args.manifest.resolve()))
        self.assertEqual(command[command.index("--cases") + 1:command.index("--rounds")],
                         ["short clip", "long-clip"])
        self.assertEqual(command[command.index("--rounds") + 1], "5")
        self.assertEqual(command[command.index("--endpoint") + 1], "http://127.0.0.1:8010")
        self.assertEqual(command[command.index("--socket") + 1], str(args.socket.absolute()))
        self.assertEqual(command[command.index("--redux-model") + 1], str(args.redux_model.absolute()))
        self.assertEqual(command[command.index("--output") + 1], str(output))


if __name__ == "__main__":
    unittest.main()
