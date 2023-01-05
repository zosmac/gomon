// Copyright © 2021 The Gomon Project.

package core

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type (
	// Formatter function prototype for functions that encode values for the Format function.
	Formatter func(name, tag string, val reflect.Value) interface{}
)

// Format recurses through a structures' fields to encode them using the Formatter.
func Format(name, tag string, val reflect.Value, fn Formatter) (ms []interface{}) {
	s := strings.Split(tag, ",")
	if len(s) > 2 && (s[2][0] == '!' && s[2][1:] == runtime.GOOS || s[2][0] != '!' && s[2] != runtime.GOOS) {
		return nil
	}

	name = SnakeCase(name)
	for {
		switch val.Kind() {
		case reflect.Invalid:
			return nil
		case reflect.Interface, reflect.Ptr:
			if val.IsNil() { // documentation
				if val.Kind() == reflect.Interface {
					ms = append(ms, fn(name, tag, val))
					return
				}
				val = reflect.Indirect(reflect.New(val.Type().Elem()))
			} else {
				val = val.Elem()
			}
			continue
		}
		break
	}

	switch val.Kind() {
	default:
		if name == "" || name[len(name)-1] == '_' { // embedded field
			name += SnakeCase(val.Type().Name())
		}
		if enc := fn(name, tag, val); enc != nil {
			ms = append(ms, enc)
		}
	case reflect.Slice:
		name += val.Type().Name()
		for i := 0; i < val.Len(); i++ {
			ms = append(ms, Format(name+"_"+strconv.Itoa(i), tag, val.Index(i), fn)...)
		}
		if val.IsNil() { // documentatation
			ms = append(ms, Format(name+"[n]", tag, reflect.Indirect(reflect.New(val.Type().Elem())), fn)...)
		}
	case reflect.Map:
		name += val.Type().Name()
		for _, k := range val.MapKeys() {
			key := fmt.Sprintf("%v", k.Interface())
			ms = append(ms, Format(name+"_"+key, tag, val.MapIndex(k), fn)...)
		}
		if val.IsNil() { // documentation
			ms = append(ms, Format(name+"[key]", tag, reflect.Indirect(reflect.New(val.Type().Elem())), fn)...)
		}
	case reflect.Struct:
		switch val.Interface().(type) {
		case time.Time:
			if enc := fn(name, tag, val); enc != nil {
				ms = append(ms, enc)
			}
			return
		case fmt.Stringer:
			if val.Type().PkgPath() == "" {
				if enc := fn(name, tag, val); enc != nil {
					ms = append(ms, enc)
				}
				return
			}
		}

		t := val.Type()
		if name != "" && name[len(name)-1] != '_' {
			name += "_"
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if tag, ok := f.Tag.Lookup("gomon"); ok {
				var n string
				if !f.Anonymous { // embedded field
					n = f.Name
				}
				ms = append(ms, Format(name+n, tag, val.Field(i), fn)...)
			}
		}
	}

	return
}

// Capitalize uppercases the leading character of a name.
func Capitalize(s string) string {
	c, n := utf8.DecodeRuneInString(s)
	return string([]rune{unicode.ToUpper(c)}) + s[n:]
}

// Uncapitalize lowercases the leading character of a name.
func Uncapitalize(s string) string {
	c, n := utf8.DecodeRuneInString(s)
	return string([]rune{unicode.ToLower(c)}) + s[n:]
}

// SnakeCase converts a CamelCase name to snake_case to follow JSON and data base naming conventions.
func SnakeCase(s string) string {
	var prev rune
	var runes, acronym []rune
	for _, curr := range s {
		if unicode.IsLower(prev) && unicode.IsUpper(curr) {
			runes = append(runes, '_', unicode.ToLower(curr))
		} else if unicode.IsUpper(prev) && unicode.IsLower(curr) {
			if len(acronym) > 0 {
				runes = append(runes, acronym[:len(acronym)-1]...)
				acronym = []rune{}
				runes = append(runes, '_', unicode.ToLower(prev), curr)
			} else {
				runes = append(runes, curr)
			}
		} else if unicode.IsUpper(prev) && unicode.IsUpper(curr) {
			acronym = append(acronym, unicode.ToLower(curr))
		} else {
			runes = append(runes, unicode.ToLower(curr))
		}
		prev = curr
	}
	if len(acronym) > 0 {
		runes = append(runes, acronym...)
	}

	return string(runes)
}

// CamelCase converts a snake_case name to CamelCase to follow Go naming conventions.
func CamelCase(s string) string {
	c, n := utf8.DecodeRuneInString(s)
	u := []rune{unicode.ToUpper(c)}
	s = s[n:]
	for len(s) > 0 {
		c, n := utf8.DecodeRuneInString(s)
		s = s[n:]
		if c == '_' && len(s) > 0 {
			c, n = utf8.DecodeRuneInString(s)
			c = unicode.ToUpper(c)
			s = s[n:]
		}
		u = append(u, c)
	}
	return string(u)
}
