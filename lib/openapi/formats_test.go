// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

const formatsDoc = `openapi: 3.1.0
info: {title: Contacts, version: "1.0"}
paths:
  /contacts:
    post:
      operationId: addContact
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                phone: {type: string, format: phone-number}
                born: {type: string, format: date}
                id: {type: string, format: uuid}
                format: {type: string, description: "a property that is called format"}
                zip: {type: string, format: postal-code}
            example: {format: csv, phone: "+1 555"}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, properties: {since: {type: string, format: phone-number}}}
`

func TestImport_Given_formats_that_are_not_in_the_registry_Then_one_note_names_them_and_registered_ones_are_not_mentioned(t *testing.T) {
	// Arrange
	n := 0
	newID := func() model.ID { n++; return model.ID(fmt.Sprintf("ID%024d", n)) }

	// Act
	_, diags := Import([]byte(formatsDoc), Options{NewID: newID})

	// Assert
	var notes []diag.Diagnostic
	for _, d := range diags {
		if d.Code == diag.CodeImportUnmapped && strings.Contains(d.Args["what"], "format") {
			notes = append(notes, d)
		}
	}
	if len(notes) != 1 || notes[0].Severity != diag.SeverityWarning {
		t.Fatalf("notes %v", notes)
	}
	detail := notes[0].Args["detail"] + " " + notes[0].Args["what"]
	for _, want := range []string{"phone-number", "postal-code"} {
		if !strings.Contains(detail, want) {
			t.Errorf("%q is missing from %q", want, detail)
		}
	}
	for _, unwanted := range []string{"date,", "uuid", "csv", `"format"`} {
		if strings.Contains(detail, unwanted) {
			t.Errorf("%q must not be in %q", unwanted, detail)
		}
	}
}
