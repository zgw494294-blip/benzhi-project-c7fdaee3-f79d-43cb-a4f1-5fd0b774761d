package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/audit"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/store"
)

type Clock func() time.Time

type Service struct {
	repository    *store.Repository
	now           Clock
	timelineMu    sync.Mutex
	timelineCache map[string]cachedTimeline
}

type cachedTimeline struct {
	version uint64
	items   []audit.TimelineItem
}

func NewService(repository *store.Repository) *Service {
	return &Service{repository: repository, now: time.Now, timelineCache: make(map[string]cachedTimeline)}
}

func NewServiceWithClock(repository *store.Repository, clock Clock) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{repository: repository, now: clock, timelineCache: make(map[string]cachedTimeline)}
}

func requestDigest(action string, command any) (string, error) {
	data, err := json.Marshal(command)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(action))
	h.Write([]byte{0})
	h.Write(data)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func encodeView(aggregate *domain.Aggregate) (json.RawMessage, error) {
	return json.Marshal(makeView(aggregate))
}

func encodeMutationView(aggregate *domain.Aggregate, command any) (json.RawMessage, error) {
	view := makeView(aggregate)
	if batch, ok := command.(AddSegmentsCommand); ok {
		view.AddedCount = len(batch.Items)
	}
	return json.Marshal(view)
}

func (s *Service) mutate(packageID, action string, meta WriteMeta, command any, role string, callback func(*domain.Aggregate, func() uint64) error) (PackageView, bool, error) {
	if err := validateMeta(meta, role); err != nil {
		return PackageView{}, false, err
	}
	digest, err := requestDigest(action, command)
	if err != nil {
		return PackageView{}, false, err
	}
	response, replayed, err := s.repository.Commit(store.CommitRequest{PackageID: packageID, ExpectedVersion: meta.ExpectedVersion, IdempotencyKey: meta.IdempotencyKey, RequestDigest: digest, Mutate: func(aggregate *domain.Aggregate, allocate func() uint64) (json.RawMessage, error) {
		if err := callback(aggregate, allocate); err != nil {
			return nil, err
		}
		return encodeMutationView(aggregate, command)
	}})
	if err != nil {
		return PackageView{}, false, err
	}
	var view PackageView
	if err := json.Unmarshal(response, &view); err != nil {
		return PackageView{}, false, err
	}
	return view, replayed, nil
}

func (s *Service) GetPackage(id string) (PackageView, error) {
	aggregate, err := s.repository.Get(id)
	if err != nil {
		return PackageView{}, err
	}
	view := makeView(aggregate)
	// 时间线缓存复用同一版本的计算结果，但缓存中的条目是不可变快照：
	// 返回给调用方前必须深拷贝，避免外部修改 TimelineItem.Actor 或
	// Details 后污染缓存、并在并发读取之间产生数据竞争。
	var canonical []audit.TimelineItem
	s.timelineMu.Lock()
	if cached, ok := s.timelineCache[id]; ok && cached.version == aggregate.Package.Version {
		canonical = cached.items
	} else {
		canonical = view.Timeline
		s.timelineCache[id] = cachedTimeline{version: aggregate.Package.Version, items: canonical}
	}
	s.timelineMu.Unlock()
	view.Timeline = cloneTimeline(canonical)
	return view, nil
}

// cloneTimeline 返回时间线条目的独立深拷贝。缓存复用同一版本的计算结果，
// 但调用方可以自由修改返回的 TimelineItem（包括 Actor 与 Details），
// 因此每次返回都必须与缓存及并发的其它读取完全隔离。
func cloneTimeline(items []audit.TimelineItem) []audit.TimelineItem {
	copied := make([]audit.TimelineItem, len(items))
	for i := range items {
		entry := items[i]
		if entry.Details != nil {
			details := make(map[string]any, len(entry.Details))
			for key, value := range entry.Details {
				details[key] = value
			}
			entry.Details = details
		}
		copied[i] = entry
	}
	return copied
}

func (s *Service) ListPackages() ([]PackageSummary, error) {
	values, err := s.repository.List()
	if err != nil {
		return nil, err
	}
	result := make([]PackageSummary, 0, len(values))
	for _, value := range values {
		summary := PackageSummary{ID: value.Package.ID, Topic: value.Package.Topic, ParticipantCode: value.Package.ParticipantCode, OwnerName: value.Package.OwnerName, Status: value.Package.Status, Version: value.Package.Version, SegmentCount: len(value.Segments), UpdatedAt: value.Package.UpdatedAt, ReviewProgress: value.CurrentReviewProgress()}
		if value.Credential != nil {
			summary.CredentialSerial = value.Credential.Serial
		}
		result = append(result, summary)
	}
	return result, nil
}

func (s *Service) ListReviewQueue() ([]PackageSummary, error) {
	values, err := s.ListPackages()
	if err != nil {
		return nil, err
	}
	result := make([]PackageSummary, 0)
	for _, value := range values {
		if value.Status == domain.StatusReviewPending {
			result = append(result, value)
		}
	}
	return result, nil
}
