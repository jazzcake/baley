"""Positive self-tests for every provider boundary guarded by the baseline."""

from __future__ import annotations

import contextlib
import importlib
import json
import logging
import os
import socket
import subprocess
import sys

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from fastapi import FastAPI, Response
from fastapi.testclient import TestClient

from topix.api.utils import decorators as response_decorators

from .provider_tripwire import (
    DeterministicFakeEmbedder,
    ExpectedProviderTripwireBlockError,
    ProviderTripwire,
    install_provider_tripwire,
)


class _ExpectedTripwireEndpointLogFilter(logging.Filter):
    """Keep expected endpoint-block logs while removing only their tracebacks."""

    def filter(self, record: logging.LogRecord) -> bool:
        """Replace exactly the test-only block subtype with a boundary diagnostic."""
        if isinstance(record.exc_info, tuple):
            exception = record.exc_info[1]
            if type(exception) is ExpectedProviderTripwireBlockError:
                record.msg = "Expected baseline provider block: %s"
                record.args = (exception.boundary,)
                record.exc_info = None
                record.exc_text = None
                record.stack_info = None
        return True


@contextlib.contextmanager
def _expected_tripwire_endpoint_log_scope() -> Iterator[_ExpectedTripwireEndpointLogFilter]:
    """Install the positive-endpoint filter for one bounded test scope."""
    log_filter = _ExpectedTripwireEndpointLogFilter()
    response_decorators.logger.addFilter(log_filter)
    try:
        yield log_filter
    finally:
        response_decorators.logger.removeFilter(log_filter)


