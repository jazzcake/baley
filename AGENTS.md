# Baley Agent Instructions

## UI/UX and React debugging

- When a UI/UX defect may involve React, event handling, state synchronization, rendering, or a third-party UI library, instrument the failing path before attempting broad fixes.
- Log the user event, calculated target state, React/application store state, library/controller state, and rendered DOM state at the relevant boundary.
- Use the resulting evidence to identify the first layer where expected and actual state diverge. Prefer this diagnosis over speculative dependency changes, framework changes, or repeated styling adjustments.
- Keep diagnostic logging development-only where practical, and retain useful structured traces until the behavior is verified.

## Local executable and firewall policy

- Do not use `go run`. Build Go executables under `C:\dev-bin\baley\` and run only those built files.
- Before suggesting a firewall rule, inspect existing rules and reuse an applicable rule when one exists.
- Never register temporary, cache, or Go build paths with the firewall.
- Do not automatically create, modify, or remove firewall rules. Provide the necessary PowerShell command to the user instead.
- Any inbound rule must be restricted to the Tailscale interface and Tailscale network only.
