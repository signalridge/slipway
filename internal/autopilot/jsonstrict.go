package autopilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/jsonstrict"
)

const utf8BOM = "\xef\xbb\xbf"

// decodeStrictJSON decodes one in-memory JSON value while enforcing the
// machine-protocol rules that encoding/json intentionally leaves permissive.
func decodeStrictJSON(raw []byte, target any) error {
	if !utf8.Valid(raw) {
		return errors.New("decode json: input is not valid utf-8")
	}
	if bytes.HasPrefix(raw, []byte(utf8BOM)) {
		return errors.New("decode json: utf-8 bom is not allowed")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("decode json: input is empty")
	}
	if err := jsonstrict.ScanStructure(raw); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := validateExactJSONSchema(raw, target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode json: trailing json value")
		}
		return fmt.Errorf("decode json: trailing data: %w", err)
	}
	return nil
}

var jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()

func validateExactJSONSchema(raw []byte, target any) error {
	targetType := reflect.TypeOf(target)
	if targetType == nil || targetType.Kind() != reflect.Pointer {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("inspect json schema: %w", err)
	}
	return validateExactJSONValue(value, targetType.Elem(), "$")
}

func validateExactJSONValue(value any, targetType reflect.Type, path string) error {
	if implementsJSONUnmarshaler(targetType) {
		return nil
	}
	for targetType.Kind() == reflect.Pointer {
		if value == nil {
			return nil
		}
		targetType = targetType.Elem()
		if implementsJSONUnmarshaler(targetType) {
			return nil
		}
	}

	if value == nil {
		switch targetType.Kind() {
		case reflect.Interface, reflect.Map, reflect.Slice:
			return nil
		default:
			return fmt.Errorf("null is not allowed at %s", path)
		}
	}

	switch targetType.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := exactJSONStructFields(targetType)
		for _, key := range slices.Sorted(maps.Keys(object)) {
			child := object[key]
			fieldType, exists := fields[key]
			if !exists {
				return fmt.Errorf("unknown field %q at %s", key, path)
			}
			if err := validateExactJSONValue(child, fieldType, jsonChildPath(path, key)); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		array, ok := value.([]any)
		if !ok {
			return nil
		}
		for index, child := range array {
			if err := validateExactJSONValue(child, targetType.Elem(), fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok || targetType.Key().Kind() != reflect.String {
			return nil
		}
		for _, key := range slices.Sorted(maps.Keys(object)) {
			if err := validateExactJSONValue(object[key], targetType.Elem(), jsonChildPath(path, key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func implementsJSONUnmarshaler(targetType reflect.Type) bool {
	if targetType.Implements(jsonUnmarshalerType) {
		return true
	}
	return targetType.Kind() != reflect.Pointer && reflect.PointerTo(targetType).Implements(jsonUnmarshalerType)
}

func exactJSONStructFields(targetType reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for field := range targetType.Fields() {
		if field.PkgPath != "" {
			continue
		}

		tagName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagName == "-" {
			continue
		}
		if field.Anonymous && tagName == "" {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Pointer {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				maps.Copy(fields, exactJSONStructFields(embeddedType))
				continue
			}
		}
		if tagName == "" {
			tagName = field.Name
		}
		fields[tagName] = field.Type
	}
	return fields
}

// jsonChildPath renders a JSON-pointer-ish location for the schema validator's
// diagnostics. (The structural scanner in internal/jsonstrict has its own
// equivalent; this one stays local because only the schema-reflection walk
// uses it, not the duplicate-key scan.)
func jsonChildPath(parent, key string) string {
	if key == "" {
		return parent + "[\"\"]"
	}
	for _, character := range key {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return fmt.Sprintf("%s[%q]", parent, key)
		}
	}
	return parent + "." + key
}