@pytest.mark.asyncio
async def test_every_current_provider_boundary_fails_closed_before_network(  # noqa: C901
    monkeypatch: pytest.MonkeyPatch,
    caplog: pytest.LogCaptureFixture,
) -> None:
    """Drive all seven live boundaries and require counted fail-closed results."""
    monkeypatch.setenv("DIM0_BASELINE_PROVIDER_TRIPWIRE", "1")
    tripwire = ProviderTripwire()
    install_provider_tripwire(
        tripwire,
        DeterministicFakeEmbedder(),
        monkeypatch.setattr,
        monkeypatch.setitem,
    )

    import agents
    import litellm

    from topix.agents import base as agent_base
    from topix.agents import run as agent_run
    from topix.agents import tool_handler
    from topix.agents.assistant import code as daytona_code
    from topix.agents.image import gen as image_gen
    from topix.agents.websearch import fetch as web_fetch
    from topix.agents.websearch import handler as websearch_handler
    from topix.api.router import ai as ai_router
    from topix.api.router import boards as boards_router
    from topix.api.utils.security import get_current_user_uid
    from topix.config import catalog
    from topix.config import config as config_module
    from topix.datatypes.stage import StageEnum
    from topix.nlp.embed import OpenAIEmbedder

    assert config_module.load_secrets(StageEnum.TEST) == "{}"
    assert tripwire.invocations == dict.fromkeys(tripwire.invocations, 0)
    assert tripwire.constructions == dict.fromkeys(tripwire.constructions, 0)

    invocation_before = tripwire.invocations["llm"]
    for runner in (agents.Runner, tool_handler.Runner, websearch_handler.Runner):
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
            await runner.run(None, None)
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
            runner.run_streamed(None, None)
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
        await agent_run.AgentRunner.run(None, None, context=None)
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
        agent_run.AgentRunner.run_streamed(None, None, context=None)
    for model in (
        "mistral/mistral-large-latest",
        "openrouter/openai/gpt-5.4-mini",
        "anthropic/claude-sonnet-4.6",
    ):
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
            await litellm.acompletion(model=model, messages=[])
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
        litellm.completion(model="never-called", messages=[])
    assert tripwire.invocations["llm"] == invocation_before + 12

    # Exercise BaseAgent's native OpenAI branch: construction retains the SDK
    # model string, and the first Runner boundary is still blocked and counted.
    monkeypatch.setattr(catalog, "normalize_code", lambda model: model)
    native = agent_base.BaseAgent.__new__(agent_base.BaseAgent)
    native.name = "tripwire-native-openai"
    native.model = "openai/gpt-5.4-mini"
    native.model_settings = None
    agent_base.BaseAgent.__post_init__(native)
    assert native.model == "openai/gpt-5.4-mini"
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
        await agents.Runner.run(native, "never sent")

    # LiteLLM, OpenRouter, and native Anthropic routes all construct through
    # LitellmModel in BaseAgent and must fail at construction.
    construction_before = tripwire.constructions["llm"]
    for model in (
        "mistral/mistral-large-latest",
        "openrouter/openai/gpt-5.4-mini",
        "anthropic/claude-sonnet-4.6",
    ):
        candidate = agent_base.BaseAgent.__new__(agent_base.BaseAgent)
        candidate.name = f"tripwire-{model}"
        candidate.model = model
        candidate.model_settings = None
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
            agent_base.BaseAgent.__post_init__(candidate)
    assert tripwire.constructions["llm"] == construction_before + 3

    # The SDK retains private AsyncOpenAI aliases in version-specific modules;
    # positively call every alias present in the locked SDK.
    sdk_aliases = []
    for module_name in (
        "agents.models.openai_provider",
        "agents.models.openai_responses",
        "agents.models.openai_chatcompletions",
    ):
        try:
            module = importlib.import_module(module_name)
        except ImportError:
            continue
        if hasattr(module, "AsyncOpenAI"):
            sdk_aliases.append(module.AsyncOpenAI)
    assert sdk_aliases, "locked OpenAI Agents SDK exposes no guarded client construction alias"
    for constructor in sdk_aliases:
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: llm"):
            constructor()

    embedding_construction_before = tripwire.constructions["embedding"]
    for provider, model in (
        ("openai", "text-embedding-3-small"),
        ("openrouter", "openai/text-embedding-3-small"),
    ):
        resolved = catalog.Resolved(
            id="text-embedding-3-small",
            label="tripwire",
            family="openai",
            dim=512,
            provider=provider,
            model=model,
            call=model,
        )
        with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: embedding"):
            catalog.openai_compatible_client(resolved)
    assert tripwire.constructions["embedding"] == embedding_construction_before + 2

    embedding_invocation_before = tripwire.invocations["embedding"]
    provider_embedder = OpenAIEmbedder(client=object())
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: embedding"):
        await provider_embedder._embed_batch(["never sent"])
    assert tripwire.invocations["embedding"] == embedding_invocation_before + 1

    # Any accidental escape past a patched boundary would hit this low-level
    # sentinel. The Docker acceptance command also runs this file with
    # --network none, so the proof has both process- and namespace-level guards.
    network_attempts: list[tuple[Any, ...]] = []

    def disabled_connect(_socket: socket.socket, *args: Any, **kwargs: Any) -> None:
        network_attempts.append(args)
        del kwargs
        raise AssertionError("network I/O reached before provider tripwire")

    monkeypatch.setattr(socket.socket, "connect", disabled_connect)

    app = FastAPI()
    app.include_router(ai_router.router)

    async def no_meter() -> None:
        return None

    async def user_id() -> str:
        return "tripwire-user"

    app.dependency_overrides[ai_router.meter_run] = no_meter
    app.dependency_overrides[ai_router.meter_run_managed] = no_meter
    app.dependency_overrides[get_current_user_uid] = user_id

    caplog.clear()
    with caplog.at_level(logging.ERROR, logger=response_decorators.logger.name):
        with _expected_tripwire_endpoint_log_scope():
            with TestClient(app) as client:
                search_before = tripwire.invocations["search"]
                for engine in ai_router._SEARCH_FNS:
                    response = client.post("/ai/search", json={"query": "never sent", "engine": engine})
                    assert response.status_code == 500
                assert tripwire.invocations["search"] == search_before + len(ai_router._SEARCH_FNS)

                fetch_before = tripwire.invocations["fetch"]
                response = client.post("/ai/fetch", json={"url": "https://provider.invalid/never"})
                assert response.status_code == 500
                assert tripwire.invocations["fetch"] == fetch_before + 1

                daytona_before = tripwire.invocations["daytona"]
                response = client.post("/ai/code", json={"code": "print('never run')"})
                assert response.status_code == 500
                assert tripwire.invocations["daytona"] == daytona_before + 1

                ocr_before = tripwire.invocations["ocr"]
                construction_before = tripwire.constructions["ocr"]
                byok_headers = {"X-Provider-Key": "route-marker"}
                for headers in ({}, byok_headers):
                    response = client.post(
                        "/ai/parse",
                        files={"file": ("probe.pdf", b"%PDF-1.4 probe", "application/pdf")},
                        headers=headers,
                    )
                    assert response.status_code == 500
                assert tripwire.constructions["ocr"] == construction_before + 2
                assert tripwire.invocations["ocr"] == ocr_before + 2

    expected_endpoint_logs = [
        record for record in caplog.records
        if record.name == response_decorators.logger.name
    ]
    expected_boundaries = [record.getMessage().removeprefix("Expected baseline provider block: ") for record in expected_endpoint_logs]
    assert expected_boundaries.count("search") == len(ai_router._SEARCH_FNS)
    assert expected_boundaries.count("fetch") == 1
    assert expected_boundaries.count("daytona") == 1
    assert expected_boundaries.count("ocr") == 2
    assert all(record.exc_info is None and record.exc_text is None and record.stack_info is None for record in expected_endpoint_logs)

    # Captured module aliases and prebuilt FunctionTool objects are runtime
    # call sites too. Positively invoke them instead of checking source names.
    for boundary, candidate, args in (
        ("fetch", web_fetch.fetch_url_content_tool.on_invoke_tool, (None, "{}")),
        ("image", image_gen.generate_image_tool.on_invoke_tool, (None, "{}")),
        ("daytona", daytona_code.run_code_tool.on_invoke_tool, (None, "{}")),
    ):
        before = tripwire.invocations[boundary]
        with pytest.raises(ExpectedProviderTripwireBlockError, match=f"provider tripwire block: {boundary}"):
            await candidate(*args)
        assert tripwire.invocations[boundary] == before + 1

    assert boards_router.execute_code is daytona_code.execute_code
    assert ai_router.execute_code is daytona_code.execute_code
    assert ai_router.fetch_content is web_fetch.fetch_content
    assert all(fn is ai_router.search_perplexity for fn in ai_router._SEARCH_FNS.values())

    # Direct SDK/client constructors are independently guarded where the
    # application has an explicit construction seam.
    daytona_construction_before = tripwire.constructions["daytona"]
    with pytest.raises(ExpectedProviderTripwireBlockError, match="provider tripwire block: daytona"):
        daytona_code.AsyncDaytona()
    assert tripwire.constructions["daytona"] == daytona_construction_before + 1

    assert all(value > 0 for value in tripwire.constructions.values())
    assert all(value > 0 for value in tripwire.invocations.values())
    assert network_attempts == []


