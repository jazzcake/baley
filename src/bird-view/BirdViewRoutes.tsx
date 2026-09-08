import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent as ReactMouseEvent } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
  type NodeProps,
  type ReactFlowInstance,
  type Viewport,
} from "@xyflow/react";
import { ArrowLeft, Edit3, Focus, Plus, Save, Trash2, X } from "lucide-react";
import { Link, Navigate, Route, Routes, useNavigate, useParams } from "react-router-dom";
import { APIError } from "../api/http";
import { useAuth } from "../auth/AuthProvider";
import { executeBirdViewCommand, fetchBirdViewGraph, fetchBirdViewNodeFocus, listBirdViews } from "./api";
import { BirdViewNode, type BirdViewFlowNode } from "./BirdViewNode";
import { layoutBirdView, semanticLevel } from "./layout";
import type { BirdView, BirdViewEdge, BirdViewFocusWorkspace, BirdViewGraph, BirdViewNode as BirdNode, BirdViewNodeFocus } from "./model";

export const BIRD_VIEW_PLANNING_GRID = {
  minor: { id: "bird-v2-planning-grid-minor", gap: 28, lineWidth: 0.55, color: "#dbe4ef" },
  major: { id: "bird-v2-planning-grid-major", gap: 140, lineWidth: 0.9, color: "#c2cfdf" },
} as const;
type Loadable<T> = { state: "loading" } | { state: "ready"; value: T } | { state: "error"; message: string };
type FocusNodeData = { label: string; kind: "task" | "gate" | "phase"; status?: string };
type FocusNode = Node<FocusNodeData>;
type FocusTransition = { nodeId: string; overviewPosition: { x: number; y: number }; anchorPosition: { x: number; y: number }; viewport?: Viewport };
type FocusPhase = "overview" | "entering" | "focused" | "exiting-start" | "exiting";
type FocusLayoutBounds = { x: number; y: number; width: number; height: number };

const FOCUS_LAYOUT = {
  columnOffsetX: 300,
  columnStepX: 290,
  taskOffsetY: 78,
  taskRowStepY: 104,
  gateGapY: 34,
  gateRowStepY: 96,
  phaseWidth: 260,
  phaseHeight: 34,
  cardWidth: 250,
  cardHeight: 72,
} as const;

function focusNode(id: string, position: { x: number; y: number }, data: FocusNodeData): FocusNode {
  const phase = data.kind === "phase";
  const width = phase ? FOCUS_LAYOUT.phaseWidth : FOCUS_LAYOUT.cardWidth;
  const height = phase ? FOCUS_LAYOUT.phaseHeight : FOCUS_LAYOUT.cardHeight;
  return { id, position, width, height, initialWidth: width, initialHeight: height, draggable: false, selectable: false, data };
}

function boundsForFocusNodes(nodes: FocusNode[]): FocusLayoutBounds | undefined {
  if (!nodes.length) return undefined;
  const left = Math.min(...nodes.map((node) => node.position.x));
  const top = Math.min(...nodes.map((node) => node.position.y));
  const right = Math.max(...nodes.map((node) => node.position.x + (node.width ?? FOCUS_LAYOUT.cardWidth)));
  const bottom = Math.max(...nodes.map((node) => node.position.y + (node.height ?? FOCUS_LAYOUT.cardHeight)));
  return { x: left, y: top, width: right - left, height: bottom - top };
}

function screenBounds(bounds: FocusLayoutBounds | undefined, viewport: Viewport): FocusLayoutBounds | undefined {
  if (!bounds) return undefined;
  return {
    x: bounds.x * viewport.zoom + viewport.x,
    y: bounds.y * viewport.zoom + viewport.y,
    width: bounds.width * viewport.zoom,
    height: bounds.height * viewport.zoom,
  };
}

