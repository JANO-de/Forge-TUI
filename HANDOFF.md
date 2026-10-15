# Handoff

## Projects and facts
Forge: Go/Bubble Tea TUI client for `opencode serve`. Repo ~/Proyectos/Forge. Spec in AGENTS.md. Editing = $EDITOR via ExecProcess, not a custom editor.

## TODO
1. [ ] flake.nix devShell + go mod + minimal Bubble Tea app that runs
2. [ ] internal/api: start or attach `opencode serve`, list providers/models, sessions, send prompt, stream SSE; CLI smoke test
3. [ ] Registry + lobby screen (recent projects, models)
4. [ ] New project wizard (folder picker, templates, git init, AGENTS/HANDOFF/opencode.json)
5. [ ] Project view: agent pane with streaming, tool calls, permission prompts
6. [ ] Status bar: model, context bar, tokens, cost, todo progress bar
7. [ ] Files pane + highlighted viewer + diff with accept/revert
8. [ ] $EDITOR integration with reload
9. [ ] Guidelines/handoff pane, sessions/tabs, Plan/Build switch
10. [ ] packages.default + module snippet for MyNixOS (ask first)

## Current state
Nothing built.

## Where the conversation stopped
Kit created; start at TODO 1.

## Open questions
Exact OpenCode server API shape (read spec first); whether todo events are exposed over SSE.
