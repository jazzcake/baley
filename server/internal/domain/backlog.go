package domain

import (
	"sort"
	"strings"
)

type BacklogItemStatus string

const (
	BacklogActive    BacklogItemStatus = "active"
	BacklogPromoted  BacklogItemStatus = "promoted"
	BacklogDiscarded BacklogItemStatus = "discarded"
)

type BacklogItem struct {
	ID, WorkspaceID, LaneID, Title, Description string
	PublicID                                    int
	Status                                      BacklogItemStatus
	Position                                    *int
	PromotedTaskID, DiscardReason               string
}

func NewBacklogItem(item BacklogItem, lane Lane, position int) (BacklogItem, error) {
	item.Title, item.Description = strings.TrimSpace(item.Title), strings.TrimSpace(item.Description)
	if item.ID == "" || item.WorkspaceID == "" || item.WorkspaceID != lane.WorkspaceID || lane.State != LaneActive || item.PublicID <= 0 || item.Title == "" || item.Status != BacklogActive || position <= 0 || item.PromotedTaskID != "" || item.DiscardReason != "" {
		return item, &Violation{Code: CodeInvalidStateTransition}
	}
	item.Position = &position
	return item, nil
}

func (item BacklogItem) Update(title *string, description *string) (BacklogItem, error) {
	if item.Status != BacklogActive || title == nil && description == nil {
		return item, &Violation{Code: CodeInvalidBacklogPatch}
	}
	next := item
	if title != nil {
		next.Title = strings.TrimSpace(*title)
		if next.Title == "" {
			return item, &Violation{Code: CodeInvalidBacklogPatch}
		}
	}
	if description != nil {
		next.Description = strings.TrimSpace(*description)
	}
	return next, nil
}

func (item BacklogItem) Discard(reason string) (BacklogItem, error) {
	reason = strings.TrimSpace(reason)
	if item.Status != BacklogActive || reason == "" {
		return item, &Violation{Code: CodeInvalidStateTransition}
	}
	next := item
	next.Status, next.Position, next.DiscardReason = BacklogDiscarded, nil, reason
	return next, nil
}

func (item BacklogItem) Promote(taskID string) (BacklogItem, error) {
	if item.Status != BacklogActive || strings.TrimSpace(taskID) == "" {
		return item, &Violation{Code: CodeInvalidStateTransition}
	}
	next := item
	next.Status, next.Position, next.PromotedTaskID = BacklogPromoted, nil, taskID
	return next, nil
}

func CompactBacklog(items []BacklogItem, laneID string) []BacklogItem {
	next := append([]BacklogItem(nil), items...)
	sort.SliceStable(next, func(i, j int) bool {
		if next[i].LaneID != next[j].LaneID {
			return next[i].LaneID < next[j].LaneID
		}
		pi, pj := 0, 0
		if next[i].Position != nil {
			pi = *next[i].Position
		}
		if next[j].Position != nil {
			pj = *next[j].Position
		}
		if pi != pj {
			return pi < pj
		}
		return next[i].PublicID < next[j].PublicID
	})
	position := 0
	for i := range next {
		if next[i].Status == BacklogActive && next[i].LaneID == laneID {
			position++
			p := position
			next[i].Position = &p
		}
	}
	return next
}

func ReorderBacklog(items []BacklogItem, laneID string, ordered []int) ([]BacklogItem, error) {
	current := map[int]int{}
	for i, item := range items {
		if item.Status == BacklogActive && item.LaneID == laneID {
			current[item.PublicID] = i
		}
	}
	if len(current) != len(ordered) {
		return items, &Violation{Code: CodeBacklogOrderMismatch}
	}
	seen, unchanged := map[int]bool{}, true
	next := append([]BacklogItem(nil), items...)
	for i, publicID := range ordered {
		index, ok := current[publicID]
		if !ok || seen[publicID] {
			return items, &Violation{Code: CodeBacklogOrderMismatch}
		}
		seen[publicID] = true
		position := i + 1
		if next[index].Position == nil || *next[index].Position != position {
			unchanged = false
		}
		next[index].Position = &position
	}
	if unchanged {
		return items, &Violation{Code: CodeBacklogOrderUnchanged}
	}
	return next, nil
}
