// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// maxSpecFindings bounds how many findings about the descriptions the page lists; the rest are counted. The Problems
// panel has all the ones that are problems: a note is a suggestion and is only listed here.
const maxSpecFindings = 40

// CollectionService backs the page of one collection: what it is about, its state, what it uses and how it is set up.
// It only gathers what the library already knows; the page edits nothing but the collection's variables, which go
// through ManagerService.SetVariables. The reads touch files and may take a moment, so they run on the caller.
type CollectionService struct {
	app   *App
	drift driftMemory
}

// PageCount is a number the About tab shows.
type PageCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// PageDependency is another collection this one depends on, and whether it is open in the workspace.
type PageDependency struct {
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Mounted bool   `json:"mounted"`
}

// PageAbout answers "what is this collection about?".
type PageAbout struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Specs       []string         `json:"specs"` // the API descriptions it kept at import
	Counts      []PageCount      `json:"counts"`
	DependsOn   []PageDependency `json:"dependsOn"`
	Exports     []PageExport     `json:"exports"` // the descriptions it can hand to others
}

// PageExport is one API description the collection can save as a file for someone outside Flowi.
type PageExport struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	File  string `json:"file"` // the name the save dialog starts with
}

// PageOperation is one operation of a kept description: the request it became and how far it is exercised.
type PageOperation struct {
	Request      string     `json:"request"` // the request's path
	Node         string     `json:"node"`    // the tree row that opens it, "" when the request is gone
	Used         []string   `json:"used"`    // the flows that call it
	StatusesSeen int        `json:"statusesSeen"`
	Statuses     int        `json:"statuses"`
	Unread       int        `json:"unread"` // response fields no flow reads
	Impact       PageImpact `json:"impact"`
	Drift        PageDrift  `json:"drift"`
}

// PageFlow is one flow with its newest saved run.
type PageFlow struct {
	Name   string `json:"name"`
	Node   string `json:"node"`
	Status string `json:"status"` // the last run's status; "" when it never ran
	Env    string `json:"env"`
	Ran    string `json:"ran"` // when, in local time as "2006-01-02 15:04"; "" when it never ran
}

// PageEndpoint is one request of the collection as the Endpoints tab lists it.
type PageEndpoint struct {
	Node      string       `json:"node"`
	Path      string       `json:"path"` // the request's path in the collection
	Name      string       `json:"name"`
	Summary   string       `json:"summary"`
	Verb      string       `json:"verb"`  // the method, or the protocol of a message, gRPC or data request
	Where     string       `json:"where"` // the URL's path; a channel or the request's own path for what has no URL
	Folder    string       `json:"folder"`
	Tags      []string     `json:"tags"` // its own and its folders'
	Status    *StatusMark  `json:"status,omitempty"`
	Operation string       `json:"operation"` // the operationId it came from
	Versions  []string     `json:"versions"`
	Used      []string     `json:"used"`     // the flows that call it
	Statuses  []StatusView `json:"statuses"` // what its description documents, and whether a saved run ever got it
	Unread    int          `json:"unread"`
	Impact    PageImpact   `json:"impact"`
	Drift     PageDrift    `json:"drift"`
}

// PageFinding is one finding about a kept API description.
type PageFinding struct {
	Severity string `json:"severity"` // error | warning | note
	Message  string `json:"message"`
	Hint     string `json:"hint"`
	Path     string `json:"path"` // where in the description, as a YAML path; "" for a .proto file
	File     string `json:"file"` // the file, relative to the collection
	Line     int    `json:"line"` // 1-based; 0 when the place is not known
}

