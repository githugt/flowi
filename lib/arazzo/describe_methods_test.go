// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/model"
)

// Before: Describe wrote only get, put, post, delete, options, head and patch, so a request that used QUERY or a method
// such as PROPFIND vanished from the description an Arazzo export is written against.
func TestDescribe_Given_query_and_custom_methods_Then_they_are_written_as_query_and_additionalOperations(t *testing.T) {
	// Arrange
	req := func(id, path, method, url string) model.Request {
		return model.Request{Meta: model.Meta{ID: model.ID(id), Name: path}, Path: path, Versions: []model.RequestVersion{{Label: "1", Method: method, URL: url}}}
	}
	c := model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Shop"}}, Requests: []model.Request{
		req("01JA00000000000000000000Q1", "search", "QUERY", "http://x/things"),
		req("01JA00000000000000000000P1", "dav/list", "PROPFIND", "http://x/dav"),
		req("01JA00000000000000000000C1", "cache/purge", "Purge", "http://x/cache"),
	}}

	// Act
	data, err := Describe(c, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Paths["/things"]["query"]; !ok {
		t.Errorf("QUERY is not under query: %v", doc.Paths["/things"])
	}
	extra := func(path, name string) bool {
		m, _ := doc.Paths[path]["additionalOperations"].(map[string]any)
		_, ok := m[name]
		return ok
	}
	if !extra("/dav", "PROPFIND") || !extra("/cache", "Purge") {
		t.Errorf("custom methods are not under additionalOperations with their case: %v %v", doc.Paths["/dav"], doc.Paths["/cache"])
	}
}
