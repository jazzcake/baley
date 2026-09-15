import { useRef, type PointerEvent as ReactPointerEvent } from "react"
import {
  asEdgeId,
  asNodeId,
  handleWorldPositions,
  screenToWorld,
  worldToScreen,
  type CanvasStore,
  type EdgeEnd,
  type Vec2,
} from "@canvas-harness/core"
import {
  useCamera,
  useCanvasStore,
  useInteractionState,
  useNode,
  useSelection,
  type ArrowToolDefaults,
} from "@canvas-harness/react"
import { useBoardAppStore } from "../store/board-app-store"
import {
  CARDINAL_CONNECTOR_SIDES,
  cardinalConnectorAngle,
  cardinalConnectorSource,
  connectorEndFromWorldPoint,
  type CardinalConnectorSide,
} from "./cardinal-connector"


const DRAG_START_PX = 4
const HANDLE_HIT_SIZE_PX = 24
const TRIANGLE_POINTS = "12,5 18,16 6,16"


type ActiveGesture = {
  pointerId: number
  startScreen: Vec2
  source: EdgeEnd
  sourceSide: CardinalConnectorSide
  active: boolean
}


type CardinalConnectorHandlesProps = {
  canEdit: boolean
  color: string
  defaults: ArrowToolDefaults
}


const screenFromEvent = (event: ReactPointerEvent<HTMLButtonElement>): Vec2 | null => {
  const host = event.currentTarget.closest<HTMLElement>("[data-canvas-host]")
  if (!host) return null
  const rect = host.getBoundingClientRect()
  return { x: event.clientX - rect.left, y: event.clientY - rect.top }
}


const resolveDefault = <T,>(value: T | (() => T | undefined) | undefined): T | undefined =>
  typeof value === "function" ? (value as () => T | undefined)() : value


const traceConnectorGesture = (
  store: CanvasStore,
  stage: "pointer-down" | "drag-start" | "commit" | "cancel",
  details: Record<string, unknown>,
): void => {
  if (!import.meta.env.DEV) return
  const renderedHandles = document.querySelectorAll("[data-cardinal-connector-handle]")
  console.debug("[baley.dim0:cardinal-connector]", {
    stage,
    ...details,
    applicationTool: useBoardAppStore.getState().tool,
    selection: store.getSelection(),
    libraryInteraction: store.getInteractionState(),
    renderedDom: {
      handleCount: renderedHandles.length,
      sides: Array.from(renderedHandles, (handle) =>
        handle.getAttribute("data-cardinal-connector-handle"),
      ),
    },
  })
}


/**
 * Replace the selected node's four side resize affordances with connector
 * triangles. The overlay owns the pointer until release, then commits through
 * the same canvas-harness store path and edge defaults as the Arrow tool.
 */
