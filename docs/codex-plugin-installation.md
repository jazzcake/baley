# Baley Codex plugin installation

Baley ships two Codex Skills for use from any project:

- `baley:baley-manage-work`: normal Backlog, Task, Run, Record, dependency,
  Gate, and approval-boundary operation.
- `baley:baley-adopt-project`: controlled onboarding of an existing repository
  into a Baley Workspace.

The plugin deliberately does not bundle another MCP server. It uses the existing
global `baley` typed MCP registration, so plugin installation cannot create a
duplicate MCP name or a second credential path.

## Install or update

From a Baley checkout, run:

```powershell
cd D:\Project_AI\baley
.\scripts\install-baley-codex-plugin.ps1
```

The installer:

1. creates or updates the personal `baley` plugin and marketplace entry;
2. copies the repository's canonical `.agents/skills/baley-*` packages;
3. applies a Codex cachebuster;
4. validates both Skills and the plugin manifest;
5. installs `baley@personal` and verifies that it is enabled.

Raw MCP credentials are not copied into the plugin. An Operator opens the
one-time MCP-visible loopback gateway URL, which verifies the pending local
request before redirecting to Baley. The Operator then signs in and explicitly
clicks `Connect local Gateway`. Baley completes the link only
when the browser's short-lived code returns to the same PC gateway that created
the request, then keeps the device credential in the OS store.

Start one new Codex thread after installation or update. This reload is for the
plugin catalog itself; adding another Baley Workspace afterward does not require
another thread, MCP registration, env file, or token copy.

## Project adoption sequence

Once a fresh project session lists both `baley:` Skills:

1. give the LLM the Baley Workspace Viewer URL;
2. when a new local gateway is first used, open its loopback Baley link, sign in,
   and click `Connect local Gateway`; this is device-link intent rather than a
   Workspace approval decision, and no token copy is required;
3. use `repository.register` and `baley-project-init` to create `baley.yaml` and
   Task Record templates;
4. add the project's concise Baley operating entry point to its durable agent
   instructions.

Design questions and read-only discussion do not need Tasks. Work that produces
planning, code, documentation, review, validation, or completion evidence should
use Baley's Backlog/Task/Run/Record lifecycle.
