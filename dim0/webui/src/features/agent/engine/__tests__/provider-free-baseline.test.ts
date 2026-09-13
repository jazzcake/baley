import { afterEach, describe, expect, it, vi } from "vitest"
import { asNodeId } from "@canvas-harness/core"
import { freshStore, resetIdb } from "@/test/canvas"
import { StoreMutator } from "../board-mutator"
import { llmClientFromResolution } from "../services/clients"


const { constructProviderClient } = vi.hoisted(() => ({
  constructProviderClient: vi.fn(),
}))


vi.mock("../byok-client", () => ({
  ByokLlmClient: { fromConfig: constructProviderClient },
}))


afterEach(() => {
  constructProviderClient.mockReset()
  resetIdb()
})


describe("provider-free baseline", () => {
  it("loads the boards page and non-agent services without constructing a BYOK provider client", async () => {
    const page = await import("@/features/board/screens/boards-home")

    expect(page.BoardsHome).toBeTypeOf("function")
    expect(llmClientFromResolution({ kind: "llm", mode: "off" })).toBeNull()
    expect(llmClientFromResolution({ kind: "search", mode: "off" })).toBeNull()
    expect(constructProviderClient).not.toHaveBeenCalled()
  }, 15_000)

  it("performs basic canvas writes without constructing a provider client", async () => {
    const store = freshStore("task-189-board")
    const mutator = new StoreMutator(store, null)

    await mutator.createNote({ id: "task-189-note", label: "Baseline", content: "provider free" })
    await mutator.patchNote("task-189-note", { content: "still provider free" })

    expect(store.getNode(asNodeId("task-189-note"))?.content).toBe("still provider free")
    expect(constructProviderClient).not.toHaveBeenCalled()
  })
})
