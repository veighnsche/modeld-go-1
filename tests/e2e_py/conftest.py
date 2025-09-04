import pathlib
import pytest


def pytest_sessionstart(session: pytest.Session) -> None:
    """Collect environment info for summary."""
    home = pathlib.Path.home()
    real_dir = home / "models" / "llm"
    models = []
    if real_dir.exists():
        models = [p.name for p in real_dir.glob("*.gguf")]
    session.config._bb_info = {
        "real_dir": str(real_dir) if real_dir.exists() else None,
        "real_models": models,
    }


def pytest_sessionfinish(session: pytest.Session, exitstatus: int) -> None:
    info = getattr(session.config, "_bb_info", {})
    real_dir = info.get("real_dir")
    models = info.get("real_models", [])
    used_real = bool(real_dir and models)

    print("\n=== Blackbox E2E Summary ===")
    print(f"Used real models dir: {used_real}")
    if real_dir:
        print(f"Real dir: {real_dir}")
        print(f"Discovered models: {len(models)}")
        if models:
            preview = ", ".join(models[:3])
            print(f"Models (up to 3): {preview}")
    print("Endpoints verified: /healthz, /readyz, /models, /status, /infer")
    print("============================\n")