function focusProjection(focus: BirdViewNodeFocus, anchorPosition: { x: number; y: number }) {
  const workspace = focus.workspaces[0];
  if (!workspace) return { nodes: [] as FocusNode[], edges: [] as Edge[], bounds: undefined };
  const visibleBindings = workspace.bindings.filter((binding) => binding.state !== "excluded");
  const directlyBound = (targetType: "phase" | "gate" | "task") => new Set(visibleBindings.filter((binding) => binding.targetType === targetType).map((binding) => binding.targetId));
  const taskIds = directlyBound("task");
  const gateIds = directlyBound("gate");
  const phaseIds = directlyBound("phase");
  // The focus endpoint is authoritative: retain only dependencies whose two ends are
  // already present.  This intentionally does not expand into a whole Workspace graph.
  const dependencies = workspace.dependencies.filter((edge) => taskIds.has(edge.fromTaskId) && taskIds.has(edge.toTaskId));
  workspace.tasks.filter((task) => taskIds.has(task.id)).forEach((task) => phaseIds.add(task.phaseId));
  // Gate membership is curated independently from Task membership. Automatic
  // Gate-entry projection and shared conditions must not invent a Node binding.
  const gates = workspace.gates.filter((gate) => gateIds.has(gate.id));
  gates.forEach((gate) => { phaseIds.add(gate.fromPhaseId); phaseIds.add(gate.toPhaseId); });
  const phases = workspace.phases.filter((phase) => phaseIds.has(phase.id)).sort((left, right) => left.position - right.position || left.id.localeCompare(right.id));
  const phaseColumns = new Map(phases.map((phase, index) => [phase.id, index]));
  const phaseX = (phaseId: string) => anchorPosition.x + FOCUS_LAYOUT.columnOffsetX + (phaseColumns.get(phaseId) ?? 0) * FOCUS_LAYOUT.columnStepX;
  const nodes: FocusNode[] = [];
  phases.forEach((phase) => nodes.push(focusNode(`phase:${phase.id}`, { x: phaseX(phase.id), y: anchorPosition.y }, { label: phase.name, kind: "phase" })));
  const taskRowsByPhase = new Map<string, number>();
  workspace.tasks.filter((task) => taskIds.has(task.id)).forEach((task) => {
    const row = taskRowsByPhase.get(task.phaseId) ?? 0;
    taskRowsByPhase.set(task.phaseId, row + 1);
    nodes.push(focusNode(`task:${task.id}`, { x: phaseX(task.phaseId) + 5, y: anchorPosition.y + FOCUS_LAYOUT.taskOffsetY + row * FOCUS_LAYOUT.taskRowStepY }, { label: `#${task.publicId} ${task.title}`, kind: "task", status: task.status }));
  });
  const taskRowCount = Math.max(1, ...taskRowsByPhase.values());
  const gateTop = anchorPosition.y + FOCUS_LAYOUT.taskOffsetY + taskRowCount * FOCUS_LAYOUT.taskRowStepY + FOCUS_LAYOUT.gateGapY;
  gates.forEach((gate, index) => {
    const fromCenter = phaseX(gate.fromPhaseId) + FOCUS_LAYOUT.phaseWidth / 2;
    const toCenter = phaseX(gate.toPhaseId) + FOCUS_LAYOUT.phaseWidth / 2;
    nodes.push(focusNode(`gate:${gate.id}`, { x: (fromCenter + toCenter) / 2 - FOCUS_LAYOUT.cardWidth / 2, y: gateTop + index * FOCUS_LAYOUT.gateRowStepY }, { label: `G#${gate.publicId} ${gate.name}`, kind: "gate", status: gate.status }));
  });
  const dependencyEdges: Edge[] = dependencies.map((edge) => ({ id: `dependency:${edge.fromTaskId}:${edge.toTaskId}`, source: `task:${edge.fromTaskId}`, target: `task:${edge.toTaskId}`, className: "dependency-edge", data: { relation: "dependency" } }));
  const gateEdges: Edge[] = gates.flatMap((gate) => [
    ...gate.conditions.filter(({ taskId }) => taskIds.has(taskId)).map(({ taskId }) => ({
      id: `gate:${gate.id}:condition:${taskId}`,
      source: `task:${taskId}`,
      target: `gate:${gate.id}`,
      className: "gate-edge gate-edge-required",
      style: { stroke: "#b87943" },
      data: { relation: "required" },
    })),
    ...gate.entryTasks.filter(({ taskId }) => taskIds.has(taskId)).map(({ taskId }) => ({
      id: `gate:${gate.id}:entry:${taskId}`,
      source: `gate:${gate.id}`,
      target: `task:${taskId}`,
      className: `gate-edge ${gate.status === "passed" ? "gate-edge-unlocked" : "gate-edge-locked"}`,
      style: { stroke: gate.status === "passed" ? "#16856c" : "#7257d9" },
      data: { relation: "unlocks", locked: gate.status !== "passed" },
    })),
  ]);
  return { nodes, edges: [...dependencyEdges, ...gateEdges], bounds: boundsForFocusNodes(nodes) };
}

function FocusNodeCard({ data }: NodeProps<FocusNode>) {
  return <article className={`bird-focus-node bird-focus-node-${data.kind} status-${data.status ?? "active"}`} data-focus-kind={data.kind}>
    <Handle type="target" position={Position.Left} />
    {data.label}
    <Handle type="source" position={Position.Right} />
  </article>;
}

const graphNodeTypes = { birdViewNode: BirdViewNode, focusCard: FocusNodeCard };

const messageFor = (error: unknown) => error instanceof APIError && error.status === 404
  ? "This Bird View is unavailable."
  : error instanceof Error ? error.message : "Bird View could not be loaded.";

