package state

import "fmt"

type Machine struct {
	name        string
	transitions map[string]map[string]struct{}
}

func NewMachine(name string, transitions map[string][]string) Machine {
	normalized := make(map[string]map[string]struct{}, len(transitions))
	for from, targets := range transitions {
		normalized[from] = make(map[string]struct{}, len(targets))
		for _, target := range targets {
			normalized[from][target] = struct{}{}
		}
	}
	return Machine{name: name, transitions: normalized}
}

func (m Machine) Can(from, to string) bool {
	if from == to {
		return true
	}
	targets, ok := m.transitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}

func (m Machine) Transition(from, to string) error {
	if m.Can(from, to) {
		return nil
	}
	return fmt.Errorf("%s transition %q -> %q is not allowed", m.name, from, to)
}

var Project = NewMachine("project", map[string][]string{
	"opportunity":       {"bidding"},
	"bidding":           {"compliance_review"},
	"compliance_review": {"submitted", "bidding"},
	"submitted":         {"closed"},
	"closed":            {},
})

var BidDocument = NewMachine("bid_document", map[string][]string{
	"draft":      {"generating"},
	"generating": {"editing"},
	"editing":    {"in_review"},
	"in_review":  {"approved", "editing"},
	"approved":   {"submitted"},
	"submitted":  {"archived"},
	"archived":   {},
})

var GenerationJob = NewMachine("bid_generation_job", map[string][]string{
	"queued":  {"running", "cancelled"},
	"running": {"paused", "done", "failed", "cancelled"},
	"paused":  {"running", "cancelled"},
	"done":    {},
	"failed":  {},
})
