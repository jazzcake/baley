import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"


const webuiRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const coreDist = path.join(webuiRoot, "node_modules", "@canvas-harness", "core", "dist")
const distFiles = ["index.js", "index.cjs"]

const replacements = [
  {
    label: "corner-only resize hit targets",
    before: 'var RESIZE_HANDLES = ["nw", "n", "ne", "e", "se", "s", "sw", "w"];',
    after: 'var RESIZE_HANDLES = ["nw", "ne", "se", "sw"];',
  },
  {
    label: "corner-only resize handle rendering",
    before: "for (const key of Object.keys(positions)) {",
    after: "for (const key of RESIZE_HANDLES) {",
  },
]


const occurrenceCount = (source, needle) => source.split(needle).length - 1


/**
 * Apply Baley.Dim0's narrow canvas-harness 0.1.27 chrome patch.
 * Exact occurrence checks deliberately fail an install when upstream changes
 * the compiled seam, forcing a rebase review instead of silently mispatching.
 */
const patchDistFile = (filename) => {
  const target = path.join(coreDist, filename)
  let source = fs.readFileSync(target, "utf8")

  for (const replacement of replacements) {
    const beforeCount = occurrenceCount(source, replacement.before)
    const afterCount = occurrenceCount(source, replacement.after)
    if (beforeCount === 0 && afterCount === 1) continue
    if (beforeCount !== 1 || afterCount !== 0) {
      throw new Error(
        `${filename}: cannot apply ${replacement.label}; expected one original seam, ` +
          `found original=${beforeCount} patched=${afterCount}`,
      )
    }
    source = source.replace(replacement.before, replacement.after)
  }

  fs.writeFileSync(target, source)
  console.log(`[baley.dim0] patched @canvas-harness/core ${filename}`)
}


for (const filename of distFiles) patchDistFile(filename)
