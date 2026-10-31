// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi-lib/model"
)

// A WebDAV answer is XML: a value in it is read with an XML path, and a later step can use it.
func TestRequestNode_Given_an_xml_response_and_an_xml_path_extract_Then_the_value_reaches_later_steps(t *testing.T) {
	// Arrange
	f := newFixture(t)
	v := get("/dav")
	v.Extract = []model.Extraction{
		{Name: "second", From: "body", Path: "/D:multistatus/D:response[2]/D:href"},
		{Name: "all", From: "body", Path: "//href"},
	}
	r := f.addRequest("dav", v)
	fl := f.addFlow("dav", requestNode("call", r),
		flowNode("check", "assert", "check", "nodes.call.status == 207 && nodes.call.second == '/files/b.txt' && len(nodes.call.all) == 2"))

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
}
