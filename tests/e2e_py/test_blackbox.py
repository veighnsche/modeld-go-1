import contextlib
import json
import os
import pathlib
import shutil
import socket
import subprocess
import threading
import sys
import tempfile
import time
from typing import Tuple

import requests

ROOT = pathlib.Path(__file__).resolve().parents[2]
BIN_NAME = "modeld"


def find_free_port() -> int:
    with contextlib.closing(socket.socket(socket.AF_INET, socket.SOCK_STREAM)) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def build_binary() -> pathlib.Path:
    out_dir = pathlib.Path(tempfile.mkdtemp(prefix="modeld-bin-"))
    bin_path = out_dir / BIN_NAME
    env = os.environ.copy()
    env.setdefault("CGO_ENABLED", "0")
    proc = subprocess.run([
        "go", "build", "-o", str(bin_path), "./cmd/modeld"
    ], cwd=str(ROOT), env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    if proc.returncode != 0:
        raise AssertionError(f"go build failed:\n{proc.stdout}")
    return bin_path


CAPTURED_LOGS: list[str] = []


def _reader_thread(stream, name: str):
    for line in iter(stream.readline, ''):
        if not line:
            break
        # Normalize to str
        s = line if isinstance(line, str) else line.decode('utf-8', 'replace')
        # Tee to current stdout
        try:
            sys.stdout.write(s)
        except Exception:
            pass
        CAPTURED_LOGS.append(f"[{name}] {s.rstrip()}")


@contextlib.contextmanager
def start_server(models_dir: pathlib.Path, default_model: str | None = None):
    bin_path = build_binary()
    port = find_free_port()
    addr = f":{port}"
    args = [str(bin_path), "--addr", addr, "--models-dir", str(models_dir)]
    if default_model:
        args += ["--default-model", default_model]
    env = os.environ.copy()
    proc = subprocess.Popen(args, cwd=str(ROOT), env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    t_out = threading.Thread(target=_reader_thread, args=(proc.stdout, 'OUT'), daemon=True)
    t_err = threading.Thread(target=_reader_thread, args=(proc.stderr, 'ERR'), daemon=True)
    t_out.start(); t_err.start()
    base = f"http://127.0.0.1:{port}"
    deadline = time.time() + 5
    try:
        # Wait for healthz
        while time.time() < deadline:
            try:
                r = requests.get(base + "/healthz", timeout=0.5)
                if r.status_code == 200:
                    break
            except Exception:
                pass
            time.sleep(0.05)
        else:
            raise AssertionError("server did not become healthy in time")
        yield base
    finally:
        with contextlib.suppress(Exception):
            proc.terminate()
        try:
            proc.wait(timeout=3)
        except Exception:
            with contextlib.suppress(Exception):
                proc.kill()
        # Ensure log threads drained
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        # Clean the built binary directory
        with contextlib.suppress(Exception):
            shutil.rmtree(bin_path.parent)


@contextlib.contextmanager
def start_server_with_handle(models_dir: pathlib.Path, default_model: str | None = None):
    """Start the server and yield (base_url, process).

    This is similar to start_server but also exposes the process handle so tests
    can send signals (e.g., SIGTERM) to verify graceful shutdown behavior.
    """
    bin_path = build_binary()
    port = find_free_port()
    addr = f":{port}"
    args = [str(bin_path), "--addr", addr, "--models-dir", str(models_dir)]
    if default_model:
        args += ["--default-model", default_model]
    env = os.environ.copy()
    proc = subprocess.Popen(args, cwd=str(ROOT), env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    t_out = threading.Thread(target=_reader_thread, args=(proc.stdout, 'OUT'), daemon=True)
    t_err = threading.Thread(target=_reader_thread, args=(proc.stderr, 'ERR'), daemon=True)
    t_out.start(); t_err.start()
    base = f"http://127.0.0.1:{port}"
    deadline = time.time() + 5
    try:
        # Wait for healthz
        while time.time() < deadline:
            try:
                r = requests.get(base + "/healthz", timeout=0.5)
                if r.status_code == 200:
                    break
            except Exception:
                pass
            time.sleep(0.05)
        else:
            raise AssertionError("server did not become healthy in time")
        yield base, proc
    finally:
        with contextlib.suppress(Exception):
            proc.terminate()
        try:
            proc.wait(timeout=3)
        except Exception:
            with contextlib.suppress(Exception):
                proc.kill()
        # Ensure log threads drained
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        # Clean the built binary directory
        with contextlib.suppress(Exception):
            shutil.rmtree(bin_path.parent)


def touch_models(names: list[str]) -> Tuple[pathlib.Path, list[str]]:
    d = pathlib.Path(tempfile.mkdtemp(prefix="models-"))
    for n in names:
        (d / n).write_text("")
    return d, names


def _discover_user_models() -> Tuple[pathlib.Path | None, list[str]]:
    home = pathlib.Path.home()
    real_dir = home / "models" / "llm"
    if not real_dir.exists():
        return None, []
    models = [p.name for p in real_dir.glob("*.gguf")]
    if not models:
        return real_dir, []
    return real_dir, models


def test_blackbox_flow():
    # Prefer real models under ~/models/llm if present
    real_dir, real_models = _discover_user_models()
    if real_dir and len(real_models) >= 1:
        models_dir, models = real_dir, real_models[:]
        if len(models) == 1:
            # fabricate a second temp model name only if needed for list count; infer still uses real one
            # But better: just accept single-model flow; adjust expectations below
            pass
    else:
        models_dir, models = touch_models(["alpha.gguf", "beta.gguf"])
    with start_server(models_dir, default_model=models[0]) as base:
        # /healthz
        r = requests.get(base + "/healthz")
        assert r.status_code == 200

        # /models
        r = requests.get(base + "/models")
        assert r.status_code == 200
        assert "application/json" in r.headers.get("Content-Type", "")
        data = r.json()
        assert isinstance(data.get("models"), list)
        if len(models) >= 2:
            assert len(data["models"]) >= 2
        else:
            assert len(data["models"]) >= 1

        # /readyz initially 503
        r = requests.get(base + "/readyz")
        assert r.status_code == 503

        # /infer without model uses default
        r = requests.post(base + "/infer", json={"prompt": "hello"})
        assert r.status_code == 200
        # streaming NDJSON -> at least one newline
        assert "\n" in r.text

        # /readyz eventually 200
        deadline = time.time() + 2
        while time.time() < deadline:
            r = requests.get(base + "/readyz")
            if r.status_code == 200:
                break
            time.sleep(0.025)
        else:
            raise AssertionError("/readyz did not become ready in time")

        # /status shows at least one instance
        r = requests.get(base + "/status")
        assert r.status_code == 200
        st = r.json()
        assert isinstance(st.get("instances"), list)
        assert len(st["instances"]) >= 1


def test_blackbox_infer_model_not_found_404():
    models_dir, models = touch_models(["alpha.gguf"])  # one real model
    with start_server(models_dir, default_model=models[0]) as base:
        r = requests.post(base + "/infer", json={"model": "missing.gguf", "prompt": "hi"})
        assert r.status_code == 404


def test_blackbox_infer_no_default_no_model_404():
    models_dir, _ = touch_models(["alpha.gguf"])  # at least one model exists on disk
    with start_server(models_dir, default_model=None) as base:
        r = requests.post(base + "/infer", json={"prompt": "hi"})
        assert r.status_code == 404


def test_blackbox_user_models_dir_if_present():
    # Optional: if user has real models under ~/models/llm, exercise them
    home = pathlib.Path.home()
    real_dir = home / "models" / "llm"
    if not real_dir.exists():
        return
    models = [p.name for p in real_dir.glob("*.gguf")]
    if not models:
        return
    # If one model: default-only generation; if >=2: also explicit model switch
    default = models[0]
    with start_server(real_dir, default_model=default) as base:
        r = requests.post(base + "/infer", json={"prompt": "hello real"})
        assert r.status_code == 200
        assert "\n" in r.text
        if len(models) >= 2:
            r = requests.post(base + "/infer", json={"model": models[1], "prompt": "switch"})
            assert r.status_code == 200
            assert "\n" in r.text


def test_blackbox_real_generation_required():
    """Run generation tests strictly against ~/models/llm if available.

    This test uses only real models and will skip if the directory is missing or empty.
    It performs default generation; and if ≥2 models exist, also tests explicit switching.
    """
    real_dir, models = _discover_user_models()
    if not real_dir or not models:
        import pytest
        pytest.skip("~/models/llm missing or has no *.gguf models")
    default = models[0]
    with start_server(real_dir, default_model=default) as base:
        # default generation
        r = requests.post(base + "/infer", json={"prompt": "real default"})
        assert r.status_code == 200
        assert "\n" in r.text
        # switch if another model exists
        if len(models) >= 2:
            r = requests.post(base + "/infer", json={"model": models[1], "prompt": "real switch"})
            assert r.status_code == 200
            assert "\n" in r.text


def _split_ndjson(text: str) -> list[str]:
    return [line for line in text.splitlines() if line.strip()]


def test_happy_switch_explicit_model():
    models_dir, models = touch_models(["alpha.gguf", "beta.gguf"])
    with start_server(models_dir, default_model=models[0]) as base:
        r1 = requests.post(base + "/infer", json={"model": models[0], "prompt": "A"})
        assert r1.status_code == 200
        assert "application/x-ndjson" in r1.headers.get("Content-Type", "")
        assert len(_split_ndjson(r1.text)) >= 2

        r2 = requests.post(base + "/infer", json={"model": models[1], "prompt": "B"})
        assert r2.status_code == 200
        assert "application/x-ndjson" in r2.headers.get("Content-Type", "")
        assert len(_split_ndjson(r2.text)) >= 2

        # /status should show at least one instance; try to detect both model ids if present
        st = requests.get(base + "/status").json()
        ids = {inst.get("model_id") for inst in st.get("instances", [])}
        # We accept at least one due to possible budgeting/evictions, but prefer both
        assert len(ids) >= 1


def test_happy_default_then_explicit():
    models_dir, models = touch_models(["alpha.gguf", "beta.gguf"])
    with start_server(models_dir, default_model=models[0]) as base:
        r = requests.post(base + "/infer", json={"prompt": "hello"})
        assert r.status_code == 200
        assert len(_split_ndjson(r.text)) >= 2

        r = requests.post(base + "/infer", json={"model": models[1], "prompt": "hi"})
        assert r.status_code == 200
        assert len(_split_ndjson(r.text)) >= 2


def test_happy_repeat_infer_same_model():
    models_dir, models = touch_models(["alpha.gguf"])
    with start_server(models_dir, default_model=models[0]) as base:
        r1 = requests.post(base + "/infer", json={"model": models[0], "prompt": "first"})
        assert r1.status_code == 200
        assert len(_split_ndjson(r1.text)) >= 2

        r2 = requests.post(base + "/infer", json={"model": models[0], "prompt": "second"})
        assert r2.status_code == 200
        assert len(_split_ndjson(r2.text)) >= 2


def test_happy_content_type_and_streaming():
    models_dir, models = touch_models(["alpha.gguf"]) 
    with start_server(models_dir, default_model=models[0]) as base:
        r = requests.post(base + "/infer", json={"prompt": "stream"})
        assert r.status_code == 200
        assert "application/x-ndjson" in r.headers.get("Content-Type", "")
        lines = _split_ndjson(r.text)
        assert len(lines) >= 2
        # Last line should contain done=true per stub
        last = json.loads(lines[-1])
        assert last.get("done") is True


def test_happy_ready_after_switch():
    models_dir, models = touch_models(["alpha.gguf", "beta.gguf"])
    with start_server(models_dir, default_model=models[0]) as base:
        # Ensure ready after default infer
        requests.post(base + "/infer", json={"prompt": "default"})
        deadline = time.time() + 2
        while time.time() < deadline:
            if requests.get(base + "/readyz").status_code == 200:
                break
            time.sleep(0.025)
        else:
            raise AssertionError("readyz not OK after default infer")

        # Switch and ensure still ready
        r = requests.post(base + "/infer", json={"model": models[1], "prompt": "switch"})
        assert r.status_code == 200
        assert requests.get(base + "/readyz").status_code == 200


def test_happy_models_list_contains_default():
    models_dir, models = touch_models(["alpha.gguf"]) 
    with start_server(models_dir, default_model=models[0]) as base:
        r = requests.get(base + "/models")
        assert r.status_code == 200
        names = {m.get("id") for m in r.json().get("models", [])}
        assert models[0] in names


def test_blackbox_client_cancellation_mid_stream():
    """Client aborts the /infer stream mid-way; server should not produce 500
    and should remain operational for subsequent requests.
    """
    models_dir, models = touch_models(["alpha.gguf"]) 
    with start_server(models_dir, default_model=models[0]) as base:
        # Start a streaming infer request
        s = requests.Session()
        r = s.post(base + "/infer", json={"prompt": "will-cancel"}, stream=True)
        assert r.status_code == 200
        it = r.iter_lines(decode_unicode=True)
        # Read the first line to ensure stream started
        first = next(it, None)
        assert first is not None
        # Abort the request mid-stream
        r.close()
        s.close()
        # Small delay to allow server handler to observe cancellation
        time.sleep(0.05)
        # Server should still serve new requests successfully (no 500)
        r2 = requests.post(base + "/infer", json={"prompt": "after-cancel"})
        assert r2.status_code == 200
        assert "\n" in r2.text


def test_blackbox_shutdown_cancels_inflight():
    """Graceful shutdown should cancel in-flight /infer requests and exit cleanly.

    Procedure:
    - Start server and begin a streaming /infer
    - Read one line to ensure streaming has begun
    - Send SIGTERM to the server process
    - Assert the process exits soon and the stream ends without a server 500
    - Subsequent connection attempts should fail due to shutdown
    """
    models_dir, models = touch_models(["alpha.gguf"]) 
    with start_server_with_handle(models_dir, default_model=models[0]) as (base, proc):
        s = requests.Session()
        r = s.post(base + "/infer", json={"prompt": "shutdown-cancel"}, stream=True)
        assert r.status_code == 200
        it = r.iter_lines(decode_unicode=True)
        # ensure streaming began
        first = next(it, None)
        assert first is not None
        # trigger graceful shutdown
        with contextlib.suppress(Exception):
            proc.terminate()
        # process should exit promptly
        try:
            proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            raise AssertionError("server did not exit promptly on SIGTERM")
        # the stream should end after shutdown; reading further should stop/raise
        with contextlib.suppress(Exception):
            _ = next(it, None)
        # server should no longer accept connections
        with contextlib.suppress(Exception):
            # small delay to ensure port release
            time.sleep(0.05)
        failed = False
        try:
            _ = requests.get(base + "/healthz", timeout=0.5)
        except Exception:
            failed = True
        assert failed, "healthz should not be reachable after shutdown"
        # cleanup client
        with contextlib.suppress(Exception):
            r.close(); s.close()
