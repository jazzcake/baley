"""Persistent stdio client for the Codex app-server runtime."""

import asyncio
import json
import os
import uuid

from collections.abc import AsyncIterator
from typing import Any

RESPONSE_SCHEMA = {
    "type": "object",
    "additionalProperties": False,
    "properties": {
        "content": {"type": ["string", "null"]},
        "tool_calls": {
            "type": "array",
            "items": {
                "type": "object",
                "additionalProperties": False,
                "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "arguments": {"type": "object"}},
                "required": ["id", "name", "arguments"],
            },
        },
    },
    "required": ["content", "tool_calls"],
}


class CodexAppServerError(RuntimeError):
    """Raised when the Codex child process or protocol fails."""


class CodexAppServerRuntime:
    """Own one long-lived app-server process and one thread per Dim0 run."""

    def __init__(self, command: str | None = None, model: str | None = None) -> None:
        self.command = command or os.getenv("DIM0_CODEX_COMMAND", "codex")
        self.model = model or os.getenv("DIM0_CODEX_MODEL") or None
        self.cwd = os.getenv("DIM0_CODEX_WORKSPACE", "/var/empty/dim0-codex")
        self._process: asyncio.subprocess.Process | None = None
        self._reader_task: asyncio.Task[None] | None = None
        self._pending: dict[int, asyncio.Future[dict[str, Any]]] = {}
        self._notifications: asyncio.Queue[dict[str, Any]] = asyncio.Queue()
        self._threads: dict[str, str] = {}
        self._next_id = 0
        self._start_lock = asyncio.Lock()
        self._turn_lock = asyncio.Lock()

    async def start(self) -> None:
        """Start and initialize app-server once; subsequent calls are no-ops."""
        async with self._start_lock:
            if self._process and self._process.returncode is None:
                return
            self._process = await asyncio.create_subprocess_exec(
                self.command, "app-server", stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE
            )
            self._reader_task = asyncio.create_task(self._read_stdout())
            await self._request("initialize", {"clientInfo": {"name": "dim0", "title": "Dim0 Codex Runtime", "version": "0.1.0"}})
            await self._send({"method": "initialized", "params": {}})

    async def close(self) -> None:
        """Terminate the managed child process."""
        if self._process and self._process.returncode is None:
            self._process.terminate()
            await self._process.wait()
        if self._reader_task:
            await self._reader_task
        self._process = None

    async def complete(self, messages: list[dict[str, Any]], tools: list[dict[str, Any]], run_id: str | None, effort: str | None = None) -> dict[str, Any]:
        """Return one OpenAI-shaped assistant message without executing canvas tools."""
        async for event in self.stream(messages, tools, run_id, effort):
            if event["type"] == "final":
                return event["message"]
        raise CodexAppServerError("Codex turn completed without an assistant message")

    async def stream(self, messages: list[dict[str, Any]], tools: list[dict[str, Any]], run_id: str | None, effort: str | None = None) -> AsyncIterator[dict[str, Any]]:
        """Stream Dim0 events while serializing turns on the stdio connection."""
        async with self._turn_lock:
            await self.start()
            thread_id = await self._thread_for(run_id or str(uuid.uuid4()))
            params: dict[str, Any] = {
                "threadId": thread_id,
                "input": [{"type": "text", "text": self._prompt(messages, tools)}],
                "cwd": self.cwd,
                "approvalPolicy": "never",
                "sandboxPolicy": {"type": "readOnly", "access": {"type": "restricted", "includePlatformDefaults": False, "readableRoots": [self.cwd]}},
                "outputSchema": RESPONSE_SCHEMA,
            }
            if effort:
                params["effort"] = effort
            turn = (await self._request("turn/start", params))["turn"]
            raw = ""
            while True:
                event = await self._notifications.get()
                data = event.get("params", {})
                if data.get("threadId") not in (None, thread_id):
                    continue
                if event["method"] == "item/agentMessage/delta":
                    raw += data.get("delta", "")
                elif event["method"] == "turn/completed" and data.get("turn", {}).get("id") == turn["id"]:
                    if data["turn"].get("status") != "completed":
                        raise CodexAppServerError(f"Codex turn ended with status {data['turn'].get('status')}")
                    message = self._decode_message(raw)
                    for call in message.get("tool_calls", []):
                        yield {"type": "tool_start", "id": call["id"], "name": call["function"]["name"]}
                    if message.get("content"):
                        yield {"type": "delta", "text": message["content"]}
                    yield {"type": "final", "message": message}
                    return

    async def _thread_for(self, run_id: str) -> str:
        """Resolve or create the Codex thread backing a browser-agent run."""
        if run_id not in self._threads:
            params = {"model": self.model} if self.model else {}
            self._threads[run_id] = (await self._request("thread/start", params))["thread"]["id"]
        return self._threads[run_id]

    @staticmethod
    def _prompt(messages: list[dict[str, Any]], tools: list[dict[str, Any]]) -> str:
        """Encode the existing model-turn contract as a constrained Codex request."""
        return "Act only as Dim0's model-turn planner. Do not inspect files, run commands, or execute tools. Choose either content or the next supplied canvas tool call. Return only the requested JSON.\nMESSAGES:\n" + json.dumps(messages, ensure_ascii=False) + "\nTOOLS:\n" + json.dumps(tools, ensure_ascii=False)

    @staticmethod
    def _decode_message(raw: str) -> dict[str, Any]:
        """Translate structured Codex output into Dim0's OpenAI-shaped message."""
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise CodexAppServerError("Codex returned invalid structured output") from exc
        calls = [{"id": c["id"], "type": "function", "function": {"name": c["name"], "arguments": json.dumps(c["arguments"], ensure_ascii=False)}} for c in payload.get("tool_calls", [])]
        message: dict[str, Any] = {"role": "assistant", "content": payload.get("content")}
        if calls:
            message["tool_calls"] = calls
        return message

    async def _request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        """Send one request and await its matching response."""
        self._next_id += 1
        future = asyncio.get_running_loop().create_future()
        self._pending[self._next_id] = future
        await self._send({"method": method, "id": self._next_id, "params": params})
        return await future

    async def _send(self, payload: dict[str, Any]) -> None:
        """Write one newline-delimited protocol message."""
        if not self._process or not self._process.stdin:
            raise CodexAppServerError("Codex app-server is not running")
        self._process.stdin.write((json.dumps(payload) + "\n").encode())
        await self._process.stdin.drain()

    async def _read_stdout(self) -> None:
        """Dispatch app-server responses and notifications."""
        assert self._process and self._process.stdout
        while line := await self._process.stdout.readline():
            message = json.loads(line)
            if "id" in message:
                future = self._pending.pop(message["id"], None)
                if future:
                    if "error" in message:
                        future.set_exception(CodexAppServerError(message["error"].get("message", "Codex request failed")))
                    else:
                        future.set_result(message.get("result", {}))
            elif "method" in message:
                await self._notifications.put(message)
