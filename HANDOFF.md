# Handoff

## Projects and facts
Forge: Go/Bubble Tea TUI client for `opencode serve`. Repo ~/Proyectos/Forge. Spec in AGENTS.md. Editing = $EDITOR via ExecProcess, not a custom editor.

## TODO
1. [x] flake.nix devShell + go mod + minimal Bubble Tea app that runs
2. [x] internal/api: start or attach `opencode serve`, list providers/models, sessions, send prompt, stream SSE; CLI smoke test
3. [x] Registry + lobby screen (recent projects, models)
4. [x] New project wizard (folder picker, templates, git init, AGENTS/HANDOFF/opencode.json)
5. [x] Project view: agent pane with streaming, tool calls, permission prompts
6. [x] Status bar: model, context bar, tokens, cost, todo progress bar
7. [x] Files pane + highlighted viewer + diff with accept/revert
8. [x] $EDITOR integration with reload
9. [x] Guidelines/handoff pane, sessions/tabs, Plan/Build switch
10. [x] packages.default + module snippet for MyNixOS (ask first)

## Current state
TODOs 1-9 built: API client + SSE, lobby, wizard, project view (agent, status bar, files with git marks, viewer, diff accept/revert, $EDITOR, sessions picker Ctrl+O, tabs, Plan/Build, Guidelines pane with /handoff). gofmt, vet, build and `go test ./...` clean.

## Where the conversation stopped
Verified build and tests. TODO 10 written (buildGoModule vendorHash=null, nix/module.nix) but `nix build` NOT run. Left: manual checklist run of ./forge against live opencode.

## Open questions
Does file.edited arrive over SSE? Do assistant/tool parts of a real loaded session render right (only unit-tested)? `forge --continue`/`-c`, lobby search (/), pin (p), remove (d), help (?), Ctrl+K palette and Settings (default model, editor) written but not compiled or run. Run `nix build .#default` and fix any error.
