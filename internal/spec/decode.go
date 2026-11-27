package spec

import (
	"encoding/json"
	"reflect"
	"strings"
)

// encoding/json accepts case-insensitive aliases; IPC accepts the exact schema.
func exactFields(data json.RawMessage, kind reflect.Type) bool {
	if kind == reflect.TypeFor[json.RawMessage]() {
		return true
	}
	switch kind.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(data, &object) != nil {
			return false
		}
		fields := make(map[string]reflect.Type, kind.NumField())
		for i := 0; i < kind.NumField(); i++ {
			field := kind.Field(i)
			fields[strings.Split(field.Tag.Get("json"), ",")[0]] = field.Type
		}
		for key, value := range object {
			field, ok := fields[key]
			if !ok || !exactFields(value, field) {
				return false
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if json.Unmarshal(data, &values) != nil {
			return false
		}
		for _, value := range values {
			if !exactFields(value, kind.Elem()) {
				return false
			}
		}
	}
	return true
}