// PageState answers "is it healthy?".
type PageState struct {
	Operations      []PageOperation `json:"operations"`
	OperationsUsed  int             `json:"operationsUsed"`
	OperationsTotal int             `json:"operationsTotal"`
	StatusesSeen    int             `json:"statusesSeen"`
	StatusesTotal   int             `json:"statusesTotal"`
	Flows           []PageFlow      `json:"flows"`
	NeverRan        int             `json:"neverRan"`
	Failing         int             `json:"failing"`
	Broken          []string        `json:"broken"` // files that could not be read
	// Spec are the findings of the spec linter on the kept descriptions, worst first, at most maxSpecFindings;
	// SpecTotal is how many of them are problems (errors and warnings), SpecErrors how many are errors and SpecNotes how
	// many are suggestions, which come last and count for nothing else on the page.
	Spec       []PageFinding `json:"spec"`
	SpecTotal  int           `json:"specTotal"`
	SpecErrors int           `json:"specErrors"`
	SpecNotes  int           `json:"specNotes"`
	// ImpactBroken counts the requests something reads that no longer fits their contract; DriftMoved the requests
	// whose responses moved in the last drift check on the page.
	ImpactBroken int `json:"impactBroken"`
	DriftMoved   int `json:"driftMoved"`
}

// PageSecret is a secret the collection writes as {{secret.name}} and how many places read it.
type PageSecret struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Where string `json:"where"` // "2 requests, 1 flow"
}

// PageUses answers "what does it rely on?".
type PageUses struct {
	Auths      []string     `json:"auths"`
	Blueprints []string     `json:"blueprints"`
	Overlays   []string     `json:"overlays"`
	Secrets    []PageSecret `json:"secrets"`
}

// PageEnvValue is what an environment says about a collection variable.
type PageEnvValue struct {
	Env    string `json:"env"`
	Value  string `json:"value"`  // "" for an environment that does not set it; hidden for a secret
	Secret bool   `json:"secret"` // the value is not shown
}

// PageVariable is a collection variable with the value each environment gives it.
type PageVariable struct {
	Name        string         `json:"name"`
	Value       string         `json:"value"`
	Description string         `json:"description"`
	PerEnv      []PageEnvValue `json:"perEnv"` // only the environments that override it
}

// PageEnvironment is a workspace environment and how many of the collection's variables it overrides.
type PageEnvironment struct {
	Name      string `json:"name"`
	Overrides int    `json:"overrides"`
}

// PageOwnEnvironment is an environment shipped in the collection's own folder.
type PageOwnEnvironment struct {
	Name string `json:"name"`
	// Have is true when the workspace has one of that name already, so "use it" would overwrite.
	Have bool `json:"have"`
}

// PageSetup answers "how is it configured?".
type PageSetup struct {
	Variables    []PageVariable       `json:"variables"`
	Environments []PageEnvironment    `json:"environments"`
	Own          []PageOwnEnvironment `json:"own"`
}

// PageRule is one rule of the spec linter, as the collection has it set.
type PageRule struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Default string `json:"default"` // off, warning or error
	Level   string `json:"level"`
	Chosen  bool   `json:"chosen"` // the collection's lint file sets it
}

// CollectionPage is everything the collection page shows.
type CollectionPage struct {
	ID        string         `json:"id"`
	About     PageAbout      `json:"about"`
	Endpoints []PageEndpoint `json:"endpoints"`
	State     PageState      `json:"state"`
	Uses      PageUses       `json:"uses"`
	Setup     PageSetup      `json:"setup"`
}

// pageInput is what the page is built from: all of it read from the library by Page, so the build is plain data.
type pageInput struct {
	Collection   model.Collection
	Specs        []string
	Exports      []flowi.ExportItem
	Impact       []analysis.Report // one for each request something uses
	Drift        []flowDrift       // the newest drift check of each flow
	Coverage     analysis.Coverage
	LastRun      map[model.ID]*flowi.RunSummary // a flow's newest saved run; absent when it never ran
	Secrets      map[string][]flowi.SecretUse
	Workspace    []model.Environment // the workspace's environments
	Own          []model.Environment // the ones in the collection's own folder
	Mounted      map[model.ID]bool
	SpecFindings diag.List // what the spec linter finds in this collection's kept descriptions, notes included
}

