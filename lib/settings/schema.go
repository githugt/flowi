// SPDX-License-Identifier: Apache-2.0

// Package settings defines every Flowi setting, parses values, and
// resolves them across layers (built-in default → user → workspace →
// collection → folder → request → node → environment variable → flag).
//
// It is the only package that reads process environment variables.
package settings

import "slices"

// Kind is a setting's value type.
type Kind int

// Kinds.
const (
	KindString Kind = iota + 1
	KindBool
	KindInt
	KindDuration
	KindEnum
	KindStringList
)

// Layer is where a value came from. Higher layers win.
type Layer int

// Layers in ascending precedence.
const (
	LayerDefault Layer = iota
	LayerUser
	LayerWorkspace
	LayerCollection
	LayerFolder
	LayerRequest
	LayerNode
	LayerEnv
	LayerFlag
)

func (l Layer) String() string {
	names := []string{"default", "user", "workspace", "collection", "folder", "request", "node", "env", "flag"}
	if int(l) < 0 || int(l) >= len(names) {
		return "unknown"
	}
	return names[l]
}

// Key describes one setting. Env and flag layers may always set a key;
// FileLayers lists which file layers may.
type Key struct {
	Name        string
	Description string
	Kind        Kind
	Default     any
	Enum        []string
	Min, Max    int64 // KindInt bounds; Max 0 means unbounded
	FileLayers  []Layer
}

// Lookup finds a key by name.
func Lookup(schema []Key, name string) (Key, bool) {
	i := slices.IndexFunc(schema, func(k Key) bool { return k.Name == name })
	if i < 0 {
		return Key{}, false
	}
	return schema[i], true
}

