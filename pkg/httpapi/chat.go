package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// chat is a stub endpoint that echoes a canned reply. Replace with a real
// LLM integration (Claude / OpenAI / etc.) later — the request/response shape
// stays the same.
func (d Deps) chat(w http.ResponseWriter, r *http.Request) {
	var in chatRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	last := ""
	for i := len(in.Messages) - 1; i >= 0; i-- {
		if in.Messages[i].Role == "user" {
			last = in.Messages[i].Content
			break
		}
	}
	reply := "Observer AI is not wired to a model yet. " +
		"I'll be able to help with devices, flows, dashboards, and telemetry. " +
		"For now I can confirm I heard you: \"" + last + "\""
	writeJSON(w, 200, chatResponse{Role: "assistant", Content: reply, CreatedAt: time.Now().UTC()})
}
