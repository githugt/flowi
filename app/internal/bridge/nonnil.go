// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"reflect"
	"strings"
)

// emptyCollections returns v with every nil slice and map in it (and in the structs inside it) made empty, so it
// reaches the page as [] and {} and not as null, which the page reads .length of. Fields tagged omitempty are left:
// they are absent from the JSON when empty. A value that holds pointers is shared, not copied.
func emptyCollections[T any](v T) T {
	rv := reflect.ValueOf(&v).Elem()
	fillEmpty(rv, 0)
	return v
}

func fillEmpty(v reflect.Value, depth int) {
	if depth > 8 {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if !f.IsExported() || tag == "-" || strings.Contains(tag, "omitempty") {
				continue
			}
			fillEmpty(v.Field(i), depth+1)
		}
	case reflect.Slice:
		if v.IsNil() {
			if v.CanSet() {
				v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			}
			return
		}
		// Elements are shared with the caller's slice, so copy before changing one.
		if v.Type().Elem().Kind() == reflect.Struct && v.CanSet() {
			c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
			reflect.Copy(c, v)
			for i := 0; i < c.Len(); i++ {
				fillEmpty(c.Index(i), depth+1)
			}
			v.Set(c)
		}
	case reflect.Map:
		if v.IsNil() && v.CanSet() {
			v.Set(reflect.MakeMap(v.Type()))
		}
	}
}
