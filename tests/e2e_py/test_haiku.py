import contextlib
import os
import pathlib
import shutil
import socket
import subprocess
import sys
import threading
import time
from typing import Optional

import requests

ROOT = pathlib.Path(__file__).resolve().parents[2]


def _find_free_port() -> int:
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.bind(("127.0.0.1", 0))
    try:
        return s.getsockname()[1]
    finally:
        s.close()


def _reader_thread(stream, name: str):
    for line in iter(stream.readline, ""):
        if not line:
            break
        s = line if isinstance(line, str) else line.decode("utf-8", "replace")
        try:
            sys.stdout.write(s)
        except Exception:
            pass


def _build_binary() -> pathlib.Path:
    out_dir = pathlib.Path(subprocess.check_output(["mktemp", "-d", "-t", "modeld-bin-XXXXXX"]).decode().strip())
    bin_path = out_dir / "modeld"
    env = os.environ.copy()
    env.setdefault("CGO_ENABLED", "0")
    proc = subprocess.run([
        "go", "build", "-o", str(bin_path), "./cmd/modeld"
    ], cwd=str(ROOT), env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    if proc.returncode != 0:
        raise AssertionError(f"go build failed:\n{proc.stdout}")
    return bin_path


def _discover_user_models_dir() -> Optional[pathlib.Path]:
    p = pathlib.Path.home() / "models" / "llm"
    if p.exists():
        return p
    return None


def _first_model_name(models_dir: pathlib.Path) -> Optional[str]:
    ggufs = sorted([x.name for x in models_dir.glob("*.gguf")])
    return ggufs[0] if ggufs else None


def _find_llama_bin() -> Optional[str]:
    # Priority: explicit env
    env_bin = os.environ.get("LLAMA_BIN")
    if env_bin and pathlib.Path(env_bin).exists():
        return env_bin
    # Common names on PATH
    for name in ["llama-server", "llama", "main", "server"]:
        p = shutil.which(name)
        if p:
            return p
    # Common build locations (adjust as needed per host)
    candidates = [
        pathlib.Path.home() / "llama.cpp" / "llama-server",
        pathlib.Path.home() / "llama.cpp" / "build" / "bin" / "llama-server",
        pathlib.Path("/usr/local/bin/llama-server"),
        pathlib.Path("/opt/homebrew/bin/llama-server"),
    ]
    for c in candidates:
        if c.exists():
            return str(c)
    return None


def test_haiku_real_infer_logs_poem():
    models_dir = _discover_user_models_dir()
    if not models_dir:
        import pytest
        pytest.skip("~/models/llm not found; real infer haiku test skipped")
    default_model = _first_model_name(models_dir)
    if not default_model:
        import pytest
        pytest.skip("~/models/llm has no *.gguf; haiku test skipped")
    llama_bin = _find_llama_bin()
    if not llama_bin:
        import pytest
        pytest.skip("llama-server binary not found; set LLAMA_BIN or install")

    bin_path = _build_binary()
    port = _find_free_port()
    addr = f":{port}"

    args = [
        str(bin_path), "--addr", addr, "--models-dir", str(models_dir),
        "--default-model", default_model,
        "--real-infer", "--llama-bin", llama_bin,
    ]

    env = os.environ.copy()
    proc = subprocess.Popen(
        args,
        cwd=str(ROOT), env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        text=True, bufsize=1,
    )
    t_out = threading.Thread(target=_reader_thread, args=(proc.stdout, "OUT"), daemon=True)
    t_err = threading.Thread(target=_reader_thread, args=(proc.stderr, "ERR"), daemon=True)
    t_out.start(); t_err.start()

    base = f"http://127.0.0.1:{port}"
    deadline = time.time() + 20.0
    try:
        # health
        ok = False
        while time.time() < deadline:
            try:
                r = requests.get(base + "/healthz", timeout=1.0)
                if r.status_code == 200:
                    ok = True
                    break
            except Exception:
                pass
            time.sleep(0.05)
        assert ok, "server did not become healthy in time"

        # request haiku
        prompt = "Write a 3-line haiku about the ocean."
        r = requests.post(base + "/infer", json={
            "prompt": prompt,
            "max_tokens": 128,
            "temperature": 0.7,
            "top_p": 0.95,
            "stream": True,
        }, timeout=20.0)
        assert r.status_code == 200
        lines = [ln for ln in r.text.splitlines() if ln.strip()]
        # Extract tokens and/or final content
        tokens: list[str] = []
        content_final: Optional[str] = None
        import json as _json
        for ln in lines:
            try:
                obj = _json.loads(ln)
            except Exception:
                continue
            if isinstance(obj, dict):
                if "token" in obj and isinstance(obj["token"], str):
                    tokens.append(obj["token"])
                if obj.get("done") is True:
                    if isinstance(obj.get("content"), str):
                        content_final = obj["content"]
        content = content_final or "".join(tokens)
        content = content.strip()
        # Print ONLY the haiku (no extra prose)
        print(content)
        assert content, "expected non-empty haiku content"
    finally:
        with contextlib.suppress(Exception):
            proc.terminate()
        with contextlib.suppress(Exception):
            proc.wait(timeout=3)
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        with contextlib.suppress(Exception):
            shutil.rmtree(str(bin_path.parent))
