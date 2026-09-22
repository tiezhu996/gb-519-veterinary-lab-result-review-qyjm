package constants

import "testing"

func TestAnimalCaseTransitionGraph(t *testing.T) {
	if !CanTransition(AnimalCaseTransitions, "registered", "sampling") {
		t.Fatalf("expected registered -> sampling transition to be allowed")
	}
	if CanTransition(AnimalCaseTransitions, "registered", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}
