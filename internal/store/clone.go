package store

import (
	"encoding/json"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

func cloneAggregate(value *domain.Aggregate) (*domain.Aggregate, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copied domain.Aggregate
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	return &copied, nil
}

func cloneMap(values map[string]*domain.Aggregate) (map[string]*domain.Aggregate, error) {
	result := make(map[string]*domain.Aggregate, len(values))
	for key, value := range values {
		copied, err := cloneAggregate(value)
		if err != nil {
			return nil, err
		}
		result[key] = copied
	}
	return result, nil
}

func cloneIdempotency(values map[string]IdempotencyRecord) map[string]IdempotencyRecord {
	result := make(map[string]IdempotencyRecord, len(values))
	for key, value := range values {
		value.Response = append(json.RawMessage(nil), value.Response...)
		result[key] = value
	}
	return result
}

func idempotencyIndex(packageID, key string) string { return packageID + "\x00" + key }
