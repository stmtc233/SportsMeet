package interchange

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

// Encode writes the same two-space JSON and trailing newline as json.Encoder.
// Top-level array records share a small encoder buffer instead of building a
// second fully indented snapshot in memory. File's tags remain the field source.
func Encode(f *File) ([]byte, error) {
	if f == nil {
		return []byte("null\n"), nil
	}
	var out, record bytes.Buffer
	enc := json.NewEncoder(&record)
	enc.SetEscapeHTML(false)
	write := func(value any, prefix string) error {
		record.Reset()
		enc.SetIndent(prefix, "  ")
		if err := enc.Encode(value); err != nil {
			return err
		}
		out.Write(record.Bytes()[:record.Len()-1]) // the caller owns separators
		return nil
	}
	v := reflect.ValueOf(f).Elem()
	t := v.Type()
	out.WriteByte('{')
	first := true
	for i := range v.NumField() {
		field, value := t.Field(i), v.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")
		name := tag[0]
		if name == "-" {
			continue
		}
		if len(tag) > 1 && tag[1] == "omitempty" && emptyJSONValue(value) {
			continue
		}
		if name == "" {
			name = field.Name
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.WriteString("\n  ")
		key, _ := json.Marshal(name)
		out.Write(key)
		out.WriteString(": ")
		if value.Kind() != reflect.Slice || value.Len() == 0 {
			if err := write(value.Interface(), "  "); err != nil {
				return nil, err
			}
			continue
		}
		out.WriteString("[\n")
		for j := range value.Len() {
			out.WriteString("    ")
			if err := write(value.Index(j).Interface(), "    "); err != nil {
				return nil, err
			}
			if j+1 < value.Len() {
				out.WriteByte(',')
			}
			out.WriteByte('\n')
		}
		out.WriteString("  ]")
	}
	out.WriteString("\n}\n")
	return out.Bytes(), nil
}

func emptyJSONValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
