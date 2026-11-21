// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	flowi "github.com/githugt/flowi/lib"
)

// dataRow is one run of a data-driven flow: its name in the results and the values for the flow's inputs.
type dataRow struct {
	label  string
	inputs map[string]string
}

// readDataRows reads the rows of a data file: a CSV with a header line (each column is a flow input) or a JSON
// array of objects (each key is a flow input). The file's extension says which. At most maxMatrixCells rows run from
// one command.
func readDataRows(file string, data []byte) ([]dataRow, error) {
	var (
		records []map[string]string
		order   []string // the columns, for naming rows
		err     error
		first   bool // a CSV has its columns in order, so its first column may name a row; a JSON object has no order
	)
	switch strings.ToLower(filepath.Ext(file)) {
	case ".csv":
		records, order, err = csvRecords(data)
		first = true
	case ".json":
		records, order, err = jsonRecords(data)
	default:
		return nil, &UsageError{Err: fmt.Errorf("--data %s: a data file ends in .csv or .json", file)}
	}
	if err != nil {
		return nil, &UsageError{Err: fmt.Errorf("--data %s: %w", file, err)}
	}
	if len(records) == 0 {
		return nil, &UsageError{Err: fmt.Errorf("--data %s has no rows", file)}
	}
	if len(records) > maxMatrixCells {
		return nil, &UsageError{Err: fmt.Errorf("--data %s has %d rows; the most one command runs is %d", file, len(records), maxMatrixCells)}
	}
	return namedRows(records, order, first), nil
}

func csvRecords(data []byte) ([]map[string]string, []string, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading the header line: %w", err)
	}
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
		if header[i] == "" {
			return nil, nil, fmt.Errorf("column %d has no name in the header line", i+1)
		}
	}
	if dup := firstDuplicate(header); dup != "" {
		return nil, nil, fmt.Errorf("the column %q appears twice in the header line", dup)
	}
	var out []map[string]string
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			return out, header, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != len(header) {
			return nil, nil, fmt.Errorf("line %d has %d values, the header has %d columns", line, len(rec), len(header))
		}
		row := make(map[string]string, len(header))
		for i, h := range header {
			row[h] = rec[i]
		}
		out = append(out, row)
	}
}

func jsonRecords(data []byte) ([]map[string]string, []string, error) {
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, errors.New("expected a JSON array of objects: " + err.Error())
	}
	keys := map[string]bool{}
	out := make([]map[string]string, len(raw))
	for i, obj := range raw {
		row := make(map[string]string, len(obj))
		for k, v := range obj {
			keys[k] = true
			row[k] = jsonText(v)
		}
		out[i] = row
	}
	return out, slices.Sorted(maps.Keys(keys)), nil
}

// jsonText is a JSON value as an input's text: a string as it is, a number or boolean as written, null as empty,
// and an object or array as JSON.
func jsonText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

func firstDuplicate(names []string) string {
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			return n
		}
		seen[n] = true
	}
	return ""
}

// namedRows gives each row the label it shows in the results: its label, name or id column, else (for a CSV) its
// first column, else its number; a repeated label gets #2, #3.
func namedRows(records []map[string]string, order []string, firstColumn bool) []dataRow {
	column := ""
	// The columns that name a row, first match wins; otherwise the first column does.
	for _, c := range []string{"label", "name", "id"} {
		if slices.Contains(order, c) {
			column = c
			break
		}
	}
	if column == "" && firstColumn && len(order) > 0 {
		column = order[0]
	}
	seen := map[string]int{}
	rows := make([]dataRow, len(records))
	for i, rec := range records {
		label := strings.TrimSpace(rec[column])
		if label == "" {
			label = "row " + strconv.Itoa(i+1)
		}
		if seen[label]++; seen[label] > 1 {
			label += "#" + strconv.Itoa(seen[label])
		}
		rows[i] = dataRow{label: label, inputs: rec}
	}
	return rows
}

// options is base with this row's values as the flow's inputs; a column beats an --input of the same name.
func (d dataRow) options(base flowi.RunOptions) flowi.RunOptions {
	out := base
	out.Inputs = make(map[string]string, len(base.Inputs)+len(d.inputs))
	maps.Copy(out.Inputs, base.Inputs)
	maps.Copy(out.Inputs, d.inputs)
	return out
}

// runData runs the flow once for each row of the data file and prints the result of each, as --matrix does. The
// exit code is the worst of the rows.
func runData(cmd *cobra.Command, s *Session, target flowi.FlowTarget, base flowi.RunOptions, reports []reportSpec, f runFlags, cwd string) error {
	file := absPath(cwd, f.data)
	data, err := os.ReadFile(file) //nolint:gosec // the person names the data file to run
	if err != nil {
		return &UsageError{Err: fmt.Errorf("reading %s: %w", f.data, err)}
	}
	rows, err := readDataRows(f.data, data)
	if err != nil {
		return err
	}
	var results []cellResult
	worst := ExitOK
	for _, row := range rows {
		r, code := runCell(cmd, s, target, row.options(base), cellFiles{reports: reports, bundle: f.bundle, label: row.label})
		r.cell = cell{values: []string{row.label}}
		results = append(results, r)
		worst = max(worst, code)
	}
	renderGrid(cmd.OutOrStdout(), []dimension{{name: "row"}}, results)
	if worst != ExitOK {
		return Exit(worst)
	}
	return nil
}
