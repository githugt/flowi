// SPDX-License-Identifier: Apache-2.0

package snippet

import (
	"fmt"
	"strings"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// The command-line tools that speak the other kinds of request: a gRPC call, a message, a SQL statement.
//
// Before: only HTTP had a Code section, although the README promised snippets, so a gRPC call, a message or a query
// could not be handed to a colleague or a script as a command.
//
// The input is the preview every kind already has: Method is GRPC, SQL or the protocol's verb (MQTT, KAFKA…), URL is
// the target (grpc://host:port/pkg.Svc/Method, mqtt://host:1883/topic, postgres://…), Body is the message or the
// statement. A request with a body is sent (publish, call, run); a message request with none listens.
const (
	Grpcurl   Format = "grpcurl"
	Mosquitto Format = "mosquitto"
	Kcat      Format = "kcat"
	NatsCLI   Format = "nats"
	RedisCLI  Format = "redis-cli"
	Websocat  Format = "websocat"
	Psql      Format = "psql"
	MySQL     Format = "mysql"
	SQLite    Format = "sqlite3"
)

var otherLabels = map[Format]string{
	Grpcurl: "gRPC – grpcurl", Mosquitto: "MQTT – mosquitto_pub / mosquitto_sub", Kcat: "Kafka – kcat", NatsCLI: "NATS – nats",
	RedisCLI: "Redis – redis-cli", Websocat: "WebSocket – websocat", Psql: "PostgreSQL – psql", MySQL: "MySQL – mysql", SQLite: "SQLite – sqlite3",
}

// FormatsFor lists the formats that can say r, in display order: the HTTP ones for an HTTP request, the tool of its
// protocol for another kind, and none when no common command-line tool speaks it (SQS, Pub/Sub, a data file).
func FormatsFor(r engine.PreviewRequest) []Format {
	switch kind := kindOf(r); kind {
	case "":
		return Formats()
	case "grpc":
		return []Format{Grpcurl}
	case "sql":
		if f, ok := sqlTool(r.URL); ok {
			return []Format{f}
		}
	default:
		if f, ok := messageTool[kind]; ok {
			return []Format{f}
		}
	}
	return nil
}

var messageTool = map[string]Format{
	model.ProtocolMQTT: Mosquitto, model.ProtocolKafka: Kcat, model.ProtocolNATS: NatsCLI, model.ProtocolRedis: RedisCLI, model.ProtocolWS: Websocat,
}

// kindOf is "" for HTTP, "grpc", "sql", or the protocol of a message request.
func kindOf(r engine.PreviewRequest) string {
	switch {
	case strings.EqualFold(r.Method, model.ProtocolVerbGRPC):
		return "grpc"
	case r.Method == "SQL":
		return "sql"
	}
	return model.ProtocolOfVerb(r.Method)
}

// generateOther renders r in a format of the other kinds; ok is false when f is not one of them.
func generateOther(f Format, r engine.PreviewRequest) (text string, ok bool, err error) {
	if _, known := otherLabels[f]; !known {
		return "", false, nil
	}
	if !contains(FormatsFor(r), f) {
		return "", true, fmt.Errorf("%s cannot say this request: it is %s", otherLabels[f], describeKind(r))
	}
	t := splitTarget(r.URL)
	switch f {
	case Grpcurl:
		return grpcurl(r, t), true, nil
	case Mosquitto:
		return mosquitto(r, t), true, nil
	case Kcat:
		return kcat(r, t), true, nil
	case NatsCLI:
		return natsCLI(r, t), true, nil
	case RedisCLI:
		return redisCLI(r, t), true, nil
	case Websocat:
		return websocat(r), true, nil
	default:
		return sqlCommand(f, r, t), true, nil
	}
}

func describeKind(r engine.PreviewRequest) string {
	if k := kindOf(r); k != "" {
		return "a " + k + " request"
	}
	return "an HTTP request"
}

func contains(list []Format, f Format) bool {
	for _, x := range list {
		if x == f {
			return true
		}
	}
	return false
}

// target is a URL taken apart by hand: a template or a secret placeholder in it would fail net/url.
type target struct {
	scheme, userinfo, host, path string // host is host:port, or a comma-separated list of them
}

func splitTarget(u string) target {
	var t target
	rest := u
	if s, r, found := strings.Cut(u, "://"); found {
		t.scheme, rest = strings.ToLower(s), r
	}
	authority, path, _ := strings.Cut(rest, "/")
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		t.userinfo, authority = authority[:at], authority[at+1:]
	}
	t.host, t.path = authority, path
	return t
}

func (t target) hostPort(defaultPort string) (host, port string) {
	host, port = t.host, defaultPort
	if i := strings.LastIndex(t.host, ":"); i >= 0 && !strings.HasSuffix(t.host, "]") {
		host, port = t.host[:i], t.host[i+1:]
	}
	return strings.Trim(host, "[]"), port
}

func (t target) userPass() (user, pass string) {
	user, pass, _ = strings.Cut(t.userinfo, ":")
	return user, pass
}

func hasBody(r engine.PreviewRequest) bool { return r.BodyKind != "" }

// printfPipe feeds the body to a tool that reads its message from standard input.
func printfPipe(body string) string { return "printf %s " + shellQuote(body) + " | " }

