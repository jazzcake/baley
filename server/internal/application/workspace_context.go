package application

import "sort"

// WorkspaceContextProjection is the small, first-read Workspace payload used by
// MCP clients. It deliberately has no Task bodies, graph edges, evidence, or
// completed Phases; callers opt into those only after selecting a Phase.
type WorkspaceContextProjection struct {
	Workspace          WorkspaceProjection      `json:"workspace"`
	Phases             []PhaseContextProjection `json:"phases"`
	FullGraphAvailable bool                     `json:"fullGraphAvailable"`
}

type PhaseContextProjection struct {
	ID         string                    `json:"id"`
	Name       string                    `json:"name"`
	State      string                    `json:"state"`
	Position   int                       `json:"position"`
	LaneCounts []LaneTaskCountProjection `json:"laneCounts"`
}

type LaneTaskCountProjection struct {
	LaneID       string         `json:"laneId"`
	LaneName     string         `json:"laneName"`
	StatusCounts map[string]int `json:"statusCounts"`
}

// WorkspaceContext returns all non-completed Phases in stable order. A Phase
// with no Tasks still lists each Lane so a caller can distinguish empty work
// from omitted data without receiving Task details.
func WorkspaceContext(snapshot Snapshot) WorkspaceContextProjection {
	lanes := append([]LaneProjection(nil), snapshot.Lanes...)
	sort.Slice(lanes, func(i, j int) bool {
		if lanes[i].Name != lanes[j].Name {
			return lanes[i].Name < lanes[j].Name
		}
		return lanes[i].ID < lanes[j].ID
	})
	phases := append([]PhaseProjection(nil), snapshot.Phases...)
	sort.Slice(phases, func(i, j int) bool {
		if phases[i].Position != phases[j].Position {
			return phases[i].Position < phases[j].Position
		}
		return phases[i].ID < phases[j].ID
	})
	result := WorkspaceContextProjection{Workspace: snapshot.Workspace, FullGraphAvailable: true}
	for _, phase := range phases {
		if phase.State == "completed" {
			continue
		}
		item := PhaseContextProjection{ID: phase.ID, Name: phase.Name, State: phase.State, Position: phase.Position, LaneCounts: make([]LaneTaskCountProjection, 0, len(lanes))}
		for _, lane := range lanes {
			item.LaneCounts = append(item.LaneCounts, LaneTaskCountProjection{LaneID: lane.ID, LaneName: lane.Name, StatusCounts: map[string]int{}})
		}
		for _, task := range snapshot.Tasks {
			if task.PhaseID != phase.ID {
				continue
			}
			for index := range item.LaneCounts {
				if item.LaneCounts[index].LaneID == task.LaneID {
					item.LaneCounts[index].StatusCounts[task.Status]++
					break
				}
			}
		}
		result.Phases = append(result.Phases, item)
	}
	return result
}

// PhaseTasksPage returns a bounded, public-ID cursor page for an explicitly
// selected active Phase. It is intentionally separate from WorkspaceContext.
func PhaseTasksPage(snapshot Snapshot, phaseID string, afterPublicID, limit int) ([]TaskProjection, int, bool) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tasks := make([]TaskProjection, 0, limit+1)
	for _, task := range snapshot.Tasks {
		if task.PhaseID == phaseID && task.PublicID > afterPublicID {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].PublicID < tasks[j].PublicID })
	hasMore := len(tasks) > limit
	if hasMore {
		tasks = tasks[:limit]
	}
	nextCursor := 0
	if len(tasks) > 0 && hasMore {
		nextCursor = tasks[len(tasks)-1].PublicID
	}
	return tasks, nextCursor, hasMore
}
