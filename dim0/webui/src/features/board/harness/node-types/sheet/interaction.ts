export type SheetBodyEvent = "click" | "double-click"
export type SheetBodyAction = "select" | "edit" | "none"


export type SheetBodyInteraction = {
  event: SheetBodyEvent
  selected: boolean
  editing: boolean
  canEdit: boolean
}


/** Resolve sheet-body pointer input without depending on React event timing. */
export const resolveSheetBodyAction = ({
  event,
  selected,
  editing,
  canEdit,
}: SheetBodyInteraction): SheetBodyAction => {
  if (editing) return "none"
  if (!selected) return "select"
  if (event === "double-click" && canEdit) return "edit"
  return "none"
}
