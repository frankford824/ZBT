package state

import "testing"

func TestProjectTransition(t *testing.T) {
	if err := Project.Transition("opportunity", "bidding"); err != nil {
		t.Fatal(err)
	}
	if err := Project.Transition("opportunity", "closed"); err == nil {
		t.Fatal("expected invalid transition to be rejected")
	}
}

func TestBidRejectReturnsToEditing(t *testing.T) {
	if err := BidDocument.Transition("in_review", "editing"); err != nil {
		t.Fatal(err)
	}
}
