import { useEffect, useMemo, useState } from "react";
import { Background, Panel, ReactFlow, ViewportPortal, useStore, useStoreApi, type Edge, type Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ChevronRight, Maximize, Minus, PanelRightClose, PanelRightOpen, Plus, RotateCcw } from "lucide-react";
import { fetchGraph } from "./api/client";
import { canvasKey, connectedTaskIds, defaultGateFocusId, laneFocusTaskIds, visibleTaskIds, type ViewSpec } from "./graph/projection";
import { laneBandRect, laneLabelTop, layoutGraph, type GraphLayout } from "./graph/layout";
import { fitViewportToCanvas, zoomViewportAtCenter } from "./graph/viewport";
import { INSPECTOR_DEFAULT_WIDTH, INSPECTOR_MAX_WIDTH, INSPECTOR_MIN_WIDTH, inspectorWidthFromKey, inspectorWidthFromPointer } from "./layout/inspector";
import { TaskNode } from "./components/TaskNode";
import { GateNode } from "./components/GateNode";
import type { GateLinkKind, Task, WorkspaceFixture } from "./domain/model";

const nodeTypes = { task: TaskNode, gate: GateNode };
const MIN_ZOOM = 0.55;
const MAX_ZOOM = 1.55;

function traceCanvas(event: string, details: Record<string, unknown>) {
  if (import.meta.env.DEV) console.info(`[Baley canvas] ${event}`, details);
}
const laneColors: Record<string, string> = {
  server: "#00a887",
  client: "#579bfc",
  art: "#fdab3d",
  research: "#a25ddc",
};

function viewFromLocation(): ViewSpec {
  const lane = location.pathname.match(/^\/lanes\/([^/]+)/)?.[1];
  const gate = location.pathname.match(/^\/gates\/([^/]+)/)?.[1];
  return lane ? { kind: "lane", id: lane } : gate ? { kind: "gate", id: gate } : { kind: "multi" };
}

