// SPDX-License-Identifier: Apache-2.0

package store

import (
	"strings"

	"github.com/githugt/flowi/lib/arazzo"
	"github.com/githugt/flowi/lib/model"
)

// descriptionsDir holds the derived and imported API descriptions flows refer to.
const descriptionsDir = "descriptions"

// GeneratedDescription is the collection-relative path of the OpenAPI
// description Flowi derives from the collection's requests.
const GeneratedDescription = descriptionsDir + "/requests.openapi.yaml"

// GeneratedAsyncDescription is the collection-relative path of the AsyncAPI
// description Flowi derives from the collection's async requests; it exists
// only while the collection has one.
const GeneratedAsyncDescription = descriptionsDir + "/requests.asyncapi.yaml"

// flowOptions are what the Arazzo codec needs for the flow at flowPath
// (relative to flows/, no suffix) of c: the flow body codec, the resolver
// over c's requests, and the description the steps are written against,
// reached from the flow's own directory.
func (s *Store) flowOptions(c model.Collection, flowPath string) arazzo.FlowOptions {
	up := strings.Repeat("../", strings.Count(flowPath, "/")+1)
	sources := []arazzo.SourceDescription{{Name: "flowi", URL: up + GeneratedDescription, Type: "openapi"}}
	if hasAsyncRequest(c) {
		sources = append(sources, arazzo.SourceDescription{Name: arazzo.AsyncSource, URL: up + GeneratedAsyncDescription, Type: "asyncapi"})
	}
	return arazzo.FlowOptions{Body: s.codec, Resolve: arazzo.NewResolver(c), Sources: sources, NewID: s.ids.New}
}

// flowYAMLPath maps a path inside a flow ("$.nodes[2]") to its place in the
// Arazzo document, where the flow's own data sits under the workflow's
// x-flowi-flow. Other files and paths already in document terms are unchanged.
func flowYAMLPath(file, path string) string {
	if !strings.HasSuffix(file, flowSuffix) || !strings.HasPrefix(path, "$.") || strings.HasPrefix(path, "$.workflows") {
		return path
	}
	return "$.workflows[0].x-flowi-flow" + path[1:]
}

// hasAsyncRequest reports whether c has an async request version, which is
// when its generated AsyncAPI description exists.
func hasAsyncRequest(c model.Collection) bool {
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			if v.Async != nil {
				return true
			}
		}
	}
	return false
}
