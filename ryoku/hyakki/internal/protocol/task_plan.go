package protocol

const MTeamsTaskPlan = "teams.task_plan"

// TeamTaskPlanParams supplies and approves a pending strategy as the Summoner.
// It is not an agent endpoint and never widens the seat's existing grants.
type TeamTaskPlanParams struct {
	ID      string   `json:"id"`
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
}
