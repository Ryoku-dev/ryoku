package protocol

// StructureTemplate is an owner-editable org-chart recipe, not a permission
// grant. Shipped defaults are immutable; Revision versions local visibility or
// custom edits. Counts include the marshal/synthesizer, never just workers.
type StructureTemplate struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Culture      string          `json:"culture"`
	Description  string          `json:"description"`
	Shape        string          `json:"shape"`
	Reporting    string          `json:"reporting"`
	JobFit       []string        `json:"jobFit"`
	Seats        []StructureSeat `json:"seats"`
	MinSeats     int             `json:"minSeats"`
	MaxSeats     int             `json:"maxSeats"`
	DefaultSeats int             `json:"defaultSeats"`
	// Variable repeats worker seats after the first (marshal) seat.
	Variable bool             `json:"variable,omitempty"`
	Builtin  bool             `json:"builtin,omitempty"`
	Hidden   bool             `json:"hidden,omitempty"`
	Revision uint64           `json:"revision"`
	Source   string           `json:"source,omitempty"`
	Notes    []string         `json:"notes,omitempty"`
	Example  StructureExample `json:"example"`
}

// StructureSeat keeps role rank separate from canonical spirit ranks. Reporting
// refers to stable seat IDs; every selectable prefix must contain each selected
// seat's full manager chain. Ordering within a fixed roster is unrestricted.
// Delegation is derived rather than duplicated as mutable lists.
type StructureSeat struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Rank                string `json:"rank"`
	Duty                string `json:"duty"`
	DefaultSpirit       string `json:"defaultSpirit,omitempty"`
	CapabilitiesHint    string `json:"capabilitiesHint,omitempty"`
	SuggestedModelClass string `json:"suggestedModelClass,omitempty"`
	ReportsToSeat       string `json:"reportsToSeat,omitempty"`
	Lead                bool   `json:"lead,omitempty"`
}

// StructureSpirit describes a source identity, never an authorization level.
// Ranks retains all canonical dual ranks. Office is empty when the preserved
// source supplies only rank; absence must not be filled with invented lore.
type StructureSpirit struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Culture     string   `json:"culture"`
	Ranks       []string `json:"ranks"`
	Office      string   `json:"office,omitempty"`
	RoleMapping string   `json:"roleMapping,omitempty"`
	Source      string   `json:"source"`
	Notes       string   `json:"notes,omitempty"`
}

// StructureExample is the complete worked workflow shown before summoning.
// Members are a filled draft; examples describe intended work, not executed runs.
type StructureExample struct {
	Title     string                 `json:"title"`
	Goal      string                 `json:"goal"`
	SeatCount int                    `json:"seatCount"`
	Members   []Member               `json:"members,omitempty"`
	Steps     []StructureExampleStep `json:"steps"`
	Result    string                 `json:"result"`
	Guardrail string                 `json:"guardrail,omitempty"`
}

// StructureExampleStep attributes an action to one or more draft member IDs.
type StructureExampleStep struct {
	Members []string `json:"members"`
	Action  string   `json:"action"`
}

// StructureAnswers are the short summoning questions. Work is build, research,
// watch, decide, triage, create, verify or harden; Flow is deep, wide, ring or
// horde; Size is few, court, horde or choose. Count overrides Size when positive.
// Oversight is every-step, risky-steps or within-budget; Culture is optional.
type StructureAnswers struct {
	Job       string `json:"job,omitempty"`
	Work      string `json:"work"`
	Flow      string `json:"flow,omitempty"`
	Size      string `json:"size,omitempty"`
	Count     int    `json:"count,omitempty"`
	Oversight string `json:"oversight,omitempty"`
	Culture   string `json:"culture,omitempty"`
}

// StructureSuggestion is deterministic, local advice. Members leave Model empty
// to inherit local routing; model classes stay descriptive seat metadata.
// Approvals/PlanApproval are advice the summoner must review, not authorization.
type StructureSuggestion struct {
	Template     StructureTemplate `json:"template"`
	SeatCount    int               `json:"seatCount"`
	Members      []Member          `json:"members"`
	Score        int               `json:"score"`
	Why          string            `json:"why"`
	Approvals    string            `json:"approvals"`
	PlanApproval bool              `json:"planApproval,omitempty"`
}

// TeamStructuresResult includes hidden entries for management and separate
// deleted custom snapshots. Neither is eligible for suggestion or drafting.
type TeamStructuresResult struct {
	Templates []StructureTemplate `json:"templates"`
	Deleted   []StructureTemplate `json:"deleted"`
	Spirits   []StructureSpirit   `json:"spirits"`
}

type StructureDraftParams struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type StructureDraftResult struct {
	Template StructureTemplate `json:"template"`
	Members  []Member          `json:"members"`
}

type StructureSuggestionsResult struct {
	Suggestions []StructureSuggestion `json:"suggestions"`
}

type StructureSaveParams struct {
	Template         StructureTemplate `json:"template"`
	ExpectedRevision uint64            `json:"expectedRevision"`
}

type StructureCloneParams struct {
	ID    string `json:"id"`
	NewID string `json:"newId"`
	Name  string `json:"name"`
}

type StructureRef struct {
	ID               string `json:"id"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}
