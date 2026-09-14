"""Live PostgreSQL/Qdrant/Redis provider-free baseline contract."""

from __future__ import annotations

import asyncio
import os
import time

import asyncpg
import pytest

from topix.config.config import Config
from topix.datatypes.graph.graph import Graph
from topix.datatypes.note.link import Link
from topix.datatypes.note.note import Note
from topix.datatypes.resource import RichText
from topix.datatypes.stage import StageEnum
from topix.store.postgres.schema import apply_schema

from .provider_tripwire import DeterministicFakeEmbedder, ProviderTripwire, install_provider_tripwire

BOARD_ID = "18900000-0000-4000-8000-000000000001"
NOTE_A_ID = "18900000-0000-4000-8000-000000000002"
NOTE_B_ID = "18900000-0000-4000-8000-000000000003"
LINK_ID = "18900000-0000-4000-8000-000000000004"


def _configure() -> Config:
    """Build test configuration only from explicit non-secret environment."""
    Config.teardown()
    config = Config(stage=StageEnum.TEST)
    config.run.databases.qdrant.collection = "task189_baseline"
    return config


async def _wait_for_postgres(config: Config, timeout_seconds: float = 30.0) -> None:
    """Wait through the bounded startup window after a Compose restart."""
    deadline = time.monotonic() + timeout_seconds
    while True:
        try:
            connection = await asyncpg.connect(config.run.databases.postgres.dsn())
            await connection.close()
            return
        except (OSError, asyncpg.CannotConnectNowError, asyncpg.PostgresConnectionError):
            if time.monotonic() >= deadline:
                raise
            await asyncio.sleep(0.5)


