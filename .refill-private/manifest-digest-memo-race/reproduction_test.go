package manifestmemo_test

import (
	"sync"
	"testing"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/audit"
	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func TestConcurrentManifestDigestsSynchronizeMemo(t *testing.T) {
	cached := manifest("pkg-cached", "已缓存公开文本")
	uncached := manifest("pkg-uncached", "首次计算公开文本")
	if _, err := audit.ManifestDigest(cached); err != nil {
		t.Fatalf("预热摘要复用状态: %v", err)
	}

	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, digestErr := audit.ManifestDigest(cached)
		errors <- digestErr
	}()
	go func() {
		defer workers.Done()
		<-start
		_, digestErr := audit.ManifestDigest(uncached)
		errors <- digestErr
	}()
	close(start)
	workers.Wait()
	close(errors)
	for digestErr := range errors {
		if digestErr != nil {
			t.Fatalf("并发计算冻结清单摘要: %v", digestErr)
		}
	}
}

func manifest(packageID, publicText string) domain.FrozenManifest {
	return domain.FrozenManifest{
		PackageID:       packageID,
		Topic:           "审计摘要并发复现",
		ParticipantCode: "P-001",
		IntendedScope:   "公开展示",
		TermsDigest:     "sha256:terms",
		ConsentSummary:  "允许用途：公开展示；署名偏好：使用代号",
		Segments: []domain.FrozenSegment{{
			ID:         "seg-1",
			Sequence:   1,
			PublicText: publicText,
			RiskTags:   []string{"privacy"},
		}},
	}
}
