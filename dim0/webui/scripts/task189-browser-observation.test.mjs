import assert from "node:assert/strict"
import test from "node:test"

import { assertBrowserObservation, classifyRequests, issueCanvasZoom } from "./task189-browser-observation.mjs"

const entry = (url) => ({ request: { url } })
const validObservation = {
  observation: {
    canvasHostPresent: true,
    width: 1280,
    height: 720,
    wheelInteraction: true,
    wheelCtrlKey: true,
    zoomBefore: "100%",
    zoomAfter: "125%",
  },
  backend: { pingStatus: 204, modelsStatus: 200, modelCount: 3 },
  messages: [],
  requestClassification: { providerRequests: [], externalRequests: [] },
  agentControlsOpened: false,
}

test("holds Control for the synthesized wheel so canvas zoom succeeds", async () => {
  let controlHeld = false
  let zoom = 100
  let wheelEvent
  const page = {
    keyboard: {
      down: async (key) => {
        assert.equal(key, "Control")
        controlHeld = true
      },
      up: async (key) => {
        assert.equal(key, "Control")
        controlHeld = false
      },
    },
    mouse: {
      move: async (x, y) => assert.deepEqual({ x, y }, { x: 70, y: 45 }),
      wheel: async (deltaX, deltaY) => {
        wheelEvent = { ctrlKey: controlHeld, deltaX, deltaY }
        if (wheelEvent.ctrlKey) zoom = 125
      },
    },
  }

  await issueCanvasZoom(page, { x: 10, y: 20, width: 120, height: 50 })

  assert.deepEqual(wheelEvent, { ctrlKey: true, deltaX: 0, deltaY: -240 })
  assert.equal(zoom, 125)
  assert.equal(controlHeld, false)
})

test("releases Control when the synthesized wheel fails", async () => {
  let controlHeld = false
  const page = {
    keyboard: {
      down: async () => { controlHeld = true },
      up: async () => { controlHeld = false },
    },
    mouse: {
      move: async () => {},
      wheel: async () => { throw new Error("wheel failed") },
    },
  }

  await assert.rejects(
    issueCanvasZoom(page, { x: 0, y: 0, width: 100, height: 100 }),
    /wheel failed/,
  )
  assert.equal(controlHeld, false)
})

test("accepts changed canvas state, successful backend probes, and internal requests", () => {
  const classification = classifyRequests(
    [entry("http://localhost/local/board"), entry("http://backend-test:8082/utils/ping"), entry("http://backend-test:8082/ai/models")],
    ["http://localhost", "http://backend-test:8082"],
  )
  assert.deepEqual(classification, { providerRequests: [], externalRequests: [] })
  assert.doesNotThrow(() => assertBrowserObservation({ ...validObservation, requestClassification: classification }))
})

test("rejects any request outside the two baseline origins", () => {
  const classification = classifyRequests(
    [entry("https://cdn.jsdelivr.net/dotlottie-player.wasm")],
    ["http://localhost", "http://backend-test:8082"],
  )
  assert.deepEqual(classification.externalRequests, ["https://cdn.jsdelivr.net/dotlottie-player.wasm"])
  assert.throws(
    () => assertBrowserObservation({ ...validObservation, requestClassification: classification }),
    /external requests observed/,
  )
})

test("rejects unchanged canvas state or an unhealthy backend", () => {
  assert.throws(
    () => assertBrowserObservation({
      ...validObservation,
      observation: { ...validObservation.observation, zoomAfter: "100%" },
    }),
    /did not change rendered zoom state/,
  )
  assert.throws(
    () => assertBrowserObservation({
      ...validObservation,
      backend: { pingStatus: 0, modelsStatus: 0, modelCount: 0 },
    }),
    /backend ping failed/,
  )
})
