package domain

type GateTaskCondition struct {
	TaskID     string
	TaskStatus TaskStatus
	Passed     bool
}

func GateReady(conditions []GateTaskCondition) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, condition := range conditions {
		if condition.TaskStatus != TaskConfirmed && !condition.Passed {
			return false
		}
	}
	return true
}
