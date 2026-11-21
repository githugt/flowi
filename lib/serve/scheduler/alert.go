// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/serve/runs"
)

const envPrefix = "env:"

// Outcome statuses an alert depends on, as the run manager reports them (the engine's status text).
const (
	statusPassed    = "passed"
	statusCancelled = "cancelled"
)

// hookProblem says what is wrong with an on_failure / on_success value, "" when it is fine or empty.
func hookProblem(v string) string {
	if v == "" {
		return ""
	}
	if name, ok := strings.CutPrefix(v, envPrefix); ok {
		if name == "" || strings.ContainsAny(name, "= \t") {
			return "env: must be followed by the name of an environment variable"
		}
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "must be an http or https address, or env:NAME to read it from an environment variable"
	}
	return ""
}

// Alert is what an alert tells: a schedule's run ended.
type Alert struct {
	Schedule string
	Flow     string
	Env      string
	Record   runs.Record
}

// alertText is the one message an alert carries: what ran, how it ended, the first reasons and the run's id.
// It holds no address and no inputs.
func alertText(a Alert) string {
	env := ""
	if a.Env != "" {
		env = " in " + a.Env
	}
	text := fmt.Sprintf("Flowi: schedule %s (flow %s)%s %s", a.Schedule, a.Flow, env, a.Record.Status)
	msgs := a.Record.Messages
	for i, m := range msgs {
		if i == 3 {
			text += fmt.Sprintf("\n… and %d more", len(msgs)-3)
			break
		}
		text += "\n• " + shorten(m, 200)
	}
	if a.Record.ID != "" {
		text += "\nrun " + a.Record.ID
	}
	return text
}

func shorten(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// alertBody is the JSON a Slack-compatible webhook accepts.
func alertBody(a Alert) []byte {
	b, _ := json.Marshal(map[string]string{"text": alertText(a)})
	return b
}

// resolveHook turns the configured value into the address to post to. An env: value that is not set is an error.
func resolveHook(v string, getenv func(string) string) (string, error) {
	name, ok := strings.CutPrefix(v, envPrefix)
	if !ok {
		return v, nil
	}
	got := getenv(name)
	if got == "" {
		return "", fmt.Errorf("the environment variable %s is not set", name)
	}
	if p := hookProblem(got); p != "" {
		return "", fmt.Errorf("the environment variable %s does not hold an http(s) address", name)
	}
	return got, nil
}

// hostOf names where an alert went without the path, which often holds a token.
func hostOf(addr string) string {
	if u, err := url.Parse(addr); err == nil && u.Host != "" {
		return u.Host
	}
	return "?"
}

// httpPost sends the alert; it follows no redirect and gives up after ten seconds.
func httpPost(ctx context.Context, addr string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr, bytes.NewReader(body))
	if err != nil {
		return errors.New("the address could not be used")
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // the url.Error text repeats the whole address, token included
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("the receiver answered %s", resp.Status)
	}
	return nil
}

func defaultGetenv(k string) string { return os.Getenv(k) } //nolint:forbidigo // the one place the scheduler reads the environment, replaced in tests
