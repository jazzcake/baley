import { useEffect, useMemo, useRef, useState } from "react";
import { Background, Panel, ReactFlow, ViewportPortal, useStore, useStoreApi, type Edge, type Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ChevronRight, Copy, Maximize, Minus, PanelRightClose, PanelRightOpen, Plus, RotateCcw } from "lucide-react";
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import { fetchGraph } from "./api/client";
import { APIError } from "./api/http";
import { AuthProvider, useAuth } from "./auth/AuthProvider";
import type { Account, WorkspaceMembership } from "./auth/model";
import { canvasKey, connectedTaskIds, defaultGateFocusId, laneFocusTaskIds, visibleTaskIds, type ViewSpec } from "./graph/projection";
import { laneBandRect, laneLabelTop, layoutGraph, NODE_HEIGHT, NODE_WIDTH, type GraphLayout } from "./graph/layout";
import { fitViewportToCanvas, zoomViewportAtCenter } from "./graph/viewport";
import { INSPECTOR_DEFAULT_WIDTH, INSPECTOR_MAX_WIDTH, INSPECTOR_MIN_WIDTH, inspectorWidthFromKey, inspectorWidthFromPointer } from "./layout/inspector";
import { TaskNode } from "./components/TaskNode";
import { GateNode } from "./components/GateNode";
import { BacklogList, BacklogRail } from "./components/BacklogRail";
import { LaneAnchorColumn } from "./components/LaneAnchorColumn";
import { TaskSearch } from "./components/TaskSearch";
import { laneColorMap } from "./components/lane-palette";
import { LoginScreen, MCPConnectionApproval, WorkspaceAccessControls, WorkspaceChooser, WorkspaceContextSwitcher } from "./components/WorkspaceAccess";
import { traceViewer } from "./debug/viewer-trace";
import type { BacklogItem, GateLinkKind, Task, WorkspaceFixture } from "./domain/model";

const nodeTypes = { task: TaskNode, gate: GateNode };
const MIN_ZOOM = 0.55;
const MAX_ZOOM = 1.55;
let graphRequestGeneration = 0;

function traceCanvas(event: string, details: Record<string, unknown>) {
  if (import.meta.env.DEV) console.info(`[Baley canvas] ${event}`, details);
}
function viewFromLocation(pathname: string): ViewSpec {
  const lane = pathname.match(/\/lanes\/([^/]+)$/)?.[1];
  const gate = pathname.match(/\/gates\/([^/]+)$/)?.[1];
  const decode = (value: string) => {
    try { return decodeURIComponent(value); } catch { return value; }
  };
  return lane ? { kind: "lane", id: decode(lane) } : gate ? { kind: "gate", id: decode(gate) } : { kind: "multi" };
}

