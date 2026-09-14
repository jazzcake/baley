import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const dotLottieCdnPattern = /https:\/\/cdn\.jsdelivr\.net\/npm\/[^"'`\\\s]+?\/dist\/dotlottie-player\.wasm/g

function javascriptFiles(root) {
  if (!fs.existsSync(root)) return []
  return fs.readdirSync(root, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".js"))
    .map((entry) => path.join(entry.parentPath ?? entry.path, entry.name))
}

export function localizeDotLottie({ wasmSource, distributionRoots }) {
  if (!fs.existsSync(wasmSource)) {
    throw new Error(`dotLottie WASM source is missing: ${wasmSource}`)
  }

  let replacements = 0
  for (const root of distributionRoots) {
    if (!fs.existsSync(root)) continue
    fs.copyFileSync(wasmSource, path.join(root, "dotlottie-player.wasm"))
    for (const file of javascriptFiles(root)) {
      const original = fs.readFileSync(file, "utf8")
      const localized = original.replace(dotLottieCdnPattern, "/dotlottie-player.wasm")
      replacements += original === localized ? 0 : 1
      fs.writeFileSync(file, localized)
      if (dotLottieCdnPattern.test(localized)) {
        throw new Error(`external dotLottie WASM URL remains in ${file}`)
      }
      dotLottieCdnPattern.lastIndex = 0
    }
  }
  if (replacements === 0) {
    throw new Error("no dotLottie CDN reference was localized")
  }
  return { replacements }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const appRoot = process.env.TASK189_WEBUI_ROOT ?? "/app"
  const result = localizeDotLottie({
    wasmSource: path.join(appRoot, "node_modules/@lottiefiles/dotlottie-web/dist/dotlottie-player.wasm"),
    distributionRoots: [path.join(appRoot, "dist"), path.join(appRoot, "dist-mini-app")],
  })
  console.log(JSON.stringify(result))
}