// Schema returns every setting in display order.
func Schema() []Key {
	all := []Layer{LayerUser, LayerWorkspace, LayerCollection, LayerFolder, LayerRequest, LayerNode}
	shared := []Layer{LayerUser, LayerWorkspace, LayerCollection}
	machine := []Layer{LayerUser, LayerWorkspace}
	personal := []Layer{LayerUser}
	const (
		mib            = 1 << 20
		maxConcurrency = 1024
		maxRetries     = 10
		maxHookDepth   = 16
		maxServeRuns   = 256
		maxTrafficLog  = 100000
	)
	return []Key{
		{Name: "timeout", Kind: KindDuration, Default: "30s", FileLayers: all,
			Description: "Maximum time for one HTTP request."},
		{Name: "responses.save", Kind: KindEnum, Default: "last", Enum: []string{"none", "last", "per-status"}, FileLayers: all,
			Description: "Which responses to keep: none, the last one, or the last one per status code."},
		{Name: "responses.pin", Kind: KindEnum, Default: "off", Enum: []string{"off", "first", "last"}, FileLayers: all,
			Description: "Pin a response as an example of the request, next to the request file, so it is committed and the replayer prefers it: off pins nothing, first pins the first response of each status and never replaces it, last pins every response and replaces the one before. Pinned examples are named after the status."},
		{Name: "schemas.infer", Kind: KindBool, Default: true, FileLayers: all,
			Description: "Learn a JSON Schema from the request and response bodies of each send, and check later sends against it (a request mismatch warns, a response mismatch is an error; neither stops the send)."},
		{Name: "empty", Kind: KindEnum, Default: "omit", Enum: []string{"keep", "omit", "error"}, FileLayers: all,
			Description: "How empty and null values are treated in headers and query parameters."},
		{Name: "http.version", Kind: KindEnum, Default: "auto", Enum: []string{"auto", "1.1", "2", "h2c", "3"}, FileLayers: all,
			Description: "HTTP protocol version; auto negotiates HTTP/2 when the server offers it; 3 uses HTTP/3 over QUIC and cannot go through a proxy."},
		{Name: "http.follow_redirects", Kind: KindBool, Default: true, FileLayers: all,
			Description: "Follow 3xx redirects automatically."},
		{Name: "http.reuse_connections", Kind: KindBool, Default: true, FileLayers: all,
			Description: "Reuse connections between the requests of one flow run: the first request to a host makes the DNS lookup and the TCP and TLS handshakes and the later ones skip them, as a real client would. Turn it off to time every request from a cold start (each shows its own DNS, connect and TLS time). A single send always starts cold."},
		{Name: "http.default_headers", Kind: KindStringList, Default: []any{"Accept: */*", "Cache-Control: no-cache"}, FileLayers: all,
			Description: "Headers (\"Name: value\") sent with every request that does not set them itself; set a header empty on a request to opt out of its default."},
		{Name: "http.user_agent", Kind: KindString, Default: "", FileLayers: all,
			Description: "User-Agent sent when a request does not set one; empty sends Flowi-UI/<version> from the app and Flowi-CLI/<version> from the command line."},
		{Name: "http.max_redirects", Kind: KindInt, Default: int64(10), Min: 0, Max: 100, FileLayers: all,
			Description: "Maximum redirects followed for one request."},
		{Name: "http.redirect_credentials", Kind: KindEnum, Default: "strip", Enum: []string{"strip", "same_site", "keep"}, FileLayers: all,
			Description: "Credential headers (Authorization, Cookie, and any header named like auth, token, key, secret, password, session) on a redirect to another origin: strip drops them when the host or port changes or https falls back to http; same_site drops them only when the registrable domain changes (api.example.com to www.example.com keeps them) or https falls back to http; keep sends them wherever the redirect goes."},
		{Name: "http.redirect_keep_headers", Kind: KindStringList, Default: []any{}, FileLayers: all,
			Description: "Header names that are sent on a redirect to another origin whatever http.redirect_credentials says (for example X-Api-Key for an API you trust)."},
		{Name: "http.cookies", Kind: KindEnum, Default: "isolated", Enum: []string{"isolated", "shared", "off"}, FileLayers: all,
			Description: "Cookie jars for batch and parallel iterations: isolated per iteration, shared, or off."},
		{Name: "http.request_compression", Kind: KindEnum, Default: "none", Enum: []string{"none", "gzip"}, FileLayers: all,
			Description: "Compress request bodies before sending."},
		{Name: "retry.max", Kind: KindInt, Default: int64(0), Min: 0, Max: maxRetries, FileLayers: all,
			Description: "Extra attempts after a retryable failure; 0 turns retries off."},
		{Name: "retry.backoff", Kind: KindEnum, Default: "exponential", Enum: []string{"exponential", "constant"}, FileLayers: all,
			Description: "Delay growth between attempts."},
		{Name: "retry.jitter", Kind: KindBool, Default: true, FileLayers: all,
			Description: "Randomise each retry delay (full jitter) so clients do not retry in lockstep."},
		{Name: "retry.on", Kind: KindStringList, Default: []any{"5xx", "timeout", "connection"}, Enum: []string{"5xx", "429", "timeout", "connection"}, FileLayers: all,
			Description: "Failures that are retried: 5xx, 429, timeout, connection."},
		{Name: "hooks.nested", Kind: KindBool, Default: true, FileLayers: all,
			Description: "Requests run inside a hook flow run their own hooks too, except the hook flow they are being sent from (so a hook that sends a request of its own collection cannot loop)."},
		{Name: "hooks.max_depth", Kind: KindInt, Default: int64(3), Min: 1, Max: maxHookDepth, FileLayers: all,
			Description: "Deepest nesting of hooks inside hooks when hooks.nested is on."},
		{Name: "downloads.stream_threshold_bytes", Kind: KindInt, Default: int64(10 * mib), Min: 1024, FileLayers: all,
			Description: "Response bodies larger than this are streamed to disk instead of memory."},
		{Name: "run.grace", Kind: KindDuration, Default: "60s", FileLayers: shared,
			Description: "Time a cancelled run may take to stop before it is aborted."},
		{Name: "run.keep", Kind: KindInt, Default: int64(200), Min: 0, Max: 100000, FileLayers: shared,
			Description: "How many saved run reports to keep for each flow; when a run ends, the older ones beyond this are deleted. 0 keeps every report (the run history then grows for ever)."},
		{Name: "run.keep_failed", Kind: KindInt, Default: int64(50), Min: 0, Max: 100000, FileLayers: shared,
			Description: "How many failed, errored or cancelled runs of each flow are kept after they fall out of run.keep, so a failure from long ago is still there to look at. 0 keeps none of them."},
		{Name: "run.max_concurrency", Kind: KindInt, Default: int64(8), Min: 1, Max: maxConcurrency, FileLayers: shared,
			Description: "Maximum concurrent nodes where a flow allows concurrency."},
		{Name: "serve.replay_addr", Kind: KindString, Default: "127.0.0.1:8525", FileLayers: machine,
			Description: "Address the replayer of flowi serve listens on."},
		{Name: "serve.trigger_addr", Kind: KindString, Default: "127.0.0.1:8526", FileLayers: machine,
			Description: "Address the HTTP flow triggers of flowi serve listen on."},
		{Name: "serve.latency", Kind: KindDuration, Default: "0s", FileLayers: machine,
			Description: "Delay the replayer adds to every answer (a request can ask for another with X-Flowi-Latency)."},
		{Name: "serve.max_runs", Kind: KindInt, Default: int64(4), Min: 1, Max: maxServeRuns, FileLayers: machine,
			Description: "Flow runs that triggers and schedules may have in flight at once."},
		{Name: "serve.shutdown_timeout", Kind: KindDuration, Default: "30s", FileLayers: machine,
			Description: "How long flowi serve lets in-flight requests and runs finish before cancelling them."},
		{Name: "serve.traffic_log", Kind: KindInt, Default: int64(1000), Min: 1, Max: maxTrafficLog, FileLayers: machine,
			Description: "Replayer calls kept in memory for the traffic log."},
		{Name: "otel.endpoint", Kind: KindString, Default: "", FileLayers: personal,
			Description: "OTLP/HTTP collector URL (for example http://localhost:4318); empty keeps tracing off."},
		{Name: "otel.headers", Kind: KindStringList, Default: []any{}, FileLayers: personal,
			Description: "Headers sent to the collector, as Name=value entries (typically an Authorization token)."},
		{Name: "otel.propagate", Kind: KindBool, Default: false, FileLayers: personal,
			Description: "Add a traceparent header to outgoing requests, so the APIs you call can join the trace."},
		{Name: "otel.trace_url", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Where a run's trace is in your tracing tool, with {trace_id} for the trace id (for example http://localhost:16686/trace/{trace_id}); empty hides the Open trace link."},
		{Name: "serve.valkey_addr", Kind: KindString, Default: "", FileLayers: machine,
			Description: "Valkey or Redis address (host:port) that several flowi serve instances share, so run records and schedule ticks are seen once; empty keeps each instance on its own."},
		{Name: "serve.valkey_tls", Kind: KindBool, Default: false, FileLayers: machine,
			Description: "Connect to serve.valkey_addr over TLS."},
		{Name: "vault.auto_lock", Kind: KindDuration, Default: "0s", FileLayers: machine,
			Description: "Lock the secrets vault after this long with no use, so opening it again asks for the passphrase (a vault whose key is in the OS keychain or FLOWI_VAULT_KEY opens again by itself). 0s never locks."},
		{Name: "defaults.environment", Kind: KindString, Default: "", FileLayers: machine,
			Description: "Environment used when none is selected."},
		{Name: "proxy.http", Kind: KindString, Default: "", FileLayers: machine,
			Description: "HTTP proxy URL for outgoing requests."},
		{Name: "tls.ca_files", Kind: KindStringList, Default: []any{}, FileLayers: machine,
			Description: "Extra CA certificate files (PEM) trusted for TLS."},
		{Name: "appearance.theme", Kind: KindEnum, Default: "system", Enum: []string{"system", "classic", "light", "dark", "high-contrast", "solarized-light", "solarized-dark", "nord", "anuppuccin-dark", "anuppuccin-light", "flowi-dark", "flowi-light", "custom"}, FileLayers: personal,
			Description: "GUI colour theme: follow the system (Anuppuccin colours in the OS light or dark variant), classic (Fyne's stock colours), light, dark, or one of the colour schemes high-contrast, solarized-light, solarized-dark, nord, anuppuccin-dark, anuppuccin-light, or custom (the palette file in appearance.palette_file)."},
		{Name: "appearance.palette_file", Kind: KindString, Default: "", FileLayers: personal,
			Description: "JSON palette file used by the custom theme (colours and the rainbow of collection rows)."},
		{Name: "appearance.accent", Kind: KindString, Default: "", FileLayers: personal,
			Description: "GUI accent colour (hex, e.g. #1d4ed8); empty uses the theme's."},
		{Name: "appearance.send_button", Kind: KindEnum, Default: "orange", Enum: []string{"orange", "violet", "emerald", "cyan", "outline"}, FileLayers: personal,
			Description: "Colour of the Send button in the GUI: orange (the default), violet, emerald, cyan, or an outline in the accent colour."},
		{Name: "appearance.density", Kind: KindEnum, Default: "comfortable", Enum: []string{"comfortable", "compact"}, FileLayers: personal,
			Description: "How much room the GUI's rows and controls take: comfortable (the default) or compact, to fit more on screen."},
		{Name: "appearance.font", Kind: KindString, Default: "", FileLayers: personal,
			Description: "GUI font: the name of a font installed on this machine (for example Inter); empty uses the built-in font. A file path does not work."},
		{Name: "appearance.mono_font", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Font of URLs, code and values: the name of an installed monospaced font (for example JetBrains Mono); empty uses the system's."},
		{Name: "appearance.font_size", Kind: KindInt, Default: int64(13), Min: 8, Max: 48, FileLayers: personal,
			Description: "GUI base font size in points."},
		{Name: "appearance.url.scheme", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of scheme (https) in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.separator", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the :// after the scheme in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.subdomain", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of a leading www. in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.host", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the domain and top-level domain in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.port", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the :port in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.path", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the path in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.variable", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of a {{variable}} that is defined in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.unknown_variable", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of a {{variable}} that is not defined in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.query_mark", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the ? and & of the query in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.equals", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the = of a query pair in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.fragment", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of the #fragment in the URL bar; empty uses the theme's."},
		{Name: "appearance.url.pairs", Kind: KindEnum, Default: "rainbow", Enum: []string{"rainbow", "single", "off"}, FileLayers: personal,
			Description: "How key=value pairs of the URL query are coloured: each pair in the next rainbow colour, all in one colour, or like the rest of the URL."},
		{Name: "appearance.url.pair_color", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Colour (#rrggbb) of query pairs when appearance.url.pairs is single; empty uses the theme's."},
		{Name: "appearance.url.pair_colors", Kind: KindStringList, Default: []any{}, FileLayers: personal,
			Description: "Colours (#rrggbb) the query pairs cycle through when appearance.url.pairs is rainbow; empty uses violet, indigo, blue, green, yellow, orange, red."},
		{Name: "language", Kind: KindEnum, Default: "en", Enum: []string{"en"}, FileLayers: personal,
			Description: "Language of the user interface."},
		{Name: "viewer.large_body_bytes", Kind: KindInt, Default: int64(2 * mib), Min: 1024, FileLayers: personal,
			Description: "Bodies larger than this open in raw view instead of the JSON tree."},
		{Name: "ui.default_section", Kind: KindEnum, Default: "auto", Enum: []string{"auto", "params", "headers", "body", "auth", "variables", "settings", "extract", "assert", "hooks", "docs"}, FileLayers: personal,
			Description: "The request section a request opens on; auto picks Docs, Body or Params from the request."},
		{Name: "ui.suggestions_max", Kind: KindInt, Default: int64(3), Min: 1, Max: 20, FileLayers: personal,
			Description: "How many suggestions the GUI shows while a {{ template is being typed."},
		{Name: "requests.seed_blueprint", Kind: KindString, Default: "default", FileLayers: all,
			Description: "The hooks a new request is created with: default (log the response status and fail on a non-2xx one), off, or the paths or ids of blueprints without parameters, separated by commas; each blueprint's nodes become the request's pre or post hook by the blueprint's phase."},
		{Name: "requests.graphql_errors", Kind: KindEnum, Default: "auto", Enum: []string{"auto", "warn", "fail", "ignore"}, FileLayers: all,
			Description: "What a response does when the request is GraphQL and the answer carries a non-empty errors list, which a server sends with status 200: fail fails the step, warn adds a warning, ignore says nothing. auto fails a request with a GraphQL body and warns for an HTTP request whose JSON body only looks like a query. A request whose assert lines mention response.body.errors handles it itself and is left alone."},
		{Name: "ui.restore_tabs", Kind: KindBool, Default: true, FileLayers: personal,
			Description: "Open the tabs that were open when the app was last closed."},
		{Name: "ui.drafts_trash_after", Kind: KindDuration, Default: "168h", FileLayers: personal,
			Description: "Move a try-out request or flow from the Drafts (Scratch) to the trash when it has not been changed for this long and is not open in a tab; 0s keeps them for ever. The trash keeps what it gets until you empty it."},
		{Name: "ui.keep_history", Kind: KindBool, Default: false, FileLayers: personal,
			Description: "Keep the run log and the response history between sessions."},
		{Name: "ui.snippet_exclude_secrets", Kind: KindBool, Default: true, FileLayers: personal,
			Description: "Code snippets show secret values as placeholders instead of the real values. Before it was off, so \"Copy as curl\" put live secrets on the clipboard by default."},
		{Name: "ui.hide_sensitive", Kind: KindBool, Default: false, FileLayers: personal,
			Description: "Blur secrets on screen, for a talk, a stream or a screenshot: header values such as Authorization and Cookie, variable values and anything that looks like a token. The values are only hidden, not removed: they are still in the window, and a copy still copies them."},
		{Name: "auth.default", Kind: KindString, Default: "", FileLayers: all,
			Description: "The named authentication (auths/<name>.auth.yaml) every request uses unless it sets its own auth. Set it in the collection's settings for the whole collection, or in a folder's for the requests under it; empty uses none."},
		{Name: "secrets.host_check", Kind: KindEnum, Default: "block", Enum: []string{"off", "warn", "block"}, FileLayers: personal,
			Description: "What happens when a request would send a secret to a host you have not trusted: block stops the send, warn sends it and says so, off does not check. A host is trusted when a variable of the selected environment names it, when it is this computer (localhost, 127.0.0.1) or when it is in secrets.allowed_hosts. A collection file cannot change this, so a cloned collection cannot send your secrets to its own server."},
		{Name: "secrets.allowed_hosts", Kind: KindStringList, Default: []any{}, FileLayers: personal,
			Description: "Hosts that may receive secrets besides those of the selected environment's variables: api.example.com (any port), api.example.com:8443 (that port only) or *.example.com (any subdomain). A collection file cannot add to this."},
		{Name: "datasets.allow_writes", Kind: KindBool, Default: false, FileLayers: personal,
			Description: "Let dataset (SQL) requests run statements that change data (INSERT, UPDATE, DELETE, DDL). Off keeps every query read-only; a collection file cannot turn this on."},
		{Name: "datasets.allow_outside_collection", Kind: KindBool, Default: false, FileLayers: personal,
			Description: "Let dataset requests read files outside the collection folder (a path that starts at / or climbs out with ../). Off keeps them inside; a collection file cannot turn this on, so a cloned collection cannot read your other files."},
		{Name: "datasets.read_only", Kind: KindBool, Default: false, FileLayers: all,
			Description: "Enforce read-only for every dataset (database and data file) request: no query may change data, whatever the request or datasets.allow_writes says. A collection or folder file may turn this on to protect what its requests touch."},
		{Name: "logging.level", Kind: KindEnum, Default: "info", Enum: []string{"debug", "info", "warn", "error"}, FileLayers: personal,
			Description: "Minimum log level."},
		{Name: "logging.format", Kind: KindEnum, Default: "text", Enum: []string{"text", "json"}, FileLayers: personal,
			Description: "Log line format."},
		{Name: "logging.file", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Append logs to this file; empty writes to standard error."},
		{Name: "telemetry", Kind: KindEnum, Default: "off", Enum: []string{"off", "minimal", "full"}, FileLayers: personal,
			Description: "Anonymous usage telemetry; off unless you opt in."},
		{Name: "telemetry_endpoint", Kind: KindString, Default: "", FileLayers: personal,
			Description: "URL that receives telemetry reports; empty sends nothing even when telemetry is on."},
		{Name: "announcements", Kind: KindBool, Default: true, FileLayers: personal,
			Description: "Show Flowi announcements at startup."},
		{Name: "announcements_url", Kind: KindString, Default: "", FileLayers: personal,
			Description: "Address of the signed announcements feed; empty fetches nothing."},
		{Name: "announcements_keys", Kind: KindStringList, Default: []any{}, FileLayers: personal,
			Description: "Trusted feed signing keys (base64 Ed25519 public keys); a feed signed by none of them is ignored."},
		{Name: "announcements_dismissed", Kind: KindStringList, Default: []any{}, FileLayers: personal,
			Description: "Ids of announcements you have dismissed with Got it; they are not shown again."},
	}
}
