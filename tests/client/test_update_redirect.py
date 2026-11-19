# SPDX-License-Identifier: Apache-2.0
"""Regression test: `reflop update` follows chained /releases/latest redirects.

The github.com -> github.com/reflopai rename made <releases>/latest redirect
once to the new owner's <releases>/latest before the final tag redirect.
Reading the first Location's last segment yields "latest", not a tag.

This module loads reflop/update.py directly so the test needs no torch.
"""

import http.server
import importlib.util
import threading
from pathlib import Path

UPDATE_PY = Path(__file__).resolve().parents[2] / "client" / "python" / "reflop" / "update.py"
_spec = importlib.util.spec_from_file_location("reflop_update_under_test", UPDATE_PY)
up = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(up)


class RedirectServer:
    """A local HEAD endpoint that redirects `hops` times before a 404."""

    def __init__(self, hops):
        self.hops = hops
        self.requests = []
        self._server = None
        self._thread = None

    def __enter__(self):
        outer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_HEAD(self):
                outer.requests.append(self.path)
                n = len(outer.requests)
                if n <= len(outer.hops):
                    self.send_response(302)
                    self.send_header("Location", outer.hops[n - 1])
                    self.end_headers()
                else:
                    self.send_response(404)
                    self.end_headers()

            def log_message(self, *a):
                pass

        self._server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        self.base = f"http://127.0.0.1:{self._server.server_address[1]}"
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()
        return self

    def __exit__(self, *a):
        self._server.shutdown()
        self._server.server_close()
        self._thread.join(timeout=5)


def _base(srv):
    return srv.base + "/oldowner/reflop/releases"


def test_latest_follows_multiple_redirects(monkeypatch):
    """Two chained redirects end at a tag; both must be followed."""
    hops = [
        "/newowner/reflop/releases/latest",
        "/newowner/reflop/releases/tag/v9.9.9",
    ]
    with RedirectServer(hops) as srv:
        monkeypatch.setenv("REFLOP_UPDATE_URL", _base(srv))
        assert up.latest() == "v9.9.9"
        assert len(srv.requests) == len(hops)


def test_latest_single_redirect_still_works(monkeypatch):
    """The original one-hop GitHub layout stays supported."""
    with RedirectServer(["/reflop/releases/tag/v1.2.3"]) as srv:
        monkeypatch.setenv("REFLOP_UPDATE_URL", _base(srv))
        assert up.latest() == "v1.2.3"


def test_latest_ignores_non_tag_redirect(monkeypatch):
    """A redirect that never ends at a tag still fails cleanly."""
    with RedirectServer(["/somewhere/latest", "/somewhere/else"]) as srv:
        monkeypatch.setenv("REFLOP_UPDATE_URL", _base(srv))
        try:
            up.latest()
        except RuntimeError as exc:
            assert "could not determine the latest" in str(exc)
        else:
            raise AssertionError("expected RuntimeError for a non-tag redirect")
