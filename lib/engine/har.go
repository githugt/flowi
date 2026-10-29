// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// WriteHAR writes records as a HAR 1.2 file, the format browsers' network tabs and most API tools read.
// Records are masked already, so the file holds no secret. Message based records (async and gRPC) have no
// HAR equivalent and are left out. version is Flowi's own, written as the creator.
func WriteHAR(w io.Writer, records []Record, version string) error {
	entries := make([]harEntry, 0, len(records))
	for i := range records {
		if records[i].HasMessages() {
			continue
		}
		entries = append(entries, harEntryOf(records[i]))
	}
	doc := harFile{Log: harLog{Version: "1.2", Creator: harCreator{Name: "flowi", Version: version}, Entries: entries}}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing HAR: %w", err)
	}
	return nil
}

type harFile struct {
	Log harLog `json:"log"`
}

type harLog struct {
	Version string     `json:"version"`
	Creator harCreator `json:"creator"`
	Entries []harEntry `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	Started  string      `json:"startedDateTime"`
	Time     float64     `json:"time"`
	Request  harRequest  `json:"request"`
	Response harResponse `json:"response"`
	Cache    struct{}    `json:"cache"`
	Timings  harTimings  `json:"timings"`
}

type harRequest struct {
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	HTTPVersion string    `json:"httpVersion"`
	Cookies     []harPair `json:"cookies"`
	Headers     []harPair `json:"headers"`
	QueryString []harPair `json:"queryString"`
	PostData    *harPost  `json:"postData,omitempty"`
	HeadersSize int       `json:"headersSize"`
	BodySize    int64     `json:"bodySize"`
}

type harPost struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type harResponse struct {
	Status      int        `json:"status"`
	StatusText  string     `json:"statusText"`
	HTTPVersion string     `json:"httpVersion"`
	Cookies     []harPair  `json:"cookies"`
	Headers     []harPair  `json:"headers"`
	Content     harContent `json:"content"`
	RedirectURL string     `json:"redirectURL"`
	HeadersSize int        `json:"headersSize"`
	BodySize    int64      `json:"bodySize"`
}

type harContent struct {
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

type harPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// harTimings uses -1 for a phase that did not happen, as the HAR spec says.
type harTimings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	SSL     float64 `json:"ssl"`
}

func harEntryOf(r Record) harEntry {
	t := r.Timings
	return harEntry{
		Started: r.At.UTC().Format(time.RFC3339Nano),
		Time:    millis(t.Total),
		Request: harRequestOf(r),
		Response: harResponse{
			Status: r.Status, HTTPVersion: r.Proto, Cookies: []harPair{}, Headers: harPairs(r.Header),
			Content:     harContentOf(r.Body),
			RedirectURL: headerValue(r.Header, "Location"), HeadersSize: -1, BodySize: r.Body.Size,
		},
		Timings: harTimings{
			Send: 0, Wait: millis(t.TTFB), Receive: millis(max(t.Total-t.TTFB, 0)),
			DNS: millis(t.DNS), Connect: millis(t.Connect), SSL: millis(t.TLS),
		},
	}
}

func harRequestOf(r Record) harRequest {
	req := harRequest{
		Method: r.Request.Method, URL: r.Request.URL, HTTPVersion: r.Proto, Cookies: []harPair{},
		Headers: harPairs(r.Request.Header), QueryString: queryPairs(r.Request.URL), HeadersSize: -1, BodySize: -1,
	}
	if b := r.Request.Body; b != nil {
		req.BodySize = b.Size
		req.PostData = &harPost{MimeType: b.ContentType, Text: b.Text}
	}
	return req
}

func harContentOf(b Body) harContent {
	c := harContent{Size: b.Size, MimeType: b.ContentType, Text: b.Text}
	if b.Base64 != "" {
		c.Text, c.Encoding = b.Base64, "base64"
	}
	return c
}

func harPairs(headers []Header) []harPair {
	out := make([]harPair, 0, len(headers))
	for _, h := range headers {
		out = append(out, harPair(h))
	}
	return out
}

func queryPairs(rawURL string) []harPair {
	out := []harPair{}
	u, err := url.Parse(rawURL)
	if err != nil {
		return out
	}
	for _, kv := range splitQuery(u.RawQuery) {
		name, _ := url.QueryUnescape(kv[0])
		value, _ := url.QueryUnescape(kv[1])
		out = append(out, harPair{Name: name, Value: value})
	}
	return out
}

// splitQuery keeps the order the query was written in, which url.Values does not.
func splitQuery(raw string) [][2]string {
	var out [][2]string
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		out = append(out, [2]string{name, value})
	}
	return out
}

func headerValue(headers []Header, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func millis(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
