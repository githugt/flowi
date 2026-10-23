// SPDX-License-Identifier: Apache-2.0

// Package openapi turns an OpenAPI 3.0/3.1 document into Flow-I requests:
// one request per operation (path versions /v1/, /v2/ grouped as request
// versions), bodies generated from schemas, response schemas kept as the
// request's contract, and flows suggested from links. It is a pure
// transform: it returns model values and writes nothing.
//
// libopenapi checks that the input is an OpenAPI 3 document; the mapping
// works on the decoded document with local $ref resolution (external
// references are reported, not followed).
package openapi
