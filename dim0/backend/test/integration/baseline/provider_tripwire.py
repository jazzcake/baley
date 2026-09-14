"""Fail-closed provider tripwire and deterministic baseline embedder.

This module is test-only.  It replaces provider boundaries before the FastAPI
lifespan is entered, while leaving PostgreSQL, Qdrant, and Redis clients live.
"""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import importlib
import json
import os

from collections.abc import Callable
from typing import Any

COUNTER_KEYS = ("llm", "embedding", "search", "fetch", "ocr", "image", "daytona")


class ProviderTripwire:
    """Record provider construction/invocation and fail every real call."""

    def __init__(self) -> None:
        """Initialize zeroed construction and invocation counters."""
        self.constructions = dict.fromkeys(COUNTER_KEYS, 0)
        self.invocations = dict.fromkeys(COUNTER_KEYS, 0)

    def construction(self, boundary: str) -> None:
        """Record and reject construction of a network-capable provider client."""
        self.constructions[boundary] += 1
        self.write_configured()
        raise AssertionError(f"provider construction blocked by baseline tripwire: {boundary}")

    def invocation(self, boundary: str) -> None:
        """Record and reject an outbound provider invocation."""
        self.invocations[boundary] += 1
        self.write_configured()
        raise AssertionError(f"provider invocation blocked by baseline tripwire: {boundary}")

    def assert_clear(self) -> None:
        """Assert the mandatory provider invocation schema remains all-zero."""
        assert self.invocations == dict.fromkeys(COUNTER_KEYS, 0)

    def write(self, path: str) -> None:
        """Write the fixed-schema invocation counters as JSON."""
        with open(path, "w", encoding="utf-8") as output:
            json.dump(self.invocations, output, indent=2, sort_keys=True)
            output.write("\n")

    def write_constructions(self, path: str) -> None:
        """Write separately recorded provider-client construction counters."""
        with open(path, "w", encoding="utf-8") as output:
            json.dump(self.constructions, output, indent=2, sort_keys=True)
            output.write("\n")

    def write_configured(self) -> None:
        """Persist counters when the Compose harness configured output paths."""
        if path := os.getenv("DIM0_BASELINE_TRIPWIRE_OUTPUT"):
            self.write(path)
        if path := os.getenv("DIM0_BASELINE_CONSTRUCTION_OUTPUT"):
            self.write_constructions(path)


class DeterministicFakeEmbedder:
    """Return stable 512-dimensional vectors without constructing a provider."""

    dimensions = 512

    def __init__(self) -> None:
        """Initialize call history and the opt-in failure switch."""
        self.calls: list[list[str]] = []
        self.fail = False

    async def embed(self, texts: list[str], batch_size: int = 1000) -> list[list[float]]:
        """Embed text deterministically, or raise before storage when requested."""
        del batch_size
        self.calls.append(list(texts))
        if self.fail:
            raise RuntimeError("forced deterministic embedding failure")
        vectors = []
        for text in texts:
            digest = hashlib.sha256(text.encode("utf-8")).digest()
            vectors.append([((digest[index % len(digest)] / 255.0) * 2.0) - 1.0 for index in range(self.dimensions)])
        return vectors


class _ProviderFreeParser:
    """Stand in for Mistral OCR without constructing its network client."""

    async def parse(self, filepath: str, max_pages: int = 200) -> list[dict[str, int | str]]:
        del filepath, max_pages
        raise AssertionError("OCR is outside the provider-free baseline")


class _ProviderFreeNewsfeedPipeline:
    """Stand in for provider-backed newsfeed agents during application startup."""

    def __getattr__(self, name: str) -> Any:
        raise AssertionError(f"newsfeed provider path is outside the baseline: {name}")


