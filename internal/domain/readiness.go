package domain

import "fmt"

type CheckState string

const (
	CheckPending  CheckState = "pending"
	CheckReady    CheckState = "ready"
	CheckBlocked  CheckState = "blocked"
	CheckComplete CheckState = "complete"
)

type WorkflowCheck struct {
	Key     string     `json:"key"`
	Label   string     `json:"label"`
	State   CheckState `json:"state"`
	Message string     `json:"message"`
	Done    int        `json:"done"`
	Total   int        `json:"total"`
}

type WorkflowReadiness struct {
	Status            PackageStatus   `json:"status"`
	CompletionPercent int             `json:"completionPercent"`
	Checks            []WorkflowCheck `json:"checks"`
	Blockers          []string        `json:"blockers"`
	NextActions       []string        `json:"nextActions"`
	CanModifyConsent  bool            `json:"canModifyConsent"`
	CanModifySegments bool            `json:"canModifySegments"`
	CanSubmitReview   bool            `json:"canSubmitReview"`
	CanApproveRelease bool            `json:"canApproveRelease"`
}

func (a *Aggregate) Readiness() WorkflowReadiness {
	result := WorkflowReadiness{Status: a.Package.Status, Checks: make([]WorkflowCheck, 0, 5), Blockers: []string{}, NextActions: []string{}}
	result.CanModifyConsent = a.Package.Status == StatusDraft
	result.CanModifySegments = a.Package.Status == StatusConsentConfirmed || a.Package.Status == StatusClassifying || a.Package.Status == StatusRedactionPending || a.Package.Status == StatusRemediation
	result.CanApproveRelease = a.Package.Status == StatusApprovalPending
	consent := a.consentCheck()
	classification := a.classificationCheck()
	redaction := a.redactionCheck()
	review := a.reviewCheck()
	release := a.releaseCheck()
	result.Checks = append(result.Checks, consent, classification, redaction, review, release)
	completed := 0
	for _, check := range result.Checks {
		if check.State == CheckComplete {
			completed++
		}
		if check.State == CheckBlocked {
			result.Blockers = append(result.Blockers, check.Message)
		}
	}
	result.CompletionPercent = completed * 100 / len(result.Checks)
	result.CanSubmitReview = (a.Package.Status == StatusRedactionPending || a.Package.Status == StatusRemediation) && redaction.State == CheckComplete
	result.NextActions = a.nextActions(consent, classification, redaction, review, release)
	return result
}

func (a *Aggregate) consentCheck() WorkflowCheck {
	check := WorkflowCheck{Key: "consent", Label: "知情同意", Total: 1}
	if a.Consent == nil {
		check.State = CheckPending
		check.Message = "尚未登记完整知情同意边界"
		return check
	}
	check.State = CheckComplete
	check.Done = 1
	check.Message = "同意条款已确认并形成稳定摘要"
	return check
}

func (a *Aggregate) classificationCheck() WorkflowCheck {
	check := WorkflowCheck{Key: "classification", Label: "片段判定", Total: len(a.Segments)}
	if len(a.Segments) == 0 {
		check.State = CheckPending
		check.Message = "尚未录入访谈片段"
		return check
	}
	for _, segment := range a.Segments {
		if segment.Decision.ValidFinal() {
			check.Done++
		}
	}
	if check.Done < check.Total {
		check.State = CheckBlocked
		check.Message = fmt.Sprintf("仍有 %d 个片段未完成敏感性判定", check.Total-check.Done)
		return check
	}
	if a.Package.Status == StatusClassifying {
		check.State = CheckReady
		check.Message = "所有片段已判定，请确认完成标注"
		return check
	}
	check.State = CheckComplete
	check.Message = "所有片段均已完成敏感性判定"
	return check
}

