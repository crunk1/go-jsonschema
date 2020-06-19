package jsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Type string

const (
	ARRAY   Type = "array"
	BOOLEAN Type = "boolean"
	INTEGER Type = "integer"
	NULL    Type = "null"
	NUMBER  Type = "number"
	OBJECT  Type = "object"
	STRING  Type = "string"
)

var (
	types = []Type{ARRAY, BOOLEAN, INTEGER, NULL, NUMBER, OBJECT, STRING}
)

// Schema is a JSON Schema object or a boolean schema value.
// To determine which, use AsBool() (value, ok). If ok == true, Schema is a boolean schema value, else it is a schema object.
type Schema struct {
	*bool
	*schema
}

type schema struct {
	// Meta-data
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.9
	Title       *string       `json:"title,omitempty"`
	Description *string       `json:"description,omitempty"`
	Default     interface{}   `json:"default,omitempty"`
	Deprecated  *bool         `json:"deprecated,omitempty"`
	ReadOnly    *bool         `json:"readOnly,omitempty"`
	WriteOnly   *bool         `json:"writeOnly,omitempty"`
	Examples    []interface{} `json:"examples,omitempty"`

	// Meta-schema
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.8.1
	Schema     *string         `json:"$schema,omitempty"`
	Vocabulary map[string]bool `json:"$vocabulary,omitempty"`

	// Schema, schema URI, root schema, anchors, references
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.8.2
	ID              *string            `json:"$id,omitempty"`
	Anchor          *string            `json:"$anchor,omitempty"`
	RecursiveAnchor *bool              `json:"$recursiveAnchor,omitempty"` // MUST start with a letter ([A-Za-z]), followed by any number of letters, digits ([0-9]), hyphens ("-"), underscores ("_"), colons (":"), or periods (".")
	Ref             *string            `json:"$ref,omitempty"`
	RecursiveRef    *string            `json:"$recursiveRef,omitempty"`
	Defs            map[string]*Schema `json:"$defs,omitempty"`
	Definitions     map[string]*Schema `json:"definitions,omitempty"` // Backwards compatibility, definitions was replaced by $defs.

	// Comments
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.8.3
	Comment *string `json:"$comment,omitempty"`

	// Subschema application
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.9.2
	AllOf            []*Schema          `json:"allOf,omitempty"`
	AnyOf            []*Schema          `json:"anyOf,omitempty"`
	OneOf            []*Schema          `json:"oneOf,omitempty"`
	Not              *Schema            `json:"not,omitempty"`
	If               *Schema            `json:"if,omitempty"`
	Then             *Schema            `json:"then,omitempty"`
	Else             *Schema            `json:"else,omitempty"`
	DependentSchemas map[string]*Schema `json:"dependentSchemas,omitempty"`

	// Validation
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.6.1
	Type  interface{} `json:"type,omitempty"` // Type or array of unique Types
	Enum  []string    `json:"enum,omitempty"`
	Const interface{} `json:"const,omitempty"`

	// Arrays
	// Additional subschema application keywords
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.9.3.1
	Items            interface{} `json:"items,omitempty"` // Schema or array of schemas
	AdditionalItems  *Schema     `json:"additionalItems,omitempty"`
	UnevaluatedItems *Schema     `json:"unevaluatedItems,omitempty"`
	Contains         *Schema     `json:"contains,omitempty"`
	// Validation
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.6.4
	MaxItems    *uint64 `json:"maxItems,omitempty"`
	MinItems    *uint64 `json:"minItems,omitempty"`
	UniqueItems *bool   `json:"uniqueItems,omitempty"`
	MaxContains *uint64 `json:"maxContains,omitempty"`
	MinContains *uint64 `json:"minContains,omitempty"`

	// Objects
	// Additional subschema application keywords
	// https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.9.3.2
	Properties            map[string]*Schema
	PatternProperties     map[string]*Schema // ECMA 262 regular expression dialect -> subschema
	AdditionalProperties  *Schema
	UnevaluatedProperties *Schema
	PropertyNames         *Schema
	// Validation
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.6.5
	MaxProperties     *uint64             `json:"maxProperties,omitempty"`
	MinProperties     *uint64             `json:"minProperties,omitempty"`
	Required          []string            `json:"required,omitempty"`
	DependentRequired map[string][]string `json:"dependentRequired,omitempty"`

	// Numerics
	// Validation
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.6.2
	MultipleOf       *float64 `json:"multipleOf,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`

	// Strings
	// Validation
	// https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.6.3
	MaxLength *uint64 `json:"maxLength,omitempty"`
	MinLength *uint64 `json:"minLength,omitempty"`
	Pattern   *string `json:"pattern,omitempty"` // ECMA 262 regular expression dialect
	Format    *string `json:"format,omitempty"`  // https://json-schema.org/draft/2019-09/json-schema-validation.html#rfc.section.7

	baseURI string
}

// AsBool returns the boolean schema value, if it is a boolean schema value.
// See https://json-schema.org/draft/2019-09/json-schema-core.html#rfc.section.4.3.2
func (s *Schema) AsBool() (value bool, ok bool) {
	if s.bool == nil {
		return false, false
	}
	return *s.bool, true
}

func (s *Schema) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("false")) {
		b := false
		s.bool = &b
	} else if bytes.Equal(data, []byte("true")) {
		b := true
		s.bool = &b
	} else {
		s.schema = &schema{}
		if err := json.Unmarshal(data, s.schema); err != nil {
			return err
		}
	}
	return nil
}

func FromURI(uri string) (*Schema, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	s := &Schema{}
	switch u.Scheme {
	case "file":
		bs, err := ioutil.ReadFile(u.Path)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(bs, s)
		if err != nil {
			return nil, err
		}
	case "http", "https":
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, errors.New("HTTP(S) URI returned a non-200 response")
		}
		defer func() { _ = resp.Body.Close() }()
		bs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(bs, s)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %q", u.Scheme)
	}
	return s, nil
}