@pytest.mark.parametrize("exception_type", [RuntimeError, AssertionError])
def test_expected_endpoint_filter_preserves_unexpected_tracebacks(
    exception_type: type[Exception],
    caplog: pytest.LogCaptureFixture,
) -> None:
    """Generic errors, including unexpected assertions, retain ``exc_info``."""
    app = FastAPI()

    @app.get("/unexpected")
    @response_decorators.with_standard_response
    async def unexpected_endpoint(response: Response) -> None:
        del response
        raise exception_type("neutral endpoint failure")

    caplog.clear()
    with caplog.at_level(logging.ERROR, logger=response_decorators.logger.name):
        with _expected_tripwire_endpoint_log_scope():
            with TestClient(app) as client:
                response = client.get("/unexpected")
                assert response.status_code == 500

    records = [record for record in caplog.records if record.name == response_decorators.logger.name]
    assert len(records) == 1
    assert records[0].exc_info is not None
    assert records[0].exc_info[0] is exception_type
    assert records[0].getMessage() == "Error in unexpected_endpoint: neutral endpoint failure"


def test_expected_endpoint_filter_is_removed_when_scope_fails() -> None:
    """The temporary logger filter is restored even when the test scope fails."""
    original_filters = tuple(response_decorators.logger.filters)
    installed_filter: _ExpectedTripwireEndpointLogFilter | None = None

    with pytest.raises(RuntimeError, match="neutral scope failure"):
        with _expected_tripwire_endpoint_log_scope() as installed_filter:
            assert installed_filter in response_decorators.logger.filters
            raise RuntimeError("neutral scope failure")

    assert installed_filter is not None
    assert installed_filter not in response_decorators.logger.filters
    assert tuple(response_decorators.logger.filters) == original_filters