func (a *Aggregate) redactionCheck() WorkflowCheck {
	check := WorkflowCheck{Key: "redaction", Label: "脱敏修订"}
	for _, segment := range a.Segments {
		if segment.Decision == DecisionRestricted {
			check.Total++
			if segment.Revision > 0 && NormalizeText(segment.PublicText) != "" {
				check.Done++
			}
		}
	}
	for _, finding := range a.Findings {
		if finding.Verdict == VerdictReturned && finding.ResolvedByRevision == nil {
			check.Total++
		}
	}
	if check.Total == 0 {
		if len(a.Segments) == 0 {
			check.State = CheckPending
			check.Message = "完成片段判定后检查脱敏项"
		} else {
			check.State = CheckComplete
			check.Message = "没有需要脱敏的阻断项"
		}
		return check
	}
	if check.Done < check.Total {
		check.State = CheckBlocked
		check.Message = fmt.Sprintf("仍有 %d 个脱敏或整改阻断项未闭环", check.Total-check.Done)
		return check
	}
	check.State = CheckComplete
	check.Message = "所有受限片段和退回整改项均已闭环"
	return check
}

func (a *Aggregate) reviewCheck() WorkflowCheck {
	check := WorkflowCheck{Key: "review", Label: "伦理复核"}
	for _, segment := range a.Segments {
		if segment.Decision != DecisionOmit {
			check.Total++
		}
	}
	for _, finding := range a.Findings {
		if finding.Round == a.Round && finding.Verdict == VerdictApproved {
			check.Done++
		}
	}
	switch a.Package.Status {
	case StatusApprovalPending, StatusFrozen, StatusReleased:
		check.State = CheckComplete
		check.Done = check.Total
		check.Message = "当前公开片段已全部通过伦理复核"
	case StatusRemediation:
		check.State = CheckBlocked
		check.Message = "伦理复核已退回，需完成明确整改项"
	case StatusReviewPending:
		check.State = CheckReady
		check.Message = fmt.Sprintf("本轮已通过 %d/%d 个公开片段", check.Done, check.Total)
	default:
		check.State = CheckPending
		check.Message = "完成脱敏后提交伦理复核"
	}
	return check
}

func (a *Aggregate) releaseCheck() WorkflowCheck {
	check := WorkflowCheck{Key: "release", Label: "冻结与凭据", Total: 2}
	if a.Manifest != nil {
		check.Done++
	}
	if a.Credential != nil {
		check.Done++
	}
	if check.Done == 2 {
		check.State = CheckComplete
		check.Message = "公开清单已冻结，授权凭据已签发"
		return check
	}
	if a.Package.Status == StatusApprovalPending {
		check.State = CheckReady
		check.Message = "复核已通过，等待开放负责人批准"
		return check
	}
	check.State = CheckPending
	check.Message = "伦理复核通过后方可冻结并签发"
	return check
}

func (a *Aggregate) nextActions(_ ...WorkflowCheck) []string {
	switch a.Package.Status {
	case StatusDraft:
		return []string{"整理员登记知情同意条款和允许用途"}
	case StatusConsentConfirmed:
		return []string{"整理员录入第一个有序访谈片段"}
	case StatusClassifying:
		check := a.classificationCheck()
		if check.State == CheckReady {
			return []string{"确认全部片段判定完成"}
		}
		return []string{"继续逐段标注敏感性和公开判定"}
	case StatusRedactionPending:
		return []string{"为所有受限片段提交脱敏修订", "阻断项闭环后提交伦理复核"}
	case StatusReviewPending:
		return []string{"伦理复核员逐项给出通过或退回结论"}
	case StatusRemediation:
		return []string{"整理员按退回原因提交新修订", "全部整改闭环后再次送审"}
	case StatusApprovalPending:
		return []string{"开放负责人核对同意范围和复核结论后批准"}
	case StatusFrozen:
		return []string{"签发与冻结摘要匹配的公开授权凭据"}
	case StatusReleased:
		return []string{"现场校验凭据摘要并留存授权序号"}
	default:
		return []string{}
	}
}
