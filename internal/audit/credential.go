package audit

import (
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type Verification struct {
	Valid          bool   `json:"valid"`
	Message        string `json:"message"`
	ComputedDigest string `json:"computedDigest"`
	Serial         uint64 `json:"serial"`
}

func IssueCredential(manifest domain.FrozenManifest, serial uint64, issuer string, now time.Time) (domain.ReleaseCredential, error) {
	if serial == 0 || manifest.Digest == "" || manifest.TermsDigest == "" || domain.NormalizeText(issuer) == "" {
		return domain.ReleaseCredential{}, domain.ErrInvalidInput
	}
	computed, err := ManifestDigest(manifest)
	if err != nil {
		return domain.ReleaseCredential{}, err
	}
	if computed != manifest.Digest {
		return domain.ReleaseCredential{}, domain.ErrIntegrity
	}
	return domain.ReleaseCredential{Serial: serial, PackageID: manifest.PackageID, ManifestDigest: manifest.Digest, TermsDigest: manifest.TermsDigest, IssuedBy: domain.NormalizeText(issuer), IssuedAt: now.UTC(), SchemaVersion: 1}, nil
}

func VerifyCredential(manifest *domain.FrozenManifest, credential *domain.ReleaseCredential) Verification {
	if manifest == nil || credential == nil {
		return Verification{Valid: false, Message: "尚未形成冻结清单或授权凭据"}
	}
	digest, err := ManifestDigest(*manifest)
	result := Verification{ComputedDigest: digest, Serial: credential.Serial}
	if err != nil {
		result.Message = "冻结清单无法规范化"
		return result
	}
	if digest != manifest.Digest {
		result.Message = "冻结清单内容摘要不匹配"
		return result
	}
	if credential.ManifestDigest != manifest.Digest || credential.TermsDigest != manifest.TermsDigest || credential.PackageID != manifest.PackageID {
		result.Message = "凭据字段与冻结清单不匹配"
		return result
	}
	if credential.SchemaVersion != 1 || credential.Serial == 0 {
		result.Message = "凭据版本或序号无效"
		return result
	}
	result.Valid = true
	result.Message = "凭据有效，摘要与冻结公开清单一致"
	return result
}
