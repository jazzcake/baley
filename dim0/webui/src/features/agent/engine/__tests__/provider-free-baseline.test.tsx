import { act, useEffect } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { asNodeId, type CanvasStore } from "@canvas-harness/core"
import { freshStore, resetIdb } from "@/test/canvas"
import { getLocalStores } from "@/features/local-stores"
import { BoardsHome } from "@/features/board/screens/boards-home"
import { BoardPersistence } from "@/features/board/persist/local/board-persistence"
import { setBoardPersistenceRef } from "@/features/board/persist/local/board-persistence-ref"
import { setBoardSyncRef } from "@/features/board/harness/sync/board-sync-ref"
import { setCanvasStoreRef } from "@/features/board/harness/canvas-store-ref"
import { useBoardAppStore } from "@/features/board/harness/store/board-app-store"
import { createBoardPageProvider } from "@/features/board/providers/board-page-provider"
import { StoreMutator } from "../board-mutator"

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true


const mocks = vi.hoisted(() => ({
  constructProviderClient: vi.fn(),
  navigate: vi.fn(),
}))


vi.mock("../byok-client", () => ({
  ByokLlmClient: { fromConfig: mocks.constructProviderClient },
}))
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mocks.navigate,
}))
vi.mock("@/store", () => ({
  useAppStore: (select: (state: { userId: string }) => unknown) => select({ userId: "root" }),
}))
vi.mock("@/features/agent/components/chat/welcome-message", () => ({ ThemedWelcome: () => null }))
vi.mock("@/features/board/api/list-boards", () => ({
  useListBoards: () => ({ data: undefined, isLoading: false }),
}))
vi.mock("@/features/board/local/use-enable-sync", () => ({
  useEnableSync: () => ({ enableSync: vi.fn(), pendingId: null }),
}))


/** Flush React work until a bounded asynchronous UI condition is true. */
const waitFor = async (condition: () => boolean): Promise<void> => {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (condition()) return
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)) })
  }
  throw new Error("condition did not become true")
}


/** Mount the real local board persistence and page-provider lifecycle. */
function ProviderLifecycleFixture({
  store,
  persistence,
  onProviderReady,
}: {
  store: CanvasStore
  persistence: BoardPersistence
  onProviderReady: () => void
}) {
  useEffect(() => {
    const detach = persistence.attach(store)
    setBoardPersistenceRef(persistence)
    setCanvasStoreRef(store)
    setBoardSyncRef(null)
    useBoardAppStore.setState({ boardId: "task-189-board", rootId: null })
    const provider = createBoardPageProvider({ boardId: "task-189-board" })
    void provider.list().then(onProviderReady)
    return () => {
      detach()
      persistence.close()
      setBoardPersistenceRef(null)
      setCanvasStoreRef(null)
      setBoardSyncRef(null)
      useBoardAppStore.setState({ boardId: null, rootId: null })
    }
  }, [onProviderReady, persistence, store])

  const createCanvasNote = (): void => {
    const mutator = new StoreMutator(store, null)
    void mutator.createNote({
      id: "task-189-note",
      label: "Baseline",
      content: "provider free",
    })
  }

  return <button onClick={createCanvasNote}>Create baseline canvas note</button>
}


let container: HTMLDivElement
let root: Root


beforeEach(() => {
  resetIdb()
  mocks.constructProviderClient.mockReset()
  mocks.navigate.mockReset()
  container = document.createElement("div")
  document.body.appendChild(container)
  root = createRoot(container)
})


afterEach(() => {
  act(() => root.unmount())
  container.remove()
  resetIdb()
})


describe("provider-free baseline", () => {
  it("renders the actual boards page lifecycle and creates a local board without a provider", async () => {
    await act(async () => { root.render(<BoardsHome />) })
    await waitFor(() => container.textContent?.includes("No local boards yet") === true)

    const newBoard = [...container.querySelectorAll("span")].find(
      (element) => element.textContent?.trim() === "New Board",
    )?.parentElement
    expect(newBoard).toBeDefined()
    await act(async () => { newBoard?.dispatchEvent(new MouseEvent("click", { bubbles: true })) })
    await waitFor(() => mocks.navigate.mock.calls.length === 1)

    expect(mocks.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: "/local/$boardId" }))
    expect(mocks.constructProviderClient).not.toHaveBeenCalled()
  }, 15_000)

  it("mounts the real board page provider lifecycle and performs a DOM-driven canvas write", async () => {
    const { engine } = await getLocalStores()
    const store = freshStore("task-189-live")
    const persistence = new BoardPersistence("task-189-board", { engine })
    const providerReady = vi.fn()

    await act(async () => {
      root.render(
        <ProviderLifecycleFixture
          store={store}
          persistence={persistence}
          onProviderReady={providerReady}
        />,
      )
    })
    await waitFor(() => providerReady.mock.calls.length === 1)

    const button = container.querySelector("button")
    expect(button).not.toBeNull()
    await act(async () => { button?.dispatchEvent(new MouseEvent("click", { bubbles: true })) })
    await waitFor(() => store.getNode(asNodeId("task-189-note")) !== undefined)

    expect(store.getNode(asNodeId("task-189-note"))?.content).toBe("provider free")
    expect(mocks.constructProviderClient).not.toHaveBeenCalled()
  })
})
