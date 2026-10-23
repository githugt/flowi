// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/engine"
)

// Format is a snippet language.
type Format string

// Formats.
const (
	Curl       Format = "curl"
	Wget       Format = "wget"
	PowerShell Format = "powershell"
	Xh         Format = "xh"
)

// Formats lists every format, in display order.
func Formats() []Format { return []Format{Curl, Wget, PowerShell, Xh} }

// Label is a format's display name.
func Label(f Format) string {
	switch f {
	case Curl:
		return "cURL"
	case Wget:
		return "Shell – wget"
	case PowerShell:
		return "PowerShell – Invoke-RestMethod"
	case Xh:
		return "xh / HTTPie"
	}
	return string(f)
}

// Generate renders r as format f.
func Generate(f Format, r engine.PreviewRequest) (string, error) {
	switch f {
	case Curl:
		return curl(r), nil
	case Wget:
		return wget(r), nil
	case PowerShell:
		return powerShell(r), nil
	case Xh:
		return xh(r), nil
	}
	return "", fmt.Errorf("unknown snippet format %q", f)
}

// shellQuote single-quotes s for a POSIX shell.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// psQuote single-quotes s for PowerShell.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

const (
	shellContinuation = " \\\n  "
	psContinuation    = " `\n  "
)

func curl(r engine.PreviewRequest) string {
	args := []string{"curl"}
	switch r.Method {
	case "", "GET":
	case "HEAD":
		args[0] = "curl -I"
	default:
		args[0] = "curl -X " + r.Method
	}
	args[0] += " " + shellQuote(r.URL)
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	switch r.BodyKind {
	case "text", "form":
		args = append(args, "--data-raw "+shellQuote(r.Body))
	case "file":
		args = append(args, "--data-binary "+shellQuote("@"+r.File))
	case "multipart":
		for _, p := range r.Parts {
			args = append(args, "-F "+shellQuote(curlPart(p)))
		}
	}
	return strings.Join(args, shellContinuation)
}

func curlPart(p engine.PreviewPart) string {
	if p.File == "" {
		return p.Name + "=" + p.Value
	}
	out := p.Name + "=@" + p.File
	if p.ContentType != "" {
		out += ";type=" + p.ContentType
	}
	return out
}

func wget(r engine.PreviewRequest) string {
	if r.BodyKind == "multipart" {
		return "# wget cannot send multipart/form-data; use the cURL or xh snippet instead."
	}
	args := []string{"wget -O -"}
	if r.Method != "" && r.Method != "GET" {
		args[0] += " --method=" + r.Method
	}
	for _, h := range r.Headers {
		args = append(args, "--header="+shellQuote(h.Name+": "+h.Value))
	}
	switch r.BodyKind {
	case "text", "form":
		args = append(args, "--body-data="+shellQuote(r.Body))
	case "file":
		args = append(args, "--body-file="+shellQuote(r.File))
	}
	args = append(args, shellQuote(r.URL))
	return strings.Join(args, shellContinuation)
}

func powerShell(r engine.PreviewRequest) string {
	method := "Get"
	if r.Method != "" {
		method = strings.ToUpper(r.Method[:1]) + strings.ToLower(r.Method[1:])
	}
	args := []string{"Invoke-RestMethod -Method " + method + " -Uri " + psQuote(r.URL)}
	var others []string
	for _, h := range r.Headers {
		if strings.EqualFold(h.Name, "Content-Type") {
			args = append(args, "-ContentType "+psQuote(h.Value))
			continue
		}
		others = append(others, psQuote(h.Name)+" = "+psQuote(h.Value))
	}
	if len(others) > 0 {
		args = append(args, "-Headers @{ "+strings.Join(others, "; ")+" }")
	}
	switch r.BodyKind {
	case "text", "form":
		args = append(args, "-Body "+psQuote(r.Body))
	case "file":
		args = append(args, "-InFile "+psQuote(r.File))
	case "multipart":
		args = append(args, "-Form @{ "+strings.Join(psParts(r.Parts), "; ")+" }")
	}
	return strings.Join(args, psContinuation)
}

func psParts(parts []engine.PreviewPart) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		value := psQuote(p.Value)
		if p.File != "" {
			value = "Get-Item -Path " + psQuote(p.File)
		}
		out[i] = psQuote(p.Name) + " = " + value
	}
	return out
}

func xh(r engine.PreviewRequest) string {
	head := "xh"
	if r.Method != "" && r.Method != "GET" {
		head += " " + r.Method
	}
	args := []string{head + " " + shellQuote(r.URL)}
	for _, h := range r.Headers {
		args = append(args, shellQuote(h.Name+":"+h.Value))
	}
	switch r.BodyKind {
	case "text", "form":
		args = append(args, "--raw "+shellQuote(r.Body))
	case "file":
		args = append(args, "< "+shellQuote(r.File)) // xh reads a body from stdin
	case "multipart":
		args = append(args, "--multipart")
		for _, p := range r.Parts {
			args = append(args, shellQuote(xhPart(p)))
		}
	}
	return strings.Join(args, shellContinuation)
}

func xhPart(p engine.PreviewPart) string {
	if p.File == "" {
		return p.Name + "=" + p.Value
	}
	out := p.Name + "@" + p.File
	if p.ContentType != "" {
		out += ";type=" + p.ContentType
	}
	return out
}
