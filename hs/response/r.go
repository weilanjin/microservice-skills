package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Resp struct {
	Code int    `json:"codes"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
	Err  string `json:"err,omitempty"`
}

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("JSON error", "v", v, "err", err)
	}
}
