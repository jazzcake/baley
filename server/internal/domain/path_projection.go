package domain

import "sort"

func projectPatchDiff(before, after *WorkspaceGraph) DependencyPatchDiff {
	diff := DependencyPatchDiff{}
	for key, dependency := range after.Dependencies {
		if _, exists := before.Dependencies[key]; !exists {
			diff.AddedDependencies = append(diff.AddedDependencies, dependency)
		}
	}
	for key, dependency := range before.Dependencies {
		if _, exists := after.Dependencies[key]; !exists {
			diff.RemovedDependencies = append(diff.RemovedDependencies, dependency)
		}
	}
	for id, afterTask := range after.Tasks {
		beforeTask, exists := before.Tasks[id]
		if exists && beforeTask.TerminalReason != afterTask.TerminalReason {
			diff.TerminalReasonChanges = append(diff.TerminalReasonChanges, TerminalReasonChange{TaskID: id, Before: beforeTask.TerminalReason, After: afterTask.TerminalReason})
		}
		if exists && before.hasIncoming(id) && !after.hasIncoming(id) {
			diff.NewRootTaskIDs = append(diff.NewRootTaskIDs, id)
		}
		if exists && before.hasOutgoing(id) && !after.hasOutgoing(id) {
			diff.NewLeafTaskIDs = append(diff.NewLeafTaskIDs, id)
		}
		if exists && !before.isDangling(id) && after.isDangling(id) {
			diff.BecameDanglingTaskIDs = append(diff.BecameDanglingTaskIDs, id)
		}
		if exists && before.isDangling(id) && !after.isDangling(id) {
			diff.ResolvedDanglingTaskIDs = append(diff.ResolvedDanglingTaskIDs, id)
		}
	}
	sort.Slice(diff.AddedDependencies, func(i, j int) bool { return edgeID(diff.AddedDependencies[i]) < edgeID(diff.AddedDependencies[j]) })
	sort.Slice(diff.RemovedDependencies, func(i, j int) bool { return edgeID(diff.RemovedDependencies[i]) < edgeID(diff.RemovedDependencies[j]) })
	sort.Slice(diff.TerminalReasonChanges, func(i, j int) bool {
		return diff.TerminalReasonChanges[i].TaskID < diff.TerminalReasonChanges[j].TaskID
	})
	sort.Strings(diff.NewRootTaskIDs)
	sort.Strings(diff.NewLeafTaskIDs)
	sort.Strings(diff.BecameDanglingTaskIDs)
	sort.Strings(diff.ResolvedDanglingTaskIDs)
	return diff
}
