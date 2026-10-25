// SPDX-License-Identifier: Apache-2.0

package diag

// Diagnostic codes. Codes are stable identifiers: never renumber or reuse
// one, because users search for them and catalogues are keyed by them.
const (
	CodeYAMLSyntax       Code = "FLW-YAML-001"
	CodeYAMLUnknownKey   Code = "FLW-YAML-002"
	CodeYAMLDuplicateKey Code = "FLW-YAML-003"
	CodeYAMLType         Code = "FLW-YAML-004"
	CodeYAMLEmpty        Code = "FLW-YAML-005"

	CodeFormatTooNew  Code = "FLW-FMT-001"
	CodeFormatMissing Code = "FLW-FMT-002"

	CodeFileUnrecognised Code = "FLW-FILE-001"
	CodeFileUnreadable   Code = "FLW-FILE-002"

	CodeIDDuplicate Code = "FLW-ID-001"
	CodeIDInvalid   Code = "FLW-ID-002"
	CodeIDMissing   Code = "FLW-ID-003"

	CodeVersionInvalid       Code = "FLW-VER-001"
	CodeVersionDuplicate     Code = "FLW-VER-002"
	CodeVersionDefaultAbsent Code = "FLW-VER-003"
	CodeVersionPinnedAbsent  Code = "FLW-VER-004"
	CodeVersionNone          Code = "FLW-VER-005"

	CodeRefUnknownRequest Code = "FLW-REF-001"
	CodeRefUnknownFlow    Code = "FLW-REF-002"
	CodeRefUndeclared     Code = "FLW-REF-003"
	CodeRefNotMounted     Code = "FLW-REF-004"
	CodeRefStalePath      Code = "FLW-REF-005"

	CodeNodeUnknownType  Code = "FLW-NODE-001"
	CodeNodeDuplicateID  Code = "FLW-NODE-002"
	CodeNodeNeedsSibling Code = "FLW-NODE-003"
	CodeNodeMissingField Code = "FLW-NODE-004"
	CodeNodeParam        Code = "FLW-NODE-005"
	CodeNodeGoto         Code = "FLW-NODE-006"
	CodeNodeWriteClash   Code = "FLW-NODE-007"
	CodeNodeSubflowInput Code = "FLW-NODE-008"
	CodeNodeUnresolved   Code = "FLW-NODE-009"
	CodeNodeContract     Code = "FLW-NODE-010"
	CodeNodeAttach       Code = "FLW-NODE-011"

	CodeImportUnmapped  Code = "FLW-IMPORT-001"
	CodeImportInvalid   Code = "FLW-IMPORT-002"
	CodeImportExists    Code = "FLW-IMPORT-003"
	CodeImportSyntax    Code = "FLW-IMPORT-004"
	CodeImportSkipped   Code = "FLW-IMPORT-005"
	CodeImportSensitive Code = "FLW-IMPORT-006"
	CodeImportKeptEdit  Code = "FLW-IMPORT-007"

	CodeMermaidPlaceholder Code = "FLW-MERMAID-001"
	CodeMermaidCycle       Code = "FLW-MERMAID-002"
	CodeArazzoDrift        Code = "FLW-ARAZZO-001"
	CodeArazzoCustom       Code = "FLW-ARAZZO-002"
	CodeArazzoLimit        Code = "FLW-ARAZZO-003"

	CodeSchemaRequest  Code = "FLW-SCHEMA-001"
	CodeSchemaResponse Code = "FLW-SCHEMA-002"
	CodeSchemaUpdated  Code = "FLW-SCHEMA-003"
	CodeSchemaMessage  Code = "FLW-SCHEMA-004"

	CodeAsyncProtocol Code = "FLW-ASYNC-001"
	CodeAsyncAction   Code = "FLW-ASYNC-002"
	CodeAsyncBinding  Code = "FLW-ASYNC-003"
	CodeAsyncServer   Code = "FLW-ASYNC-004"
	CodeAsyncTimeout  Code = "FLW-ASYNC-005"

	CodeGRPCRequest Code = "FLW-GRPC-001"
	CodeGRPCMethod  Code = "FLW-GRPC-002"
	CodeGRPCProto   Code = "FLW-GRPC-003"
	CodeGRPCBody    Code = "FLW-GRPC-004"
	CodeGRPCStatus  Code = "FLW-GRPC-005"
	CodeGRPCField   Code = "FLW-GRPC-006"

	CodeExportLossy Code = "FLW-EXPORT-001"
	CodeExportValue Code = "FLW-EXPORT-002"

	CodeServeSchedule     Code = "FLW-SERVE-001"
	CodeServeScheduleFlow Code = "FLW-SERVE-002"

	CodeBlueprintParamType   Code = "FLW-BP-001"
	CodeBlueprintParamName   Code = "FLW-BP-002"
	CodeBlueprintRef         Code = "FLW-BP-003"
	CodeBlueprintParamUnused Code = "FLW-BP-004"
	CodeBlueprintUse         Code = "FLW-BP-005"
	CodeBlueprintNotFound    Code = "FLW-BP-006"
	CodeBlueprintAmbiguous   Code = "FLW-BP-007"

	CodeWriteDropsComments Code = "FLW-WRITE-001"
	CodeWriteRequestPath   Code = "FLW-WRITE-002"
	CodeWriteEditNone      Code = "FLW-WRITE-003"
	CodeWriteEditLabel     Code = "FLW-WRITE-004"

	CodeContractBroken  Code = "FLW-CONTRACT-001"
	CodeContractInput   Code = "FLW-CONTRACT-002"
	CodeContractExample Code = "FLW-CONTRACT-003"

	CodeFlowOption    Code = "FLW-FLOW-001"
	CodeFlowNotFound  Code = "FLW-FLOW-002"
	CodeFlowAmbiguous Code = "FLW-FLOW-003"

	CodeCycleNeeds   Code = "FLW-CYC-001"
	CodeCycleSubflow Code = "FLW-CYC-002"

	CodeLintWhitespace Code = "FLW-LINT-001"
	CodeLintControl    Code = "FLW-LINT-002"
	CodeLintInvisible  Code = "FLW-LINT-003"

	CodeWSMissing           Code = "FLW-WS-001"
	CodeWSAlreadyMounted    Code = "FLW-WS-002"
	CodeWSNotCollection     Code = "FLW-WS-003"
	CodeWSManyCollections   Code = "FLW-WS-004"
	CodeWSUnknownCollection Code = "FLW-WS-005"

	CodeSettingUnknown    Code = "FLW-SET-001"
	CodeSettingInvalid    Code = "FLW-SET-002"
	CodeSettingWrongLayer Code = "FLW-SET-003"

	CodeTemplateUndefined Code = "FLW-TPL-001"
	CodeTemplateSyntax    Code = "FLW-TPL-002"
	CodeTemplateEmpty     Code = "FLW-TPL-003"
	CodeTemplateGenerator Code = "FLW-TPL-004"

	// Diagnostic code names, not credentials (gosec G101 matches "Secret").
	CodeSecretMissing         Code = "FLW-SEC-001" //nolint:gosec // see above
	CodeSecretLocked          Code = "FLW-SEC-002" //nolint:gosec // see above
	CodeSecretUndeclared      Code = "FLW-SEC-003" //nolint:gosec // see above
	CodeSecretFromEnvironment Code = "FLW-SEC-004" //nolint:gosec // see above

	CodeRequestNotFound    Code = "FLW-REQ-001"
	CodeRequestAmbiguous   Code = "FLW-REQ-002"
	CodeRequestFileOutside Code = "FLW-REQ-003"
	CodeRequestEnvUnknown  Code = "FLW-REQ-004"
	CodeRequestHeader      Code = "FLW-REQ-005"
	CodeRequestFileRead    Code = "FLW-REQ-006"
	CodeRequestExampleName Code = "FLW-REQ-007"

	CodeRunAssertFailed  Code = "FLW-RUN-001"
	CodeRunAssertWarning Code = "FLW-RUN-002"
	CodeRunNodeType      Code = "FLW-RUN-003"
	CodeRunSetTarget     Code = "FLW-RUN-004"
	CodeRunExtractEmpty  Code = "FLW-RUN-005"
	CodeRunHookDepth     Code = "FLW-RUN-006"
	CodeRunNodeTimeout   Code = "FLW-RUN-007"
	CodeRunTillLimit     Code = "FLW-RUN-008"
	CodeRunInputMissing  Code = "FLW-RUN-009"
	CodeRunInputUnknown  Code = "FLW-RUN-010"
	CodeRunCalcNumber    Code = "FLW-RUN-011"
	CodeRunBatchOver     Code = "FLW-RUN-012"
	CodeRunExtractNoResp Code = "FLW-RUN-013"
	CodeRunDigestTarget  Code = "FLW-RUN-014"
	CodeRunHandled       Code = "FLW-RUN-015"
	CodeRunBackground    Code = "FLW-RUN-016"
	CodeRunFileName      Code = "FLW-RUN-017"
	CodeRunSaveStatus    Code = "FLW-RUN-018"
	CodeRunNoBranchOK    Code = "FLW-RUN-019"
	CodeRunWriteFailed   Code = "FLW-RUN-020"
	CodeRunStopped       Code = "FLW-RUN-021"

	CodeHTTPFailed  Code = "FLW-HTTP-001"
	CodeHTTPTimeout Code = "FLW-HTTP-002"

	CodeCycleHooks Code = "FLW-CYC-003"
)

