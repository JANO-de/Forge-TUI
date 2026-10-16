package api

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// Server is an `opencode serve` child process bound to 127.0.0.1.
type Server struct {
	Client *Client
	cmd    *exec.Cmd
}

// freePort asks the kernel for an unused localhost port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Start runs `opencode serve --hostname 127.0.0.1 --port <random>` with dir as
// working directory (this is the project OpenCode operates on) and waits until
// /global/health answers. Stop must be called to terminate it.
func Start(ctx context.Context, dir string) (*Server, error) {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return nil, fmt.Errorf("opencode not found in PATH (nix develop provides it): %w", err)
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "serve", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "OPENCODE_ENABLE_QUESTION_TOOL=1")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Server{Client: New(fmt.Sprintf("http://127.0.0.1:%d", port)), cmd: cmd}

	deadline := time.Now().Add(30 * time.Second)
	for {
		hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		h, err := s.Client.Health(hctx)
		cancel()
		if err == nil && h.Healthy {
			return s, nil
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			s.Stop()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("opencode serve did not become healthy on port %d: %v", port, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Stop terminates the child process.
func (s *Server) Stop() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
	_, _ = s.cmd.Process.Wait()
}
