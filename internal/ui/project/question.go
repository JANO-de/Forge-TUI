package project

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"forge/internal/api"
)

// QuestionPrompt is one question request from the agent's question tool
// (event question.asked) and the user's progress through it.
type QuestionPrompt struct {
	ID       string
	Items    []api.QuestionInfo
	Cur      int          // index of the question being answered
	Answers  [][]string   // answers given so far, one per question
	Sel      map[int]bool // toggled options (multiple-choice questions)
	Typing   bool         // typing a custom answer
	Text     string
	Active   bool // currently waiting for the user
	Done     bool
	Rejected bool
}

func (r *runState) addPermission(req *PermissionRequest) {
	if r.pending == nil {
		r.pending = req
		return
	}
	r.permQ = append(r.permQ, req)
}

func (r *runState) donePermission(id string) {
	if r.pending != nil && r.pending.ID == id {
		r.pending = nil
		if len(r.permQ) > 0 {
			r.pending, r.permQ = r.permQ[0], r.permQ[1:]
		}
		return
	}
	for i, q := range r.permQ {
		if q.ID == id {
			r.permQ = append(r.permQ[:i], r.permQ[i+1:]...)
			return
		}
	}
}

func (r *runState) addQuestion(q *QuestionPrompt) {
	if r.question == nil {
		q.Active = true
		r.question = q
		return
	}
	r.questQ = append(r.questQ, q)
}

func (r *runState) doneQuestion(id string, rejected bool) {
	if r.question != nil && r.question.ID == id {
		r.question.Active, r.question.Done, r.question.Rejected = false, true, rejected
		r.question = nil
		if len(r.questQ) > 0 {
			r.question, r.questQ = r.questQ[0], r.questQ[1:]
			r.question.Active = true
		}
		return
	}
	for i, q := range r.questQ {
		if q.ID == id {
			q.Done, q.Rejected = true, rejected
			r.questQ = append(r.questQ[:i], r.questQ[i+1:]...)
			return
		}
	}
}

// clearPrompts drops prompts that can no longer be answered (run finished).
func (r *runState) clearPrompts() {
	r.pending, r.permQ = nil, nil
	if r.question != nil {
		r.question.Active, r.question.Done = false, true
	}
	for _, q := range r.questQ {
		q.Done = true
	}
	r.question, r.questQ = nil, nil
}

func (p ProjectView) hasQuestion() bool {
	return p.picker == nil && p.activePane == PaneChat && len(p.sessions) > 0 &&
		p.sessions[p.activeSession].run.question != nil
}

// handleQuestionKey answers the pending question. ok is false for keys it
// does not use (pane/session shortcuts), which then take the normal path.
func (p *ProjectView) handleQuestionKey(msg tea.KeyMsg) (cmd tea.Cmd, ok bool) {
	s := &p.sessions[p.activeSession]
	q := s.run.question
	if q == nil || q.Cur >= len(q.Items) {
		return nil, false
	}
	item := q.Items[q.Cur]
	var answer []string

	if q.Typing {
		switch msg.Type {
		case tea.KeyEnter:
			t := strings.TrimSpace(q.Text)
			if t == "" {
				return nil, true
			}
			q.Typing, q.Text = false, ""
			answer = []string{t}
		case tea.KeyEsc:
			q.Typing, q.Text = false, ""
			return nil, true
		case tea.KeyBackspace:
			if r := []rune(q.Text); len(r) > 0 {
				q.Text = string(r[:len(r)-1])
			}
			return nil, true
		case tea.KeySpace:
			q.Text += " "
			return nil, true
		case tea.KeyRunes:
			q.Text += string(msg.Runes)
			return nil, true
		default:
			return nil, false
		}
		return p.advanceQuestion(s, answer), true
	}

	switch msg.Type {
	case tea.KeyEsc:
		id := q.ID
		s.run.doneQuestion(id, true)
		p.note(s, MsgSystem, "question dismissed")
		return p.agent.rejectQuestion(id), true
	case tea.KeyPgUp:
		p.scrollBy(p.chatHeight() - 2)
		return nil, true
	case tea.KeyPgDown:
		p.scrollBy(-(p.chatHeight() - 2))
		return nil, true
	case tea.KeyEnter:
		if item.Multiple {
			for i, o := range item.Options {
				if q.Sel[i] {
					answer = append(answer, o.Label)
				}
			}
		}
		if len(answer) == 0 {
			return nil, true
		}
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return nil, true
		}
		r := msg.Runes[0]
		switch {
		case r == 'c' && item.AllowCustom():
			q.Typing = true
			return nil, true
		case r >= '1' && r <= '9':
			i := int(r - '1')
			if i >= len(item.Options) {
				return nil, true
			}
			if item.Multiple {
				if q.Sel == nil {
					q.Sel = map[int]bool{}
				}
				q.Sel[i] = !q.Sel[i]
				return nil, true
			}
			answer = []string{item.Options[i].Label}
		default:
			return nil, true
		}
	default:
		return nil, false
	}
	return p.advanceQuestion(s, answer), true
}

// advanceQuestion records an answer and sends the reply after the last one.
func (p *ProjectView) advanceQuestion(s *Session, answer []string) tea.Cmd {
	q := s.run.question
	q.Answers = append(q.Answers, answer)
	q.Sel = nil
	q.Cur++
	if q.Cur < len(q.Items) {
		return nil
	}
	id, answers := q.ID, q.Answers
	s.run.doneQuestion(id, false)
	return p.agent.replyQuestion(id, answers)
}

func questionHint(q *QuestionPrompt, w int) string {
	if q.Typing {
		r := []rune(q.Text)
		if limit := w - 6; limit > 0 && len(r) > limit {
			r = append([]rune("…"), r[len(r)-limit:]...)
		}
		return "› " + string(r) + "▌   (Enter send · Esc back)"
	}
	if q.Cur >= len(q.Items) {
		return ""
	}
	it := q.Items[q.Cur]
	h := "Question: [1-9] choose"
	if it.Multiple {
		h = "Question: [1-9] toggle · Enter confirm"
	}
	if it.AllowCustom() {
		h += " · c type answer"
	}
	return h + " · Esc dismiss"
}

func renderQuestion(q *QuestionPrompt, w int) []string {
	var b strings.Builder
	for i, it := range q.Items {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if it.Header != "" {
			b.WriteString(it.Header + ": ")
		}
		b.WriteString(it.Question)
		switch {
		case i < len(q.Answers):
			b.WriteString("\n→ " + strings.Join(q.Answers[i], ", "))
		case q.Active && i == q.Cur:
			for j, o := range it.Options {
				mark := ""
				if it.Multiple {
					mark = "[ ] "
					if q.Sel[j] {
						mark = "[x] "
					}
				}
				fmt.Fprintf(&b, "\n %d. %s%s", j+1, mark, o.Label)
				if o.Description != "" {
					b.WriteString(" — " + o.Description)
				}
			}
			if q.Typing {
				b.WriteString("\n (typing a custom answer below)")
			}
		}
	}
	title := "Question"
	switch {
	case q.Rejected:
		title += " (dismissed)"
	case q.Done && len(q.Answers) < len(q.Items):
		title += " (closed)"
	}
	return withBar(title, colPermission, wrapPlain(b.String(), w-2))
}
