// Command smoke is a CLI smoke test of internal/api against a real
// `opencode serve`. Usage:
//
//	go run ./cmd/smoke [-dir .] [-url http://127.0.0.1:4096] [-prompt "say hi"] [-model provider/model]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"forge/internal/api"
)

func main() {
	dir := flag.String("dir", ".", "project directory (used when starting a server)")
	url := flag.String("url", "", "attach to an existing server instead of starting one")
	prompt := flag.String("prompt", "Reply with the single word: pong", "prompt to send")
	model := flag.String("model", "", "provider/model (default: server default)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var c *api.Client
	if *url != "" {
		c = api.New(*url)
	} else {
		srv, err := api.Start(ctx, *dir)
		if err != nil {
			fail("start", err)
		}
		defer srv.Stop()
		c = srv.Client
		fmt.Println("started", c.BaseURL())
	}

	h, err := c.Health(ctx)
	if err != nil {
		fail("health", err)
	}
	fmt.Printf("health: healthy=%v version=%s\n", h.Healthy, h.Version)

	p, err := c.Providers(ctx)
	if err != nil {
		fail("providers", err)
	}
	for _, pr := range p.Providers {
		fmt.Printf("provider %s (%d models) default=%s\n", pr.ID, len(pr.Models), p.Default[pr.ID])
	}

	var mref *api.ModelRef
	if *model != "" {
		pid, mid, ok := strings.Cut(*model, "/")
		if !ok {
			fail("model", fmt.Errorf("want provider/model"))
		}
		mref = &api.ModelRef{ProviderID: pid, ModelID: mid}
	}

	sess, err := c.CreateSession(ctx, "forge smoke")
	if err != nil {
		fail("create session", err)
	}
	fmt.Println("session", sess.ID)

	// Subscribe before sending so no event is missed.
	evCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	events, errs := c.Events(evCtx)

	if err := c.PromptAsync(ctx, sess.ID, *prompt, mref, ""); err != nil {
		fail("prompt", err)
	}

	asm := api.NewPartAssembler()
	lastTool := map[string]string{}
	userMsgs := map[string]bool{}
	for ev := range events {
		if sid := ev.SessionID(); sid != "" && sid != sess.ID {
			continue
		}
		switch ev.Type {
		case api.EventMessageUpdated:
			if m, err := ev.MessageInfo(); err == nil && m.Role == "user" {
				userMsgs[m.ID] = true
			}
		case api.EventPartUpdated, api.EventPartDelta:
			part, chunk, ok := asm.Apply(ev)
			if !ok || userMsgs[part.MessageID] {
				continue
			}
			switch part.Type {
			case "text":
				fmt.Print(chunk)
			case "reasoning":
				// not shown in the smoke test
			case "tool":
				if k := part.ID + part.State.Status; k != lastTool[part.ID] {
					lastTool[part.ID] = k
					fmt.Printf("\n[tool %s: %s]\n", part.Tool, part.State.Status)
				}
			}
		case api.EventPermissionAsked:
			pm, _ := ev.Permission()
			fmt.Printf("\n[permission requested: %s %v -> rejecting in smoke test]\n", pm.Permission, pm.Patterns)
			_ = c.RespondPermission(ctx, pm.ID, "reject")
		case api.EventTodoUpdated:
			t, _ := ev.TodoUpdate()
			fmt.Printf("\n[todos: %d]\n", len(t.Todos))
		case api.EventSessionError:
			fmt.Printf("\n[session error] %s\n", string(ev.Properties))
			return
		case api.EventSessionIdle:
			fmt.Println("\n[idle]")
			msgs, err := c.Messages(ctx, sess.ID)
			if err != nil {
				fail("messages", err)
			}
			fmt.Printf("%d messages in session\n", len(msgs))
			return
		}
	}
	select {
	case err := <-errs:
		if err != nil {
			fail("events", err)
		}
	default:
	}
	fail("events", fmt.Errorf("stream ended before session.idle"))
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}
