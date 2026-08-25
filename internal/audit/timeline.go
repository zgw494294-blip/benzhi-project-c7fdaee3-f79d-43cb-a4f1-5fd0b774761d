package audit

import (
	"fmt"
	"sort"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type TimelineItem struct {
	EventID string           `json:"eventID"`
	Type    domain.EventType `json:"type"`
	Title   string           `json:"title"`
	Actor   string           `json:"actor"`
	At      time.Time        `json:"at"`
	Version uint64           `json:"version"`
	Details map[string]any   `json:"details,omitempty"`
}

var eventTitles = map[domain.EventType]string{
	domain.EventPackageCreated: "创建访谈包", domain.EventConsentConfirmed: "确认知情同意", domain.EventSegmentAdded: "录入访谈片段", domain.EventSegmentsAdded: "批量录入访谈片段", domain.EventSegmentClassified: "完成片段判定", domain.EventSegmentsClassified: "批量完成片段判定", domain.EventClassified: "完成全部敏感性标注", domain.EventRevisionSubmitted: "提交脱敏修订", domain.EventReviewSubmitted: "提交伦理复核", domain.EventReviewReturned: "伦理复核退回", domain.EventReviewApproved: "伦理复核通过", domain.EventManifestFrozen: "冻结公开清单", domain.EventCredentialIssued: "签发公开授权凭据",
}

func Timeline(events []domain.BusinessEvent) []TimelineItem {
	ordered := append([]domain.BusinessEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].At.Equal(ordered[j].At) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].At.Before(ordered[j].At)
	})
	items := make([]TimelineItem, 0, len(ordered))
	for _, event := range ordered {
		title := eventTitles[event.Type]
		if title == "" {
			title = fmt.Sprintf("业务事件：%s", event.Type)
		}
		items = append(items, TimelineItem{EventID: event.ID, Type: event.Type, Title: title, Actor: event.Actor, At: event.At, Version: event.Version, Details: event.Details})
	}
	return items
}