// Page gathers the page of a collection. A part that cannot be read is left empty; the rest still shows.
func (s *CollectionService) Page(collection string) (CollectionPage, error) {
	id := model.ID(collection)
	var found *model.Collection
	for _, c := range s.app.backend.Collections() {
		if c.Manifest.ID == id {
			c := c
			found = &c
			break
		}
	}
	if found == nil {
		return CollectionPage{}, fmt.Errorf("collection %q is not in the workspace", collection)
	}
	in := pageInput{Collection: *found, LastRun: map[model.ID]*flowi.RunSummary{}, Mounted: map[model.ID]bool{}}
	in.Specs = s.app.backend.DescriptionFiles(id)
	if reports, err := s.app.backend.CollectionImpact(id); err == nil {
		in.Impact = reports
	}
	if items, err := s.app.backend.ExportItems(id); err == nil {
		in.Exports = items
	}
	in.Drift = s.drift.of(collection)
	if cov, err := s.app.backend.Coverage(id); err == nil {
		in.Coverage = cov
	}
	for _, f := range found.Flows {
		if runs := s.app.backend.FlowRuns(flowi.FlowTarget{Collection: id, Flow: f.ID}, 1); len(runs) > 0 {
			in.LastRun[f.ID] = &runs[0]
		}
	}
	if found, err := s.app.backend.LintSpecs(id); err == nil {
		in.SpecFindings = found
	}
	in.Secrets = s.app.backend.SecretUses()
	in.Workspace = s.app.backend.Environments()
	if own, _, err := s.app.backend.CollectionEnvironments(id); err == nil {
		in.Own = own
	}
	for _, c := range s.app.backend.Collections() {
		in.Mounted[c.Manifest.ID] = true
	}
	return emptyCollections(pageOf(in)), nil
}

// pageOf builds the page from what was read.
func pageOf(in pageInput) CollectionPage {
	c := in.Collection
	return CollectionPage{
		ID:        string(c.Manifest.ID),
		About:     pageAboutOf(in),
		Endpoints: pageEndpointsOf(in),
		State:     pageStateOf(in),
		Uses:      pageUsesOf(in),
		Setup:     pageSetupOf(in),
	}
}

func pageAboutOf(in pageInput) PageAbout {
	c := in.Collection
	a := PageAbout{Name: c.Manifest.Name, Description: c.Manifest.Description, Specs: nonNil(in.Specs), DependsOn: []PageDependency{}}
	a.Counts = []PageCount{
		{"Requests", len(c.Requests)},
		{"Flows", len(c.Flows)},
		{"Operations", len(in.Coverage.Operations)},
		{"Folders", len(c.Folders)},
		{"Environments", len(c.Environments)},
	}
	for _, e := range in.Exports {
		a.Exports = append(a.Exports, PageExport{ID: e.ID, Label: e.Label, File: e.File})
	}
	for _, d := range c.Manifest.DependsOn {
		a.DependsOn = append(a.DependsOn, PageDependency{Name: d.Name, Repo: d.Repo, Mounted: in.Mounted[d.ID]})
	}
	return a
}

