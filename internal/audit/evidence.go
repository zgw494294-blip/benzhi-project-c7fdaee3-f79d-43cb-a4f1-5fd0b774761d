package audit

import (
	"fmt"
	"time"

	"benzhi-project-c7fdaee3-f79d-43cb-a4f1-5fd0b774761d/internal/domain"
)

type PublicEvidence struct {
	CredentialLabel string       `json:"credentialLabel"`
	PackageID       string       `json:"packageID"`
	ManifestDigest  string       `json:"manifestDigest"`
	TermsDigest     string       `json:"termsDigest"`
	EvidenceDigest  string       `json:"evidenceDigest"`
	IssuedBy        string       `json:"issuedBy"`
	IssuedAt        time.Time    `json:"issuedAt"`
	SchemaVersion   int          `json:"schemaVersion"`
	Verification    Verification `json:"verification"`
}

type evidenceCanonical struct {
	Serial         uint64 `json:"serial"`
	PackageID      string `json:"packageID"`
	ManifestDigest string `json:"manifestDigest"`
	TermsDigest    string `json:"termsDigest"`
	IssuedBy       string `json:"issuedBy"`
	IssuedAt       string `json:"issuedAt"`
	SchemaVersion  int    `json:"schemaVersion"`
}

func BuildEvidence(manifest *domain.FrozenManifest, credential *domain.ReleaseCredential) *PublicEvidence {
	if manifest == nil || credential == nil {
		return nil
	}
	verification := VerifyCredential(manifest, credential)
	canonical := evidenceCanonical{Serial: credential.Serial, PackageID: credential.PackageID, ManifestDigest: credential.ManifestDigest, TermsDigest: credential.TermsDigest, IssuedBy: credential.IssuedBy, IssuedAt: credential.IssuedAt.UTC().Format(time.RFC3339Nano), SchemaVersion: credential.SchemaVersion}
	digest, err := JSONDigest(canonical)
	if err != nil {
		verification.Valid = false
		verification.Message = "无法计算凭据证据摘要"
	}
	return &PublicEvidence{CredentialLabel: fmt.Sprintf("OH-%s-%06d", credential.IssuedAt.UTC().Format("2006"), credential.Serial), PackageID: credential.PackageID, ManifestDigest: credential.ManifestDigest, TermsDigest: credential.TermsDigest, EvidenceDigest: digest, IssuedBy: credential.IssuedBy, IssuedAt: credential.IssuedAt, SchemaVersion: credential.SchemaVersion, Verification: verification}
}

func VerifyEvidence(evidence PublicEvidence, manifest domain.FrozenManifest, credential domain.ReleaseCredential) Verification {
	base := VerifyCredential(&manifest, &credential)
	if !base.Valid {
		return base
	}
	canonical := evidenceCanonical{Serial: credential.Serial, PackageID: credential.PackageID, ManifestDigest: credential.ManifestDigest, TermsDigest: credential.TermsDigest, IssuedBy: credential.IssuedBy, IssuedAt: credential.IssuedAt.UTC().Format(time.RFC3339Nano), SchemaVersion: credential.SchemaVersion}
	digest, err := JSONDigest(canonical)
	if err != nil {
		return Verification{Valid: false, Message: "无法计算凭据证据摘要", Serial: credential.Serial}
	}
	if evidence.EvidenceDigest != digest || evidence.ManifestDigest != manifest.Digest || evidence.PackageID != manifest.PackageID {
		return Verification{Valid: false, Message: "凭据证据包与冻结清单不匹配", ComputedDigest: digest, Serial: credential.Serial}
	}
	base.Message = "凭据、证据摘要与冻结公开清单一致"
	return base
}
