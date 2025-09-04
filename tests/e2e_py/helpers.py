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
        s = line if isinstance(line, str) else line.decode('utf-8', 'replace')
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
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        with contextlib.suppress(Exception):
            shutil.rmtree(bin_path.parent)


@contextlib.contextmanager
def start_server_with_handle(models_dir: pathlib.Path, default_model: str | None = None):
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
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        with contextlib.suppress(Exception):
            shutil.rmtree(bin_path.parent)


@contextlib.contextmanager
def start_server_with_config(models_dir: pathlib.Path, default_model: str | None = None, fmt: str = "yaml"):
    bin_path = build_binary()
    port = find_free_port()
    addr = f":{port}"
    cfg_dir = pathlib.Path(tempfile.mkdtemp(prefix="modeld-cfg-"))
    cfg_path = cfg_dir / f"models.{fmt}"

    cfg = {
        "addr": addr,
        "models_dir": str(models_dir),
    }
    if default_model:
        cfg["default_model"] = default_model

    fmt_l = fmt.lower()
    if fmt_l == "json":
        cfg_path.write_text(json.dumps(cfg))
    elif fmt_l in ("yaml", "yml"):
        lines = [
            f"addr: \"{cfg['addr']}\"",
            f"models_dir: \"{cfg['models_dir']}\"",
        ]
        if "default_model" in cfg:
            lines.append(f"default_model: \"{cfg['default_model']}\"")
        cfg_path.write_text("\n".join(lines) + "\n")
    elif fmt_l == "toml":
        lines = [
            f"addr = \"{cfg['addr']}\"",
            f"models_dir = \"{cfg['models_dir']}\"",
        ]
        if "default_model" in cfg:
            lines.append(f"default_model = \"{cfg['default_model']}\"")
        cfg_path.write_text("\n".join(lines) + "\n")
    else:
        raise AssertionError(f"unsupported fmt: {fmt}")

    args = [str(bin_path), "--config", str(cfg_path)]
    env = os.environ.copy()
    proc = subprocess.Popen(args, cwd=str(ROOT), env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    t_out = threading.Thread(target=_reader_thread, args=(proc.stdout, 'OUT'), daemon=True)
    t_err = threading.Thread(target=_reader_thread, args=(proc.stderr, 'ERR'), daemon=True)
    t_out.start(); t_err.start()
    base = f"http://127.0.0.1:{port}"
    deadline = time.time() + 5
    try:
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
        with contextlib.suppress(Exception):
            if proc.stdout:
                proc.stdout.close()
            if proc.stderr:
                proc.stderr.close()
            t_out.join(timeout=1)
            t_err.join(timeout=1)
        with contextlib.suppress(Exception):
            shutil.rmtree(bin_path.parent)
        with contextlib.suppress(Exception):
            shutil.rmtree(cfg_dir)


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


def _split_ndjson(text: str) -> list[str]:
    return [line for line in text.splitlines() if line.strip()]
