"""Sets MANAGENTS_FW_VERSION, the firmware version the display reports in its hello.

The version is taken from, in order: the MANAGENTS_VERSION environment variable (the release
workflow sets it from the tag, so a shallow checkout needs no tags), `git describe` of the
checkout, and finally "0.0.0-dev". A leading "v" is dropped, so tag v0.2.0 gives "0.2.0", the
same string the helper reports.

The define goes to the project sources only: a new version recompiles src/, not the framework.
merge_image.py reads the version back from the build environment.
"""

import os
import subprocess

Import("env", "projenv")  # noqa: F821  (provided by PlatformIO)


def describe(directory):
    """`git describe` of the checkout containing `directory`, or "" outside a git checkout."""
    try:
        return subprocess.run(
            ["git", "describe", "--tags", "--match", "v*", "--always", "--dirty"],
            cwd=directory,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return ""


version = os.environ.get("MANAGENTS_VERSION") or describe(env.subst("$PROJECT_DIR")) or "0.0.0-dev"
version = version.removeprefix("v")

env.Replace(MANAGENTS_FW_VERSION=version)
projenv.Append(CPPDEFINES=[("MANAGENTS_FW_VERSION", env.StringifyMacro(version))])
