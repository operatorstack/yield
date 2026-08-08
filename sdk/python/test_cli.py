from __future__ import annotations

import os
import subprocess
import tempfile
import unittest
import venv
from pathlib import Path
from unittest import mock

from yieldskill import _cli


class CliTest(unittest.TestCase):
    def test_missing_runtime_fails_without_path_fallback(self) -> None:
        with mock.patch.object(_cli.Path, "is_file", return_value=False):
            with self.assertRaisesRegex(FileNotFoundError, "packaged Yield runtime is missing"):
                _cli.runtime_path("linux")

    def test_unix_replaces_process_and_forwards_arguments(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / "yskill"
            binary.touch()
            with mock.patch.object(_cli, "runtime_path", return_value=binary):
                with mock.patch.dict(os.environ, {}, clear=True):
                    with mock.patch.object(
                        os, "execve", side_effect=RuntimeError("exec")
                    ) as execute:
                        with self.assertRaisesRegex(RuntimeError, "exec"):
                            _cli.run(["test", "skill"], "linux")
            call = execute.call_args
            self.assertEqual(call.args[:2], (str(binary), [str(binary), "test", "skill"]))
            self.assertEqual(call.args[2]["YIELD_LANGUAGE"], "python")
            self.assertEqual(call.args[2]["YIELD_PYTHON"], os.sys.executable)
            self.assertEqual(
                call.args[2]["PATH"].split(os.pathsep)[0],
                str(Path(os.sys.executable).absolute().parent),
            )

    def test_windows_preserves_exit_code_and_arguments(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / "yskill.exe"
            binary.touch()
            completed = mock.Mock(returncode=23)
            with mock.patch.object(_cli, "runtime_path", return_value=binary):
                with mock.patch.dict(os.environ, {}, clear=True):
                    with mock.patch.object(
                        _cli.subprocess, "run", return_value=completed
                    ) as execute:
                        self.assertEqual(_cli.run(["--version"], "win32"), 23)
            call = execute.call_args
            self.assertEqual(call.args[0], [str(binary), "--version"])
            self.assertFalse(call.kwargs["check"])
            self.assertEqual(call.kwargs["env"]["YIELD_LANGUAGE"], "python")
            self.assertEqual(call.kwargs["env"]["YIELD_PYTHON"], os.sys.executable)
            self.assertEqual(
                call.kwargs["env"]["PATH"].split(os.pathsep)[0],
                str(Path(os.sys.executable).absolute().parent),
            )

    def test_selected_virtual_environment_is_first_on_path(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / "yskill"
            binary.touch()
            python = Path(directory) / ".venv" / "bin" / "python"
            python.parent.mkdir(parents=True)
            python.touch()
            with mock.patch.object(_cli, "runtime_path", return_value=binary):
                with mock.patch.dict(
                    os.environ, {"YIELD_PYTHON": str(python), "PATH": "/usr/bin"}, clear=True
                ):
                    with mock.patch.object(
                        os, "execve", side_effect=RuntimeError("exec")
                    ) as execute:
                        with self.assertRaisesRegex(RuntimeError, "exec"):
                            _cli.run([], "linux")
            environment = execute.call_args.args[2]
            self.assertEqual(environment["YIELD_PYTHON"], str(python))
            self.assertEqual(environment["PATH"], f"{python.absolute().parent}{os.pathsep}/usr/bin")

    def test_virtual_environment_symlink_keeps_its_lexical_bin_directory(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / "yskill"
            binary.touch()
            system = Path(directory) / "system" / "python"
            system.parent.mkdir(parents=True)
            system.touch()
            python = Path(directory) / ".venv" / "bin" / "python"
            python.parent.mkdir(parents=True)
            python.symlink_to(system)
            with mock.patch.object(_cli, "runtime_path", return_value=binary):
                with mock.patch.dict(
                    os.environ, {"YIELD_PYTHON": str(python), "PATH": "/usr/bin"}, clear=True
                ):
                    with mock.patch.object(
                        os, "execve", side_effect=RuntimeError("exec")
                    ) as execute:
                        with self.assertRaisesRegex(RuntimeError, "exec"):
                            _cli.run([], "linux")
            environment = execute.call_args.args[2]
            self.assertEqual(environment["PATH"].split(os.pathsep)[0], str(python.parent))

    @unittest.skipIf(os.name == "nt", "Unix execve integration test")
    def test_unactivated_virtual_environment_commands_reach_the_runtime(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            environment = root / ".venv"
            venv.EnvBuilder(with_pip=False).create(environment)
            python = environment / "bin" / "python"
            tool = environment / "bin" / "yield-venv-tool"
            tool.write_text("#!/bin/sh\necho venv-tool-ready\n")
            tool.chmod(0o755)
            runtime = root / "runtime"
            runtime.write_text("#!/bin/sh\npython --version >/dev/null && yield-venv-tool\n")
            runtime.chmod(0o755)
            script = (
                "from yieldskill import _cli; "
                f"_cli.runtime_path=lambda platform=None: __import__('pathlib').Path({str(runtime)!r}); "
                "_cli.run([], 'linux')"
            )
            inherited = {**os.environ, "PATH": "/usr/bin:/bin", "YIELD_PYTHON": str(python)}
            result = subprocess.run(
                [str(python), "-c", script],
                cwd=Path(__file__).parent,
                env=inherited,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("venv-tool-ready", result.stdout)


if __name__ == "__main__":
    unittest.main()
