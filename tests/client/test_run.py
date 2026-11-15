# SPDX-License-Identifier: Apache-2.0
"""Issue #8: when `reflop run` sees its child die from a signal it names the
signal and exits 128 + n, the status the shell would have reported itself."""

import signal
import subprocess

from reflop.cli.main import main


def test_signal_death_is_reported(monkeypatch, capsys):
    monkeypatch.setattr(subprocess, "call", lambda command, env=None: -signal.SIGSEGV)

    rc = main(["run", "python", "-c", "raise"])

    assert rc == 128 + signal.SIGSEGV
    err = capsys.readouterr().err
    assert "python -c raise was killed by SIGSEGV (signal 11)" in err


def test_exit_status_passes_through(monkeypatch):
    monkeypatch.setattr(subprocess, "call", lambda command, env=None: 3)

    assert main(["run", "false"]) == 3


def test_unknown_signal_number_still_reported(monkeypatch, capsys):
    monkeypatch.setattr(subprocess, "call", lambda command, env=None: -1000)

    assert main(["run", "crash"]) == 1128
    assert "crash was killed by signal 1000 (signal 1000)" in capsys.readouterr().err
