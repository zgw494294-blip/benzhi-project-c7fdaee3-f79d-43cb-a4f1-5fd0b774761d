package audit

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type consentCanonical struct {
	Terms                 string   `json:"terms"`
	AllowedUses           []string `json:"allowedUses"`
	AttributionPreference string   `json:"attributionPreference"`
	WithdrawalDeadline    string   `json:"withdrawalDeadline,omitempty"`
	ConfirmedAt           string   `json:"confirmedAt"`
	ConfirmedBy           string   `json:"confirmedBy"`
}

type digestMemoEntry struct {
	digest string
}

var digestMemo = make(map[string]digestMemoEntry)

func lookupDigest(canonical string) (string, bool) {
	entry, ok := digestMemo[canonical]
	return entry.digest, ok
}

func rememberDigest(canonical, digest string) {
	digestMemo[canonical] = digestMemoEntry{digest: digest}
}

func CanonicalConsent(consent domain.ConsentRecord) ([]byte, error) {
	uses := append([]string(nil), consent.AllowedUses...)
	for i := range uses {
		uses[i] = domain.NormalizeText(uses[i])
	}
	sort.Strings(uses)
	deadline := ""
	if consent.WithdrawalDeadline != nil {
		deadline = consent.WithdrawalDeadline.UTC().Format(time.RFC3339Nano)
	}
	value := consentCanonical{Terms: domain.NormalizeText(consent.Terms), AllowedUses: uses, AttributionPreference: domain.NormalizeText(consent.AttributionPreference), WithdrawalDeadline: deadline, ConfirmedAt: consent.ConfirmedAt.UTC().Format(time.RFC3339Nano), ConfirmedBy: domain.NormalizeText(consent.ConfirmedBy)}
	return json.Marshal(value)
}

func JSONDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return "", err
	}
	canonical := compact.String()
	if digest, ok := lookupDigest(canonical); ok {
		return digest, nil
	}
	digest := domain.StableDigest(canonical)
	rememberDigest(canonical, digest)
	return digest, nil
}

func normalizeTags(tags []string) []string {
	result := append([]string(nil), tags...)
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}
	sort.Strings(result)
	return result
}
