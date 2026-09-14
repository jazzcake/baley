import fs from "node:fs"
import path from "node:path"
import { chromium } from "@playwright/test"

const baseUrl = process.env.TASK189_WEBUI_URL ?? "http://localhost"
const outputDir = process.env.TASK189_EVIDENCE_DIR ?? "/baseline-evidence"
const consolePath = path.join(outputDir, "browser-console.json")
const harPath = path.join(outputDir, "browser-network.har")
const rawHarPath = path.join(outputDir, ".browser-network.raw.har")
const providerHost = /(openai|anthropic|openrouter|mistral|perplexity|tavily|linkup|exa|daytona|doppler)/i
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
await canvas.dispatchEvent("wheel", { deltaY: -120, deltaMode: 0 })
await page.waitForTimeout(250)

const observation = await canvas.evaluate((element) => {
  const bounds = element.getBoundingClientRect()
  return {
    canvasHostPresent: true,
    width: Math.round(bounds.width),
    height: Math.round(bounds.height),
  }
})
const pageUrl = page.url()
const agentControlsOpened = await page.getByText("Board Assistant", { exact: true }).isVisible().catch(() => false)

await context.close()
await browser.close()

const rawHar = JSON.parse(fs.readFileSync(rawHarPath, "utf8"))
const entries = rawHar.log.entries.map((entry) => ({
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

const requestedUrls = entries.map((entry) => entry.request.url)
const providerRequests = requestedUrls.filter((url) => providerHost.test(new URL(url).hostname))
if (providerRequests.length > 0) {
  throw new Error(`provider requests observed: ${providerRequests.join(", ")}`)
}
if (agentControlsOpened) {
  throw new Error("agent controls were opened during the baseline observation")
}

fs.writeFileSync(consolePath, `${JSON.stringify({
  schemaVersion: 1,
  observedAt: new Date().toISOString(),
  pageUrl,
  observation: { ...observation, wheelInteraction: true, agentControlsOpened },
  messages,
}, null, 2)}\n`)
fs.writeFileSync(harPath, `${JSON.stringify({
  log: {
    version: "1.2",
    creator: { name: "task-189-browser-observation", version: "1" },
    entries,
    _task189: { providerRequests: 0, agentControlsOpened, canvasHostPresent: observation.canvasHostPresent },
  },
}, null, 2)}\n`)
fs.rmSync(rawHarPath)

console.log(JSON.stringify({ pageUrl, ...observation, wheelInteraction: true, providerRequests: 0, agentControlsOpened }))
