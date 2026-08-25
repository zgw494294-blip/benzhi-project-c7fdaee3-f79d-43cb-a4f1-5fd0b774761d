package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

var validRiskTags = map[string]bool{
	"privacy": true, "third_party": true, "sensitive_place": true, "time_limited": true,
}

func NormalizeText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func StableDigest(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func ValidateRiskTags(tags []string, until *time.Time) ([]string, error) {
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if !validRiskTags[tag] {
			return nil, invalid("riskTags", "invalid_risk_tag", "包含未知的敏感性标签")
		}
		if !seen[tag] {
			seen[tag] = true
			normalized = append(normalized, tag)
		}
	}
	if seen["time_limited"] && until == nil {
		return nil, invalid("restrictionUntil", "deadline_required", "限制期限标签必须填写截止时间")
	}
	if !seen["time_limited"] && until != nil {
		return nil, invalid("restrictionUntil", "deadline_without_tag", "仅限制期限标签可设置截止时间")
	}
	sort.Strings(normalized)
	return normalized, nil
}

func ValidateConsent(c ConsentRecord, now time.Time) error {
	if NormalizeText(c.Terms) == "" {
		return invalid("terms", "required", "知情同意条款不能为空")
	}
	if len(c.AllowedUses) == 0 {
		return invalid("allowedUses", "required", "至少登记一种允许用途")
	}
	for _, use := range c.AllowedUses {
		if NormalizeText(use) == "" {
			return invalid("allowedUses", "blank_use", "允许用途不能包含空项")
		}
	}
	if NormalizeText(c.AttributionPreference) == "" {
		return invalid("attributionPreference", "required", "署名偏好不能为空")
	}
	if NormalizeText(c.ConfirmedBy) == "" {
		return invalid("confirmedBy", "required", "确认人不能为空")
	}
	if c.ConfirmedAt.IsZero() || c.ConfirmedAt.After(now.Add(time.Minute)) {
		return invalid("confirmedAt", "invalid_time", "确认时间无效")
	}
	if c.WithdrawalDeadline != nil && c.WithdrawalDeadline.Before(c.ConfirmedAt) {
		return invalid("withdrawalDeadline", "before_confirmation", "撤回截止点不能早于确认时间")
	}
	return nil
}

func ContainsUse(uses []string, intended string) bool {
	want := strings.ToLower(NormalizeText(intended))
	for _, use := range uses {
		normal := strings.ToLower(NormalizeText(use))
		if normal == want || normal == "公开展示" || normal == "public access" {
			return true
		}
	}
	return false
}
