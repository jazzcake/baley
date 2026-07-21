# Baley Agent Instructions

## UI/UX and React debugging

- When a UI/UX defect may involve React, event handling, state synchronization, rendering, or a third-party UI library, instrument the failing path before attempting broad fixes.
- Log the user event, calculated target state, React/application store state, library/controller state, and rendered DOM state at the relevant boundary.
- Use the resulting evidence to identify the first layer where expected and actual state diverge. Prefer this diagnosis over speculative dependency changes, framework changes, or repeated styling adjustments.
- Keep diagnostic logging development-only where practical, and retain useful structured traces until the behavior is verified.
