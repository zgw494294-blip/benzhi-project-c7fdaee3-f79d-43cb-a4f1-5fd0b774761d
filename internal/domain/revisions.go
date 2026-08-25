package domain

import "sort"

type RevisionComparison struct {
	SegmentID string          `json:"segmentID"`
	From      SegmentRevision `json:"from"`
	To        SegmentRevision `json:"to"`
}

func (a *Aggregate) RevisionHistory(segmentID string) ([]SegmentRevision, error) {
	if _, err := a.findSegment(segmentID); err != nil {
		return nil, err
	}
	result := make([]SegmentRevision, 0)
	for _, revision := range a.Revisions {
		if revision.SegmentID == segmentID {
			result = append(result, revision)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Revision < result[j].Revision })
	return result, nil
}

func (a *Aggregate) CompareRevisions(segmentID string, from, to uint64) (RevisionComparison, error) {
	if to != from+1 || from == 0 {
		return RevisionComparison{}, invalid("revision", "not_adjacent", "只能对照相邻修订版本")
	}
	history, err := a.RevisionHistory(segmentID)
	if err != nil {
		return RevisionComparison{}, err
	}
	comparison := RevisionComparison{SegmentID: segmentID}
	foundFrom := false
	foundTo := false
	for _, revision := range history {
		if revision.Revision == from {
			comparison.From = revision
			foundFrom = true
		}
		if revision.Revision == to {
			comparison.To = revision
			foundTo = true
		}
	}
	if !foundFrom || !foundTo {
		return RevisionComparison{}, ErrNotFound
	}
	return comparison, nil
}