export function resolveGateReference(gates: WorkspaceFixture["gates"], reference: string) {
  const publicMatch = reference.match(/^G#([1-9]\d*)$/i);
  if (publicMatch) {
    const publicId = Number(publicMatch[1]);
    const gate = gates.find((item) => item.publicId === publicId);
    if (gate) return gate;
    return undefined;
  }
  const exact = gates.find((gate) => gate.id === reference);
  if (exact) return exact;
  return gates.find((gate) => gate.alias?.toLowerCase() === reference.trim().toLowerCase());
}

export default function App() {
  return <BrowserRouter><AuthProvider><AppRoutes /></AuthProvider></BrowserRouter>;
}

function AppRoutes() {
  const auth = useAuth();
  if (auth.state.status === "booting") {
    return <main className="server-state" data-auth-state="booting"><h1>Baley</h1><p>계정과 Workspace를 확인하는 중입니다…</p></main>;
  }
  if (auth.state.status === "unavailable") {
    return <main className="server-state error" data-auth-state="unavailable">
      <h1>Authentication unavailable</h1><p>{auth.state.message}</p>
      <button className="primary-button server-retry" type="button" onClick={auth.retryBootstrap}>다시 시도</button>
    </main>;
  }
  if (auth.state.status === "anonymous") {
    return <Routes>
      <Route path="/login" element={<LoginScreen onLogin={auth.login} />} />
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>;
  }
  return <Routes>
    <Route path="/login" element={<Navigate to="/workspaces" replace />} />
    <Route path="/workspaces" element={<WorkspaceChooser
      account={auth.state.account}
      memberships={auth.state.memberships}
      csrfToken={auth.state.csrfToken}
      onMembershipsChanged={auth.refreshWorkspaces}
    />} />
    <Route path="/workspaces/:workspaceId/*" element={<WorkspaceRoute />} />
    <Route path="*" element={<Navigate to={auth.state.memberships.length === 1 ? `/workspaces/${encodeURIComponent(auth.state.memberships[0]!.id)}` : "/workspaces"} replace />} />
  </Routes>;
}

function WorkspaceRoute() {
  const { workspaceId = "" } = useParams();
  const auth = useAuth();
  if (auth.state.status !== "authenticated") return null;
  const membership = auth.state.memberships.find((item) => item.id === workspaceId);
  if (!membership) {
    return <main className="server-state error" data-auth-state="authenticated">
      <h1>Workspace access unavailable</h1>
      <p>이 계정이 참여 중인 Workspace가 아닙니다.</p>
      <a href="/workspaces">Workspace 목록으로 돌아가기</a>
    </main>;
  }
  return <WorkspaceViewer
    key={workspaceId}
    workspaceId={workspaceId}
    membership={membership}
    memberships={auth.state.memberships}
    account={auth.state.account}
    csrfToken={auth.state.csrfToken}
    onLogout={auth.logout}
    onMembershipsChanged={auth.refreshWorkspaces}
    onSessionExpired={auth.expireSession}
  />;
}

function WorkspaceViewer({
  workspaceId,
  membership,
  memberships,
  account,
  csrfToken,
  onLogout,
  onMembershipsChanged,
  onSessionExpired,
}: {
  workspaceId: string;
  membership: WorkspaceMembership;
  memberships: WorkspaceMembership[];
  account: Account;
  csrfToken: string;
  onLogout: () => Promise<void>;
  onMembershipsChanged: () => Promise<void>;
  onSessionExpired: () => void;
}) {
  const location = useLocation();
  const routeNavigate = useNavigate();
  const [fixture, setFixture] = useState<WorkspaceFixture | undefined>();
  const [loadError, setLoadError] = useState<string>();
  const graph: WorkspaceFixture = fixture ?? { workspace: { id: "", name: "Baley", revision: 0 }, phases: [], lanes: [], tasks: [], backlogItems: [], dependencies: [], gates: [], gateLinks: [], decisions: [] };
  const routeView = useMemo(() => viewFromLocation(location.pathname), [location.pathname]);
  const view = useMemo<ViewSpec>(() => {
    if (routeView.kind !== "gate") return routeView;
    const gate = resolveGateReference(graph.gates, routeView.id);
    return gate ? { kind: "gate", id: gate.id } : routeView;
  }, [graph.gates, routeView]);
  const selectedId = useMemo(() => new URLSearchParams(location.search).get("task") ?? undefined, [location.search]);
  const selectedBacklogId = useMemo(() => new URLSearchParams(location.search).get("backlog") ?? undefined, [location.search]);
  const mcpConnectionId = useMemo(() => location.pathname.match(/\/mcp-connect\/([^/]+)$/)?.[1], [location.pathname]);
  const [layout, setLayout] = useState<GraphLayout | undefined>();
  const [layoutViewKey, setLayoutViewKey] = useState<string>();
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [inspectorWidth, setInspectorWidth] = useState(INSPECTOR_DEFAULT_WIDTH);
  const [backlogListOpen, setBacklogListOpen] = useState(false);
  const [taskFocusRequest, setTaskFocusRequest] = useState<{ taskId: string; requestId: number }>();
  const [workspaceIDCopyState, setWorkspaceIDCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const backlogExpandButtonRef = useRef<HTMLButtonElement | null>(null);
  const graphStageRef = useRef<HTMLDivElement>(null);
  const requestGenerationRef = useRef(0);
  const taskFocusRequestIdRef = useRef(0);
  const workspaceIDCopyTimerRef = useRef<number>();

  const copyWorkspaceID = async () => {
    const workspaceID = graph.workspace.id;
    traceViewer("workspace-id-copy:event", {
      workspaceIdPresent: Boolean(workspaceID),
      clipboardAvailable: Boolean(navigator.clipboard?.writeText),
      secureContext: window.isSecureContext,
    });
    if (!workspaceID) return;
    try {
      let method = "clipboard";
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(workspaceID);
      } else {
        method = "selection-fallback";
        const input = document.createElement("textarea");
        input.value = workspaceID;
        input.setAttribute("readonly", "");
        input.style.cssText = "position:fixed;opacity:0;pointer-events:none";
        document.body.append(input);
        input.select();
        const copied = document.execCommand("copy");
        input.remove();
        if (!copied) throw new Error("Clipboard fallback was rejected");
      }
      setWorkspaceIDCopyState("copied");
      traceViewer("workspace-id-copy:state", { state: "copied", method });
    } catch (error) {
      setWorkspaceIDCopyState("failed");
      traceViewer("workspace-id-copy:state", { state: "failed", error: error instanceof Error ? error.name : "unknown" });
    }
    window.clearTimeout(workspaceIDCopyTimerRef.current);
    workspaceIDCopyTimerRef.current = window.setTimeout(() => setWorkspaceIDCopyState("idle"), 1600);
  };

  useEffect(() => () => window.clearTimeout(workspaceIDCopyTimerRef.current), []);
  const routeNavigateRef = useRef(routeNavigate);
  routeNavigateRef.current = routeNavigate;
  const visible = useMemo(() => visibleTaskIds(graph, view), [graph, view]);
  const laneColors = useMemo(() => laneColorMap(graph.lanes), [graph.lanes]);
  const laneFocus = useMemo(
    () => view.kind === "lane" ? laneFocusTaskIds(graph, view.id) : undefined,
    [graph, view],
  );
  const connected = useMemo(() => selectedId ? connectedTaskIds(graph, selectedId) : undefined, [graph, selectedId]);

  useEffect(() => {
    let active = true;
    const requestedViewKey = canvasKey(view);
    void layoutGraph(graph, visible, view.kind !== "gate").then((nextLayout) => {
      if (active) {
        setLayout(nextLayout);
        setLayoutViewKey(requestedViewKey);
      }
    });
    return () => { active = false; };
  }, [graph, visible, view.kind]);
  useEffect(() => {
    let active = true;
    let controller: AbortController | undefined;
    setFixture(undefined);
    setLayout(undefined);
    setLayoutViewKey(undefined);
    setBacklogListOpen(false);
    const refresh = () => {
      controller?.abort();
      controller = new AbortController();
      const generation = ++graphRequestGeneration;
      requestGenerationRef.current = generation;
      const requestId = `${workspaceId}:${generation}:${Date.now()}`;
      traceViewer("graph:request", {
        event: "refresh",
        targetWorkspaceId: workspaceId,
        authState: "authenticated",
        route: location.pathname,
        requestGeneration: generation,
        requestId,
        controllerState: "active",
      });
      void fetchGraph(workspaceId, controller.signal).then((next) => {
        if (!active || requestGenerationRef.current !== generation || next.workspace.id !== workspaceId) {
          traceViewer("graph:response-ignored", {
            targetWorkspaceId: workspaceId,
            responseWorkspaceId: next.workspace.id,
            requestGeneration: generation,
            currentGeneration: requestGenerationRef.current,
          });
          return;
        }
        setFixture((current) => current && JSON.stringify(current) === JSON.stringify(next) ? current : next);
        setLoadError(undefined);
        traceViewer("graph:store-committed", {
          targetWorkspaceId: workspaceId,
          committedGraphWorkspaceId: next.workspace.id,
          requestGeneration: generation,
          revision: next.workspace.revision,
        });
        window.requestAnimationFrame(() => {
          const rendered = document.querySelector<HTMLElement>("[data-workspace-id]");
          traceViewer("graph:dom-rendered", {
            targetWorkspaceId: workspaceId,
            committedGraphWorkspaceId: next.workspace.id,
            renderedWorkspaceId: rendered?.dataset.workspaceId,
            requestGeneration: generation,
          });
        });
      }).catch((error: unknown) => {
        if (!active || controller?.signal.aborted) return;
        if (error instanceof APIError && error.status === 401) {
          onSessionExpired();
          return;
        }
        if (error instanceof APIError && (error.status === 403 || error.status === 404)) {
          setFixture(undefined);
          traceViewer("workspace-access:revoked", {
            targetWorkspaceId: workspaceId,
            status: error.status,
            requestGeneration: generation,
          });
          void onMembershipsChanged().finally(() => routeNavigateRef.current("/workspaces", { replace: true }));
          return;
        }
        setLoadError(error instanceof Error ? error.message : "Server unavailable");
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 2000);
    window.addEventListener("focus", refresh);
    return () => {
      active = false;
      controller?.abort();
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
      traceViewer("graph:request-aborted", {
        targetWorkspaceId: workspaceId,
        requestGeneration: requestGenerationRef.current,
        controllerState: "aborted",
      });
    };
  }, [workspaceId, onSessionExpired, onMembershipsChanged]);
  useEffect(() => {
    const stage = graphStageRef.current;
    if (!stage) return;
    if (backlogListOpen) stage.setAttribute("inert", "");
    else stage.removeAttribute("inert");
    window.requestAnimationFrame(() => traceViewer("backlog-list:rendered", {
      open: backlogListOpen,
      stageInert: stage.hasAttribute("inert"),
      listPresent: Boolean(document.querySelector(".backlog-list")),
    }));
  }, [backlogListOpen]);
  const nodes = useMemo<Node[]>(() => {
    const taskNodes: Node[] = graph.tasks.filter((task) => visible.has(task.id)).map((task) => ({
      id: task.id, type: "task", position: layout?.taskPositions.get(task.id) ?? { x: 0, y: 0 }, selected: task.id === selectedId,
      data: {
        title: task.title,
        publicId: task.publicId,
        status: task.status,
        lane: graph.lanes.find((lane) => lane.id === task.laneId)?.name ?? "",
        laneColor: laneColors[task.laneId] ?? "#579bfc",
        dimmed: Boolean(
          (laneFocus && !laneFocus.has(task.id)) ||
          (connected && !connected.has(task.id)),
        ),
        external: view.kind === "lane" && task.laneId !== view.id,
      },
    }));
    const gateNodes: Node[] = graph.gates.filter((gate) =>
      view.kind === "gate"
        ? gate.id === view.id
        : view.kind !== "lane" || graph.gateLinks.some((link) => link.gateId === gate.id && visible.has(link.taskId)),
    ).map((gate) => {
      const required = graph.gateLinks.filter((link) => link.gateId === gate.id && link.kind === "required");
      const done = required.filter((link) => link.satisfied).length;
      return { id: gate.id, type: "gate", position: layout?.gatePositions.get(gate.id) ?? { x: 0, y: 0 }, selected: gate.id === selectedId, data: { title: gate.name, publicId: gate.publicId, alias: gate.alias, gateId: gate.id, status: gate.status, summary: `${done}/${required.length} conditions satisfied`, dimmed: Boolean(selectedId && selectedId !== gate.id && !graph.gateLinks.some((link) => link.gateId === gate.id && link.taskId === selectedId)) } };
    });
    return [...taskNodes, ...gateNodes];
  }, [graph, visible, layout, selectedId, connected, laneFocus, view]);

  const edges = useMemo<Edge[]>(() => {
    const dependencies: Edge[] = graph.dependencies.filter((edge) => visible.has(edge.fromTaskId) && visible.has(edge.toTaskId)).map((edge) => ({
      id: edge.id,
      source: edge.fromTaskId,
      target: edge.toTaskId,
      className:
        (laneFocus && (!laneFocus.has(edge.fromTaskId) || !laneFocus.has(edge.toTaskId))) ||
        (connected && (!connected.has(edge.fromTaskId) || !connected.has(edge.toTaskId)))
          ? "edge-dimmed"
          : "dependency-edge",
      animated: graph.tasks.find((task) => task.id === edge.toTaskId)?.status === "in_progress",
    }));
    const colors: Record<GateLinkKind, string> = { required: "#8d5f39", reference: "#8b8b82", unlocks: "#366b62" };
    const gateEdges: Edge[] = graph.gateLinks.filter((link) => visible.has(link.taskId)).map((link) => ({
      id: `${link.gateId}-${link.taskId}-${link.kind}`,
      source: link.kind === "unlocks" ? link.gateId : link.taskId,
      target: link.kind === "unlocks" ? link.taskId : link.gateId,
      label: link.kind,
      style: { stroke: colors[link.kind], strokeDasharray: link.kind === "reference" ? "5 5" : undefined },
      className:
        (laneFocus && !laneFocus.has(link.taskId)) ||
        (selectedId && selectedId !== link.taskId && selectedId !== link.gateId)
          ? "edge-dimmed"
          : "gate-edge",
    }));
    return [...dependencies, ...gateEdges];
  }, [graph, visible, connected, laneFocus, selectedId]);

  const selectedTask = graph.tasks.find((task) => task.id === selectedId);
  const selectedGate = graph.gates.find((gate) => gate.id === selectedId);
  const selectedBacklog = graph.backlogItems.find((item) => item.id === selectedBacklogId);
  const defaultLaneId = graph.lanes.find((lane) => lane.name === "Client")?.id ?? graph.lanes[0]?.id;
  const defaultGateId = defaultGateFocusId(graph);
  const workspaceBase = `/workspaces/${encodeURIComponent(workspaceId)}`;
  const workspaceContextLabel = backlogListOpen
    ? "Lane backlog"
    : view.kind === "lane"
      ? `${graph.lanes.find((lane) => lane.id === view.id)?.name} lane`
      : view.kind === "gate"
        ? (() => {
            const gate = graph.gates.find((item) => item.id === view.id);
            return gate ? `G#${gate.publicId} ${gate.name} gate` : "Unknown gate";
          })()
        : undefined;
  const setSelectedId = (nextSelectedId: string | undefined) => {
    routeNavigate({
      pathname: location.pathname,
      search: nextSelectedId ? `?task=${encodeURIComponent(nextSelectedId)}` : "",
    }, { replace: true });
  };
  const setSelectedBacklog = (item: BacklogItem) => {
    traceViewer("backlog:select", {
      itemId: item.id,
      publicId: item.publicId,
      calculatedTarget: "backlog-inspector",
      listOpen: backlogListOpen,
    });
    routeNavigate({ pathname: location.pathname, search: `?backlog=${encodeURIComponent(item.id)}` }, { replace: true });
  };
  const openBacklogList = (source: "canvas" | "header" | "rail") => {
    traceViewer("backlog-list:open", {
      source,
      currentOpen: backlogListOpen,
      calculatedOpen: true,
      activeItems: graph.backlogItems.filter((item) => item.status === "active").length,
    });
    setBacklogListOpen(true);
  };
  const selectBacklogFromRail = (item: BacklogItem) => {
    traceViewer("backlog-rail:item-click", {
      itemId: item.id,
      publicId: item.publicId,
      calculatedListOpen: true,
      calculatedInspector: "backlog-inspector",
      currentListOpen: backlogListOpen,
    });
    setSelectedBacklog(item);
    openBacklogList("rail");
  };
  const navigate = (next: ViewSpec) => {
    const path = next.kind === "multi"
      ? workspaceBase
      : next.kind === "lane"
        ? `${workspaceBase}/lanes/${encodeURIComponent(next.id)}`
        : (() => {
            const gate = graph.gates.find((item) => item.id === next.id);
            return `${workspaceBase}/gates/${encodeURIComponent(gate ? `G#${gate.publicId}` : next.id)}`;
          })();
    routeNavigate(path);
    setBacklogListOpen(false);
  };
  const closeBacklogList = () => {
    traceViewer("backlog-list:close", { currentOpen: backlogListOpen, calculatedOpen: false });
    setBacklogListOpen(false);
    window.setTimeout(() => backlogExpandButtonRef.current?.focus(), 0);
  };
  const selectSearchResult = (task: Task, query: string) => {
    const requestId = ++taskFocusRequestIdRef.current;
    const targetPosition = layout?.taskPositions.get(task.id);
    traceViewer("task-search:select", {
      query,
      taskId: task.id,
      publicId: task.publicId,
      sourceView: view.kind,
      targetView: "multi",
      targetPosition,
      selectedTaskId: selectedId,
      renderedNodePresent: nodes.some((node) => node.id === task.id),
      requestId,
    });
    setTaskFocusRequest({ taskId: task.id, requestId });
    routeNavigate({ pathname: workspaceBase, search: `?task=${encodeURIComponent(task.id)}` });
    setBacklogListOpen(false);
  };

  if (!fixture && !loadError) return <main className="server-state" data-workspace-target={workspaceId}><h1>Baley</h1><p>Workspace graph를 불러오는 중입니다…</p></main>;
  if (!fixture && loadError) return <main className="server-state error"><h1>Server unavailable</h1><p>{loadError}</p><small>Viewer는 fixture로 대체 표시하지 않습니다.</small></main>;
  return (
    <main className="app-shell" data-workspace-id={graph.workspace.id} data-auth-state="authenticated" data-role={membership.role}>
      <header className="topbar">
        <button type="button" className="brand" aria-label="Go to Home" onClick={() => navigate({ kind: "multi" })}><div className="brand-mark">B</div><div><strong>Baley</strong><span>Visual MVP</span></div></button>
        <nav className="view-tabs" aria-label="Graph views">
          <button className={view.kind === "multi" ? "active" : ""} onClick={() => navigate({ kind: "multi" })}>Multi-lane</button>
          <button className={view.kind === "lane" ? "active" : ""} disabled={!defaultLaneId} onClick={() => defaultLaneId && navigate({ kind: "lane", id: view.kind === "lane" ? view.id : defaultLaneId })}>Lane focus</button>
          <button className={view.kind === "gate" ? "active" : ""} disabled={!defaultGateId} onClick={() => defaultGateId && navigate({ kind: "gate", id: view.kind === "gate" ? view.id : defaultGateId })}>Gate focus</button>
        </nav>
        <div className="topbar-actions">
          <button type="button" className="quiet-button" aria-label="Open full backlog" onClick={() => openBacklogList("header")}>Backlog</button>
          <WorkspaceAccessControls
            account={account}
            membership={membership}
            csrfToken={csrfToken}
            onLogout={onLogout}
            onMembershipsChanged={onMembershipsChanged}
          />
          <button className="icon-button" aria-label="Toggle inspector" onClick={() => setInspectorOpen((open) => !open)}>{inspectorOpen ? <PanelRightClose size={18} /> : <PanelRightOpen size={18} />}</button>
        </div>
      </header>

      <section className={`workspace ${inspectorOpen ? "with-inspector" : ""}`} style={{ "--inspector-width": `${inspectorWidth}px` } as React.CSSProperties}>
        <div className="graph-wrap">
          <div className="context-row"><div><button type="button" className="workspace-home-link" aria-label="Go to Workspace Home" onClick={() => navigate({ kind: "multi" })}>WORKSPACE · REVISION {graph.workspace.revision}</button><h1 className="workspace-context-title"><WorkspaceContextSwitcher membership={membership} memberships={memberships} currentWorkspaceName={graph.workspace.name} csrfToken={csrfToken} onMembershipsChanged={onMembershipsChanged} /><button type="button" className="workspace-id-copy" aria-label="Copy Workspace UUID" title="Copy Workspace UUID" onClick={() => void copyWorkspaceID()}><Copy size={14} /></button>{workspaceIDCopyState !== "idle" && <span className={`workspace-id-copied ${workspaceIDCopyState === "failed" ? "workspace-id-copy-failed" : ""}`} role="status">{workspaceIDCopyState === "copied" ? "UUID copied" : "Copy failed"}</span>}{workspaceContextLabel && <span className="workspace-view-context">/ {workspaceContextLabel}</span>}</h1></div><div className="context-actions">{loadError && <span className="poll-error">refresh failed</span>}<span className="readonly-badge">READ ONLY</span><button className="quiet-button" onClick={() => setSelectedId(undefined)}><RotateCcw size={14} /> Clear focus</button></div></div>
          <div className="graph-canvas">
            <div ref={graphStageRef} className="graph-stage" aria-hidden={backlogListOpen || undefined}>
              <ReactFlow key={canvasKey(view)} nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodeClick={(_, node) => setSelectedId(node.id)} onMoveEnd={(_, nextViewport) => traceCanvas("move:end", nextViewport)} minZoom={MIN_ZOOM} maxZoom={MAX_ZOOM} nodesDraggable={false} proOptions={{ hideAttribution: true }}>
                <Background color="#d8d6ce" gap={24} size={1} />
                <ViewportPortal><CanvasOverlay graph={graph} layout={layout} view={view} navigate={navigate} laneColors={laneColors} onOpenBacklog={() => openBacklogList("canvas")} onSelectBacklog={selectBacklogFromRail} setBacklogExpandButton={(node) => { backlogExpandButtonRef.current = node; }} /></ViewportPortal>
                <CanvasControls layout={layout} />
                <Panel position="bottom-center" className="task-search-panel"><TaskSearch tasks={graph.tasks} onSelect={selectSearchResult} /></Panel>
                <TaskFocusController request={taskFocusRequest} layout={layoutViewKey === canvasKey(view) ? layout : undefined} />
              </ReactFlow>
            </div>
            {backlogListOpen && <BacklogList lanes={graph.lanes} items={graph.backlogItems} laneColors={laneColors} onClose={closeBacklogList} onSelect={setSelectedBacklog} />}
          </div>
        </div>
        {inspectorOpen && <div className="inspector-panel">
          <InspectorResizeHandle width={inspectorWidth} onWidth={setInspectorWidth} />
          <Inspector fixture={graph} task={selectedTask} backlog={selectedBacklog} gateId={selectedGate?.id} onLane={(id) => navigate({ kind: "lane", id })} onGate={(id) => navigate({ kind: "gate", id })} />
        </div>}
      </section>
      {mcpConnectionId && <MCPConnectionApproval
        workspace={membership}
        connectionId={decodeURIComponent(mcpConnectionId)}
        csrfToken={csrfToken}
        onClose={() => routeNavigate(workspaceBase, { replace: true })}
      />}
    </main>
  );
}

function TaskFocusController({ request, layout }: { request?: { taskId: string; requestId: number }; layout?: GraphLayout }) {
  const store = useStoreApi();
  const controllerReady = useStore((state) => Boolean(state.panZoom));
  const handledRequestRef = useRef<number>();
  const inFlightRequestRef = useRef<number>();

  useEffect(() => {
    if (!request || !controllerReady || handledRequestRef.current === request.requestId || inFlightRequestRef.current === request.requestId) return;
    const position = layout?.taskPositions.get(request.taskId);
    if (!position) return;
    const state = store.getState();
    const panZoom = state.panZoom;
    const canvasWidth = state.width || state.domNode?.clientWidth || 0;
    const canvasHeight = state.height || state.domNode?.clientHeight || 0;
    if (!panZoom || !canvasWidth || !canvasHeight) {
      traceViewer("task-search:focus-waiting", {
        requestId: request.requestId,
        taskId: request.taskId,
        panZoomReady: Boolean(panZoom),
        storeSize: { width: state.width, height: state.height },
        domSize: { width: state.domNode?.clientWidth, height: state.domNode?.clientHeight },
      });
      return;
    }
    const target = {
      x: position.x + NODE_WIDTH / 2,
      y: position.y + NODE_HEIGHT / 2,
      zoom: Math.max(state.transform[2], 0.9),
    };
    const viewport = {
      x: canvasWidth / 2 - target.x * target.zoom,
      y: canvasHeight / 2 - target.y * target.zoom,
      zoom: target.zoom,
    };
    const viewportElement = state.domNode?.querySelector<HTMLElement>(".react-flow__viewport");
    inFlightRequestRef.current = request.requestId;
    traceViewer("task-search:focus-request", {
      requestId: request.requestId,
      taskId: request.taskId,
      target,
      viewport,
      canvas: { width: canvasWidth, height: canvasHeight },
      store: { x: state.transform[0], y: state.transform[1], zoom: state.transform[2] },
      domTransform: viewportElement?.style.transform,
      controllerReady,
      renderedNodePresent: state.nodeLookup.has(request.taskId),
    });
    const controllerUpdate = panZoom.setViewport(viewport, { duration: 0 });
    const applied = store.getState();
    const renderedViewport = applied.domNode?.querySelector<HTMLElement>(".react-flow__viewport");
    handledRequestRef.current = request.requestId;
    inFlightRequestRef.current = undefined;
    store.setState({ transform: [viewport.x, viewport.y, viewport.zoom] });
    if (renderedViewport) renderedViewport.style.transform = `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})`;
    traceViewer("task-search:focus-applied", {
      requestId: request.requestId,
      taskId: request.taskId,
      store: { x: viewport.x, y: viewport.y, zoom: viewport.zoom },
      domTransform: renderedViewport?.style.transform,
      renderedNodePresent: applied.nodeLookup.has(request.taskId),
    });
    void controllerUpdate.then((result) => {
      const committed = store.getState();
      const renderer = committed.domNode?.querySelector<HTMLElement & { __zoom?: unknown }>(".react-flow__renderer");
      if (result && renderer) renderer.__zoom = result;
      traceViewer("task-search:controller-settled", {
        requestId: request.requestId,
        taskId: request.taskId,
        controllerResult: result,
        store: { x: committed.transform[0], y: committed.transform[1], zoom: committed.transform[2] },
      });
    }).catch((error: unknown) => {
      traceViewer("task-search:controller-failed", {
        requestId: request.requestId,
        taskId: request.taskId,
        error: error instanceof Error ? error.message : String(error),
      });
    });
  }, [controllerReady, layout, request, store]);

  return null;
}

function InspectorResizeHandle({ width, onWidth }: { width: number; onWidth: (width: number) => void }) {
  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    const startX = event.clientX;
    const startWidth = width;
    const onPointerMove = (moveEvent: PointerEvent) => onWidth(inspectorWidthFromPointer(startWidth, startX, moveEvent.clientX));
    const finish = () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", finish);
      window.removeEventListener("pointercancel", finish);
      document.body.classList.remove("resizing-inspector");
    };
    event.preventDefault();
    document.body.classList.add("resizing-inspector");
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", finish);
    window.addEventListener("pointercancel", finish);
  };
  return <div
    className="inspector-resize-handle"
    role="separator"
    aria-label="Resize inspector"
    aria-orientation="vertical"
    aria-valuemin={INSPECTOR_MIN_WIDTH}
    aria-valuemax={INSPECTOR_MAX_WIDTH}
    aria-valuenow={width}
    tabIndex={0}
    onPointerDown={onPointerDown}
    onDoubleClick={() => onWidth(INSPECTOR_DEFAULT_WIDTH)}
    onKeyDown={(event) => {
      if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      const next = inspectorWidthFromKey(width, event.key, event.shiftKey);
      event.preventDefault();
      onWidth(next);
    }}
  />;
}

function Inspector({ fixture, task, backlog, gateId, onLane, onGate }: { fixture: WorkspaceFixture; task?: Task; backlog?: BacklogItem; gateId?: string; onLane: (id: string) => void; onGate: (id: string) => void }) {
  if (backlog) {
    const lane = fixture.lanes.find((item) => item.id === backlog.laneId);
    return <aside className="inspector"><div className="inspector-kicker">BACKLOG INSPECTOR</div><div className="inspector-id">BACKLOG B#{backlog.publicId}</div><h2>{backlog.title}</h2><span className={`status-pill status-${backlog.status}`}>{backlog.status}</span><p>{backlog.description}</p><Section title="Context"><button className="text-link" onClick={() => lane && onLane(lane.id)}>{lane?.name ?? "Unknown"} lane</button><span className="meta-value">Phase unassigned</span></Section><Section title="Planning"><span className="meta-value">Position {backlog.position ?? "unranked"}</span>{backlog.promotedTaskPublicId && <span className="evidence-copy">Promoted to Task #{backlog.promotedTaskPublicId}</span>}</Section><section className="command-hint"><strong>LLM command only</strong><p>Use Baley Skill commands to update backlog B#{backlog.publicId}.</p></section></aside>;
  }
  if (gateId) {
    const gate = fixture.gates.find((item) => item.id === gateId)!;
    const links = fixture.gateLinks.filter((link) => link.gateId === gateId);
    return <aside className="inspector"><div className="inspector-kicker">GATE INSPECTOR</div><div className="inspector-id">GATE G#{gate.publicId}</div><h2>{gate.name}</h2><span className={`status-pill status-${gate.status}`}>{gate.status}</span><p>Build Phase를 완료하고 Validate Phase로 진입하기 위한 동기화 지점입니다.</p><Section title="Identity"><span className="meta-value">{gate.alias ? `Alias · ${gate.alias}` : "No alias"}</span><code>{gate.id}</code></Section><Section title="Conditions">{links.map((link) => <div className="relation-row" key={`${link.taskId}-${link.kind}`}><span>{link.satisfied ? link.satisfactionReason : link.kind}</span><strong>{fixture.tasks.find((task) => task.id === link.taskId)?.title}</strong></div>)}</Section><button className="primary-button" onClick={() => onGate(gate.id)}>Open gate focus</button></aside>;
  }
  if (!task) return <aside className="inspector empty"><div className="empty-symbol">↗</div><h2>Follow the work</h2><p>Task 또는 Gate를 선택하면 현재 상태와 연결 관계를 확인할 수 있습니다.</p><div className="legend"><span><i className="dot done" />Done</span><span><i className="dot running" />Running</span><span><i className="dot blocked" />Blocked</span><span><i className="dot ready" />Ready</span></div></aside>;
  const lane = fixture.lanes.find((item) => item.id === task.laneId)!;
  const phase = fixture.phases.find((item) => item.id === task.phaseId)!;
  const gateLinks = fixture.gateLinks.filter((link) => link.taskId === task.id);
  const upstream = fixture.dependencies.filter((edge) => edge.toTaskId === task.id);
  const downstream = fixture.dependencies.filter((edge) => edge.fromTaskId === task.id);
  const runs = (fixture.runs ?? []).filter((run) => run.taskId === task.id);
  const acceptanceEvidence = (fixture.acceptanceEvidence ?? []).filter((evidence) => evidence.taskId === task.id);
  const records = [
    ...(fixture.records ?? []).filter((record) => record.taskId === task.id),
    ...acceptanceEvidence.map((evidence) => ({
      id: evidence.id,
      taskId: evidence.taskId,
      recordType: `acceptance-evidence-v${evidence.version}`,
      state: `${evidence.verificationVerdict}/review:${evidence.reviewVerdict}`,
      relativePath: `${evidence.verificationReferenceKind ?? "reference"}:${evidence.verificationReference ?? "none"}`,
      shortSummary: `completion=${evidence.completionReportRecordId}; review=${evidence.independentReviewRecordId}; blockers=${evidence.unresolvedBlockingCount}`,
    })),
  ];
  return <aside className="inspector"><div className="inspector-kicker">TASK INSPECTOR</div><div className="inspector-id">TASK #{task.publicId}</div><h2>{task.title}</h2><span className={`status-pill status-${task.status}`}>{task.status}</span><p>{task.description}</p><Section title="Context"><button className="text-link" onClick={() => onLane(lane.id)}>{lane.name} lane</button><span className="meta-value">{phase.name} Phase</span></Section>{task.currentSummary && <Section title="Current summary"><span className="evidence-copy">{task.currentSummary}</span></Section>}{task.nextAction && <Section title="Next action"><span className="evidence-copy">{task.nextAction}</span></Section>}{task.implementedAssessment && <Section title="Implementation assessment"><span className="evidence-copy">{task.implementedAssessment}</span></Section>}{task.effectiveAcceptanceMode && <Section title="Acceptance"><span className="meta-value">{task.effectiveAcceptanceMode}</span><span className="evidence-copy">Policy {task.acceptancePolicyVersion} · Profile {task.evidenceProfileId}</span>{task.acceptanceEvaluation && <span className="evidence-copy">{task.acceptanceEvaluation.eligible ? "Evidence eligible" : `Evidence pending: ${task.acceptanceEvaluation.reasons.join(", ")}`}</span>}</Section>}{task.blocker && <Section title="Blocker"><div className="blocker-box">{task.blocker}</div></Section>}<Section title="Flow">{upstream.map((edge) => <div className="relation-row" key={edge.id}><span>from</span><strong>#{fixture.tasks.find((item) => item.id === edge.fromTaskId)?.publicId} {fixture.tasks.find((item) => item.id === edge.fromTaskId)?.title}</strong></div>)}{downstream.map((edge) => <div className="relation-row" key={edge.id}><span>to</span><strong>#{fixture.tasks.find((item) => item.id === edge.toTaskId)?.publicId} {fixture.tasks.find((item) => item.id === edge.toTaskId)?.title}</strong></div>)}{!upstream.length && !downstream.length && <span className="muted">Independent path</span>}</Section>{gateLinks.length > 0 && <Section title="Gate relations">{gateLinks.map((link) => { const linkedGate = fixture.gates.find((gate) => gate.id === link.gateId); return <button className="relation-row clickable" key={link.gateId} onClick={() => onGate(link.gateId)}><span>{link.kind}</span><strong>{linkedGate ? `G#${linkedGate.publicId} ${linkedGate.name}` : link.gateId}</strong></button>; })}</Section>}<Section title="Runs">{runs.map((run) => <div className="evidence-row" key={run.id}><div><strong>{run.kind.replaceAll("_", " ")}</strong><span>{run.status}</span></div>{(run.resultSummary || run.errorSummary) && <p>{run.resultSummary || run.errorSummary}</p>}</div>)}{runs.length === 0 && <span className="muted">No Runs recorded</span>}</Section><Section title="Task Records">{records.map((record) => <div className="evidence-row" key={record.id}><div><strong>{record.recordType}</strong><span>{record.state}</span></div><code>{record.relativePath}</code><p>{record.shortSummary}</p></div>)}{records.length === 0 && <span className="muted">No Task Records indexed</span>}</Section><section className="command-hint"><strong>LLM command only</strong><p>Use Baley Skill commands to update task #{task.publicId}.</p></section></aside>;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) { return <section className="inspector-section"><h3>{title}</h3>{children}</section>; }

function CanvasControls({ layout }: { layout?: GraphLayout }) {
  const store = useStoreApi();
  const zoom = useStore((state) => state.transform[2]);
  const minZoom = useStore((state) => state.minZoom);
  const maxZoom = useStore((state) => state.maxZoom);
  const canvasSize = () => {
    const state = store.getState();
    return {
      width: state.width || state.domNode?.clientWidth || 0,
      height: state.height || state.domNode?.clientHeight || 0,
    };
  };
  const apply = async (action: string, viewport: { x: number; y: number; zoom: number } | undefined) => {
    const state = store.getState();
    const panZoom = state.panZoom;
    const { width, height } = canvasSize();
    traceCanvas(`${action}:click`, {
      before: { x: state.transform[0], y: state.transform[1], zoom: state.transform[2] },
      target: viewport,
      canvas: { width, height },
      layout: layout ? { width: layout.width, height: layout.height } : undefined,
      panZoomReady: Boolean(panZoom),
    });
    if (!viewport || !panZoom) {
      if (import.meta.env.DEV) console.warn(`[Baley canvas] ${action}:skipped`, { viewport, panZoomReady: Boolean(panZoom) });
      return;
    }
    try {
      const result = await panZoom.setViewport(viewport);
      const latest = store.getState();
      const renderer = latest.domNode?.querySelector<HTMLElement & { __zoom?: unknown }>(".react-flow__renderer");
      const viewportElement = latest.domNode?.querySelector<HTMLElement>(".react-flow__viewport");
      if (result && renderer) renderer.__zoom = result;
      store.setState({ transform: [viewport.x, viewport.y, viewport.zoom] });
      if (viewportElement) viewportElement.style.transform = `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})`;
      window.requestAnimationFrame(() => {
        const after = store.getState();
        const renderedViewport = after.domNode?.querySelector<HTMLElement>(".react-flow__viewport");
        traceCanvas(`${action}:applied`, {
          result,
          store: { x: after.transform[0], y: after.transform[1], zoom: after.transform[2] },
          panZoom: after.panZoom?.getViewport(),
          domTransform: renderedViewport?.style.transform,
          rendererZoomStateUpdated: Boolean(result && renderer),
        });
      });
    } catch (error) {
      console.error(`[Baley canvas] ${action}:failed`, error);
    }
  };
  const zoomBy = (factor: number) => {
    const state = store.getState();
    const { width, height } = canvasSize();
    void apply(factor > 1 ? "zoom-in" : "zoom-out", zoomViewportAtCenter(
      { x: state.transform[0], y: state.transform[1], zoom: state.transform[2] },
      factor,
      width,
      height,
      state.minZoom,
      state.maxZoom,
    ));
  };
  const fit = () => {
    if (!layout) return;
    const state = store.getState();
    const { width, height } = canvasSize();
    void apply("fit", fitViewportToCanvas(layout.width, layout.height, width, height, state.minZoom, state.maxZoom));
  };
  return <Panel position="bottom-left" className="canvas-controls" aria-label="Viewport controls">
    <button type="button" aria-label="Zoom in" title="Zoom in" disabled={zoom >= maxZoom - 0.001} onClick={() => zoomBy(1.2)}><Plus size={17} /></button>
    <button type="button" aria-label="Zoom out" title="Zoom out" disabled={zoom <= minZoom + 0.001} onClick={() => zoomBy(1 / 1.2)}><Minus size={17} /></button>
    <button type="button" aria-label="Fit view" title="Fit view" onClick={fit}><Maximize size={15} /></button>
  </Panel>;
}

function CanvasOverlay({ graph, layout, view, navigate, laneColors, onOpenBacklog, onSelectBacklog, setBacklogExpandButton }: { graph: WorkspaceFixture; layout?: GraphLayout; view: ViewSpec; navigate: (view: ViewSpec) => void; laneColors: Record<string, string>; onOpenBacklog: () => void; onSelectBacklog: (item: BacklogItem) => void; setBacklogExpandButton: React.RefCallback<HTMLButtonElement> }) {
  const focusedLaneId = view.kind === "lane" ? view.id : undefined;
  const band = focusedLaneId && layout ? laneBandRect(layout, focusedLaneId) : undefined;
  return <div className="graph-overlay" style={{ width: layout?.width, height: layout?.height }}>
    {layout?.phaseRects.map((rect, index) => {
      const phase = graph.phases.find((item) => item.id === rect.id);
      return <div key={rect.id} className={`phase-container phase-${phase?.state}`} style={{ left: rect.x, top: rect.y, width: rect.width, height: rect.height }}><span>PHASE {String(index + 1).padStart(2, "0")} · {phase?.state}</span><strong>{phase?.name}</strong></div>;
    })}
    {band && <div className="lane-focus-band" style={{ left: band.x, top: band.y, width: band.width, height: band.height, "--lane-color": laneColors[focusedLaneId!] ?? "#579bfc" } as React.CSSProperties} />}
    {layout && graph.gates.map((gate) => {
      const position = layout.gatePositions.get(gate.id);
      const nextPhase = layout.phaseRects.find((phase) => phase.id === gate.toPhaseId);
      const previousPhase = layout.phaseRects.find((phase) => phase.id === gate.fromPhaseId);
      if (!position || !nextPhase || !previousPhase) return null;
      return <div key={`${gate.id}-corridor`} className="gate-corridor" style={{ left: previousPhase.x + previousPhase.width, top: 0, width: nextPhase.x - (previousPhase.x + previousPhase.width), height: layout.height }} />;
    })}
    {layout && view.kind !== "gate" && <LaneAnchorColumn lanes={graph.lanes} layout={layout} />}
    {layout && view.kind !== "gate" && <BacklogRail lanes={graph.lanes} items={graph.backlogItems} layout={layout} focusedLaneId={focusedLaneId} laneColors={laneColors} onExpand={onOpenBacklog} onSelect={onSelectBacklog} expandButtonRef={setBacklogExpandButton} />}
    {view.kind !== "gate" && graph.lanes.map((lane, index) => {
      const focused = view.kind === "lane" && lane.id === view.id;
      return <button type="button" key={lane.id} className={`lane-label ${focused ? "focused" : ""} ${view.kind === "lane" && !focused ? "dimmed" : ""}`} aria-label={`Open ${lane.name} lane`} aria-current={focused ? "true" : undefined} style={{ top: layout ? laneLabelTop(layout, lane.id) : 0, "--lane-color": laneColors[lane.id] } as React.CSSProperties} onClick={() => navigate({ kind: "lane", id: lane.id })}><span className="lane-number">LANE {String(index + 1).padStart(2, "0")}</span><strong className="lane-name">{lane.name}</strong><small className="lane-lifecycle">{lane.lifecycle}</small><ChevronRight size={15} /></button>;
    })}
  </div>;
}