func pageStateOf(in pageInput) PageState {
	c := in.Collection
	st := PageState{Operations: []PageOperation{}, Flows: []PageFlow{}, Broken: []string{}}
	impact, drift := impactByPath(in), pageDriftOf(c, in.Drift)
	nodeOfRequest := map[string]string{}
	for _, r := range c.Requests {
		nodeOfRequest[r.Path] = "r:" + string(c.Manifest.ID) + ":" + string(r.ID)
	}
	for _, op := range in.Coverage.Operations {
		o := PageOperation{Request: op.Request, Node: nodeOfRequest[op.Request], Used: nonNil(op.Used), Statuses: len(op.Statuses), Unread: len(op.Unread), Impact: impact[op.Request], Drift: drift[op.Request]}
		for _, s := range op.Statuses {
			if s.Seen {
				o.StatusesSeen++
			}
		}
		st.Operations = append(st.Operations, o)
	}
	t := in.Coverage.Totals()
	st.OperationsUsed, st.OperationsTotal, st.StatusesSeen, st.StatusesTotal = t.Used, t.Operations, t.SeenStatuses, t.Statuses
	// What no flow calls stands out first, then by name.
	slices.SortStableFunc(st.Operations, func(a, b PageOperation) int {
		if (len(a.Used) == 0) != (len(b.Used) == 0) {
			if len(a.Used) == 0 {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Request, b.Request)
	})
	for _, f := range c.Flows {
		pf := PageFlow{Name: pageFlowLabel(f), Node: "f:" + string(c.Manifest.ID) + ":" + string(f.ID)}
		if r := in.LastRun[f.ID]; r != nil {
			pf.Status, pf.Env = r.Status, r.Env
			pf.Ran = r.Started.Local().Format("2006-01-02 15:04")
			if r.Status != "passed" {
				st.Failing++
			}
		} else {
			st.NeverRan++
		}
		st.Flows = append(st.Flows, pf)
	}
	slices.SortStableFunc(st.Flows, func(a, b PageFlow) int { return strings.Compare(a.Name, b.Name) })
	for _, b := range c.Broken {
		st.Broken = append(st.Broken, b.Path)
	}
	st.Spec, st.SpecTotal, st.SpecErrors, st.SpecNotes = specFindings(in)
	for _, i := range impact {
		if i.Broken > 0 {
			st.ImpactBroken++
		}
	}
	for _, d := range drift {
		if d.Moved() > 0 {
			st.DriftMoved++
		}
	}
	return st
}

// specFindings lists the spec linter's findings about this collection's files, errors first, then warnings, then notes.
// total counts the problems among them, and notes the suggestions, which the rest of the page does not count.
//
// Before: a missing tag was a warning here and in the Problems panel; it is a note now, listed on this page only.
func specFindings(in pageInput) (list []PageFinding, total, errs, notes int) {
	var mine diag.List
	for _, d := range in.SpecFindings {
		if !flowi.IsSpecDiagnostic(d) {
			continue
		}
		if rel, err := filepath.Rel(in.Collection.Root, d.File); err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		mine = append(mine, d)
	}
	slices.SortStableFunc(mine, func(a, b diag.Diagnostic) int { return int(a.Severity) - int(b.Severity) })
	list = []PageFinding{}
	for i, d := range mine {
		switch d.Severity {
		case diag.SeverityError:
			errs++
			total++
		case diag.SeverityNote:
			notes++
		default:
			total++
		}
		if i >= maxSpecFindings {
			continue
		}
		msg, hint := diag.English().Message(d)
		rel, _ := filepath.Rel(in.Collection.Root, d.File)
		list = append(list, PageFinding{Severity: d.Severity.String(), Message: msg, Hint: hint, Path: d.Path, File: filepath.ToSlash(rel), Line: d.Pos.Line})
	}
	return list, total, errs, notes
}

func pageFlowLabel(f model.Flow) string {
	if f.Path != "" {
		return f.Path
	}
	return f.Name
}

func pageUsesOf(in pageInput) PageUses {
	c := in.Collection
	u := PageUses{Auths: []string{}, Blueprints: []string{}, Overlays: []string{}, Secrets: []PageSecret{}}
	for _, a := range c.Auths {
		u.Auths = append(u.Auths, a.Name)
	}
	for _, b := range c.Blueprints {
		u.Blueprints = append(u.Blueprints, b.Name)
	}
	for _, o := range c.Overlays {
		u.Overlays = append(u.Overlays, o.Name)
	}
	for name, uses := range in.Secrets {
		kinds := map[string]int{}
		n := 0
		for _, use := range uses {
			if use.Collection != c.Manifest.ID {
				continue
			}
			kinds[use.Kind]++
			n++
		}
		if n > 0 {
			u.Secrets = append(u.Secrets, PageSecret{Name: name, Count: n, Where: pageUsedWhere(kinds)})
		}
	}
	slices.SortFunc(u.Secrets, func(a, b PageSecret) int { return strings.Compare(a.Name, b.Name) })
	return u
}

// pageUsedWhere says "2 requests, 1 flow" for the kinds that use something.
func pageUsedWhere(kinds map[string]int) string {
	var parts []string
	for _, k := range []string{"request", "flow", "authentication"} {
		if n := kinds[k]; n > 0 {
			word := k
			if n != 1 {
				word += "s"
			}
			parts = append(parts, fmt.Sprintf("%d %s", n, word))
		}
	}
	return strings.Join(parts, ", ")
}

func pageSetupOf(in pageInput) PageSetup {
	c := in.Collection
	s := PageSetup{Variables: []PageVariable{}, Environments: []PageEnvironment{}, Own: []PageOwnEnvironment{}}
	names := map[string]bool{}
	for _, v := range c.Manifest.Variables {
		pv := PageVariable{Name: v.Key, Value: v.Value, Description: v.Description, PerEnv: []PageEnvValue{}}
		names[v.Key] = true
		for _, e := range in.Workspace {
			for _, ev := range e.Vars {
				if ev.Name != v.Key {
					continue
				}
				val := PageEnvValue{Env: e.Name, Value: ev.Value, Secret: ev.Secret}
				if ev.Secret {
					val.Value = ""
				}
				pv.PerEnv = append(pv.PerEnv, val)
			}
		}
		s.Variables = append(s.Variables, pv)
	}
	have := map[string]bool{}
	for _, e := range in.Workspace {
		have[e.Name] = true
		n := 0
		for _, ev := range e.Vars {
			if names[ev.Name] {
				n++
			}
		}
		s.Environments = append(s.Environments, PageEnvironment{Name: e.Name, Overrides: n})
	}
	for _, e := range in.Own {
		s.Own = append(s.Own, PageOwnEnvironment{Name: e.Name, Have: have[e.Name]})
	}
	return s
}

// SpecRules lists the spec linter's rules with the level this collection gives each.
func (s *CollectionService) SpecRules(collection string) ([]PageRule, error) {
	rules, err := s.app.backend.SpecLintRules(model.ID(collection))
	if err != nil {
		return nil, errors.New(vm.ErrorText(err))
	}
	out := make([]PageRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, PageRule{Name: r.Name, About: r.About, Default: r.Default, Level: r.Level, Chosen: r.Chosen})
	}
	return out, nil
}

