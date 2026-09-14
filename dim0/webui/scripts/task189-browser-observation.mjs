import fs from "node:fs"
import path from "node:path"
import { pathToFileURL } from "node:url"
import { chromium } from "@playwright/test"

const providerHost = /(openai|anthropic|openrouter|mistral|perplexity|tavily|linkup|exa|daytona|doppler)/i

export function sanitizeHarEntries(rawEntries) {
  return rawEntries.map((entry) => ({
    startedDateTime: entry.startedDateTime,
    time: entry.time,
    request: {
      method: entry.request.method,
      url: new URL(entry.request.url).origin + new URL(entry.request.url).pathname,
      httpVersion: entry.request.httpVersion,
      headers: [],
      queryString: [],
      cookies: [],
      headersSize: -1,
      bodySize: entry.request.bodySize ?? -1,
    },
    response: {
      status: entry.response.status,
      statusText: entry.response.statusText,
      httpVersion: entry.response.httpVersion,
      headers: [],
      cookies: [],
      content: { size: entry.response.content?.size ?? 0, mimeType: entry.response.content?.mimeType ?? "" },
      redirectURL: "",
      headersSize: -1,
      bodySize: entry.response.bodySize ?? -1,
    },
    cache: {},
    timings: entry.timings,
  }))
}

export function classifyRequests(entries, allowedOrigins) {
  const allowed = new Set(allowedOrigins)
  const requestedUrls = entries.map((entry) => entry.request.url)
  return {
    providerRequests: requestedUrls.filter((url) => providerHost.test(new URL(url).hostname)),
    externalRequests: requestedUrls.filter((url) => !allowed.has(new URL(url).origin)),
  }
}

export function assertBrowserObservation({ observation, backend, messages, requestClassification, agentControlsOpened }) {
  if (!observation.canvasHostPresent || observation.width <= 0 || observation.height <= 0) {
    throw new Error("canvas host was not rendered with positive dimensions")
  }
  if (!observation.wheelInteraction || observation.zoomBefore === observation.zoomAfter) {
    throw new Error("canvas wheel interaction did not change rendered zoom state")
  }
  if (backend.pingStatus < 200 || backend.pingStatus >= 300) {
    throw new Error(`browser backend ping failed with status ${backend.pingStatus}`)
  }
  if (backend.modelsStatus !== 200 || backend.modelCount <= 0) {
    throw new Error(`browser model catalog failed: status=${backend.modelsStatus} count=${backend.modelCount}`)
  }
  if (requestClassification.providerRequests.length > 0) {
    throw new Error(`provider requests observed: ${requestClassification.providerRequests.join(", ")}`)
  }
  if (requestClassification.externalRequests.length > 0) {
    throw new Error(`external requests observed: ${requestClassification.externalRequests.join(", ")}`)
  }
  const errors = messages.filter((message) => message.type === "error" || message.type === "pageerror")
  if (errors.length > 0) {
    throw new Error(`browser errors observed: ${errors.map((message) => message.text).join(" | ")}`)
  }
  if (agentControlsOpened) {
    throw new Error("agent controls were opened during the baseline observation")
  }
}

async function run() {
  const baseUrl = process.env.TASK189_WEBUI_URL ?? "http://localhost"
  const backendUrl = process.env.TASK189_BACKEND_URL ?? "http://backend-test:8082"
  const outputDir = process.env.TASK189_EVIDENCE_DIR ?? "/baseline-evidence"
  const consolePath = path.join(outputDir, "browser-console.json")
  const harPath = path.join(outputDir, "browser-network.har")
  const rawHarPath = path.join(outputDir, ".browser-network.raw.har")
  const messages = []

  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    recordHar: { path: rawHarPath, content: "omit" },
    serviceWorkers: "block",
  })
  const page = await context.newPage()

  page.on("console", (message) => {
    messages.push({ type: message.type(), text: message.text().slice(0, 500) })
  })
  page.on("pageerror", (error) => {
    messages.push({ type: "pageerror", text: error.message.slice(0, 500) })
  })

  await page.goto(new URL("/local", baseUrl).href, { waitUntil: "networkidle" })
  await page.getByText("New Board", { exact: true }).click()
  await page.waitForURL(/\/local\/[a-z0-9-]+/i)
  const canvas = page.locator("[data-canvas-host]")
  await canvas.waitFor({ state: "visible" })
  const zoomControl = page.getByRole("button", { name: "Reset zoom to 100%" })
  await zoomControl.waitFor({ state: "visible" })
  const zoomBefore = (await zoomControl.textContent())?.trim() ?? ""
  const bounds = await canvas.boundingBox()
  if (!bounds) throw new Error("canvas bounds are unavailable")
  await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2)
  await page.mouse.wheel(0, -240)
  await page.waitForFunction(
    (before) => document.querySelector('[aria-label="Reset zoom to 100%"]')?.textContent?.trim() !== before,
    zoomBefore,
  )
  const zoomAfter = (await zoomControl.textContent())?.trim() ?? ""

  const observation = {
    canvasHostPresent: true,
    width: Math.round(bounds.width),
    height: Math.round(bounds.height),
    wheelInteraction: true,
    zoomBefore,
    zoomAfter,
  }
  const backend = await page.evaluate(async (origin) => {
    const pingResponse = await fetch(new URL("/utils/ping", origin), { cache: "no-store" })
    const modelsResponse = await fetch(new URL("/ai/models", origin), { cache: "no-store" })
    const models = await modelsResponse.json()
    return {
      pingStatus: pingResponse.status,
      modelsStatus: modelsResponse.status,
      modelCount: Array.isArray(models?.data?.llm) ? models.data.llm.length : 0,
    }
  }, backendUrl)
  const pageUrl = page.url()
  const agentControlsOpened = await page.getByText("Board Assistant", { exact: true }).isVisible().catch(() => false)

  await context.close()
  await browser.close()

  const rawHar = JSON.parse(fs.readFileSync(rawHarPath, "utf8"))
  const entries = sanitizeHarEntries(rawHar.log.entries)
  const requestClassification = classifyRequests(entries, [new URL(baseUrl).origin, new URL(backendUrl).origin])
  const result = {
    schemaVersion: 2,
    observedAt: new Date().toISOString(),
    pageUrl,
    observation: { ...observation, agentControlsOpened },
    backend,
    messages,
  }
  fs.writeFileSync(consolePath, `${JSON.stringify(result, null, 2)}\n`)
  fs.writeFileSync(harPath, `${JSON.stringify({
    log: {
      version: "1.2",
      creator: { name: "task-189-browser-observation", version: "2" },
      entries,
      _task189: {
        providerRequests: requestClassification.providerRequests.length,
        externalRequests: requestClassification.externalRequests.length,
        agentControlsOpened,
        canvasHostPresent: observation.canvasHostPresent,
        backend,
      },
    },
  }, null, 2)}\n`)
  fs.rmSync(rawHarPath)

  assertBrowserObservation({ observation, backend, messages, requestClassification, agentControlsOpened })
  console.log(JSON.stringify({ pageUrl, ...observation, ...backend, providerRequests: 0, externalRequests: 0, agentControlsOpened }))
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  await run()
}
