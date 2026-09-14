import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"

import { localizeDotLottie } from "./task189-localize-dotlottie.mjs"

test("localizes the packaged dotLottie WASM without leaving a CDN request", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "task189-dotlottie-"))
  try {
    const dist = path.join(root, "dist")
    const assets = path.join(dist, "assets")
    fs.mkdirSync(assets, { recursive: true })
    const wasm = path.join(root, "dotlottie-player.wasm")
    fs.writeFileSync(wasm, "local-wasm")
    const bundle = path.join(assets, "app.js")
    fs.writeFileSync(bundle, 'const wasm="https://cdn.jsdelivr.net/npm/@lottiefiles/dotlottie-web@0.50.2/dist/dotlottie-player.wasm"')

    assert.deepEqual(localizeDotLottie({ wasmSource: wasm, distributionRoots: [dist] }), { replacements: 1 })
    assert.equal(fs.readFileSync(bundle, "utf8").includes("cdn.jsdelivr.net"), false)
    assert.equal(fs.readFileSync(path.join(dist, "dotlottie-player.wasm"), "utf8"), "local-wasm")
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})

test("fails closed when the built bundle has no recognized dotLottie reference", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "task189-dotlottie-"))
  try {
    const dist = path.join(root, "dist")
    fs.mkdirSync(dist, { recursive: true })
    const wasm = path.join(root, "dotlottie-player.wasm")
    fs.writeFileSync(wasm, "local-wasm")
    fs.writeFileSync(path.join(dist, "app.js"), "console.log('no player')")
    assert.throws(
      () => localizeDotLottie({ wasmSource: wasm, distributionRoots: [dist] }),
      /no dotLottie CDN reference was localized/,
    )
  } finally {
    fs.rmSync(root, { recursive: true, force: true })
  }
})