@pytest.mark.asyncio
async def test_provider_free_board_content_crud_and_persistence(monkeypatch: pytest.MonkeyPatch) -> None:
    """Exercise canonical stores while every provider boundary is fail-closed."""
    monkeypatch.setenv("DIM0_BASELINE_PROVIDER_TRIPWIRE", "1")
    monkeypatch.setenv("DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION", "512")
    config = _configure()
    assert config.run.databases.postgres.hostname == os.environ["POSTGRES_HOST"]
    await _wait_for_postgres(config)

    tripwire = ProviderTripwire()
    embedder = DeterministicFakeEmbedder()
    install_provider_tripwire(tripwire, embedder, monkeypatch.setattr)

    from topix.api.app import create_app

    app = create_app(StageEnum.TEST)
    try:
        async with app.router.lifespan_context(app):
            await apply_schema(app.pg_pool)
            await apply_schema(app.pg_pool)
            content_store = app.graph_store._content_store
            await content_store.create_collection(force_recreate=True, quantized=False)

            # Task 189 deliberately preserves its named persistence volumes.
            # Remove only this harness-owned fixed record so a fresh evidence
            # run can seed the same browser-observable board deterministically.
            if await app.graph_store.get_graph(BOARD_ID) is not None:
                await app.graph_store.delete_graph(BOARD_ID, hard_delete=True)

            graph = Graph(uid=BOARD_ID, label="Task 189 baseline")
            note_a = Note(id=NOTE_A_ID, graph_uid=BOARD_ID, label=RichText(markdown="Alpha"), content=RichText(markdown="Original text"))
            note_b = Note(id=NOTE_B_ID, graph_uid=BOARD_ID, label=RichText(markdown="Beta"), content=RichText(markdown="Second note"))
            link = Link(id=LINK_ID, graph_uid=BOARD_ID, source=NOTE_A_ID, target=NOTE_B_ID, content=RichText(markdown="connects"))

            await app.graph_store.add_graph(graph, user_uid="root")
            await app.graph_store.add_notes([note_a, note_b])
            await app.graph_store.add_links([link])
            stored = await app.graph_store.get_graph(BOARD_ID)
            assert stored is not None
            assert {node.id for node in stored.nodes} == {NOTE_A_ID, NOTE_B_ID}
            assert {edge.id for edge in stored.edges} == {LINK_ID}

            collection = await content_store.client.get_collection(content_store.collection)
            assert collection.config.params.vectors.size == 512

            calls_before_text = len(embedder.calls)
            await app.graph_store.patch_note(NOTE_A_ID, {"content": {"markdown": "Updated text"}})
            assert len(embedder.calls) == calls_before_text + 1

            calls_before_spatial = len(embedder.calls)
            before_spatial = (await content_store.get([NOTE_A_ID], with_vector=True))[0]
            vector_operations: list[str] = []
            original_update_vectors = content_store.client.update_vectors
            original_upsert = content_store.client.upsert
            original_batch_update = content_store.client.batch_update_points

            async def tracked_update_vectors(*args, **kwargs):
                """Record a direct Qdrant vector update before delegating."""
                vector_operations.append("update_vectors")
                return await original_update_vectors(*args, **kwargs)

            async def tracked_upsert(*args, **kwargs):
                """Record a Qdrant upsert before delegating."""
                vector_operations.append("upsert")
                return await original_upsert(*args, **kwargs)

            async def tracked_batch_update(*args, **kwargs):
                """Record vector-bearing Qdrant batch operations before delegating."""
                operations = kwargs.get("update_operations")
                if operations is None and len(args) > 1:
                    operations = args[1]
                for operation in operations or ():
                    operation_name = type(operation).__name__.lower()
                    if "vector" in operation_name or "upsert" in operation_name:
                        vector_operations.append(type(operation).__name__)
                return await original_batch_update(*args, **kwargs)

            monkeypatch.setattr(content_store.client, "update_vectors", tracked_update_vectors)
            monkeypatch.setattr(content_store.client, "upsert", tracked_upsert)
            monkeypatch.setattr(content_store.client, "batch_update_points", tracked_batch_update)
            await app.graph_store.patch_note(
                NOTE_A_ID,
                {"properties": {"node_position": {"type": "position", "position": {"x": 189, "y": 512}}}},
            )
            after_spatial = (await content_store.get([NOTE_A_ID], with_vector=True))[0]
            assert len(embedder.calls) == calls_before_spatial
            assert vector_operations == []
            assert after_spatial.vector == before_spatial.vector

            before_failure = (await content_store.get([NOTE_A_ID], with_vector=True))[0]
            embedder.fail = True
            with pytest.raises(RuntimeError, match="forced deterministic embedding failure"):
                await app.graph_store.patch_note(NOTE_A_ID, {"content": {"markdown": "must not persist"}})
            embedder.fail = False
            after_failure = (await content_store.get([NOTE_A_ID], with_vector=True))[0]
            assert after_failure.resource.content.markdown == before_failure.resource.content.markdown
            assert after_failure.vector == before_failure.vector

            seq_one = await app.collab_oplog.next_seq(BOARD_ID)
            seq_two = await app.collab_oplog.next_seq(BOARD_ID)
            assert seq_two == seq_one + 1
            tripwire.assert_clear()
    finally:
        Config.teardown()


@pytest.mark.asyncio
async def test_persisted_after_restart(monkeypatch: pytest.MonkeyPatch) -> None:
    """Read the exact records seeded by the pre-restart baseline run."""
    monkeypatch.setenv("DIM0_BASELINE_PROVIDER_TRIPWIRE", "1")
    config = _configure()
    await _wait_for_postgres(config)
    tripwire = ProviderTripwire()
    embedder = DeterministicFakeEmbedder()
    install_provider_tripwire(tripwire, embedder, monkeypatch.setattr)

    from topix.api.app import create_app

    app = create_app(StageEnum.TEST)
    try:
        async with app.router.lifespan_context(app):
            stored = await app.graph_store.get_graph(BOARD_ID)
            assert stored is not None
            assert {node.id for node in stored.nodes} == {NOTE_A_ID, NOTE_B_ID}
            assert {edge.id for edge in stored.edges} == {LINK_ID}
            point = (await app.graph_store._content_store.get([NOTE_A_ID], with_vector=True))[0]
            assert point.resource.content.markdown == "Updated text"
            assert point.vector and all(len(vector) == 512 for vector in point.vector)
            assert await app.collab_oplog.next_seq(BOARD_ID) >= 3
            tripwire.assert_clear()
    finally:
        Config.teardown()