export default function App() {
  const [fixture, setFixture] = useState<WorkspaceFixture | undefined>();
  const [loadError, setLoadError] = useState<string>();
  const graph: WorkspaceFixture = fixture ?? { workspace: { id: "", name: "Baley", revision: 0 }, phases: [], lanes: [], tasks: [], dependencies: [], gates: [], gateLinks: [], decisions: [] };
  const [view, setView] = useState<ViewSpec>(viewFromLocation);
  const [selectedId, setSelectedId] = useState<string | undefined>(() => new URLSearchParams(location.search).get("task") ?? undefined);
  const [layout, setLayout] = useState<GraphLayout | undefined>();
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [inspectorWidth, setInspectorWidth] = useState(INSPECTOR_DEFAULT_WIDTH);
  const visible = useMemo(() => visibleTaskIds(graph, view), [graph, view]);
  const laneFocus = useMemo(
    () => view.kind === "lane" ? laneFocusTaskIds(graph, view.id) : undefined,
    [graph, view],
  );
  const connected = useMemo(() => selectedId ? connectedTaskIds(graph, selectedId) : undefined, [graph, selectedId]);

  useEffect(() => {
    let active = true;
    void layoutGraph(graph, visible).then((nextLayout) => { if (active) setLayout(nextLayout); });
    return () => { active = false; };
  }, [graph, visible]);
  useEffect(() => {
    let active = true;
    const refresh = () => void fetchGraph().then((next) => {
      if (!active) return;
      setFixture((current) => current && JSON.stringify(current) === JSON.stringify(next) ? current : next);
      setLoadError(undefined);
    }).catch((error: unknown) => { if (active) setLoadError(error instanceof Error ? error.message : "Server unavailable"); });
    refresh();
    const timer = window.setInterval(refresh, 2000);
    window.addEventListener("focus", refresh);
    return () => { active = false; window.clearInterval(timer); window.removeEventListener("focus", refresh); };
  }, []);
  useEffect(() => {
    const path = view.kind === "multi" ? "/" : view.kind === "lane" ? `/lanes/${view.id}` : `/gates/${view.id}`;
    const query = selectedId ? `?task=${selectedId}` : "";
    window.history.replaceState({}, "", path + query);
  }, [view, selectedId]);
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
      return { id: gate.id, type: "gate", position: layout?.gatePositions.get(gate.id) ?? { x: 0, y: 0 }, selected: gate.id === selectedId, data: { title: gate.name, status: gate.status, summary: `${done}/${required.length} conditions satisfied`, dimmed: Boolean(selectedId && selectedId !== gate.id && !graph.gateLinks.some((link) => link.gateId === gate.id && link.taskId === selectedId)) } };
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
  const defaultLaneId = graph.lanes.find((lane) => lane.name === "Client")?.id ?? graph.lanes[0]?.id;
  const defaultGateId = defaultGateFocusId(graph);
  const navigate = (next: ViewSpec) => { setView(next); setSelectedId(undefined); };

  if (!fixture && !loadError) return <main className="server-state"><h1>Baley</h1><p>Workspace graph를 불러오는 중입니다…</p></main>;
  if (!fixture && loadError) return <main className="server-state error"><h1>Server unavailable</h1><p>{loadError}</p><small>Viewer는 fixture로 대체 표시하지 않습니다.</small></main>;
  return (
    <main className="app-shell">
      <header className="topbar">
        <button type="button" className="brand" aria-label="Go to Home" onClick={() => navigate({ kind: "multi" })}><div className="brand-mark">B</div><div><strong>Baley</strong><span>Visual MVP</span></div></button>
        <nav className="view-tabs" aria-label="Graph views">
          <button className={view.kind === "multi" ? "active" : ""} onClick={() => navigate({ kind: "multi" })}>Multi-lane</button>
          <button className={view.kind === "lane" ? "active" : ""} disabled={!defaultLaneId} onClick={() => defaultLaneId && navigate({ kind: "lane", id: view.kind === "lane" ? view.id : defaultLaneId })}>Lane focus</button>
          <button className={view.kind === "gate" ? "active" : ""} disabled={!defaultGateId} onClick={() => defaultGateId && navigate({ kind: "gate", id: view.kind === "gate" ? view.id : defaultGateId })}>Gate focus</button>
        </nav>
        <button className="icon-button" aria-label="Toggle inspector" onClick={() => setInspectorOpen((open) => !open)}>{inspectorOpen ? <PanelRightClose size={18} /> : <PanelRightOpen size={18} />}</button>
      </header>

      <section className={`workspace ${inspectorOpen ? "with-inspector" : ""}`} style={{ "--inspector-width": `${inspectorWidth}px` } as React.CSSProperties}>
        <div className="graph-wrap">
          <div className="context-row"><div><button type="button" className="workspace-home-link" aria-label="Go to Workspace Home" onClick={() => navigate({ kind: "multi" })}>WORKSPACE · REVISION {graph.workspace.revision}</button><h1>{view.kind === "multi" ? graph.workspace.name : view.kind === "lane" ? `${graph.lanes.find((lane) => lane.id === view.id)?.name} lane` : `${graph.gates.find((gate) => gate.id === view.id)?.name ?? "Unknown"} gate`}</h1></div><div className="context-actions">{loadError && <span className="poll-error">refresh failed</span>}<span className="readonly-badge">READ ONLY</span><button className="quiet-button" onClick={() => setSelectedId(undefined)}><RotateCcw size={14} /> Clear focus</button></div></div>
          <div className="graph-canvas">
            <ReactFlow key={canvasKey(view)} nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodeClick={(_, node) => setSelectedId(node.id)} onMoveEnd={(_, nextViewport) => traceCanvas("move:end", nextViewport)} minZoom={MIN_ZOOM} maxZoom={MAX_ZOOM} nodesDraggable={false} proOptions={{ hideAttribution: true }}>
              <Background color="#d8d6ce" gap={24} size={1} />
              <ViewportPortal><CanvasOverlay graph={graph} layout={layout} view={view} navigate={navigate} /></ViewportPortal>
              <CanvasControls layout={layout} />
            </ReactFlow>
          </div>
        </div>
        {inspectorOpen && <div className="inspector-panel">
          <InspectorResizeHandle width={inspectorWidth} onWidth={setInspectorWidth} />
          <Inspector fixture={graph} task={selectedTask} gateId={selectedGate?.id} onLane={(id) => navigate({ kind: "lane", id })} onGate={(id) => navigate({ kind: "gate", id })} />
        </div>}
      </section>
    </main>
  );
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

function Inspector({ fixture, task, gateId, onLane, onGate }: { fixture: WorkspaceFixture; task?: Task; gateId?: string; onLane: (id: string) => void; onGate: (id: string) => void }) {
  if (gateId) {
    const gate = fixture.gates.find((item) => item.id === gateId)!;
    const links = fixture.gateLinks.filter((link) => link.gateId === gateId);
    return <aside className="inspector"><div className="inspector-kicker">GATE INSPECTOR</div><h2>{gate.name}</h2><span className={`status-pill status-${gate.status}`}>{gate.status}</span><p>Build Phase를 완료하고 Validate Phase로 진입하기 위한 동기화 지점입니다.</p><Section title="Conditions">{links.map((link) => <div className="relation-row" key={`${link.taskId}-${link.kind}`}><span>{link.satisfied ? link.satisfactionReason : link.kind}</span><strong>{fixture.tasks.find((task) => task.id === link.taskId)?.title}</strong></div>)}</Section><button className="primary-button" onClick={() => onGate(gate.id)}>Open gate focus</button></aside>;
  }
  if (!task) return <aside className="inspector empty"><div className="empty-symbol">↗</div><h2>Follow the work</h2><p>Task 또는 Gate를 선택하면 현재 상태와 연결 관계를 확인할 수 있습니다.</p><div className="legend"><span><i className="dot done" />Done</span><span><i className="dot running" />Running</span><span><i className="dot blocked" />Blocked</span><span><i className="dot ready" />Ready</span></div></aside>;
  const lane = fixture.lanes.find((item) => item.id === task.laneId)!;
  const phase = fixture.phases.find((item) => item.id === task.phaseId)!;
  const gateLinks = fixture.gateLinks.filter((link) => link.taskId === task.id);
  const upstream = fixture.dependencies.filter((edge) => edge.toTaskId === task.id);
  const downstream = fixture.dependencies.filter((edge) => edge.fromTaskId === task.id);
  const runs = (fixture.runs ?? []).filter((run) => run.taskId === task.id);
  const records = (fixture.records ?? []).filter((record) => record.taskId === task.id);
  return <aside className="inspector"><div className="inspector-kicker">TASK INSPECTOR</div><div className="inspector-id">TASK #{task.publicId}</div><h2>{task.title}</h2><span className={`status-pill status-${task.status}`}>{task.status}</span><p>{task.description}</p><Section title="Context"><button className="text-link" onClick={() => onLane(lane.id)}>{lane.name} lane</button><span className="meta-value">{phase.name} Phase</span></Section>{task.currentSummary && <Section title="Current summary"><span className="evidence-copy">{task.currentSummary}</span></Section>}{task.nextAction && <Section title="Next action"><span className="evidence-copy">{task.nextAction}</span></Section>}{task.implementedAssessment && <Section title="Implementation assessment"><span className="evidence-copy">{task.implementedAssessment}</span></Section>}{task.blocker && <Section title="Blocker"><div className="blocker-box">{task.blocker}</div></Section>}<Section title="Flow">{upstream.map((edge) => <div className="relation-row" key={edge.id}><span>from</span><strong>#{fixture.tasks.find((item) => item.id === edge.fromTaskId)?.publicId} {fixture.tasks.find((item) => item.id === edge.fromTaskId)?.title}</strong></div>)}{downstream.map((edge) => <div className="relation-row" key={edge.id}><span>to</span><strong>#{fixture.tasks.find((item) => item.id === edge.toTaskId)?.publicId} {fixture.tasks.find((item) => item.id === edge.toTaskId)?.title}</strong></div>)}{!upstream.length && !downstream.length && <span className="muted">Independent path</span>}</Section>{gateLinks.length > 0 && <Section title="Gate relations">{gateLinks.map((link) => <button className="relation-row clickable" key={link.gateId} onClick={() => onGate(link.gateId)}><span>{link.kind}</span><strong>{fixture.gates.find((gate) => gate.id === link.gateId)?.name}</strong></button>)}</Section>}<Section title="Runs">{runs.map((run) => <div className="evidence-row" key={run.id}><div><strong>{run.kind.replaceAll("_", " ")}</strong><span>{run.status}</span></div>{(run.resultSummary || run.errorSummary) && <p>{run.resultSummary || run.errorSummary}</p>}</div>)}{runs.length === 0 && <span className="muted">No Runs recorded</span>}</Section><Section title="Task Records">{records.map((record) => <div className="evidence-row" key={record.id}><div><strong>{record.recordType}</strong><span>{record.state}</span></div><code>{record.relativePath}</code><p>{record.shortSummary}</p></div>)}{records.length === 0 && <span className="muted">No Task Records indexed</span>}</Section><section className="command-hint"><strong>LLM command only</strong><p>Use Baley Skill commands to update task #{task.publicId}.</p></section></aside>;
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
      window.requestAnimationFrame(() => {
        const after = store.getState();
        const viewportElement = after.domNode?.querySelector<HTMLElement>(".react-flow__viewport");
        traceCanvas(`${action}:applied`, {
          result,
          store: { x: after.transform[0], y: after.transform[1], zoom: after.transform[2] },
          panZoom: after.panZoom?.getViewport(),
          domTransform: viewportElement?.style.transform,
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

function CanvasOverlay({ graph, layout, view, navigate }: { graph: WorkspaceFixture; layout?: GraphLayout; view: ViewSpec; navigate: (view: ViewSpec) => void }) {
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
    {view.kind !== "gate" && graph.lanes.map((lane, index) => {
      const focused = view.kind === "lane" && lane.id === view.id;
      return <div key={lane.id} className={`lane-label ${focused ? "focused" : ""} ${view.kind === "lane" && !focused ? "dimmed" : ""}`} aria-current={focused ? "true" : undefined} style={{ top: layout ? laneLabelTop(layout, lane.id) : 0, "--lane-color": laneColors[lane.id] } as React.CSSProperties} onClick={() => navigate({ kind: "lane", id: lane.id })}><span>{String(index + 1).padStart(2, "0")}</span><strong>{lane.name}</strong><small>{lane.lifecycle}</small><ChevronRight size={14} /></div>;
    })}
  </div>;
}
