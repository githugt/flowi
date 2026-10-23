// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// maxExampleDepth bounds how deep an example goes into nested messages (a
// message that contains itself would not end).
const maxExampleDepth = 3

// exampleMessage is a message as compact JSON with every field at its zero
// value, in declaration order: strings "", numbers 0, bools false, an enum its
// first name, repeated fields [], maps {}, a nested message its own example
// and, of the fields of a oneof, only the first.
func exampleMessage(md protoreflect.MessageDescriptor) string {
	var b strings.Builder
	writeMessage(&b, md, 0)
	return b.String()
}

func writeMessage(b *strings.Builder, md protoreflect.MessageDescriptor, depth int) {
	if wk, ok := wellKnownExample(md.FullName()); ok {
		b.WriteString(wk)
		return
	}
	b.WriteByte('{')
	first := true
	seenOneof := map[protoreflect.Name]bool{}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			if seenOneof[oneof.Name()] {
				continue
			}
			seenOneof[oneof.Name()] = true
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteString(strconv.Quote(string(fd.Name())))
		b.WriteByte(':')
		writeField(b, fd, depth)
	}
	b.WriteByte('}')
}

func writeField(b *strings.Builder, fd protoreflect.FieldDescriptor, depth int) {
	switch {
	case fd.IsMap():
		b.WriteString("{}")
	case fd.IsList():
		b.WriteString("[]")
	default:
		writeValue(b, fd, depth)
	}
}

func writeValue(b *strings.Builder, fd protoreflect.FieldDescriptor, depth int) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b.WriteString("false")
	case protoreflect.StringKind, protoreflect.BytesKind:
		b.WriteString(`""`)
	case protoreflect.EnumKind:
		b.WriteString(strconv.Quote(string(fd.Enum().Values().Get(0).Name())))
	case protoreflect.MessageKind, protoreflect.GroupKind:
		if depth >= maxExampleDepth {
			b.WriteString("{}")
			return
		}
		writeMessage(b, fd.Message(), depth+1)
	default: // every number kind
		b.WriteString("0")
	}
}

// wellKnownExample is the JSON of the well-known types whose JSON is not an
// object of their fields.
func wellKnownExample(name protoreflect.FullName) (string, bool) {
	switch name {
	case "google.protobuf.Timestamp":
		return `"1970-01-01T00:00:00Z"`, true
	case "google.protobuf.Duration":
		return `"0s"`, true
	case "google.protobuf.FieldMask":
		return `""`, true
	case "google.protobuf.Value":
		return "null", true
	case "google.protobuf.ListValue":
		return "[]", true
	case "google.protobuf.Struct", "google.protobuf.Empty", "google.protobuf.Any":
		return "{}", true
	case "google.protobuf.BoolValue":
		return "false", true
	case "google.protobuf.StringValue", "google.protobuf.BytesValue":
		return `""`, true
	case "google.protobuf.Int32Value", "google.protobuf.Int64Value", "google.protobuf.UInt32Value", "google.protobuf.UInt64Value",
		"google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return "0", true
	}
	return "", false
}