function traceBird(event: string, details: Record<string, unknown>) {
  if (!import.meta.env.DEV) return;
  const traceWindow = window as Window & { __BALEY_BIRD_VIEW_TRACE__?: Array<{ event: string; details: Record<string, unknown> }> };
  traceWindow.__BALEY_BIRD_VIEW_TRACE__ = [...(traceWindow.__BALEY_BIRD_VIEW_TRACE__ ?? []).slice(-49), { event, details }];
  console.info(`[Bird View V2] ${event}`, details);
}

const prefersReducedMotion = () => window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;

function renderedNode(id: string) {
  return [...document.querySelectorAll<HTMLElement>("[data-id]")].find((element) => element.dataset.id === id);
}

function renderedPlanningGrid() {
  return [...document.querySelectorAll<SVGElement>(".bird-v2-canvas .bird-v2-planning-grid")].map((layer) => {
    const pattern = layer.querySelector("pattern");
    return {
      className: layer.getAttribute("class"),
      patternId: pattern?.id,
      width: pattern?.getAttribute("width"),
      height: pattern?.getAttribute("height"),
      transform: pattern?.getAttribute("patternTransform"),
      shape: pattern?.firstElementChild?.tagName,
    };
  });
}

function isBirdViewFlowNode(node: Node): node is BirdViewFlowNode {
  return node.type === "birdViewNode";
}

export function BirdViewRoutes() {
  return <Routes>
    <Route index element={<BirdViewEntry />} />
    <Route path=":birdViewId" element={<BirdViewCanvas />} />
    <Route path="*" element={<Navigate to="/bird-views" replace />} />
  </Routes>;
}

function BirdViewEntry() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [result, setResult] = useState<Loadable<BirdView[]>>({ state: "loading" });
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();
  useEffect(() => {
    const controller = new AbortController();
    listBirdViews(controller.signal).then((views) => {
      if (views[0]) navigate(`/bird-views/${encodeURIComponent(views[0].id)}`, { replace: true });
      else setResult({ state: "ready", value: [] });
    }).catch((cause) => { if (!controller.signal.aborted) setResult({ state: "error", message: messageFor(cause) }); });
    return () => controller.abort();
  }, [navigate]);
  if (auth.state.status !== "authenticated") return null;
  const session = auth.state;
  const create = async () => {
    setCreating(true); setError(undefined);
    const id = crypto.randomUUID();
    try {
      await executeBirdViewCommand({
        name: "bird_view.create",
        arguments: { birdViewId: id, title: "My Bird View", description: "" },
        envelope: { idempotencyKey: crypto.randomUUID(), executedByActorId: session.account.actorId },
      }, session.csrfToken);
      navigate(`/bird-views/${encodeURIComponent(id)}`, { replace: true });
    } catch (cause) { setError(messageFor(cause)); setCreating(false); }
  };
  return <main className="bird-v2-entry">
    <Link to="/workspaces" className="bird-v2-back"><ArrowLeft size={16} /> Workspaces</Link>
    {result.state === "loading" && <p>Opening Bird View...</p>}
    {result.state === "error" && <p role="alert">{result.message}</p>}
    {result.state === "ready" && <button type="button" className="bird-v2-create-first" onClick={() => void create()} disabled={creating}><Plus size={18} /> Create Bird View</button>}
    {error && <p role="alert">{error}</p>}
  </main>;
}

