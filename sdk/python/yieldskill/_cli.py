"""Launch the version-locked Yield runtime carried by this wheel."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import NoReturn, Sequence


def runtime_path(platform: str | None = None) -> Path:
    """Return the wheel-local runtime or fail without checking PATH."""
    selected = platform or sys.platform
    name = "yskill.exe" if selected == "win32" else "yskill"
    path = Path(__file__).with_name("_runtime") / name
    if not path.is_file():
        raise FileNotFoundError(
            f"the packaged Yield runtime is missing at {path}; reinstall yieldskill"
        )
    return path


def run(argv: Sequence[str] | None = None, platform: str | None = None) -> int | NoReturn:
    """Replace this process on Unix; preserve the child exit code on Windows."""
    args = [str(runtime_path(platform)), *(argv if argv is not None else sys.argv[1:])]
    selected = platform or sys.platform
    python = os.environ.get("YIELD_PYTHON", sys.executable)
    resolved_python = shutil.which(python) or python
    python_bin = str(Path(resolved_python).resolve().parent)
    inherited_path = os.environ.get("PATH", "")
    environment = {
        **os.environ,
        "YIELD_LANGUAGE": os.environ.get("YIELD_LANGUAGE", "python"),
        "YIELD_PYTHON": python,
        "PATH": python_bin + (os.pathsep + inherited_path if inherited_path else ""),
    }
    if selected == "win32":
        return subprocess.run(args, check=False, env=environment).returncode
    os.execve(args[0], args, environment)
    raise AssertionError("os.execve returned")


def main() -> NoReturn:
    try:
        code = run()
    except FileNotFoundError as error:
        print(f"yskill: {error}", file=sys.stderr)
        raise SystemExit(1) from error
    raise SystemExit(code)
