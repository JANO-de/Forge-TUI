package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Events opens GET /event (SSE) and delivers decoded events on the returned
// channel until ctx is cancelled or the stream ends. The channel is closed
// when done; the error channel yields at most one error, then closes.
func (c *Client) Events(ctx context.Context) (<-chan Event, <-chan error) {
	events := make(chan Event, 64)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/event", nil)
		if err != nil {
			errs <- err
			return
		}
		req.Header.Set("Accept", "text/event-stream")
		// No overall timeout for the stream.
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			errs <- fmt.Errorf("GET /event: %s", resp.Status)
			return
		}

		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		var data strings.Builder
		flush := func() bool {
			if data.Len() == 0 {
				return true
			}
			var ev Event
			err := json.Unmarshal([]byte(data.String()), &ev)
			data.Reset()
			if err != nil {
				return true // skip malformed event
			}
			select {
			case events <- ev:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if !flush() {
					return
				}
			case strings.HasPrefix(line, "data:"):
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		flush()
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()
	return events, errs
}