function BirdViewCanvas() {
  const { birdViewId = "" } = useParams();
  const auth = useAuth();
  const [result, setResult] = useState<Loadable<BirdViewGraph>>({ state: "loading" });
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string>();
  const [selectedEdgeId, setSelectedEdgeId] = useState<string>();
  const [focused, setFocused] = useState<Loadable<BirdViewNodeFocus> | undefined>();
  const [focusPhase, setFocusPhase] = useState<FocusPhase>("overview");
  const [focusTransition, setFocusTransition] = useState<FocusTransition>();
  const [error, setError] = useState<string>();
  const [zoom, setZoom] = useState(1);
  const [flow, setFlow] = useState<ReactFlowInstance>();
  const viewportRef = useRef<Viewport>();
  const clickTimer = useRef<number>();
  const revisionRef = useRef(0);
  const mutationQueue = useRef<Promise<unknown>>(Promise.resolve());
  const focusTimer = useRef<number>();
  const focusFrame = useRef<number>();
  const projection = useMemo(() => focused?.state === "ready" && focusTransition
    ? focusProjection(focused.value, focusTransition.anchorPosition)
    : { nodes: [] as FocusNode[], edges: [] as Edge[], bounds: undefined }, [focused, focusTransition]);

  const load = useCallback(async (signal?: AbortSignal) => {
    const value = await fetchBirdViewGraph(birdViewId, signal);
    const next = await layoutBirdView(value.nodes, value.edges, semanticLevel(zoom) === "detail");
    traceBird("graph-load:calculated", {
      event: "fetch-complete",
      apiGraphPayload: value,
      calculatedTargetState: { nodeIds: next.nodes.map((node) => node.id), edgeIds: next.edges.map((edge) => edge.id) },
      reactState: { zoom, semanticLevel: semanticLevel(zoom) },
      reactFlowControllerState: "not-committed",
      renderedDOMState: { edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length },
    });
    revisionRef.current = value.birdView.revision;
    setResult({ state: "ready", value });
    setNodes(next.nodes);
    setEdges(next.edges.map((edge) => ({ ...edge, selectable: true })));
    return value;
  }, [birdViewId]);

  useEffect(() => {
    const controller = new AbortController();
    load(controller.signal).catch((cause) => { if (!controller.signal.aborted) setResult({ state: "error", message: messageFor(cause) }); });
    return () => controller.abort();
  }, [load]);

  useEffect(() => {
    setNodes((current) => current.map((node) => ({ ...node, data: { ...node.data, detailed: semanticLevel(zoom) === "detail" } })));
  }, [zoom]);

  useEffect(() => {
    if (!import.meta.env.DEV || !flow) return;
    const frame = requestAnimationFrame(() => {
      const viewport = flow.getViewport();
      const canvasRect = document.querySelector<HTMLElement>(".bird-v2-canvas")?.getBoundingClientRect();
      traceBird("rendered-boundary", {
        event: "react-commit",
        calculatedTargetState: { focusPhase, focusNodeId: focusTransition?.nodeId, focusPayloadState: focused?.state ?? "none", focusLayoutBounds: projection.bounds, focusLayoutScreenBounds: screenBounds(projection.bounds, viewport) },
        reactState: { selectedNodeId, selectedEdgeId, focusPhase, nodeCount: nodes.length, edgeCount: edges.length },
        applicationStoreState: { birdViewId, graphState: result.state, birdViewRevision: revisionRef.current, focusPayloadState: focused?.state ?? "none" },
        reactFlowControllerState: { viewport, nodes: flow.getNodes().map((node) => ({ id: node.id, position: node.position })), edges: flow.getEdges().map((edge) => edge.id) },
        renderedDOMState: {
          focusPhase: document.querySelector(".bird-v2-canvas")?.getAttribute("data-focus-phase"),
          canvasBounds: canvasRect && { x: canvasRect.x, y: canvasRect.y, width: canvasRect.width, height: canvasRect.height },
          nodes: [...document.querySelectorAll<HTMLElement>(".bird-v2-canvas .react-flow__node")].map((node) => { const rect = node.getBoundingClientRect(); return { id: node.dataset.id, text: node.innerText, bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, bottom: rect.bottom, right: rect.right } }; }),
          edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length,
          nodeAddControlPresent: Boolean(document.querySelector(".bird-v2-add-node")),
          planningGrid: renderedPlanningGrid(),
        },
      });
    });
    return () => cancelAnimationFrame(frame);
  }, [birdViewId, edges, flow, focusPhase, focusTransition?.nodeId, focused, nodes, projection, result.state, selectedEdgeId, selectedNodeId]);

  useEffect(() => () => {
    window.clearTimeout(clickTimer.current);
    window.clearTimeout(focusTimer.current);
    if (focusFrame.current !== undefined) cancelAnimationFrame(focusFrame.current);
  }, []);

  const runMutation = useCallback(<T,>(name: string, args: Record<string, unknown>, optimistic: () => void, rollback?: () => void): Promise<T> => {
    if (auth.state.status !== "authenticated") return Promise.reject(new Error("Authentication unavailable"));
    optimistic(); setError(undefined);
    const actorId = auth.state.account.actorId;
    const csrfToken = auth.state.csrfToken;
    const operation = mutationQueue.current.then(async () => {
      const targetRevision = revisionRef.current;
      traceBird("mutation-boundary", { event: name, calculatedTargetState: args, reactState: { selectedNodeId, selectedEdgeId }, reactFlowControllerState: flow ? { viewport: flow.getViewport(), nodeCount: flow.getNodes().length, edgeCount: flow.getEdges().length } : "not-ready", renderedDOMState: { selectedNode: document.querySelector(".bird-node.selected")?.textContent } });
      try {
        const execution = await executeBirdViewCommand({ name, arguments: { birdViewId, ...args }, envelope: { idempotencyKey: crypto.randomUUID(), expectedBirdViewRevision: targetRevision, executedByActorId: actorId } }, csrfToken);
        revisionRef.current = execution.birdViewRevision;
        setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, birdView: { ...current.value.birdView, revision: execution.birdViewRevision } } } : current);
        return execution as T;
      } catch (cause) {
        rollback?.();
        setError(messageFor(cause));
        await load().catch(() => undefined);
        throw cause;
      }
    });
    mutationQueue.current = operation.catch(() => undefined);
    return operation.catch(() => undefined as T);
  }, [auth.state, birdViewId, flow, load, selectedEdgeId, selectedNodeId]);

  const onNodesChange = (changes: NodeChange[]) => {
    traceBird("nodes-change:event", {
      event: "react-flow-nodes-change",
      calculatedTargetState: { changes, persistence: focusPhase === "overview" ? "apply-to-overview" : "ignore-focus-projection" },
      reactState: { nodeCount: nodes.length, edgeCount: edges.length },
      reactFlowControllerState: flow ? { nodeCount: flow.getNodes().length, edgeCount: flow.getEdges().length } : "not-ready",
      renderedDOMState: { nodeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__node").length, edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length },
    });
    // Focus cards are a calculated projection, not editable Bird View nodes.
    // Applying their measurement events to the overview collection creates a
    // controlled-render loop and prevents React Flow's edge layer stabilizing.
    if (focusPhase !== "overview") return;
    setNodes((current) => applyNodeChanges(changes, current));
  };
  const onEdgesChange = (changes: EdgeChange[]) => {
    traceBird("edges-change:event", {
      event: "react-flow-edges-change",
      calculatedTargetState: changes,
      reactState: { nodeCount: nodes.length, edgeCount: edges.length },
      reactFlowControllerState: flow ? { nodeCount: flow.getNodes().length, edgeCount: flow.getEdges().length } : "not-ready",
      renderedDOMState: { edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length },
    });
    setEdges((current) => applyEdgeChanges(changes.filter((change) => change.type !== "remove"), current));
  };
  const onConnect = (connection: Connection) => {
    if (!connection.source || !connection.target || connection.source === connection.target) return;
    const id = crypto.randomUUID();
    // Workspace edges omit a type and use React Flow's default bezier
    // renderer; new Bird View connections follow the same grammar.
    const edge: Edge = { id, source: connection.source, target: connection.target, selectable: true };
    const model: BirdViewEdge = { id, fromNodeId: connection.source, toNodeId: connection.target, label: "", createdAt: "" };
    void runMutation("bird_view.edge.connect", { edgeId: id, fromNodeId: connection.source, toNodeId: connection.target, label: "" }, () => {
      setEdges((current) => [...current, edge]);
      setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, edges: [...current.value.edges, model] } } : current);
    }, () => setEdges((current) => current.filter((item) => item.id !== id)));
  };
  const onNodeDragStop = (_: unknown, node: Node) => {
    const previous = result.state === "ready" ? result.value.nodes.find((item) => item.id === node.id) : undefined;
    void runMutation("bird_view.node.update", { nodeId: node.id, positionX: node.position.x, positionY: node.position.y }, () => setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, nodes: current.value.nodes.map((item) => item.id === node.id ? { ...item, positionX: node.position.x, positionY: node.position.y } : item) } } : current), () => previous && setNodes((current) => current.map((item) => item.id === node.id ? { ...item, position: { x: previous.positionX, y: previous.positionY } } : item)));
  };
  const selectNode = (event: { isTrusted: boolean; detail: number }, node: Node) => {
    window.clearTimeout(clickTimer.current);
    clickTimer.current = window.setTimeout(() => {
      traceBird("node-single-click", {
        event: "click",
        userEvent: { isTrusted: event.isTrusted, detail: event.detail },
        calculatedTargetState: { selectedNodeId: node.id },
        reactState: { selectedNodeId, selectedEdgeId, focused: Boolean(focused), nodeCount: nodes.length, edgeCount: edges.length },
        reactFlowControllerState: flow ? { viewport: flow.getViewport(), selectedNodeIds: flow.getNodes().filter((item) => item.selected).map((item) => item.id), edgeIds: flow.getEdges().map((item) => item.id) } : "not-ready",
        renderedDOMState: { nodeId: renderedNode(node.id)?.dataset.id, nodeText: renderedNode(node.id)?.innerText, editorPresent: Boolean(document.querySelector(".bird-v2-editor")) },
      });
      setSelectedNodeId(node.id); setSelectedEdgeId(undefined);
    }, 220);
  };
  const traceFocusTransition = (event: string, targetPhase: FocusPhase, calculatedTargetState: Record<string, unknown>) => {
    traceBird("focus-transition", {
      event,
      calculatedTargetState: { focusPhase: targetPhase, ...calculatedTargetState },
      reactState: { focusPhase, focused: focused?.state ?? "none", selectedNodeId, selectedEdgeId, nodeCount: nodes.length, edgeCount: edges.length },
      applicationStoreState: { birdViewId, graphState: result.state, birdViewRevision: revisionRef.current, focusPayloadState: focused?.state ?? "none" },
      reactFlowControllerState: flow ? { viewport: flow.getViewport(), nodes: flow.getNodes().map((node) => ({ id: node.id, type: node.type, position: node.position })), edges: flow.getEdges().map((edge) => edge.id) } : "not-ready",
      renderedDOMState: {
        focusPhase: document.querySelector(".bird-v2-canvas")?.getAttribute("data-focus-phase"),
        nodes: [...document.querySelectorAll<HTMLElement>(".bird-v2-canvas .react-flow__node")].map((element) => ({ id: element.dataset.id, className: element.className })),
        edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length,
      },
    });
  };
  const enterFocus = async (event: { isTrusted: boolean; detail: number }, node: BirdViewFlowNode) => {
    window.clearTimeout(clickTimer.current);
    window.clearTimeout(focusTimer.current);
    if (focusFrame.current !== undefined) cancelAnimationFrame(focusFrame.current);
    const viewport = flow?.getViewport();
    viewportRef.current = viewport;
    // Calculate in Flow space so the anchor stays at the same screen point under a saved viewport.
    const anchorPosition = flow?.screenToFlowPosition({ x: 32, y: 96 }) ?? node.position;
    setFocusTransition({ nodeId: node.id, overviewPosition: { ...node.position }, anchorPosition, viewport });
    setSelectedNodeId(node.id); setSelectedEdgeId(undefined); setFocused({ state: "loading" });
    traceFocusTransition("double-click", "entering", { focusNodeId: node.id, anchorPosition, overviewPosition: node.position, viewport, userEvent: { isTrusted: event.isTrusted, detail: event.detail } });
    try {
      const value = await fetchBirdViewNodeFocus(birdViewId, node.id);
      const reducedMotion = prefersReducedMotion();
      setFocused({ state: "ready", value });
      if (reducedMotion) {
        setFocusPhase("focused");
        traceFocusTransition("focus-payload-ready-reduced-motion", "focused", { focusNodeId: node.id, delayMs: 0 });
      } else {
        setFocusPhase("entering");
        traceFocusTransition("focus-payload-ready", "entering", { focusNodeId: node.id, delayMs: 260 });
        focusTimer.current = window.setTimeout(() => {
          setFocusPhase("focused");
          traceFocusTransition("enter-timer-complete", "focused", { focusNodeId: node.id });
        }, 260);
      }
    }
    catch (cause) { setFocused({ state: "error", message: messageFor(cause) }); setFocusTransition(undefined); }
  };
  const leaveFocus = () => {
    if (focusPhase === "exiting-start" || focusPhase === "exiting") return;
    const restore = viewportRef.current;
    window.clearTimeout(focusTimer.current);
    if (focusFrame.current !== undefined) cancelAnimationFrame(focusFrame.current);
    const finishExit = (event: string) => {
      setFocused(undefined); setFocusTransition(undefined); setFocusPhase("overview");
      if (restore && flow) void flow.setViewport(restore, { duration: 0 });
      traceFocusTransition(event, "overview", { viewport: restore, delayMs: 0 });
    };
    if (prefersReducedMotion()) {
      finishExit("exit-reduced-motion");
      return;
    }
    setFocusPhase("exiting-start");
    traceFocusTransition("exit-click", "exiting-start", { viewport: restore, overviewNodeOpacity: 0 });
    focusFrame.current = requestAnimationFrame(() => {
      setFocusPhase("exiting");
      traceFocusTransition("exit-animation-frame", "exiting", { viewport: restore, overviewNodeOpacity: 1 });
      focusTimer.current = window.setTimeout(() => finishExit("exit-timer-complete"), 260);
    });
  };

  if (auth.state.status !== "authenticated") return null;
  if (result.state === "loading") return <main className="bird-v2-state">Opening Bird View...</main>;
  if (result.state === "error") return <main className="bird-v2-state" role="alert">{result.message}</main>;
  const selectedNode = result.value.nodes.find((node) => node.id === selectedNodeId);
  const selectedEdge = result.value.edges.find((edge) => edge.id === selectedEdgeId);
  const focusNodeId = focusTransition?.nodeId;
  const focusAnchor = focusNodeId ? nodes.find((node) => node.id === focusNodeId) : undefined;
  const entering = focusPhase === "entering";
  const exiting = focusPhase === "exiting-start" || focusPhase === "exiting";
  const transitioning = (entering || exiting) && Boolean(focusAnchor);
  const displayNodes: Node[] = focusPhase === "focused" && focusAnchor && focusTransition
    ? [{ ...focusAnchor, position: focusTransition.anchorPosition, data: { ...focusAnchor.data, detailed: true }, className: "bird-focus-anchor" }, ...projection.nodes.map((node) => ({ ...node, type: "focusCard", className: "bird-focus-subset" }))]
    : nodes.map((node) => node.id === focusNodeId && focusTransition
      ? { ...node, position: entering ? focusTransition.anchorPosition : exiting ? focusTransition.overviewPosition : node.position, className: transitioning ? "bird-focus-anchor" : node.className }
      : { ...node, className: transitioning ? [node.className, "bird-focus-fading"].filter(Boolean).join(" ") : node.className });
  const displayEdges: Edge[] = focusPhase === "focused" ? projection.edges : edges.map((edge) => ({ ...edge, className: transitioning ? [edge.className, "bird-focus-fading"].filter(Boolean).join(" ") : edge.className }));
  return <main className="bird-v2-shell">
    <div className="bird-v2-canvas" data-pattern="planning-grid" data-focus-phase={focusPhase} data-semantic-zoom={semanticLevel(zoom)}>
      <ReactFlow nodes={displayNodes} edges={displayEdges} nodeTypes={graphNodeTypes} onInit={(instance) => { setFlow(instance); traceBird("react-flow:init", { event: "controller-init", calculatedTargetState: { nodeCount: nodes.length, edgeCount: edges.length, pattern: "planning-grid" }, reactState: { nodeCount: nodes.length, edgeCount: edges.length, semanticZoom: semanticLevel(zoom) }, reactFlowControllerState: { nodes: instance.getNodes().map((node) => ({ id: node.id, width: node.measured?.width, height: node.measured?.height })), edges: instance.getEdges().map((edge) => edge.id) }, renderedDOMState: { edgeCount: document.querySelectorAll(".bird-v2-canvas .react-flow__edge").length, planningGrid: renderedPlanningGrid() } }); const restore = viewportRef.current; if (restore) void instance.setViewport(restore, { duration: 0 }); else void instance.fitView(); }} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onConnect={onConnect} onNodeDragStop={onNodeDragStop} onNodeClick={(event, node) => { if (isBirdViewFlowNode(node)) selectNode(event, node); }} onNodeDoubleClick={(event, node) => { if (isBirdViewFlowNode(node)) void enterFocus(event, node); }} onEdgeClick={(_, edge) => { window.clearTimeout(clickTimer.current); setSelectedEdgeId(edge.id); setSelectedNodeId(undefined); }} onPaneClick={() => { setSelectedNodeId(undefined); setSelectedEdgeId(undefined); }} onMoveStart={(_, viewport) => { viewportRef.current = viewport; }} onMoveEnd={(_, viewport) => { viewportRef.current = viewport; setZoom(viewport.zoom); traceBird("viewport:move-end", { event: "pan-or-zoom", calculatedTargetState: { viewport }, reactState: { zoom: viewport.zoom, semanticZoom: semanticLevel(viewport.zoom) }, reactFlowControllerState: flow?.getViewport(), renderedDOMState: { planningGrid: renderedPlanningGrid() } }); }} nodesDraggable={focusPhase === "overview"} nodesConnectable={focusPhase === "overview"} edgesReconnectable={false} connectOnClick={false} deleteKeyCode={null} minZoom={0.25} maxZoom={1.6} proOptions={{ hideAttribution: true }}>
        <Background id={BIRD_VIEW_PLANNING_GRID.minor.id} className="bird-v2-planning-grid bird-v2-planning-grid-minor" patternClassName="bird-v2-planning-pattern-minor" variant={BackgroundVariant.Lines} gap={BIRD_VIEW_PLANNING_GRID.minor.gap} lineWidth={BIRD_VIEW_PLANNING_GRID.minor.lineWidth} color={BIRD_VIEW_PLANNING_GRID.minor.color} />
        <Background id={BIRD_VIEW_PLANNING_GRID.major.id} className="bird-v2-planning-grid bird-v2-planning-grid-major" patternClassName="bird-v2-planning-pattern-major" variant={BackgroundVariant.Lines} gap={BIRD_VIEW_PLANNING_GRID.major.gap} lineWidth={BIRD_VIEW_PLANNING_GRID.major.lineWidth} color={BIRD_VIEW_PLANNING_GRID.major.color} />
        <Controls showInteractive={false} /><MiniMap pannable zoomable />
      </ReactFlow>
      {focusPhase !== "overview" && <button type="button" className="bird-focus-exit" aria-label="Exit focus" onClick={leaveFocus}><ArrowLeft size={17} /> Overview</button>}
      <nav className="bird-v2-toolbar" aria-label="Bird View tools">
        <Link to="/workspaces" className="bird-v2-toolbar-icon" aria-label="Back to Workspaces"><ArrowLeft size={17} /></Link>
        <div className="bird-v2-toolbar-context">
          <span>BIRD VIEW / REVISION {result.value.birdView.revision}</span>
          <strong>{result.value.birdView.title}</strong>
        </div>
        {focusPhase === "overview" && !focusTransition && <button type="button" className="bird-v2-add-node" onClick={() => {
          const id = crypto.randomUUID();
          const position = flow?.screenToFlowPosition({ x: window.innerWidth / 2, y: window.innerHeight / 2 }) ?? { x: 80, y: 80 };
          const model: BirdNode = { id, title: "New node", summary: "", content: "", status: "active", positionX: position.x, positionY: position.y, createdAt: "", updatedAt: "" };
          const node: BirdViewFlowNode = { id, type: "birdViewNode", position, data: { ...model, detailed: true }, selected: true };
          void runMutation("bird_view.node.create", { nodeId: id, title: model.title, summary: "", content: "", positionX: position.x, positionY: position.y }, () => { setNodes((current) => [...current, node]); setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, nodes: [...current.value.nodes, model] } } : current); setSelectedNodeId(id); }, () => setNodes((current) => current.filter((item) => item.id !== id)));
        }}><Plus size={16} /> Node</button>}
      </nav>
      <div className="bird-v2-hint">Drag to arrange / connect handles to link / double-click a node to focus</div>
      {error && <div className="bird-v2-error" role="alert">{error}<button type="button" aria-label="Dismiss error" onClick={() => setError(undefined)}><X size={14} /></button></div>}
      {focusPhase === "overview" && selectedNode && <NodeEditor key={selectedNode.id} node={selectedNode} onSave={(values) => void runMutation("bird_view.node.update", { nodeId: selectedNode.id, ...values }, () => { setNodes((current) => current.map((node) => node.id === selectedNode.id ? { ...node, data: { ...node.data, ...values } } : node)); setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, nodes: current.value.nodes.map((node) => node.id === selectedNode.id ? { ...node, ...values } : node) } } : current); })} onDelete={() => void runMutation("bird_view.node.delete", { nodeId: selectedNode.id }, () => { setNodes((current) => current.filter((node) => node.id !== selectedNode.id)); setEdges((current) => current.filter((edge) => edge.source !== selectedNode.id && edge.target !== selectedNode.id)); setSelectedNodeId(undefined); })} onFocus={(event) => { const node = nodes.find((item) => item.id === selectedNode.id); if (node && isBirdViewFlowNode(node)) void enterFocus(event, node); }} />}
      {selectedEdge && <EdgeEditor key={selectedEdge.id} edge={selectedEdge} onSave={(label) => void runMutation("bird_view.edge.update", { edgeId: selectedEdge.id, label }, () => { setEdges((current) => current.map((edge) => edge.id === selectedEdge.id ? { ...edge, label } : edge)); setResult((current) => current.state === "ready" ? { state: "ready", value: { ...current.value, edges: current.value.edges.map((edge) => edge.id === selectedEdge.id ? { ...edge, label } : edge) } } : current); })} onDelete={() => void runMutation("bird_view.edge.disconnect", { edgeId: selectedEdge.id }, () => { setEdges((current) => current.filter((edge) => edge.id !== selectedEdge.id)); setSelectedEdgeId(undefined); })} />}
    </div>
  </main>;
}

