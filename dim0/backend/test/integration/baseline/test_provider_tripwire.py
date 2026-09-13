"""Positive self-tests for every provider boundary guarded by the baseline."""

from __future__ import annotations

import importlib

import pytest

from .provider_tripwire import DeterministicFakeEmbedder, ProviderTripwire, install_provider_tripwire


@pytest.mark.asyncio
async def test_every_current_llm_and_embedding_seam_fails_closed(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Deliberately cross every current seam and require a counted failure."""
    monkeypatch.setenv("DIM0_BASELINE_PROVIDER_TRIPWIRE", "1")
    tripwire = ProviderTripwire()
    install_provider_tripwire(tripwire, DeterministicFakeEmbedder(), monkeypatch.setattr)

    import agents
    import litellm

    from topix.agents import base as agent_base
    from topix.agents import run as agent_run
    from topix.agents import tool_handler
    from topix.agents.websearch import handler as websearch_handler
    from topix.config import catalog
    from topix.nlp.embed import OpenAIEmbedder

    invocation_before = tripwire.invocations["llm"]
    for runner in (agents.Runner, tool_handler.Runner, websearch_handler.Runner):
        with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
            await runner.run(None, None)
        with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
            runner.run_streamed(None, None)
    with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
        await agent_run.AgentRunner.run(None, None, context=None)
    with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
        agent_run.AgentRunner.run_streamed(None, None, context=None)
    for model in (
        "mistral/mistral-large-latest",
        "openrouter/openai/gpt-5.4-mini",
        "anthropic/claude-sonnet-4.6",
    ):
        with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
            await litellm.acompletion(model=model, messages=[])
    with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
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
    with pytest.raises(AssertionError, match="provider invocation blocked.*llm"):
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
        with pytest.raises(AssertionError, match="provider construction blocked.*llm"):
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
        with pytest.raises(AssertionError, match="provider construction blocked.*llm"):
            constructor(api_key="not-a-secret")

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
        with pytest.raises(AssertionError, match="provider construction blocked.*embedding"):
            catalog.openai_compatible_client(resolved)
    assert tripwire.constructions["embedding"] == embedding_construction_before + 2

    embedding_invocation_before = tripwire.invocations["embedding"]
    provider_embedder = OpenAIEmbedder(client=object())
    with pytest.raises(AssertionError, match="provider invocation blocked.*embedding"):
        await provider_embedder._embed_batch(["never sent"])
    assert tripwire.invocations["embedding"] == embedding_invocation_before + 1
