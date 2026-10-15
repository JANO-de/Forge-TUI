# Project: Forge
A project-focused terminal UI for OpenCode: lobby, projects, live agent info, progress bar, file/diff/editor panes. Runs on NixOS. Keep every OpenCode capability; Forge is a client of `opencode serve`, never a fork.

# Stack
- Go, Bubble Tea + Lipgloss + Bubbles, Chroma for highlighting. stdlib first; every new dependency needs one line of justification.
- Talk to OpenCode only through its HTTP API + SSE events. Before coding the client, read opencode.ai/docs/server and the OpenAPI spec the server exposes (check /doc). Never guess endpoints; generate or hand-write types from the spec.
- Nix flake: devShell (go, gopls, opencode) and `packages.default` via buildGoModule.

# Screens
1. Lobby: recent projects (name, path, last opened, last session, git branch/dirty, TODO progress from its HANDOFF.md), available models/providers (from the API), actions: open, new project, settings.
2. New project wizard: name, folder picker (browse dirs, create folder; default parent ~/Proyectos), template (empty/Go/Nix/Web/QML), model, guidelines template. Creates dir, `git init`, AGENTS.md, HANDOFF.md, opencode.json. Registers it.
3. Project view, panes (toggle each, resizable):
   - Files: tree with git status marks.
   - Viewer: highlighted file, diff of agent edits, accept/revert per hunk (`git restore -p` equivalent).
   - Editor: open file in $EDITOR via tea.ExecProcess, reload on return.
   - Agent: chat with streaming, tool calls, permission prompts (allow/deny), todo list, sessions/tabs, Plan/Build mode switch, model switch.
   - Guidelines: edit AGENTS.md and HANDOFF.md, run /handoff, show TODO checklist.
4. Status bar, always visible: model, mode, session, context-window % bar, tokens in/out, cost, elapsed, current tool, and the **progress bar** = done/total of the TODO list (from the agent's todo events, fallback HANDOFF.md `[x]`/`[ ]`). Spinner while working.

# Data
- Registry: `~/.config/forge/projects.json` (path, name, lastOpened, model). Settings: `~/.config/forge/config.json`. Nothing else outside project dirs.
- Never store API keys; OpenCode owns auth.

# Rules
- Build in stages per HANDOFF TODO; one milestone per delivery; each ends runnable.
- Complete files; no "rest unchanged". Files over ~300 lines: split by pane/package.
- Layout: cmd/forge, internal/{api,registry,ui/{lobby,wizard,project,status,panes},nix}. Pure model/update/view per screen; no goroutine touches UI state except through tea.Msg.
- Verification each milestone: `gofmt -l .`, `go vet ./...`, `go test ./...`, and a real run against a live `opencode serve` on 127.0.0.1 (random port). Test the UI with teatest or a golden-view test. State what you could not check.
- Never touch Tailscale/SSH/network beyond localhost, reboot/suspend, or `nixos-rebuild switch`. Never commit unless asked.
- Output: terse, no recap, one question max.
