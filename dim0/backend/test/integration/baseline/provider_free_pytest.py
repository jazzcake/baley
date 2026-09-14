"""Pytest bootstrap for provider-free, network-isolated baseline unit tests."""

from __future__ import annotations

from typing import Any


def pytest_configure(config: Any) -> None:
    """Replace the Doppler config fetch before application modules are collected."""
    del config
    import topix.config.config as config_module

    config_module.load_secrets = lambda *_args, **_kwargs: "{}"