func grpcurl(r engine.PreviewRequest, t target) string {
	args := []string{"grpcurl"}
	if t.scheme == model.GRPCSchemePlaintext {
		args = append(args, "-plaintext")
	}
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+": "+h.Value))
	}
	if hasBody(r) {
		args = append(args, "-d "+shellQuote(r.Body))
	}
	args = append(args, shellQuote(t.host), shellQuote(t.path))
	return strings.Join(args, shellContinuation)
}

func mosquitto(r engine.PreviewRequest, t target) string {
	host, port := t.hostPort("1883")
	args := []string{"-h " + shellQuote(host), "-p " + port, "-t " + shellQuote(t.path)}
	if t.scheme == "mqtts" || t.scheme == "ssl" {
		args = append(args, "--capath /etc/ssl/certs")
	}
	if user, pass := t.userPass(); user != "" {
		args = append(args, "-u "+shellQuote(user))
		if pass != "" {
			args = append(args, "-P "+shellQuote(pass))
		}
	}
	if hasBody(r) {
		return strings.Join(append([]string{"mosquitto_pub"}, append(args, "-m "+shellQuote(r.Body))...), shellContinuation)
	}
	return strings.Join(append([]string{"mosquitto_sub"}, append(args, "-C 1")...), shellContinuation)
}

func kcat(r engine.PreviewRequest, t target) string {
	args := []string{"kcat", "-b " + shellQuote(t.host), "-t " + shellQuote(t.path)}
	if !hasBody(r) {
		return strings.Join(append(args, "-C", "-c 1", "-e"), shellContinuation)
	}
	args = append(args, "-P")
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+"="+h.Value))
	}
	return printfPipe(r.Body) + strings.Join(args, shellContinuation)
}

func natsCLI(r engine.PreviewRequest, t target) string {
	server := "nats://" + t.host
	if t.userinfo != "" {
		server = "nats://" + t.userinfo + "@" + t.host
	}
	args := []string{"nats", "--server " + shellQuote(server)}
	if !hasBody(r) {
		return strings.Join(append(args, "sub "+shellQuote(t.path), "--count 1"), shellContinuation)
	}
	args = append(args, "pub "+shellQuote(t.path), shellQuote(r.Body))
	for _, h := range r.Headers {
		args = append(args, "-H "+shellQuote(h.Name+":"+h.Value))
	}
	return strings.Join(args, shellContinuation)
}

func redisCLI(r engine.PreviewRequest, t target) string {
	server := "redis://" + t.host
	if t.userinfo != "" {
		server = "redis://" + t.userinfo + "@" + t.host
	}
	if t.scheme == "rediss" {
		server = "rediss" + strings.TrimPrefix(server, "redis")
	}
	command := "SUBSCRIBE " + shellQuote(t.path)
	if hasBody(r) {
		command = "PUBLISH " + shellQuote(t.path) + " " + shellQuote(r.Body)
	}
	return strings.Join([]string{"redis-cli -u " + shellQuote(server), command}, shellContinuation)
}

func websocat(r engine.PreviewRequest) string {
	if hasBody(r) {
		return printfPipe(r.Body) + "websocat -n1 " + shellQuote(r.URL)
	}
	return "websocat " + shellQuote(r.URL)
}

// sqlTool is the command-line client of the database r.URL names. A scheme-less path is SQLite only when it looks
// like a database file: a data file (CSV, Excel, Parquet) is read by Flowi's own engine, not by a client.
func sqlTool(url string) (Format, bool) {
	t := splitTarget(url)
	switch t.scheme {
	case "postgres", "postgresql":
		return Psql, true
	case "mysql", "mariadb":
		return MySQL, true
	case "sqlite", "file":
		return SQLite, true
	case "":
		for _, ext := range []string{".db", ".sqlite", ".sqlite3"} {
			if strings.HasSuffix(strings.ToLower(url), ext) {
				return SQLite, true
			}
		}
	}
	return "", false
}

func sqlCommand(f Format, r engine.PreviewRequest, t target) string {
	switch f {
	case Psql:
		return strings.Join([]string{"psql " + shellQuote(r.URL), "-c " + shellQuote(r.Body)}, shellContinuation)
	case MySQL:
		host, port := t.hostPort("3306")
		args := []string{"mysql", "-h " + shellQuote(host), "-P " + port}
		if user, pass := t.userPass(); user != "" {
			args = append(args, "-u "+shellQuote(user))
			if pass != "" {
				args = append(args, "-p"+shellQuote(pass))
			}
		}
		if db, _, _ := strings.Cut(t.path, "?"); db != "" {
			args = append(args, shellQuote(db))
		}
		return strings.Join(append(args, "-e "+shellQuote(r.Body)), shellContinuation)
	}
	file := r.URL
	if t.scheme != "" {
		file = "/" + strings.TrimPrefix(t.path, "/")
		if t.host != "" { // sqlite://relative/path.db: the host part is the first folder
			file = t.host + file
		}
	}
	return strings.Join([]string{"sqlite3 " + shellQuote(strings.TrimPrefix(file, "file:")), shellQuote(r.Body)}, shellContinuation)
}
