package application

import (
	"errors"
	"strings"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

var ErrForbidden = errors.New("当前角色无权执行该操作")

func validateMeta(meta WriteMeta, requiredRole string) error {
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return &domain.RuleError{Field: "idempotencyKey", Code: "required", Message: "写操作必须提供 idempotencyKey"}
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return &domain.RuleError{Field: "actor", Code: "required", Message: "操作人不能为空"}
	}
	if meta.Role != requiredRole {
		return ErrForbidden
	}
	return nil
}