def install_provider_tripwire(  # noqa: C901
    tripwire: ProviderTripwire,
    embedder: DeterministicFakeEmbedder,
    set_attribute: Callable[[Any, str, Any], None] = setattr,
) -> None:
    """Install test doubles at existing provider seams before app lifespan."""
    if os.getenv("DIM0_BASELINE_PROVIDER_TRIPWIRE") != "1":
        raise RuntimeError("DIM0_BASELINE_PROVIDER_TRIPWIRE=1 is required")

    # LiteLLM otherwise fetches its price map during import, before its callable
    # boundary can be replaced. Force its bundled map before importing it.
    os.environ["LITELLM_LOCAL_MODEL_COST_MAP"] = "True"

    import agents
    import litellm
    import openai

    from topix.agents import base as agent_base
    from topix.agents import run as agent_run
    from topix.agents import tool_handler, websearch
    from topix.agents.assistant import auto_model
    from topix.agents.assistant import code as daytona_code
    from topix.agents.image import gen as image_gen
    from topix.agents.newsfeed import config as newsfeed_config
    from topix.agents.websearch import fetch as web_fetch
    from topix.agents.websearch import handler as websearch_handler
    from topix.agents.websearch import tools as web_tools
    from topix.api.router import ai as ai_router
    from topix.config import config as config_module
    from topix.nlp import embed as embed_module
    from topix.nlp import parser as parser_module
    from topix.nlp.pipeline import parsing
    from topix.store import subscription
    from topix.store.qdrant import store as qdrant_store

    qdrant_client_type = qdrant_store.AsyncQdrantClient

    async def forbidden_llm_async(*args: Any, **kwargs: Any) -> Any:
        """Count and reject an asynchronous LLM invocation."""
        del args, kwargs
        tripwire.invocation("llm")

    def forbidden_llm_sync(*args: Any, **kwargs: Any) -> Any:
        """Count and reject a synchronous LLM invocation."""
        del args, kwargs
        tripwire.invocation("llm")

    def forbidden_llm_construction(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.construction("llm")

    async def forbidden_embedding(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.invocation("embedding")

    async def forbidden_search(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.invocation("search")

    async def forbidden_fetch(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.invocation("fetch")

    async def forbidden_image(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.invocation("image")

    async def forbidden_daytona(*args: Any, **kwargs: Any) -> Any:
        del args, kwargs
        tripwire.invocation("daytona")

    def forbidden_embedding_construction(*args: Any, **kwargs: Any) -> Any:
        """Count and reject an embedding-client construction."""
        del args, kwargs
        tripwire.construction("embedding")

    def forbidden_llm_client_construction(*args: Any, **kwargs: Any) -> Any:
        """Count and reject an SDK LLM-client construction."""
        del args, kwargs
        tripwire.construction("llm")

    def baseline_qdrant_client(*args: Any, **kwargs: Any) -> Any:
        """Give live baseline index creation enough time on cold storage."""
        kwargs.setdefault("timeout", 60)
        return qdrant_client_type(*args, **kwargs)

    set_attribute(openai, "AsyncOpenAI", forbidden_embedding_construction)
    # The baseline carries its complete non-secret configuration in /.env.
    # Do not let the test-profile startup consult Doppler before the provider
    # tripwires can observe the application. This is deliberately test-only
    # and leaves every LLM/embedding tripwire intact.
    set_attribute(config_module, "load_secrets", lambda *_args, **_kwargs: "{}")
    set_attribute(qdrant_store, "AsyncQdrantClient", baseline_qdrant_client)
    set_attribute(embed_module, "AsyncOpenAI", forbidden_embedding_construction)
    set_attribute(embed_module.OpenAIEmbedder, "from_config", classmethod(lambda cls: embedder))
    set_attribute(embed_module.OpenAIEmbedder, "_embed_batch", forbidden_embedding)
    set_attribute(qdrant_store.OpenAIEmbedder, "from_config", classmethod(lambda cls: embedder))

    # Cover both LiteLLM entry points and every application module that keeps a
    # module alias. The explicit aliases make refactors visible to the positive
    # self-test instead of relying on shared-module identity by accident.
    set_attribute(litellm, "acompletion", forbidden_llm_async)
    set_attribute(litellm, "completion", forbidden_llm_sync)
    set_attribute(ai_router.litellm, "acompletion", forbidden_llm_async)
    set_attribute(auto_model.litellm, "acompletion", forbidden_llm_async)
    set_attribute(agent_base.LitellmModel, "__init__", forbidden_llm_construction)
    set_attribute(agent_run.AgentRunner, "run", classmethod(forbidden_llm_async))
    set_attribute(agent_run.AgentRunner, "run_streamed", classmethod(forbidden_llm_sync))

    # The SDK Runner is used directly as well as through aliases imported into
    # tool_handler and websearch.handler. run_streamed is synchronous at the
    # call boundary, so it must raise immediately even when a caller forgets to
    # await it.
    for runner in (agents.Runner, tool_handler.Runner, websearch_handler.Runner):
        set_attribute(runner, "run", classmethod(forbidden_llm_async))
        set_attribute(runner, "run_streamed", classmethod(forbidden_llm_sync))

    # Native OpenAI models are built lazily inside the Agents SDK and therefore
    # bypass LitellmModel.__init__. Patch the SDK's retained AsyncOpenAI aliases
    # as well as the public module so neither construction route can escape.
    for module_name in (
        "agents.models.openai_provider",
        "agents.models.openai_responses",
        "agents.models.openai_chatcompletions",
    ):
        try:
            sdk_module = importlib.import_module(module_name)
        except ImportError:
            continue
        if hasattr(sdk_module, "AsyncOpenAI"):
            set_attribute(sdk_module, "AsyncOpenAI", forbidden_llm_client_construction)

    set_attribute(parser_module.MistralParser, "from_config", classmethod(lambda cls: _ProviderFreeParser()))
    set_attribute(parsing, "DocumentMindmapAgent", lambda: object())
    set_attribute(newsfeed_config.NewsfeedPipelineConfig, "from_yaml", classmethod(lambda cls, *args, **kwargs: object()))
    set_attribute(subscription.NewsfeedPipeline, "from_config", classmethod(lambda cls, *args, **kwargs: _ProviderFreeNewsfeedPipeline()))

    for name in ("search_perplexity", "search_tavily", "search_linkup", "search_exa"):
        set_attribute(web_tools, name, forbidden_search)
    set_attribute(web_tools, "fetch_content", forbidden_fetch)
    set_attribute(web_fetch, "fetch_url", forbidden_fetch)
    set_attribute(image_gen, "generate_image", forbidden_image)
    for name in ("execute_code", "execute_python_code", "run_code"):
        set_attribute(daytona_code, name, forbidden_daytona)

    # Keep the imported package referenced so import-time wiring cannot be
    # optimized away by a refactor without this harness noticing at collection.
    assert websearch is not None


async def _serve() -> None:
    """Run the test-profile backend with provider seams replaced."""
    import uvicorn

    from topix.api.app import create_app
    from topix.config.config import Config
    from topix.datatypes.stage import StageEnum
    from topix.setup import setup

    tripwire = ProviderTripwire()
    embedder = DeterministicFakeEmbedder()
    install_provider_tripwire(tripwire, embedder)
    tripwire.write_configured()
    try:
        await setup(StageEnum.TEST, env_filename="/.env")
        app = create_app(StageEnum.TEST)
        port = Config.instance().app.settings.port
        await uvicorn.Server(uvicorn.Config(app, host="0.0.0.0", port=port)).serve()
    finally:
        tripwire.write_configured()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Run the provider-free baseline backend")
    parser.parse_args()
    asyncio.run(_serve())
