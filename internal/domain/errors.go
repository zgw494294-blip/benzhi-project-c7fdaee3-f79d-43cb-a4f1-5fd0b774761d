package domain

import "errors"

var (
	ErrInvalidInput      = errors.New("输入不符合业务规则")
	ErrInvalidTransition = errors.New("当前状态不允许该操作")
	ErrNotFound          = errors.New("记录不存在")
	ErrFrozen            = errors.New("公开清单已冻结，不允许修改")
	ErrIncomplete        = errors.New("前置事项尚未完成")
	ErrConsentScope      = errors.New("操作超出知情同意范围")
	ErrVersionConflict   = errors.New("版本冲突")
	ErrIdempotencyKey    = errors.New("幂等键已用于不同请求")
	ErrIntegrity         = errors.New("持久化数据完整性校验失败")
)

type RuleError struct {
	Field     string `json:"field,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Line      int    `json:"line,omitempty"`
	SegmentID string `json:"segmentID,omitempty"`
}

func (e *RuleError) Error() string { return e.Message }

func invalid(field, code, message string) error {
	return &RuleError{Field: field, Code: code, Message: message}
}
