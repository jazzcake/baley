import { useEffect, useMemo, useState } from "react";
import { Background, Controls, ReactFlow, type Edge, type Node, type Viewport } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ChevronRight, PanelRightClose, PanelRightOpen, RotateCcw } from "lucide-react";
import { pilotReadyFixture as fixture } from "./fixtures/pilot-ready";
import { validateFixture } from "./domain/validate-fixture";
import { connectedTaskIds, laneFocusTaskIds, visibleTaskIds, type ViewSpec } from "./graph/projection";
import { layoutGraph, type GraphLayout } from "./graph/layout";
import { TaskNode } from "./components/TaskNode";
import { GateNode } from "./components/GateNode";
import type { GateLinkKind, Task } from "./domain/model";

validateFixture(fixture);
const nodeTypes = { task: TaskNode, gate: GateNode };
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
  const [view, setView] = useState<ViewSpec>(viewFromLocation);
  const [selectedId, setSelectedId] = useState<string | undefined>(() => new URLSearchParams(location.search).get("task") ?? undefined);
  const [layout, setLayout] = useState<GraphLayout | undefined>();
  const [inspectorOpen, setInspectorOpen] = useState(true);
  const [viewport, setViewport] = useState({ x: 0, y: 0, zoom: 1 });
  const visible = useMemo(() => visibleTaskIds(fixture, view), [view]);
  const laneFocus = useMemo(
    () => view.kind === "lane" ? laneFocusTaskIds(fixture, view.id) : undefined,
    [view],
  );
  const connected = useMemo(() => selectedId ? connectedTaskIds(fixture, selectedId) : undefined, [selectedId]);

  useEffect(() => { void layoutGraph(fixture, visible).then(setLayout); }, [visible]);
  useEffect(() => {
    const path = view.kind === "multi" ? "/" : view.kind === "lane" ? `/lanes/${view.id}` : `/gates/${view.id}`;
    const query = selectedId ? `?task=${selectedId}` : "";
    history.replaceState({}, "", path + query);
  }, [view, selectedId]);

  const nodes = useMemo<Node[]>(() => {
    const taskNodes: Node[] = fixture.tasks.filter((task) => visible.has(task.id)).map((task) => ({
      id: task.id, type: "task", position: layout?.taskPositions.get(task.id) ?? { x: 0, y: 0 }, selected: task.id === selectedId,
      data: {
        title: task.title,
        status: task.status,
        lane: fixture.lanes.find((lane) => lane.id === task.laneId)?.name ?? "",
        laneColor: laneColors[task.laneId] ?? "#579bfc",
        laneFocused: view.kind === "lane" && task.laneId === view.id,
        dimmed: Boolean(
          (laneFocus && !laneFocus.has(task.id)) ||
          (connected && !connected.has(task.id)),
        ),
        external: view.kind === "lane" && task.laneId !== view.id,
      },
    }));
    const gateNodes: Node[] = fixture.gates.filter((gate) => view.kind !== "lane" || fixture.gateLinks.some((link) => link.gateId === gate.id && visible.has(link.taskId))).map((gate) => {
      const required = fixture.gateLinks.filter((link) => link.gateId === gate.id && link.kind === "required");
      const done = required.filter((link) => fixture.tasks.find((task) => task.id === link.taskId)?.status === "done").length;
      return { id: gate.id, type: "gate", position: layout?.gatePositions.get(gate.id) ?? { x: 0, y: 0 }, selected: gate.id === selectedId, data: { title: gate.name, status: gate.status, summary: `${done}/${required.length} required ready`, dimmed: Boolean(selectedId && selectedId !== gate.id && !fixture.gateLinks.some((link) => link.gateId === gate.id && link.taskId === selectedId)) } };
    });
    return [...taskNodes, ...gateNodes];
  }, [visible, layout, selectedId, connected, laneFocus, view]);

  const edges = useMemo<Edge[]>(() => {
    const dependencies: Edge[] = fixture.dependencies.filter((edge) => visible.has(edge.fromTaskId) && visible.has(edge.toTaskId)).map((edge) => ({
      id: edge.id,
      source: edge.fromTaskId,
      target: edge.toTaskId,
      className:
        (laneFocus && (!laneFocus.has(edge.fromTaskId) || !laneFocus.has(edge.toTaskId))) ||
        (connected && (!connected.has(edge.fromTaskId) || !connected.has(edge.toTaskId)))
          ? "edge-dimmed"
          : "dependency-edge",
      animated: fixture.tasks.find((task) => task.id === edge.toTaskId)?.status === "running",
    }));
    const colors: Record<GateLinkKind, string> = { required: "#8d5f39", reference: "#8b8b82", unlocks: "#366b62" };
    const gateEdges: Edge[] = fixture.gateLinks.filter((link) => visible.has(link.taskId)).map((link) => ({
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
  }, [visible, connected, laneFocus, selectedId]);

  const selectedTask = fixture.tasks.find((task) => task.id === selectedId);
  const selectedGate = fixture.gates.find((gate) => gate.id === selectedId);
  const navigate = (next: ViewSpec) => { setView(next); setSelectedId(undefined); };

  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="brand"><div className="brand-mark">B</div><div><strong>Baley</strong><span>Visual MVP</span></div></div>
        <nav className="view-tabs" aria-label="Graph views">
          <button className={view.kind === "multi" ? "active" : ""} onClick={() => navigate({ kind: "multi" })}>Multi-lane</button>
          <button className={view.kind === "lane" ? "active" : ""} onClick={() => navigate({ kind: "lane", id: view.kind === "lane" ? view.id : "client" })}>Lane focus</button>
          <button className={view.kind === "gate" ? "active" : ""} onClick={() => navigate({ kind: "gate", id: "pilot-ready" })}>Gate focus</button>
        </nav>
        <button className="icon-button" aria-label="Toggle inspector" onClick={() => setInspectorOpen((open) => !open)}>{inspectorOpen ? <PanelRightClose size={18} /> : <PanelRightOpen size={18} />}</button>
      </header>

      <section className={`workspace ${inspectorOpen ? "with-inspector" : ""}`}>
        <div className="graph-wrap">
          <div className="context-row"><div><span>WORKSPACE</span><h1>{view.kind === "multi" ? "Pilot delivery" : view.kind === "lane" ? `${fixture.lanes.find((lane) => lane.id === view.id)?.name} lane` : "Pilot Ready gate"}</h1></div><button className="quiet-button" onClick={() => setSelectedId(undefined)}><RotateCcw size={14} /> Clear focus</button></div>
          <div className="graph-canvas">
            <div className="graph-overlay" style={{ width: layout?.width, height: layout?.height, transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})` }}>
              {layout?.phaseRects.map((rect, index) => {
                const phase = fixture.phases.find((item) => item.id === rect.id);
                return <div key={rect.id} className="phase-container" style={{ left: rect.x, top: rect.y, width: rect.width, height: rect.height }}><span>PHASE {String(index + 1).padStart(2, "0")}</span><strong>{phase?.name}</strong></div>;
              })}
              {layout && fixture.gates.map((gate) => {
                const position = layout.gatePositions.get(gate.id);
                const nextPhase = layout.phaseRects.find((phase) => phase.id === gate.toPhaseId);
                const previousPhase = layout.phaseRects.find((phase) => phase.id === gate.fromPhaseId);
                if (!position || !nextPhase || !previousPhase) return null;
                return <div key={`${gate.id}-corridor`} className="gate-corridor" style={{ left: previousPhase.x + previousPhase.width, top: 0, width: nextPhase.x - (previousPhase.x + previousPhase.width), height: layout.height }} />;
              })}
              {view.kind !== "gate" && fixture.lanes.map((lane, index) => <div key={lane.id} className={`lane-label ${view.kind === "lane" && lane.id !== view.id ? "dimmed" : ""}`} style={{ top: layout?.lanePositions.get(lane.id) ?? 0, "--lane-color": laneColors[lane.id] } as React.CSSProperties} onClick={() => navigate({ kind: "lane", id: lane.id })}><span>{String(index + 1).padStart(2, "0")}</span><strong>{lane.name}</strong><small>{lane.lifecycle}</small><ChevronRight size={14} /></div>)}
            </div>
            <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodeClick={(_, node) => setSelectedId(node.id)} onMove={(_event: MouseEvent | TouchEvent | null, nextViewport: Viewport) => setViewport(nextViewport)} fitView fitViewOptions={{ padding: 0.16 }} minZoom={0.55} maxZoom={1.55} proOptions={{ hideAttribution: true }}>
              <Background color="#d8d6ce" gap={24} size={1} />
              <Controls showInteractive={false} position="bottom-left" />
            </ReactFlow>
          </div>
        </div>
        {inspectorOpen && <Inspector task={selectedTask} gateId={selectedGate?.id} onLane={(id) => navigate({ kind: "lane", id })} onGate={(id) => navigate({ kind: "gate", id })} />}
      </section>
    </main>
  );
}

function Inspector({ task, gateId, onLane, onGate }: { task?: Task; gateId?: string; onLane: (id: string) => void; onGate: (id: string) => void }) {
  if (gateId) {
    const gate = fixture.gates.find((item) => item.id === gateId)!;
    const links = fixture.gateLinks.filter((link) => link.gateId === gateId);
    return <aside className="inspector"><div className="inspector-kicker">GATE INSPECTOR</div><h2>{gate.name}</h2><span className="status-pill status-open">{gate.status}</span><p>Build Phase를 완료하고 Validate Phase로 진입하기 위한 동기화 지점입니다.</p><Section title="Conditions">{links.map((link) => <div className="relation-row" key={`${link.taskId}-${link.kind}`}><span>{link.kind}</span><strong>{fixture.tasks.find((task) => task.id === link.taskId)?.title}</strong></div>)}</Section><button className="primary-button" onClick={() => onGate(gate.id)}>Open gate focus</button></aside>;
  }
  if (!task) return <aside className="inspector empty"><div className="empty-symbol">↗</div><h2>Follow the work</h2><p>Task 또는 Gate를 선택하면 현재 상태와 연결 관계를 확인할 수 있습니다.</p><div className="legend"><span><i className="dot done" />Done</span><span><i className="dot running" />Running</span><span><i className="dot blocked" />Blocked</span><span><i className="dot ready" />Ready</span></div></aside>;
  const lane = fixture.lanes.find((item) => item.id === task.laneId)!;
  const phase = fixture.phases.find((item) => item.id === task.phaseId)!;
  const gateLinks = fixture.gateLinks.filter((link) => link.taskId === task.id);
  const upstream = fixture.dependencies.filter((edge) => edge.toTaskId === task.id).map((edge) => fixture.tasks.find((item) => item.id === edge.fromTaskId)?.title);
  const downstream = fixture.dependencies.filter((edge) => edge.fromTaskId === task.id).map((edge) => fixture.tasks.find((item) => item.id === edge.toTaskId)?.title);
  return <aside className="inspector"><div className="inspector-kicker">TASK INSPECTOR</div><h2>{task.title}</h2><span className={`status-pill status-${task.status}`}>{task.status}</span><p>{task.description}</p><Section title="Context"><button className="text-link" onClick={() => onLane(lane.id)}>{lane.name} lane</button><span className="meta-value">{phase.name} Phase</span></Section>{task.blocker && <Section title="Blocker"><div className="blocker-box">{task.blocker}</div></Section>}<Section title="Flow">{upstream.map((value) => <div className="relation-row" key={value}><span>from</span><strong>{value}</strong></div>)}{downstream.map((value) => <div className="relation-row" key={value}><span>to</span><strong>{value}</strong></div>)}{!upstream.length && !downstream.length && <span className="muted">독립 경로</span>}</Section>{gateLinks.length > 0 && <Section title="Gate relations">{gateLinks.map((link) => <button className="relation-row clickable" key={link.gateId} onClick={() => onGate(link.gateId)}><span>{link.kind}</span><strong>Pilot Ready</strong></button>)}</Section>}</aside>;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) { return <section className="inspector-section"><h3>{title}</h3>{children}</section>; }
