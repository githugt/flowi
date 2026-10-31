// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/githugt/flowi/lib/serve/runs"
)

// runView is a run as the API shows it.
type runView struct {
	RunID    string     `json:"run_id"`
	Status   string     `json:"status"`
	Flow     string     `json:"flow"`
	Source   string     `json:"source"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
	ReportID string     `json:"report_id,omitempty"` // the id the run's report is saved under
	Seed     uint64     `json:"seed,omitempty"`
	Messages []string   `json:"messages,omitempty"`
}

func view(r runs.Record) runView {
	v := runView{RunID: r.ID, Status: r.Status, Flow: r.Flow, Source: r.Source, Started: r.Started,
		ReportID: r.EngineRunID, Seed: r.Seed, Messages: r.Messages}
	if len(v.Messages) > maxMessages {
		v.Messages = v.Messages[:maxMessages]
	}
	if !r.Finished.IsZero() {
		f := r.Finished
		v.Finished = &f
	}
	return v
}

func contextWithWait(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		status, body = http.StatusInternalServerError, []byte(`{"error":"could not encode the answer"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n')) // the caller hung up: nothing to do about it
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
