package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type smokeView struct {
	Package struct {
		ID      string `json:"id"`
		Version uint64 `json:"version"`
		Status  string `json:"status"`
	} `json:"package"`
	Segments []struct {
		ID       string `json:"id"`
		Sequence int    `json:"sequence"`
	} `json:"segments"`
	AddedCount             int `json:"addedCount"`
	ClassificationProgress struct {
		Pending       int            `json:"pending"`
		RiskTagCounts map[string]int `json:"riskTagCounts"`
	} `json:"classificationProgress"`
	ReviewRound    int `json:"reviewRound"`
	ReviewProgress struct {
		Remaining int `json:"remaining"`
	} `json:"reviewProgress"`
	Credential *struct {
		Serial         uint64 `json:"serial"`
		ManifestDigest string `json:"manifestDigest"`
	} `json:"credential"`
	Verification struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message"`
	} `json:"verification"`
}

func runSelfcheck(baseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	packageID := fmt.Sprintf("selfcheck-%d", time.Now().UnixNano())
	version := uint64(0)
	counter := 0
	post := func(path string, role, actor string, fields map[string]any) (smokeView, error) {
		counter++
		fields["expectedVersion"] = version
		fields["idempotencyKey"] = fmt.Sprintf("selfcheck-%02d", counter)
		fields["role"] = role
		fields["actor"] = actor
		data, err := json.Marshal(fields)
		if err != nil {
			return smokeView{}, err
		}
		request, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
		if err != nil {
			return smokeView{}, err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return smokeView{}, err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		if err != nil {
			return smokeView{}, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return smokeView{}, fmt.Errorf("%s 返回 %d: %s", path, response.StatusCode, string(body))
		}
		var view smokeView
		if err := json.Unmarshal(body, &view); err != nil {
			return smokeView{}, err
		}
		version = view.Package.Version
		return view, nil
	}
	created, err := post("/api/packages", "organizer", "自检整理员", map[string]any{"id": packageID, "topic": "自检口述史", "participantCode": "SC-001", "ownerName": "自检整理员", "intendedScope": "公开展示"})
	if err != nil {
		return err
	}
	if created.Package.ID != packageID || version != 1 {
		return errors.New("创建访谈包结果不正确")
	}
	confirmed := time.Now().UTC().Add(-time.Minute)
	if _, err := post("/api/packages/"+packageID+"/consent", "organizer", "自检整理员", map[string]any{"terms": "参与者同意脱敏后用于公开展示", "allowedUses": []string{"公开展示"}, "attributionPreference": "使用代号", "confirmedAt": confirmed, "confirmedBy": "参与者代理确认"}); err != nil {
		return err
	}
	segments, err := post("/api/packages/"+packageID+"/segments/batch", "organizer", "自检整理员", map[string]any{"items": []map[string]any{{"id": "seg-2", "sequence": 2, "sourceText": "张三住在老街十七号并参与过相关事件。"}, {"id": "seg-1", "sequence": 1, "sourceText": "这是一段可以公开的行业记忆。"}}})
	if err != nil {
		return err
	}
	if segments.AddedCount != 2 || len(segments.Segments) != 2 || segments.Segments[0].ID != "seg-1" {
		return errors.New("批量片段新增数量或顺序不正确")
	}
	classified, err := post("/api/packages/"+packageID+"/classification/batch", "organizer", "自检整理员", map[string]any{"items": []map[string]any{{"segmentID": "seg-1", "decision": "public", "riskTags": []string{}}, {"segmentID": "seg-2", "decision": "restricted", "riskTags": []string{"privacy", "sensitive_place"}}}})
	if err != nil {
		return err
	}
	if classified.ClassificationProgress.Pending != 0 || classified.ClassificationProgress.RiskTagCounts["privacy"] != 1 {
		return errors.New("批量判定进度不正确")
	}
	if _, err := post("/api/packages/"+packageID+"/classification/complete", "organizer", "自检整理员", map[string]any{}); err != nil {
		return err
	}
	if _, err := post("/api/packages/"+packageID+"/segments/seg-2/revision", "organizer", "自检整理员", map[string]any{"segmentID": "seg-2", "publicText": "某位受访者居住在老街一带并参与过相关事件。", "reason": "隐去第三方姓名和精确住址"}); err != nil {
		return err
	}
	historyResponse, err := client.Get(baseURL + "/api/packages/" + packageID + "/segments/seg-2/revisions")
	if err != nil {
		return err
	}
	var history struct {
		Revisions []json.RawMessage `json:"revisions"`
	}
	if err := json.NewDecoder(historyResponse.Body).Decode(&history); err != nil {
		historyResponse.Body.Close()
		return err
	}
	historyResponse.Body.Close()
	if historyResponse.StatusCode != http.StatusOK || len(history.Revisions) != 1 {
		return errors.New("修订历史查询失败")
	}
	if _, err := post("/api/packages/"+packageID+"/review/submit", "organizer", "自检整理员", map[string]any{}); err != nil {
		return err
	}
	queueResponse, err := client.Get(baseURL + "/api/review-queue")
	if err != nil {
		return err
	}
	var queue struct {
		Packages []json.RawMessage `json:"packages"`
	}
	if err := json.NewDecoder(queueResponse.Body).Decode(&queue); err != nil {
		queueResponse.Body.Close()
		return err
	}
	queueResponse.Body.Close()
	if queueResponse.StatusCode != http.StatusOK || len(queue.Packages) != 1 {
		return errors.New("待复核队列查询失败")
	}
	reviewed, err := post("/api/packages/"+packageID+"/review/batch", "reviewer", "自检复核员", map[string]any{"reviewRound": 1, "items": []map[string]any{{"segmentID": "seg-1", "verdict": "approved", "reason": ""}, {"segmentID": "seg-2", "verdict": "approved", "reason": "脱敏充分"}}})
	if err != nil {
		return err
	}
	if reviewed.Package.Status != "approval_pending" || reviewed.ReviewProgress.Remaining != 0 {
		return errors.New("批量复核结果不正确")
	}
	previewResponse, err := client.Get(baseURL + "/api/packages/" + packageID + "/release/preview")
	if err != nil {
		return err
	}
	var preview struct {
		ManifestDigest string `json:"manifestDigest"`
		PackageVersion uint64 `json:"packageVersion"`
	}
	if err := json.NewDecoder(previewResponse.Body).Decode(&preview); err != nil {
		previewResponse.Body.Close()
		return err
	}
	previewResponse.Body.Close()
	if previewResponse.StatusCode != http.StatusOK || preview.ManifestDigest == "" || preview.PackageVersion != version {
		return errors.New("冻结清单预览失败")
	}
	released, err := post("/api/packages/"+packageID+"/release/approve", "release_manager", "自检开放负责人", map[string]any{"previewManifestDigest": preview.ManifestDigest})
	if err != nil {
		return err
	}
	if released.Package.Status != "released" || released.Credential == nil || released.Credential.Serial == 0 || !released.Verification.Valid {
		return errors.New("凭据签发或现场校验失败")
	}
	response, err := client.Get(baseURL + "/api/packages/" + packageID + "/credential/verify")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var verification struct {
		Valid bool `json:"valid"`
	}
	if err := json.NewDecoder(response.Body).Decode(&verification); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !verification.Valid {
		return errors.New("凭据查询入口校验失败")
	}
	return nil
}
