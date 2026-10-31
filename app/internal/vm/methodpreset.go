// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// A method such as PROPFIND is only a verb to the library: it adds no headers and no body. WebDAV and CalDAV
// servers need Depth, Destination or an XML body with it, so choosing one of these methods fills the usual ones in.
// A preset only adds: a header the person already has stays as typed, and a body is only written into an empty one.

type methodPreset struct {
	headers []model.KeyValue
	// xml is an example body, sent as application/xml; "" for a method that has none.
	xml string
}

const davHeader = `<?xml version="1.0" encoding="utf-8"?>` + "\n"

// methodPresets are keyed by the registered spelling of the method.
func methodPresets() map[string]methodPreset {
	return map[string]methodPreset{
		"PROPFIND": {
			headers: []model.KeyValue{{Key: "Depth", Value: "1"}},
			xml: davHeader + `<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:resourcetype/>
    <D:getcontentlength/>
    <D:getlastmodified/>
  </D:prop>
</D:propfind>`,
		},
		"PROPPATCH": {
			xml: davHeader + `<D:propertyupdate xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:displayname>New name</D:displayname>
    </D:prop>
  </D:set>
</D:propertyupdate>`,
		},
		"REPORT": {
			headers: []model.KeyValue{{Key: "Depth", Value: "1"}},
			xml: davHeader + `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
		},
		"MKCALENDAR": {
			xml: davHeader + `<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>New calendar</D:displayname>
    </D:prop>
  </D:set>
</C:mkcalendar>`,
		},
		// Overwrite F: a copy or move that would replace something fails instead.
		"COPY": {headers: []model.KeyValue{{Key: "Destination", Value: "{{env.base_url}}/new-location"}, {Key: "Overwrite", Value: "F"}}},
		"MOVE": {headers: []model.KeyValue{{Key: "Destination", Value: "{{env.base_url}}/new-location"}, {Key: "Overwrite", Value: "F"}}},
		"LOCK": {
			headers: []model.KeyValue{{Key: "Timeout", Value: "Second-3600"}},
			xml: davHeader + `<D:lockinfo xmlns:D="DAV:">
  <D:lockscope><D:exclusive/></D:lockscope>
  <D:locktype><D:write/></D:locktype>
  <D:owner>flowi</D:owner>
</D:lockinfo>`,
		},
		// The token is a variable to fill from the LOCK response, not a made-up value that could be sent.
		"UNLOCK": {headers: []model.KeyValue{{Key: "Lock-Token", Value: "<{{lock_token}}>"}}},
		"LINK":   {headers: []model.KeyValue{{Key: "Link", Value: `<{{env.base_url}}/target>; rel="related"`}}},
		"UNLINK": {headers: []model.KeyValue{{Key: "Link", Value: `<{{env.base_url}}/target>; rel="related"`}}},
		"SEARCH": {
			xml: davHeader + `<D:searchrequest xmlns:D="DAV:">
  <D:basicsearch>
    <D:select><D:prop><D:displayname/></D:prop></D:select>
    <D:from><D:scope><D:href>/</D:href><D:depth>infinity</D:depth></D:scope></D:from>
    <D:where><D:like><D:prop><D:displayname/></D:prop><D:literal>%report%</D:literal></D:like></D:where>
  </D:basicsearch>
</D:searchrequest>`,
		},
	}
}

// ApplyMethodPreset adds to v what its method usually needs (see methodPresets): headers it does not have, and
// an example XML body when it has no body at all. Any other method, or a name in the wrong case, changes nothing.
func ApplyMethodPreset(v *model.RequestVersion) {
	preset, ok := methodPresets()[v.Method]
	if !ok {
		return
	}
	for _, h := range preset.headers {
		addHeaderIfAbsent(v, h)
	}
	if preset.xml != "" && !bodyHasContent(v.Body) {
		v.Body.Text = preset.xml
		addHeaderIfAbsent(v, model.KeyValue{Key: contentTypeHeader, Value: "application/xml"})
	}
}

func addHeaderIfAbsent(v *model.RequestVersion, h model.KeyValue) {
	if slices.ContainsFunc(v.Headers, func(x model.KeyValue) bool { return strings.EqualFold(x.Key, h.Key) }) {
		return
	}
	v.Headers = append(slices.Clone(v.Headers), h)
}
