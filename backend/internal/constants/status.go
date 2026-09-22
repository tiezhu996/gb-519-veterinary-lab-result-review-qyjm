package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type SpecimenState string

const (
	SpecimenStateReceived SpecimenState = "received"
	SpecimenStateTesting  SpecimenState = "testing"
	SpecimenStateHold     SpecimenState = "hold"
	SpecimenStateReleased SpecimenState = "released"
	SpecimenStateDisposed SpecimenState = "disposed"
)

var AllSpecimenState = []string{"received", "testing", "hold", "released", "disposed"}

type SignoffState string

const (
	SignoffStateDraft      SignoffState = "draft"
	SignoffStatePeerReview SignoffState = "peer_review"
	SignoffStateSigned     SignoffState = "signed"
	SignoffStateRejected   SignoffState = "rejected"
)

var AllSignoffState = []string{"draft", "peer_review", "signed", "rejected"}

var AnimalCaseTransitions = map[string]map[string]bool{
	"registered": {"sampling": true, "testing": true},
	"sampling":   {"testing": true, "closed": true, "registered": true},
	"testing":    {"closed": true, "sampling": true},
	"closed":     {"testing": true},
}

var SpecimenTransitions = map[string]map[string]bool{
	"received": {"testing": true, "hold": true},
	"testing":  {"hold": true, "released": true, "received": true},
	"hold":     {"released": true, "disposed": true, "testing": true},
	"released": {"disposed": true, "hold": true},
	"disposed": {"released": true},
}

var AssayRunTransitions = map[string]map[string]bool{
	"planned":   {"running": true, "validated": true},
	"running":   {"validated": true, "invalid": true, "planned": true},
	"validated": {"invalid": true, "running": true},
	"invalid":   {"validated": true},
}

var ResultSignoffTransitions = map[string]map[string]bool{
	"draft":       {"peer_review": true},
	"peer_review": {"signed": true, "rejected": true},
	"signed":      {},
	"rejected":    {},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
