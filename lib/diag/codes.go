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
	// CodeFormatNeedsNewer: flowi.yaml names a minimum Flowi release newer than the one running.
	CodeFormatNeedsNewer Code = "FLW-FMT-003"

	CodeFileUnrecognised Code = "FLW-FILE-001"
	CodeFileUnreadable   Code = "FLW-FILE-002"
	// CodeFileConflict: the file still has the markers git leaves where a merge could not decide.
	CodeFileConflict Code = "FLW-FILE-003"

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
	CodeNodeRemovedType  Code = "FLW-NODE-012"

	CodeImportUnmapped  Code = "FLW-IMPORT-001"
	CodeImportInvalid   Code = "FLW-IMPORT-002"
	CodeImportExists    Code = "FLW-IMPORT-003"
	CodeImportSyntax    Code = "FLW-IMPORT-004"
	CodeImportSkipped   Code = "FLW-IMPORT-005"
	CodeImportSensitive Code = "FLW-IMPORT-006"
	CodeImportKeptEdit  Code = "FLW-IMPORT-007"
	CodeImportNoOpID    Code = "FLW-IMPORT-008"

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
	CodeAsyncConn     Code = "FLW-ASYNC-006"
	CodeAsyncConnName Code = "FLW-ASYNC-007"

	CodeGRPCRequest Code = "FLW-GRPC-001"
	CodeGRPCMethod  Code = "FLW-GRPC-002"
	CodeGRPCProto   Code = "FLW-GRPC-003"
	CodeGRPCBody    Code = "FLW-GRPC-004"
	CodeGRPCStatus  Code = "FLW-GRPC-005"
	CodeGRPCField   Code = "FLW-GRPC-006"

	CodeDatasetRequest Code = "FLW-DATA-001"
	CodeDatasetQuery   Code = "FLW-DATA-002"
	CodeDatasetTimeout Code = "FLW-DATA-003"

	CodeOutcome Code = "FLW-OUT-001"

	CodeExportLossy Code = "FLW-EXPORT-001"
	CodeExportValue Code = "FLW-EXPORT-002"

	CodeServeSchedule     Code = "FLW-SERVE-001"
	CodeServeScheduleFlow Code = "FLW-SERVE-002"
	CodeServeToken        Code = "FLW-SERVE-003" //nolint:gosec // an error code, not a credential

	CodeBlueprintParamType   Code = "FLW-BP-001"
	CodeBlueprintParamName   Code = "FLW-BP-002"
	CodeBlueprintRef         Code = "FLW-BP-003"
	CodeBlueprintParamUnused Code = "FLW-BP-004"
	CodeBlueprintUse         Code = "FLW-BP-005"
	CodeBlueprintNotFound    Code = "FLW-BP-006"
	CodeBlueprintAmbiguous   Code = "FLW-BP-007"

	CodeAuthInvalid  Code = "FLW-AUTH-001"
	CodeAuthUnknown  Code = "FLW-AUTH-002"
	CodeAuthNotFound Code = "FLW-AUTH-003"
	CodeAuthToken    Code = "FLW-AUTH-004" //nolint:gosec // an error code, not a credential

	CodeOverlayInvalid    Code = "FLW-OVL-001"
	CodeOverlayNotFound   Code = "FLW-OVL-002"
	CodeOverlayNotFitting Code = "FLW-OVL-003"
	CodeOverlayDrift      Code = "FLW-OVL-004"
	CodeOverlayName       Code = "FLW-OVL-005"

	CodeWriteDropsComments Code = "FLW-WRITE-001"
	CodeWriteRequestPath   Code = "FLW-WRITE-002"
	CodeWriteEditNone      Code = "FLW-WRITE-003"
	CodeWriteEditLabel     Code = "FLW-WRITE-004"
	CodeWriteChangedOnDisk Code = "FLW-WRITE-005"

	CodeContractBroken  Code = "FLW-CONTRACT-001"
	CodeContractInput   Code = "FLW-CONTRACT-002"
	CodeContractExample Code = "FLW-CONTRACT-003"

	CodeFlowOption    Code = "FLW-FLOW-001"
	CodeFlowNotFound  Code = "FLW-FLOW-002"
	CodeFlowAmbiguous Code = "FLW-FLOW-003"
	CodeFlowTagEmpty  Code = "FLW-FLOW-004"
	CodeFlowNone      Code = "FLW-FLOW-005"

	CodeCycleNeeds   Code = "FLW-CYC-001"
	CodeCycleSubflow Code = "FLW-CYC-002"

	CodeLintWhitespace Code = "FLW-LINT-001"
	CodeLintControl    Code = "FLW-LINT-002"
	CodeLintInvisible  Code = "FLW-LINT-003"
	CodeLintLiteralKey Code = "FLW-LINT-004"
	CodeLintPlainKey   Code = "FLW-LINT-005"
	CodeLintPlainGRPC  Code = "FLW-LINT-006"

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
	CodeTemplateShadow    Code = "FLW-TPL-005"

	// Diagnostic code names, not credentials (gosec G101 matches "Secret").
	CodeSecretMissing         Code = "FLW-SEC-001" //nolint:gosec // see above
	CodeSecretLocked          Code = "FLW-SEC-002" //nolint:gosec // see above
	CodeSecretUndeclared      Code = "FLW-SEC-003" //nolint:gosec // see above
	CodeSecretFromEnvironment Code = "FLW-SEC-004" //nolint:gosec // see above
	CodeSecretHost            Code = "FLW-SEC-005" //nolint:gosec // see above
	CodeSecretShipped         Code = "FLW-SEC-006" //nolint:gosec // see above

	CodeRequestNotFound    Code = "FLW-REQ-001"
	CodeRequestAmbiguous   Code = "FLW-REQ-002"
	CodeRequestFileOutside Code = "FLW-REQ-003"
	CodeRequestEnvUnknown  Code = "FLW-REQ-004"
	CodeRequestHeader      Code = "FLW-REQ-005"
	CodeRequestFileRead    Code = "FLW-REQ-006"
	CodeRequestExampleName Code = "FLW-REQ-007"
	CodeRequestEnvExtends  Code = "FLW-REQ-008"
	CodeRequestMethod      Code = "FLW-REQ-009"
	CodeRequestMethodCase  Code = "FLW-REQ-010"
	CodeRequestEnvShipped  Code = "FLW-REQ-011"

	CodeRunAssertFailed   Code = "FLW-RUN-001"
	CodeRunAssertWarning  Code = "FLW-RUN-002"
	CodeRunNodeType       Code = "FLW-RUN-003"
	CodeRunSetTarget      Code = "FLW-RUN-004"
	CodeRunExtractEmpty   Code = "FLW-RUN-005"
	CodeRunHookDepth      Code = "FLW-RUN-006"
	CodeRunNodeTimeout    Code = "FLW-RUN-007"
	CodeRunTillLimit      Code = "FLW-RUN-008"
	CodeRunInputMissing   Code = "FLW-RUN-009"
	CodeRunInputUnknown   Code = "FLW-RUN-010"
	CodeRunCalcNumber     Code = "FLW-RUN-011"
	CodeRunBatchOver      Code = "FLW-RUN-012"
	CodeRunExtractNoResp  Code = "FLW-RUN-013"
	CodeRunDigestTarget   Code = "FLW-RUN-014"
	CodeRunHandled        Code = "FLW-RUN-015"
	CodeRunBackground     Code = "FLW-RUN-016"
	CodeRunFileName       Code = "FLW-RUN-017"
	CodeRunSaveStatus     Code = "FLW-RUN-018"
	CodeRunNoBranchOK     Code = "FLW-RUN-019"
	CodeRunWriteFailed    Code = "FLW-RUN-020"
	CodeRunStopped        Code = "FLW-RUN-021"
	CodeRunSkipRead       Code = "FLW-RUN-022"
	CodeRunAssertNoSchema Code = "FLW-RUN-023"
	CodeRunFuzzFailed     Code = "FLW-RUN-024"
	CodeRunNotProblem     Code = "FLW-RUN-025"
	CodeRunNoResponse     Code = "FLW-RUN-026"
	CodeRunDeprecated     Code = "FLW-RUN-027"
	CodeRunEnvMissing     Code = "FLW-RUN-028"
	CodeRunFlaky          Code = "FLW-RUN-029"
	CodeRunGraphQLError   Code = "FLW-RUN-030"
	CodeRunGraphQLWarning Code = "FLW-RUN-031"
	CodeRunExpectFailed   Code = "FLW-RUN-032"
	CodeExpectInvalid     Code = "FLW-EXPECT-001"

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
		CodeFileConflict:     {e, "unresolved git merge conflict: a marker (<<<<<<<, ======= or >>>>>>>) is left in the file" + hintMarker + "open it, keep the lines you want, delete the three marker lines, then run `flowi validate`; `git checkout --conflict=merge <file>` brings the markers back if you want to start over"},
		CodeYAMLSyntax:       {e, "YAML syntax error: {detail} — hint: check the indentation and the quotes near the line it names"},
		CodeYAMLUnknownKey:   {e, `unknown key "{key}" — hint: check the spelling; flowi schema <kind> prints the keys a file may have`},
		CodeYAMLDuplicateKey: {e, `duplicate key "{key}" — hint: keep one of them`},
		CodeYAMLType:         {e, `"{key}" must be {expected}, got {got} — hint: change the value to the expected type`},
		CodeYAMLEmpty:        {e, "file is empty — hint: write its content, or delete the file"},

		CodeFormatTooNew:     {e, "format {got} is newer than supported {max}; upgrade Flowi"},
		CodeFormatMissing:    {e, `missing "format" in flowi.yaml`},
		CodeFormatNeedsNewer: {e, "this collection needs Flowi {min} or newer; this is {have} — hint: install a newer release, or lower min_version in flowi.yaml if the collection does not need it"},

		CodeFileUnrecognised: {w, `unrecognised file "{name}" (expected *.req.yaml / *.arazzo.yaml / _folder.yaml)`},
		CodeFileUnreadable:   {w, `cannot read "{name}": {detail}`},

		CodeIDDuplicate: {e, "duplicate id {id} — hint: give one of the two a new id"},
		CodeIDInvalid:   {e, `invalid id "{id}" (must be a ULID) — hint: use a 26-character ULID; a new request or flow gets a valid one`},
		CodeIDMissing:   {e, "missing id — hint: add an id: line with a ULID"},

		CodeVersionInvalid:       {e, `invalid version label "{label}"`},
		CodeVersionDuplicate:     {e, `version "{label}" duplicates "{other}"`},
		CodeVersionDefaultAbsent: {e, `default version "{label}" does not exist`},
		CodeVersionPinnedAbsent:  {e, `pinned version "{label}" does not exist on request {request}`},
		CodeVersionNone:          {e, "request has no versions"},

		CodeRefUnknownRequest: {e, "unknown request id {id} — hint: the request was deleted or its id changed: pick another one in the step"},
		CodeRefUnknownFlow:    {e, "unknown flow id {id} — hint: the flow was deleted or its id changed: pick another one"},
		CodeRefUndeclared:     {e, `collection "{collection}" is not declared in depends_on — hint: add it to depends_on in the collection file`},
		CodeRefNotMounted:     {e, `collection "{collection}" is not mounted` + hintMarker + `clone {repo} and run "flowi workspace mount"`},
		CodeRefStalePath:      {w, `path hint "{got}" is out of date, should be "{want}" (run "flowi fmt")`},

		CodeNodeUnknownType:  {e, `unknown node type "{type}" — hint: use one of the step types offered when you add a step`},
		CodeNodeRemovedType:  {e, `node type "{type}" no longer exists` + hintMarker + "use {use}"},
		CodeNodeDuplicateID:  {e, `duplicate node id "{node}" in flow — hint: rename one of the steps`},
		CodeNodeNeedsSibling: {e, `needs "{target}" is not a sibling node` + hintMarker + "needs can only reference nodes in the same list"},
		CodeNodeMissingField: {e, `node type "{type}" requires "{field}"`},
		CodeNodeParam:        {e, `invalid {field} of node "{node}": {detail}`},
		CodeNodeGoto:         {e, `on_error of node "{node}" goes to "{target}", which is not a later sibling` + hintMarker + "goto can only jump forward within the same list; use run_till to repeat"},
		CodeNodeAttach:       {w, `node "{node}" is shown with "{target}", but {detail}` + hintMarker + "an attached node sits right next to its request, or next to others attached to it: reorder it, or clear its attach"},
		CodeNodeWriteClash:   {e, `nodes "{a}" and "{b}" may run at the same time and both write {target}`},
		CodeNodeSubflowInput: {e, `subflow node "{node}": {detail} "{name}" of flow {flow}`},
		CodeNodeUnresolved:   {e, "nothing produces `{name}` used in {field}{hint}"},
		CodeNodeContract:     {w, `node "{node}" reads "{name}", which request {request} version {version} does not extract`},

		CodeImportUnmapped:  {w, "{what} has no Flowi equivalent and was {how}: {detail}"},
		CodeImportInvalid:   {e, "not a document Flowi can import: {detail}"},
		CodeImportExists:    {e, "{count} of the files to import already exist ({files})" + hintMarker + "overwrite them, or skip them to add only the new files"},
		CodeImportSyntax:    {e, "{format} syntax error: {detail}"},
		CodeImportSkipped:   {e, "{item} was skipped: {detail}"},
		CodeImportSensitive: {w, "{where} looked like a credential and was replaced by {placeholder}" + hintMarker + "set the value with flowi secret set"},
		CodeImportNoOpID:    {w, "the operation {operation} has no operationId, so it is named {name} and an Arazzo step cannot refer to it" + hintMarker + "give it an operationId with an overlay (flowi import --overlay), or in the spec"},
		CodeImportKeptEdit:  {w, "{what} was changed both by you and by the spec; your edit was kept" + hintMarker + "compare it with the spec, and save it to the spec overlay if the spec should say so too"},

		CodeMermaidPlaceholder: {w, `"{label}" does not resolve and became a placeholder note: {detail}`},
		CodeMermaidCycle:       {e, "the diagram has a cycle through {nodes}; flows run once, top to bottom"},
		CodeArazzoDrift:        {w, "the steps of {flow} were edited outside Flowi; Flowi's own data is used"},
		CodeArazzoLimit:        {w, `step "{step}" of {flow} has a {what} that other Arazzo tools do not see`},
		CodeArazzoCustom:       {w, `step "{step}" of {flow} is a Flowi step ({type}); other Arazzo tools cannot run it`},

		CodeSchemaRequest:  {w, "the request body of {request} does not match its schema at {path}: {message}"},
		CodeSchemaResponse: {e, "the response ({status}) of {request} does not match its schema at {path}: {message}"},
		CodeSchemaMessage:  {e, "a message received by {request} does not match its schema at {path}: {message}"},
		CodeSchemaUpdated:  {w, "the schema of the {what} of {request} was updated at {path}: {detail}"},

		CodeAsyncProtocol: {e, `unknown async protocol "{protocol}" (use ws, sse, mqtt, kafka, amqp or nats)`},
		CodeAsyncAction:   {e, "async request needs a send or a receive, and receive count and timeout must be positive: {detail}"},
		CodeAsyncBinding:  {e, `binding "{name}" is not an option of {protocol}`},
		CodeAsyncServer:   {e, `server "{server}" does not fit protocol {protocol} (expected {schemes})`},
		CodeAsyncTimeout:  {e, "received {got} of {want} messages before the {timeout} timeout"},
		CodeAsyncConn:     {e, "{detail}"},
		CodeAsyncConnName: {w, `no step of this flow opens a connection named "{name}" (a flow that calls this one may)`},

		CodeGRPCRequest: {e, "{detail}"},
		CodeGRPCMethod:  {e, "method {method} is not available: {detail}"},
		CodeGRPCProto:   {e, "{detail}"},
		CodeGRPCBody:    {e, "the message does not fit {method}: {detail}"},
		CodeGRPCStatus:  {e, "the call ended with {code}: {message}"},
		CodeGRPCField:   {w, "{detail}"},

		CodeOutcome:        {e, "{detail}"},
		CodeDatasetRequest: {e, "{detail}"},
		CodeDatasetQuery:   {e, "the query failed: {detail}"},
		CodeDatasetTimeout: {e, "the query did not finish in {timeout}"},

		CodeExportLossy:       {w, `node "{node}" ({type}) has no Arazzo equivalent and was left out`},
		CodeServeSchedule:     {e, `schedule "{name}": {detail}`},
		CodeServeToken:        {e, "trigger token: {detail} — hint: flowi serve tokens ls lists the tokens; flowi serve tokens new <name> makes one"},
		CodeServeScheduleFlow: {w, `schedule "{name}" cannot run: {detail}` + hintMarker + "it is skipped until the flow can be found"},
		CodeExportValue:       {w, `{what} of "{node}" is {value}, which has no Arazzo equivalent, and was left out`},

		CodeAuthInvalid:       {e, "auth: {detail}"},
		CodeAuthUnknown:       {e, `auth uses "{name}", but the collection has no authentication with that name` + hintMarker + "add auths/{name}.auth.yaml, or pick another one"},
		CodeAuthNotFound:      {e, `no authentication named "{name}"`},
		CodeAuthToken:         {e, `{detail}` + hintMarker + "run `flowi auth tokens` to see the saved tokens and their ids"},
		CodeOverlayInvalid:    {e, "flowi overlay is not valid: {detail}"},
		CodeOverlayNotFound:   {e, `no flowi overlay named "{name}"` + hintMarker + "list them with: flowi overlay ls"},
		CodeOverlayNotFitting: {w, `flowi overlay "{overlay}": {detail} was left out: {reason}`},
		CodeOverlayDrift:      {w, `flowi overlay "{overlay}": {detail}: {reason}`},
		CodeOverlayName:       {e, `flowi overlay name "{name}": {detail}`},

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
		CodeWriteChangedOnDisk: {e, "{file} changed on disk since it was opened, so saving would overwrite that change" + hintMarker + "load the file again, or save with the overwrite option to keep your version"},

		CodeContractBroken:  {w, "{reader} reads {read} from {request} version {version}, which {source} does not contain"},
		CodeContractInput:   {w, `node "{node}" does not fill input "{name}" that {request} version {version} needs`},
		CodeContractExample: {w, "example is not readable: {detail}"},

		CodeFlowOption:    {e, "invalid {field}: {detail}"},
		CodeFlowNotFound:  {e, `no flow matches "{ref}"`},
		CodeFlowTagEmpty:  {e, `no flow carries the tag "{tag}"` + hintMarker + "add `tags: [{tag}]` to a flow, or to its folder's _folder.yaml"},
		CodeFlowNone:      {e, "there is no flow to run" + hintMarker + "add a flow, or check --collection"},
		CodeFlowAmbiguous: {e, `"{ref}" matches flows in {count} collections` + hintMarker + "use --collection"},

		CodeCycleNeeds:   {e, "dependency cycle: {cycle} — hint: remove one needs link to break the loop"},
		CodeCycleSubflow: {e, "subflow cycle: {cycle} — hint: make one of the flows stop calling the other"},

		CodeLintWhitespace: {w, "leading or trailing whitespace in {field}"},
		CodeLintControl:    {w, "newline or tab in single-line {field}"},
		CodeLintInvisible:  {w, "invisible character {char} in {field}"},
		CodeLintPlainGRPC:  {w, "the gRPC call to {host} is not encrypted: {server} is plaintext" + hintMarker + "write grpcs:// to use TLS, unless this host is on a network you trust"},
		CodeLintPlainKey:   {w, "{field} sends a credential over {scheme} to {host}, which is not encrypted" + hintMarker + "use https (or wss), unless this host is on a network you trust"},
		CodeLintLiteralKey: {w, "{field} holds a credential typed in directly" + hintMarker + "keep it as a secret and write {{secret.name}} here, so it does not reach git"},

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
		CodeTemplateShadow:    {w, "{{{name}}} reads the flow {kind} {name}, which hides the environment variable {name} of {envs}" + hintMarker + "write {{env.{name}}} for the environment's value, or rename the {kind}"},

		CodeSecretMissing:         {e, `secret "{name}" is not set for environment "{env}"` + hintMarker + "set it with `flowi secret set {env} {name}`"},
		CodeSecretLocked:          {e, "cannot unlock the vault: {detail}"},
		CodeSecretHost:            {e, `{request} would send a secret to "{host}", which is not trusted with secrets in environment "{env}"` + hintMarker + "if {host} is the right place, allow it with `flowi secret allow-host {host}`, or in the app under Settings > Secrets (your own settings: a collection cannot allow it), or point a variable of the environment at it"},
		CodeSecretShipped:         {e, `secret "{name}" is not set for environment "{env}", which the collection ships` + hintMarker + "set {var} in the environment of the run (CI), or run `flowi env import {env}` and then `flowi secret set {env} {name}` to keep it in the vault"},
		CodeSecretFromEnvironment: {e, `secret "{name}" of environment "{env}" is provided by an environment variable, not stored` + hintMarker + "read it from where that variable is set"},
		CodeSecretUndeclared:      {e, `"{name}" is not a secret variable of environment "{env}"` + hintMarker + "declare it as `{name}: { secret: true }` in environments/{env}.yaml"},

		CodeRequestNotFound:    {e, `no request matches "{ref}"`},
		CodeRequestAmbiguous:   {e, `"{ref}" matches requests in {count} collections` + hintMarker + "use --collection"},
		CodeRequestFileOutside: {e, `file "{path}" is outside the collection`},
		CodeRequestEnvShipped:  {e, `environment "{env}" comes with the collection "{collection}" and is not in the workspace yet` + hintMarker + "copy it with `flowi env import {env}`, then try again"},
		CodeRequestEnvUnknown:  {e, `environment "{env}" not found` + hintMarker + "see `flowi env ls`"},
		CodeRequestHeader:      {e, "invalid header {name}: {detail}"},
		CodeRequestFileRead:    {e, `file "{path}" cannot be read: {detail}`},
		CodeRequestEnvExtends:  {e, `environment "{env}" cannot be built: {detail}` + hintMarker + "check the `extends:` line and the variables of the environment file"},
		CodeRequestMethod:      {e, `"{method}" cannot be sent as an HTTP method` + hintMarker + "a method is one word of letters, digits and !#$%&'*+-.^_`|~: GET, QUERY, PROPFIND, PURGE or your own"},
		CodeRequestMethodCase:  {w, `method "{method}" is not written as {spelling}` + hintMarker + "method names are case-sensitive on the wire: a server that knows {spelling} will not know {method}"},
		CodeRequestExampleName: {e, `invalid example name "{name}"` + hintMarker + "use letters, digits, '.', '_' and '-', not starting with '.'"},

		CodeRunAssertFailed:   {e, "assertion failed: {check}{message}"},
		CodeRunAssertWarning:  {w, "assertion failed: {check}{message}"},
		CodeRunNodeType:       {e, `node type "{type}" cannot run here`},
		CodeRunSetTarget:      {e, "cannot set {target}: {detail}"},
		CodeRunExtractEmpty:   {e, `value "{name}" found nothing in the response ({detail})`},
		CodeRunHookDepth:      {e, "hooks nested deeper than max_depth {max}"},
		CodeRunNodeTimeout:    {e, `node "{node}" timed out after {timeout}`},
		CodeRunTillLimit:      {e, `node "{node}" stopped at {limit} before its until condition held`},
		CodeRunInputMissing:   {e, `input "{name}" of flow {flow} has no value and no default` + hintMarker + "pass it with --input {name}=…"},
		CodeRunInputUnknown:   {e, `flow {flow} has no input "{name}"`},
		CodeRunCalcNumber:     {e, `value "{name}" of node "{node}" is not a finite number: {detail}`},
		CodeRunBatchOver:      {e, `over of batch "{node}" is not a list: {detail}`},
		CodeRunExtractNoResp:  {e, `set "{node}" reads a response, but none is in reach` + hintMarker + "outside post-hooks read an earlier step: from: expr with {{nodes.<id>.body.…}}"},
		CodeRunDigestTarget:   {e, `digest "{node}" cannot summarise "{of}": it is unknown or has not finished`},
		CodeRunFlaky:          {w, `node "{node}" passed on try {try} of {tries}` + hintMarker + "a step that needs a retry to pass is flaky: find out why, or the retries only hide it"},
		CodeRunGraphQLError:   {e, `the GraphQL response of "{request}" carries {count}: {detail}` + hintMarker + "a GraphQL server answers 200 even when the query failed: the errors are in the body (requests.graphql_errors: warn or ignore stops this)"},
		CodeRunGraphQLWarning: {w, `the GraphQL response of "{request}" carries {count}: {detail}` + hintMarker + "a GraphQL server answers 200 even when the query failed: assert on response.body.errors, or set requests.graphql_errors to fail so the step fails, or to ignore"},
		CodeRunExpectFailed:   {e, `"{request}" expected {expected}, got {status}` + hintMarker + "if {status} is right, change expect: in the request (a list or a range such as 4xx works); if not, the server answers differently than the test assumes"},
		CodeExpectInvalid:     {e, "expect: {detail}" + hintMarker + "write the status the request should answer with: 401, a range 400-499, a class 4xx or a list 200, 404"},
		CodeRunHandled:        {w, `node "{node}" failed; continuing (on_error: {on_error})`},
		CodeRunBackground:     {w, `background webhook "{node}" failed: {detail}`},
		CodeRunFileName:       {e, `node "{node}" cannot write "{name}": not a plain file name`},
		CodeRunSaveStatus:     {e, `save "{node}" uses latest-by-status but its value has no status`},
		CodeRunNoBranchOK:     {e, `no branch of parallel "{node}" succeeded`},
		CodeRunWriteFailed:    {e, `node "{node}" could not write "{name}": {detail}`},
		CodeRunStopped:        {e, `the run was stopped by "{node}" as failed{message}`},
		CodeRunFuzzFailed:     {e, `{failed} of {cases} bad inputs sent by "{node}" broke the check: {first}` + hintMarker + "the step's outputs hold every failure with its case name; send that body by hand to see what the server did"},
		CodeRunNotProblem:     {e, `"{node}" expects "{step}" to have received a problem details document (RFC 9457), but {problems}` + hintMarker + "an error response should be application/problem+json with the standard members used as the RFC says"},
		CodeRunDeprecated:     {w, `{request} is announced as going away: {notice}` + hintMarker + "the server sent a Deprecation or Sunset header; move to the successor before the sunset date"},
		CodeRunNoResponse:     {e, `"{node}" checks "{step}", but "{step}" has no response in this run` + hintMarker + "name an earlier request step of the same flow"},
		CodeRunAssertNoSchema: {e, `"{node}" checks the schema of "{step}", but no response schema is kept for what "{step}" received` + hintMarker + "send the request once with schemas.infer on (Settings → Requests → Learn schemas from sends), or import an OpenAPI document, so a schema is kept"},
		CodeRunEnvMissing:     {w, `flow "{flow}" reads {what}, but environment "{env}" {problem}` + hintMarker + "add it to the environment (`flowi env`), set the secret (`flowi secret set`), or run with another environment"},
		CodeRunSkipRead:       {e, `"{reader}" reads {reads}, which "{skipped}" would have made, and "{skipped}" is skipped` + hintMarker + "keep its last values (hold), skip \"{reader}\" too, or add a default with ??"},

		CodeHTTPFailed:  {e, "request failed: {detail} — hint: check the address and the network, then send again"},
		CodeHTTPTimeout: {e, "request timed out after {timeout} — hint: raise the timeout in the request settings, or check the server"},

		CodeCycleHooks: {e, "hook flows call each other in a cycle: {cycle} — hint: make one of the hook flows stop calling the other"},
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
