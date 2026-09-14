import assert from "node:assert/strict"
import test from "node:test"

import { assertBrowserObservation, classifyRequests } from "./task189-browser-observation.mjs"

const entry = (url) => ({ request: { url } })
const validObservation = {
  observation: { canvasHostPresent: true, width: 1280, height: 720, wheelInteraction: true, zoomBefore: "100%", zoomAfter: "125%" },
  backend: { pingStatus: 204, modelsStatus: 200, modelCount: 3 },
  messages: [],
  requestClassification: { providerRequests: [], externalRequests: [] },
  agentControlsOpened: false,
}

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