export function CardinalConnectorHandles({
  canEdit,
  color,
  defaults,
}: CardinalConnectorHandlesProps) {
  const store = useCanvasStore()
  const selection = useSelection()
  const camera = useCamera()
  const interaction = useInteractionState()
  const selectedNodeId =
    selection.length === 1 && store.getNode(selection[0] as ReturnType<typeof asNodeId>)
      ? (selection[0] as ReturnType<typeof asNodeId>)
      : asNodeId("__no-selected-node__")
  const node = useNode(selectedNodeId)
  const gestureRef = useRef<ActiveGesture | null>(null)
  const defaultsRef = useRef(defaults)
  defaultsRef.current = defaults

  if (
    !canEdit ||
    !node ||
    (interaction.mode !== "idle" && interaction.mode !== "creating-edge")
  ) {
    return null
  }

  const positions = handleWorldPositions(node)

  const stopEvent = (event: ReactPointerEvent<HTMLButtonElement>): void => {
    event.stopPropagation()
    event.preventDefault()
  }

  const handlePointerDown = (
    event: ReactPointerEvent<HTMLButtonElement>,
    side: CardinalConnectorSide,
  ): void => {
    stopEvent(event)
    if (event.button !== 0 || store.getInteractionState().mode !== "idle") return
    const startScreen = screenFromEvent(event)
    if (!startScreen) return

    event.currentTarget.setPointerCapture(event.pointerId)
    gestureRef.current = {
      pointerId: event.pointerId,
      startScreen,
      source: cardinalConnectorSource(node, side),
      sourceSide: side,
      active: false,
    }
    traceConnectorGesture(store, "pointer-down", {
      event: { pointerId: event.pointerId, pointerType: event.pointerType },
      calculatedSource: gestureRef.current.source,
      sourceSide: side,
    })
  }

  const handlePointerMove = (event: ReactPointerEvent<HTMLButtonElement>): void => {
    const gesture = gestureRef.current
    if (!gesture || gesture.pointerId !== event.pointerId) return
    stopEvent(event)
    const screen = screenFromEvent(event)
    if (!screen) return
    const dx = screen.x - gesture.startScreen.x
    const dy = screen.y - gesture.startScreen.y
    if (!gesture.active && Math.abs(dx) < DRAG_START_PX && Math.abs(dy) < DRAG_START_PX) return

    const world = screenToWorld(screen, store.getCamera())
    const target = connectorEndFromWorldPoint(store, world)
    const isStarting = !gesture.active
    if (isStarting) {
      gesture.active = true
      useBoardAppStore.getState().setTool("arrow")
    }
    store.setInteractionState({
      mode: "creating-edge",
      draftEdge: {
        source: gesture.source,
        target: target.end,
        reconnectingId: null,
        snapTargetNodeId: target.nodeId,
      },
    })
    if (isStarting) {
      traceConnectorGesture(store, "drag-start", {
        calculatedSource: gesture.source,
        calculatedTarget: target,
        sourceSide: gesture.sourceSide,
      })
    }
  }

  const finishGesture = (
    event: ReactPointerEvent<HTMLButtonElement>,
    cancelled: boolean,
  ): void => {
    const gesture = gestureRef.current
    if (!gesture || gesture.pointerId !== event.pointerId) return
    stopEvent(event)
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }

    try {
      if (gesture.active && !cancelled) {
        const screen = screenFromEvent(event)
        if (!screen) return
        const target = connectorEndFromWorldPoint(
          store,
          screenToWorld(screen, store.getCamera()),
        )
        const currentDefaults = defaultsRef.current
        const style = resolveDefault(currentDefaults.style)
        const data = resolveDefault(currentDefaults.data)
        const edgeId = asEdgeId(store.generateId())
        store.addEdge({
          id: edgeId,
          source: gesture.source,
          target: target.end,
          pathStyle: currentDefaults.pathStyle ?? "bezier",
          groups: [],
          ...(style ? { style } : {}),
          ...(data ? { data } : {}),
        })
        traceConnectorGesture(store, "commit", {
          edgeId,
          calculatedSource: gesture.source,
          calculatedTarget: target,
          sourceSide: gesture.sourceSide,
        })
      } else if (cancelled) {
        traceConnectorGesture(store, "cancel", {
          sourceSide: gesture.sourceSide,
          wasActive: gesture.active,
        })
      }
    } finally {
      store.resetInteractionState()
      gestureRef.current = null
    }
  }

  return (
    <>
      {CARDINAL_CONNECTOR_SIDES.map((side) => {
        const screen = worldToScreen(positions[side], camera)
        const triangleAngle = cardinalConnectorAngle(side) + (node.angle * 180) / Math.PI
        return (
          <button
            key={side}
            type="button"
            aria-label={`Create connector from ${side} side`}
            data-cardinal-connector-handle={side}
            data-node-id={node.id}
            className="absolute m-0 border-0 bg-transparent p-0"
            style={{
              left: screen.x,
              top: screen.y,
              width: HANDLE_HIT_SIZE_PX,
              height: HANDLE_HIT_SIZE_PX,
              transform: "translate(-50%, -50%)",
              cursor: "crosshair",
              touchAction: "none",
              zIndex: 4,
            }}
            onPointerDownCapture={(event) => handlePointerDown(event, side)}
            onPointerMoveCapture={handlePointerMove}
            onPointerUpCapture={(event) => finishGesture(event, false)}
            onPointerCancelCapture={(event) => finishGesture(event, true)}
          >
            <svg
              aria-hidden="true"
              width={HANDLE_HIT_SIZE_PX}
              height={HANDLE_HIT_SIZE_PX}
              viewBox="0 0 24 24"
              className="block overflow-visible"
            >
              <polygon
                points={TRIANGLE_POINTS}
                fill={color}
                transform={`rotate(${triangleAngle} 12 12)`}
              />
            </svg>
          </button>
        )
      })}
    </>
  )
}
