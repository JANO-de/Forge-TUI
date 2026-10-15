package api

// PartAssembler turns message.part.updated and message.part.delta events into
// parts with their text so far. A delta can arrive before the update that
// names its part type, so such text is held until the type is known.
// Not safe for concurrent use: call it from the single event loop.
type PartAssembler struct {
	parts   map[string]Part   // last known part by ID
	texts   map[string]string // accumulated text by part ID
	pending map[string]string // deltas that arrived before their part
}

func NewPartAssembler() *PartAssembler {
	return &PartAssembler{
		parts:   map[string]Part{},
		texts:   map[string]string{},
		pending: map[string]string{},
	}
}

// Apply consumes one event. When it changed a part it returns that part with
// Text set to the full text so far, the new text chunk, and true.
func (a *PartAssembler) Apply(ev Event) (Part, string, bool) {
	switch ev.Type {
	case EventPartUpdated:
		u, err := ev.PartUpdate()
		if err != nil || u.Part.ID == "" {
			return Part{}, "", false
		}
		p := u.Part
		old := a.texts[p.ID]
		text := old
		if len(p.Text) >= len(old) {
			text = p.Text
		}
		if pend := a.pending[p.ID]; pend != "" {
			text += pend
			delete(a.pending, p.ID)
		}
		a.texts[p.ID] = text
		p.Text = text
		a.parts[p.ID] = p
		return p, text[len(old):], true
	case EventPartDelta:
		d, err := ev.PartDelta()
		if err != nil || d.PartID == "" || d.Field != "text" {
			return Part{}, "", false
		}
		p, known := a.parts[d.PartID]
		if !known {
			a.pending[d.PartID] += d.Delta
			return Part{}, "", false
		}
		a.texts[d.PartID] += d.Delta
		p.Text = a.texts[d.PartID]
		a.parts[d.PartID] = p
		return p, d.Delta, true
	}
	return Part{}, "", false
}
