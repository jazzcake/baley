"""Unit tests for the Codex message translation."""

import json

from topix.ai_runtime.codex import CodexAppServerRuntime


def test_decode_message_preserves_canvas_tool_call() -> None:
    """Structured tool choices retain the existing OpenAI wire shape."""
    message = CodexAppServerRuntime._decode_message(
        json.dumps({"content": None, "tool_calls": [{"id": "call-1", "name": "create_note", "arguments": {"title": "Goal"}}]})
    )
    assert message["tool_calls"][0]["function"] == {"name": "create_note", "arguments": '{"title": "Goal"}'}


def test_prompt_contains_messages_and_tools() -> None:
    """The prompt supplies both halves of the browser model-turn contract."""
    prompt = CodexAppServerRuntime._prompt([{"role": "user", "content": "map it"}], [{"function": {"name": "create_note"}}])
    assert "map it" in prompt
    assert "create_note" in prompt