// SetSpecRule sets how seriously a rule is taken in this collection: "off", "warning", "error", or "" for the rule's
// default. The workspace is read again so the findings follow at once.
func (s *CollectionService) SetSpecRule(collection, rule, level string) error {
	if err := s.app.backend.SetSpecLintRule(model.ID(collection), rule, level); err != nil {
		return errors.New(vm.ErrorText(err))
	}
	s.app.actor.Do(s.app.workspace.Load)
	return nil
}

// pageEndpointsOf lists the collection's requests, by where they are called, with what the descriptions and runs say
// about each.
func pageEndpointsOf(in pageInput) []PageEndpoint {
	c := in.Collection
	ops := map[string]analysis.OperationCoverage{}
	for _, op := range in.Coverage.Operations {
		if _, seen := ops[op.Request]; !seen {
			ops[op.Request] = op
		}
	}
	callers := flowsCalling(c)
	impact, drift := impactByPath(in), pageDriftOf(c, in.Drift)
	out := []PageEndpoint{}
	for _, r := range c.Requests {
		e := PageEndpoint{
			Node: "r:" + string(c.Manifest.ID) + ":" + string(r.ID), Path: r.Path, Name: r.Name, Summary: r.Description,
			Tags: nonNil(c.RequestTags(r)), Operation: r.Operation, Versions: []string{}, Used: []string{}, Statuses: []StatusView{},
			Folder: pageFolder(r.Path), Where: r.Path, Impact: impact[r.Path], Drift: drift[r.Path],
		}
		for _, v := range r.Versions {
			e.Versions = append(e.Versions, v.Label)
		}
		if len(r.Versions) > 0 {
			v := r.Versions[pageDefaultVersion(r)]
			e.Verb = vm.Verb(v)
			if e.Summary == "" {
				e.Summary = firstLine(v.Docs)
			}
			switch {
			case v.Async != nil:
				e.Where = v.Async.Channel
			case v.GRPC == nil && v.SQL() == nil && v.URL != "":
				e.Where = pageURLPath(v.URL)
			}
		}
		e.Used = callers[r.ID]
		if op, ok := ops[r.Path]; ok {
			for _, f := range op.Used { // the description's own count also knows hooks
				if !slices.Contains(e.Used, f) {
					e.Used = append(e.Used, f)
				}
			}
			slices.Sort(e.Used)
			e.Unread = len(op.Unread)
			for _, st := range op.Statuses {
				e.Statuses = append(e.Statuses, StatusView{Status: st.Status, Seen: st.Seen})
			}
		}
		e.Status = pageStatusMark(c, c.RequestStatus(r))
		out = append(out, e)
	}
	slices.SortStableFunc(out, func(a, b PageEndpoint) int {
		if d := strings.Compare(a.Where, b.Where); d != 0 {
			return d
		}
		return strings.Compare(a.Verb, b.Verb)
	})
	return out
}

