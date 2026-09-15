import { describe, expect, it } from "vitest"
import { resolveSheetBodyAction } from "./interaction"


describe("resolveSheetBodyAction", () => {
  it("selects the whole sheet when its body is clicked while unselected", () => {
    expect(resolveSheetBodyAction({
      event: "click",
      selected: false,
      editing: false,
      canEdit: true,
    })).toBe("select")
  })

  it("does nothing when the selected sheet body receives a single click", () => {
    expect(resolveSheetBodyAction({
      event: "click",
      selected: true,
      editing: false,
      canEdit: true,
    })).toBe("none")
  })

  it("edits only when a selected editable sheet receives a double-click", () => {
    expect(resolveSheetBodyAction({
      event: "double-click",
      selected: true,
      editing: false,
      canEdit: true,
    })).toBe("edit")
  })

  it("selects instead of editing when a double-click starts unselected", () => {
    expect(resolveSheetBodyAction({
      event: "double-click",
      selected: false,
      editing: false,
      canEdit: true,
    })).toBe("select")
  })

  it("preserves native text interaction after editing has started", () => {
    expect(resolveSheetBodyAction({
      event: "double-click",
      selected: true,
      editing: true,
      canEdit: true,
    })).toBe("none")
  })

  it("does not enter edit mode on a read-only sheet", () => {
    expect(resolveSheetBodyAction({
      event: "double-click",
      selected: true,
      editing: false,
      canEdit: false,
    })).toBe("none")
  })
})