// codeInfo is the data behind each code: its default severity and English
// template. Keeping it in one table makes "every code has a message" a
// property of the data rather than of scattered switch statements.
type codeInfo struct {
	severity Severity
	english  string // "{arg}" placeholders; text after hintMarker becomes the hint
}

const hintMarker = " — hint: "

func codeTable() map[Code]codeInfo {
	e, w := SeverityError, SeverityWarning
	return map[Code]codeInfo{
		CodeYAMLSyntax:       {e, "YAML syntax error: {detail}"},
		CodeYAMLUnknownKey:   {e, `unknown key "{key}"`},
		CodeYAMLDuplicateKey: {e, `duplicate key "{key}"`},
		CodeYAMLType:         {e, `"{key}" must be {expected}, got {got}`},
		CodeYAMLEmpty:        {e, "file is empty"},

		CodeFormatTooNew:  {e, "format {got} is newer than supported {max}; upgrade Flow-I"},
		CodeFormatMissing: {e, `missing "format" in flowi.yaml`},

		CodeFileUnrecognised: {w, `unrecognised file "{name}" (expected *.req.yaml / *.arazzo.yaml / _folder.yaml)`},
		CodeFileUnreadable:   {w, `cannot read "{name}": {detail}`},

		CodeIDDuplicate: {e, "duplicate id {id}"},
		CodeIDInvalid:   {e, `invalid id "{id}" (must be a ULID)`},
		CodeIDMissing:   {e, "missing id"},

		CodeVersionInvalid:       {e, `invalid version label "{label}"`},
		CodeVersionDuplicate:     {e, `version "{label}" duplicates "{other}"`},
		CodeVersionDefaultAbsent: {e, `default version "{label}" does not exist`},
		CodeVersionPinnedAbsent:  {e, `pinned version "{label}" does not exist on request {request}`},
		CodeVersionNone:          {e, "request has no versions"},

		CodeRefUnknownRequest: {e, "unknown request id {id}"},
		CodeRefUnknownFlow:    {e, "unknown flow id {id}"},
		CodeRefUndeclared:     {e, `collection "{collection}" is not declared in depends_on`},
		CodeRefNotMounted:     {e, `collection "{collection}" is not mounted` + hintMarker + `clone {repo} and run "flowi workspace mount"`},
		CodeRefStalePath:      {w, `path hint "{got}" is out of date, should be "{want}" (run "flowi fmt")`},

		CodeNodeUnknownType:  {e, `unknown node type "{type}"`},
		CodeNodeDuplicateID:  {e, `duplicate node id "{node}" in flow`},
		CodeNodeNeedsSibling: {e, `needs "{target}" is not a sibling node` + hintMarker + "needs can only reference nodes in the same list"},
		CodeNodeMissingField: {e, `node type "{type}" requires "{field}"`},
		CodeNodeParam:        {e, `invalid {field} of node "{node}": {detail}`},
		CodeNodeGoto:         {e, `on_error of node "{node}" goes to "{target}", which is not a later sibling` + hintMarker + "goto can only jump forward within the same list; use run_till to repeat"},
		CodeNodeAttach:       {w, `node "{node}" is shown with "{target}", but {detail}` + hintMarker + "an attached node sits right next to its request, or next to others attached to it: reorder it, or clear its attach"},
		CodeNodeWriteClash:   {e, `nodes "{a}" and "{b}" may run at the same time and both write {target}`},
		CodeNodeSubflowInput: {e, `subflow node "{node}": {detail} "{name}" of flow {flow}`},
		CodeNodeUnresolved:   {e, "nothing produces `{name}` used in {field}{hint}"},
		CodeNodeContract:     {w, `node "{node}" reads "{name}", which request {request} version {version} does not extract`},

		CodeImportUnmapped:  {w, "{what} has no Flow-I equivalent and was {how}: {detail}"},
		CodeImportInvalid:   {e, "not a document Flow-I can import: {detail}"},
		CodeImportExists:    {e, "{count} of the files to import already exist ({files})" + hintMarker + "overwrite them, or skip them to add only the new files"},
		CodeImportSyntax:    {e, "{format} syntax error: {detail}"},
		CodeImportSkipped:   {e, "{item} was skipped: {detail}"},
		CodeImportSensitive: {w, "{where} looked like a credential and was replaced by {placeholder}" + hintMarker + "set the value with flowi secret set"},
		CodeImportKeptEdit:  {w, "{what} was changed both by you and by the spec; your edit was kept" + hintMarker + "compare it with the spec, and save it to the spec overlay if the spec should say so too"},

		CodeMermaidPlaceholder: {w, `"{label}" does not resolve and became a placeholder note: {detail}`},
		CodeMermaidCycle:       {e, "the diagram has a cycle through {nodes}; flows run once, top to bottom"},
		CodeArazzoDrift:        {w, "the steps of {flow} were edited outside Flow-I; Flow-I's own data is used"},
		CodeArazzoLimit:        {w, `step "{step}" of {flow} has a {what} that other Arazzo tools do not see`},
		CodeArazzoCustom:       {w, `step "{step}" of {flow} is a Flow-I step ({type}); other Arazzo tools cannot run it`},

		CodeSchemaRequest:  {w, "the request body of {request} does not match its schema at {path}: {message}"},
		CodeSchemaResponse: {e, "the response ({status}) of {request} does not match its schema at {path}: {message}"},
		CodeSchemaMessage:  {e, "a message received by {request} does not match its schema at {path}: {message}"},
		CodeSchemaUpdated:  {w, "the schema of the {what} of {request} was updated at {path}: {detail}"},

		CodeAsyncProtocol: {e, `unknown async protocol "{protocol}" (use ws, sse, mqtt, kafka, amqp or nats)`},
		CodeAsyncAction:   {e, "async request needs a send or a receive, and receive count and timeout must be positive: {detail}"},
		CodeAsyncBinding:  {e, `binding "{name}" is not an option of {protocol}`},
		CodeAsyncServer:   {e, `server "{server}" does not fit protocol {protocol} (expected {schemes})`},
		CodeAsyncTimeout:  {e, "received {got} of {want} messages before the {timeout} timeout"},

		CodeGRPCRequest: {e, "{detail}"},
		CodeGRPCMethod:  {e, "method {method} is not available: {detail}"},
		CodeGRPCProto:   {e, "{detail}"},
		CodeGRPCBody:    {e, "the message does not fit {method}: {detail}"},
		CodeGRPCStatus:  {e, "the call ended with {code}: {message}"},
		CodeGRPCField:   {w, "{detail}"},

		CodeExportLossy:       {w, `node "{node}" ({type}) has no Arazzo equivalent and was left out`},
		CodeServeSchedule:     {e, `schedule "{name}": {detail}`},
		CodeServeScheduleFlow: {w, `schedule "{name}" cannot run: {detail}` + hintMarker + "it is skipped until the flow can be found"},
		CodeExportValue:       {w, `{what} of "{node}" is {value}, which has no Arazzo equivalent, and was left out`},

		CodeBlueprintParamType:   {e, `parameter "{name}" has {detail}`},
		CodeBlueprintParamName:   {e, `parameter "{name}": {detail}`},
		CodeBlueprintRef:         {e, `node "{node}" refers to parameter "{param}": {detail}`},
		CodeBlueprintParamUnused: {w, `parameter "{name}" is never used by the blueprint`},
		CodeBlueprintUse:         {e, `cannot use blueprint "{blueprint}": {detail}`},
		CodeBlueprintNotFound:    {e, `no blueprint matches "{ref}"` + hintMarker + "use an id, or a path under blueprints/ without the .blueprint.yaml suffix"},
		CodeBlueprintAmbiguous:   {e, `"{ref}" matches blueprints in {count} collections` + hintMarker + "name the collection"},

		CodeWriteDropsComments: {e, "saving rewrites {file} in canonical form and drops its comments" + hintMarker + "confirm to save anyway, or edit the file by hand"},
		CodeWriteRequestPath:   {e, `cannot create request "{path}": {detail}`},
		CodeWriteEditNone:      {e, "request {request} has no edit to {action}" + hintMarker + "make an edit first"},
		CodeWriteEditLabel:     {e, `cannot save the edit as version "{label}": {detail}` + hintMarker + "choose a version label that does not exist yet"},

		CodeContractBroken:  {w, "{reader} reads {read} from {request} version {version}, which {source} does not contain"},
		CodeContractInput:   {w, `node "{node}" does not fill input "{name}" that {request} version {version} needs`},
		CodeContractExample: {w, "example is not readable: {detail}"},

		CodeFlowOption:    {e, "invalid {field}: {detail}"},
		CodeFlowNotFound:  {e, `no flow matches "{ref}"`},
		CodeFlowAmbiguous: {e, `"{ref}" matches flows in {count} collections` + hintMarker + "use --collection"},

		CodeCycleNeeds:   {e, "dependency cycle: {cycle}"},
		CodeCycleSubflow: {e, "subflow cycle: {cycle}"},

		CodeLintWhitespace: {w, "leading or trailing whitespace in {field}"},
		CodeLintControl:    {w, "newline or tab in single-line {field}"},
		CodeLintInvisible:  {w, "invisible character {char} in {field}"},

		CodeWSMissing:           {e, "collection {id} not found at {path}" + hintMarker + `run "flowi workspace locate"`},
		CodeWSAlreadyMounted:    {e, "{path} is already mounted"},
		CodeWSNotCollection:     {e, "{path} is not a collection (no flowi.yaml)"},
		CodeWSManyCollections:   {e, "{count} collections are mounted" + hintMarker + "choose one with --collection"},
		CodeWSUnknownCollection: {e, `no mounted collection "{name}"`},

		CodeSettingUnknown:    {w, `unknown setting "{key}"`},
		CodeSettingInvalid:    {e, `invalid value for "{key}": {detail}`},
		CodeSettingWrongLayer: {e, `"{key}" cannot be set at {layer} level`},

		// {hint} and {message} are pre-formatted: empty, or starting with a
		// separator, so these stay plain templates.
		CodeTemplateUndefined: {e, "undefined `{name}` in {field}{hint}"},
		CodeTemplateSyntax:    {e, "invalid expression in {field}: {detail}"},
		CodeTemplateEmpty:     {e, "{field} is empty (empty: error)"},
		CodeTemplateGenerator: {e, "unknown generator ${name}"},

		CodeSecretMissing:         {e, `secret "{name}" is not set for environment "{env}"` + hintMarker + "set it with `flowi secret set {env} {name}`"},
		CodeSecretLocked:          {e, "cannot unlock the vault: {detail}"},
		CodeSecretFromEnvironment: {e, `secret "{name}" of environment "{env}" is provided by an environment variable, not stored` + hintMarker + "read it from where that variable is set"},
		CodeSecretUndeclared:      {e, `"{name}" is not a secret variable of environment "{env}"` + hintMarker + "declare it as `{name}: { secret: true }` in environments/{env}.yaml"},

		CodeRequestNotFound:    {e, `no request matches "{ref}"`},
		CodeRequestAmbiguous:   {e, `"{ref}" matches requests in {count} collections` + hintMarker + "use --collection"},
		CodeRequestFileOutside: {e, `file "{path}" is outside the collection`},
		CodeRequestEnvUnknown:  {e, `environment "{env}" not found` + hintMarker + "see `flowi env ls`"},
		CodeRequestHeader:      {e, "invalid header {name}: {detail}"},
		CodeRequestFileRead:    {e, `file "{path}" cannot be read: {detail}`},
		CodeRequestExampleName: {e, `invalid example name "{name}"` + hintMarker + "use letters, digits, '.', '_' and '-', not starting with '.'"},

		CodeRunAssertFailed:  {e, "assertion failed: {check}{message}"},
		CodeRunAssertWarning: {w, "assertion failed: {check}{message}"},
		CodeRunNodeType:      {e, `node type "{type}" cannot run here`},
		CodeRunSetTarget:     {e, "cannot set {target}: {detail}"},
		CodeRunExtractEmpty:  {e, `extract "{name}" found nothing ({detail})`},
		CodeRunHookDepth:     {e, "hooks nested deeper than max_depth {max}"},
		CodeRunNodeTimeout:   {e, `node "{node}" timed out after {timeout}`},
		CodeRunTillLimit:     {e, `node "{node}" stopped at {limit} before its until condition held`},
		CodeRunInputMissing:  {e, `input "{name}" of flow {flow} has no value and no default` + hintMarker + "pass it with --input {name}=…"},
		CodeRunInputUnknown:  {e, `flow {flow} has no input "{name}"`},
		CodeRunCalcNumber:    {e, `calc "{name}" of node "{node}" is not a finite number: {detail}`},
		CodeRunBatchOver:     {e, `over of batch "{node}" is not a list: {detail}`},
		CodeRunExtractNoResp: {e, `extract "{node}" reads a response, but none is in reach` + hintMarker + "outside post-hooks use the map form: name: \"{{nodes.<id>.body.…}}\""},
		CodeRunDigestTarget:  {e, `digest "{node}" cannot summarise "{of}": it is unknown or has not finished`},
		CodeRunHandled:       {w, `node "{node}" failed; continuing (on_error: {on_error})`},
		CodeRunBackground:    {w, `background webhook "{node}" failed: {detail}`},
		CodeRunFileName:      {e, `node "{node}" cannot write "{name}": not a plain file name`},
		CodeRunSaveStatus:    {e, `save "{node}" uses latest-by-status but its value has no status`},
		CodeRunNoBranchOK:    {e, `no branch of parallel "{node}" succeeded`},
		CodeRunWriteFailed:   {e, `node "{node}" could not write "{name}": {detail}`},
		CodeRunStopped:       {e, `the run was stopped by "{node}" as failed{message}`},

		CodeHTTPFailed:  {e, "request failed: {detail}"},
		CodeHTTPTimeout: {e, "request timed out after {timeout}"},

		CodeCycleHooks: {e, "hook flows call each other in a cycle: {cycle}"},
	}
}

// Codes returns every diagnostic code.
func Codes() []Code {
	table := codeTable()
	codes := make([]Code, 0, len(table))
	for c := range table {
		codes = append(codes, c)
	}
	return codes
}

// DefaultSeverity returns the severity a code carries unless a caller
// overrides it. Unknown codes are errors.
func DefaultSeverity(c Code) Severity {
	if info, ok := codeTable()[c]; ok {
		return info.severity
	}
	return SeverityError
}