// pageDefaultVersion is the index of the version a request opens on: the one named by its default, else the first.
func pageDefaultVersion(r model.Request) int {
	if i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == r.Default }); i >= 0 {
		return i
	}
	return 0
}

// pageFolder is the folder a request path is in, "" for the top.
func pageFolder(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

// pageURLPath is the path of a request's URL without its host, its template base ({{env.base_url}}) or its query.
func pageURLPath(u string) string {
	u, _, _ = strings.Cut(u, "?")
	if strings.HasPrefix(u, "{{") {
		if _, rest, ok := strings.Cut(u, "}}"); ok {
			u = rest
		}
	}
	if _, rest, ok := strings.Cut(u, "://"); ok {
		_, path, found := strings.Cut(rest, "/")
		u = "/" + path
		if !found {
			u = "/"
		}
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return u
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(line)
}

// pageStatusMark is the development status of an item as the window draws it; nil when it has none.
func pageStatusMark(c model.Collection, st model.StatusOf) *StatusMark {
	if st.Name == "" {
		return nil
	}
	mark := StatusMark{Name: st.Name, From: st.From, Inherited: st.From != ""}
	if def, ok := c.StatusNamed(st.Name); ok {
		mark.Color, mark.Icon = def.Color, def.Icon
	} else {
		mark.Unknown = true
	}
	return &mark
}

// flowsCalling lists, for each request, the flows of the collection that send it from a step at any depth.
func flowsCalling(c model.Collection) map[model.ID][]string {
	byPath := map[string]model.ID{}
	for _, r := range c.Requests {
		byPath[r.Path] = r.ID
	}
	out := map[model.ID][]string{}
	var walk func(flow string, nodes []model.Node)
	walk = func(flow string, nodes []model.Node) {
		for _, n := range nodes {
			if n.Request != nil {
				ref := n.Request.Request
				id := ref.ID
				if id == "" {
					id = byPath[ref.Path]
				}
				if id != "" && !slices.Contains(out[id], flow) {
					out[id] = append(out[id], flow)
				}
			}
			for _, children := range n.Children() {
				walk(flow, children)
			}
		}
	}
	for _, f := range c.Flows {
		walk(pageFlowLabel(f), f.Nodes)
	}
	return out
}

// impactByPath sums each request's report, by the request's path. A request nothing uses is not in it, and the
// zero value says "no consumers".
func impactByPath(in pageInput) map[string]PageImpact {
	out := map[string]PageImpact{}
	for _, rep := range in.Impact {
		out[rep.Request] = pageImpactOf(rep)
	}
	return out
}

// SaveExport asks where to save one of the descriptions the page lists (see PageExport) and writes it there. It
// returns the file, "" when the person cancelled. The description is read when the dialog is answered, so what is
// saved is what the collection says then.
func (s *CollectionService) SaveExport(collection, id string) (string, error) {
	if s.app.pickers.Save == nil {
		return "", errNoDialogs
	}
	items, err := s.app.backend.ExportItems(model.ID(collection))
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	i := slices.IndexFunc(items, func(e flowi.ExportItem) bool { return e.ID == id })
	if i < 0 {
		return "", flowi.ErrNoExport
	}
	path, err := s.app.pickers.Save("Save the API description", items[i].File)
	if err != nil || path == "" {
		return "", err
	}
	data, err := s.app.backend.ExportDescription(model.ID(collection), id)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("saving the description: %w", err)
	}
	return path, nil
}
