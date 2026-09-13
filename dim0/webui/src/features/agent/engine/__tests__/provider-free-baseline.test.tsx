import { act } from "react"
import { createRoot, type Root } from "react-dom/client"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { resetIdb } from "@/test/canvas"
import { ThemeProvider } from "@/components/theme-provider"
import { BoardsHome } from "@/features/board/screens/boards-home"
import { HarnessCanvas } from "@/features/board/harness/canvas"
import { getCanvasStoreRef } from "@/features/board/harness/canvas-store-ref"
import { useBoardAppStore } from "@/features/board/harness/store/board-app-store"

Object.assign(globalThis, {
  IS_REACT_ACT_ENVIRONMENT: true,
  ResizeObserver: class {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  },
})


const mocks = vi.hoisted(() => ({
  constructProviderClient: vi.fn(),
  navigate: vi.fn(),
}))


vi.mock("../byok-client", () => ({
  ByokLlmClient: { fromConfig: mocks.constructProviderClient },
}))
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...await importOriginal<typeof import("@tanstack/react-router")>(),
  useNavigate: () => mocks.navigate,
  useRouterState: ({ select }: { select: (state: { location: { pathname: string } }) => unknown }) =>
    select({ location: { pathname: "/local/task-189-board" } }),
  useSearch: () => undefined,
}))
vi.mock("@/store", () => ({
  useAppStore: (select: (state: { userId: string; userEmail: string }) => unknown) =>
    select({ userId: "root", userEmail: "root@localhost" }),
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
  useBoardAppStore.getState().setBoardScope({})
  container.remove()
  localStorage.clear()
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

  it("mounts the real canvas boundary and performs a non-agent viewport interaction", async () => {
    localStorage.setItem("topix-ui-theme", JSON.stringify({ themeId: "noir", mode: "light" }))
    useBoardAppStore.getState().setBoardScope({ boardId: "task-189-board" })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <ThemeProvider defaultMode="light">
            <HarnessCanvas local />
          </ThemeProvider>
        </QueryClientProvider>,
      )
    })
    await waitFor(() => container.querySelector("[data-canvas-host]") !== null)
    await waitFor(() => getCanvasStoreRef() !== null)

    const store = getCanvasStoreRef()
    expect(store).not.toBeNull()
    act(() => store?.setCamera({ z: 0.5 }))
    const resetZoom = container.querySelector<HTMLButtonElement>('[aria-label="Reset zoom to 100%"]')
    expect(resetZoom).not.toBeNull()
    await act(async () => { resetZoom?.click() })

    expect(store?.getCamera().z).toBe(1)
    expect(mocks.constructProviderClient).not.toHaveBeenCalled()
    queryClient.clear()
  })
})
