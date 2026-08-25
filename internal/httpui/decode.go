package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maximumBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maximumBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errors.New("请求体超过 1 MiB 限制")
		}
		return errors.New("JSON 请求体格式错误或包含未知字段")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}