function NodeEditor({ node, onSave, onDelete, onFocus }: { node: BirdNode; onSave: (values: Pick<BirdNode, "title" | "summary" | "content">) => void; onDelete: () => void; onFocus: (event: ReactMouseEvent<HTMLButtonElement>) => void }) {
  const [title, setTitle] = useState(node.title); const [summary, setSummary] = useState(node.summary); const [content, setContent] = useState(node.content);
  return <aside className="bird-v2-editor inspector" aria-label="Edit Bird View node">
    <header className="bird-v2-editor-title"><span className="inspector-kicker">BIRD VIEW INSPECTOR</span><h2><Edit3 size={17} /> Node</h2><span className={`status-pill bird-node-status-${node.status}`}>{node.status}</span></header>
    <section className="inspector-section bird-v2-editor-fields"><label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} /></label><label>Summary<textarea value={summary} onChange={(event) => setSummary(event.target.value)} /></label><label>Notes<textarea value={content} onChange={(event) => setContent(event.target.value)} /></label></section>
    <div className="bird-v2-editor-actions"><button type="button" className="primary-button" onClick={() => onSave({ title, summary, content })} disabled={!title.trim()}><Save size={14} /> Save changes</button><button type="button" onClick={onFocus}><Focus size={14} /> Focus</button><button type="button" className="danger" onClick={onDelete}><Trash2 size={14} /> Delete</button></div>
  </aside>;
}

function EdgeEditor({ edge, onSave, onDelete }: { edge: BirdViewEdge; onSave: (label: string) => void; onDelete: () => void }) {
  const [label, setLabel] = useState(edge.label);
  return <aside className="bird-v2-editor inspector bird-v2-edge-editor" aria-label="Edit Bird View edge">
    <header className="bird-v2-editor-title"><span className="inspector-kicker">BIRD VIEW INSPECTOR</span><h2>Connection</h2></header>
    <section className="inspector-section bird-v2-editor-fields"><label>Connection label<input value={label} onChange={(event) => setLabel(event.target.value)} /></label></section>
    <div className="bird-v2-editor-actions"><button type="button" className="primary-button" onClick={() => onSave(label)}><Save size={14} /> Save label</button><button type="button" className="danger" onClick={onDelete}><Trash2 size={14} /> Delete</button></div>
  </aside>;
}