def _run_counter_process(directory: Path, source: str) -> subprocess.CompletedProcess[str]:
    """Run one isolated counter writer process against shared evidence files."""
    environment = os.environ.copy()
    environment.update({
        "DIM0_BASELINE_TRIPWIRE_OUTPUT": str(directory / "provider-invocations.json"),
        "DIM0_BASELINE_CONSTRUCTION_OUTPUT": str(directory / "provider-constructions.json"),
    })
    backend_root = Path(__file__).resolve().parents[3]
    return subprocess.run(
        [sys.executable, "-c", source],
        cwd=backend_root,
        env=environment,
        check=True,
        capture_output=True,
        text=True,
        timeout=15,
    )


def _read_counters(path: Path) -> dict[str, int]:
    return json.loads(path.read_text(encoding="utf-8"))


def test_restart_aggregation_preserves_nonzero_and_two_process_zero(tmp_path: Path) -> None:
    """A later process cannot erase prior counts; two zero writers stay zero."""
    nonzero = tmp_path / "nonzero"
    nonzero.mkdir()
    _run_counter_process(
        nonzero,
        """\
import contextlib
from test.integration.baseline.provider_tripwire import ProviderTripwire
tripwire = ProviderTripwire()
tripwire.record_construction("llm")
with contextlib.suppress(AssertionError):
    tripwire.invocation("llm")
""",
    )
    _run_counter_process(
        nonzero,
        """\
from test.integration.baseline.provider_tripwire import ProviderTripwire
ProviderTripwire().write_configured()
""",
    )
    assert _read_counters(nonzero / "provider-constructions.json")["llm"] == 1
    assert _read_counters(nonzero / "provider-invocations.json")["llm"] == 1

    all_zero = tmp_path / "all-zero"
    all_zero.mkdir()
    zero_source = """\
from test.integration.baseline.provider_tripwire import ProviderTripwire
ProviderTripwire().write_configured()
"""
    _run_counter_process(all_zero, zero_source)
    _run_counter_process(all_zero, zero_source)
    expected = dict.fromkeys(("llm", "embedding", "search", "fetch", "ocr", "image", "daytona"), 0)
    assert _read_counters(all_zero / "provider-constructions.json") == expected
    assert _read_counters(all_zero / "provider-invocations.json") == expected


def test_concurrent_process_counter_merges_are_lossless(tmp_path: Path) -> None:
    """Concurrent writers serialize through the bounded cross-process lock."""
    shared = tmp_path / "shared"
    shared.mkdir()
    environment = os.environ.copy()
    environment.update({
        "DIM0_BASELINE_TRIPWIRE_OUTPUT": str(shared / "provider-invocations.json"),
        "DIM0_BASELINE_CONSTRUCTION_OUTPUT": str(shared / "provider-constructions.json"),
    })
    backend_root = Path(__file__).resolve().parents[3]
    processes = []
    for boundary in ("search", "fetch"):
        source = f"""\
import contextlib
from test.integration.baseline.provider_tripwire import ProviderTripwire
tripwire = ProviderTripwire()
for _ in range(5):
    tripwire.record_construction("{boundary}")
    with contextlib.suppress(AssertionError):
        tripwire.invocation("{boundary}")
"""
        processes.append(subprocess.Popen(
            [sys.executable, "-c", source],
            cwd=backend_root,
            env=environment,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        ))
    for process in processes:
        stdout, stderr = process.communicate(timeout=15)
        assert process.returncode == 0, (stdout, stderr)

    constructions = _read_counters(shared / "provider-constructions.json")
    invocations = _read_counters(shared / "provider-invocations.json")
    assert (constructions["search"], constructions["fetch"]) == (5, 5)
    assert (invocations["search"], invocations["fetch"]) == (5, 5)


def test_counter_merge_rejects_invalid_existing_schema(tmp_path: Path) -> None:
    """Malformed prior evidence fails closed instead of being reset."""
    output = tmp_path / "provider-invocations.json"
    output.write_text('{"llm": "zero"}', encoding="utf-8")
    with pytest.raises(ValueError, match="invalid provider counter schema"):
        ProviderTripwire().write(str(output))
