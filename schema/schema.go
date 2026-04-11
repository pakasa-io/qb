package schema

import (
	"errors"
	"fmt"
	"sort"

	"github.com/pakasa-io/qb"
)

var (
	// ErrUnknownField indicates the schema does not recognize a field.
	ErrUnknownField = errors.New("unknown field")
	// ErrFilteringNotAllowed indicates the field cannot be used in filters.
	ErrFilteringNotAllowed = errors.New("filtering not allowed")
	// ErrSortingNotAllowed indicates the field cannot be used in sort clauses.
	ErrSortingNotAllowed = errors.New("sorting not allowed")
	// ErrOperatorNotAllowed indicates the operator is not permitted for the field.
	ErrOperatorNotAllowed = errors.New("operator not allowed")
)

// Decoder converts a parsed field value into a domain-specific type.
type Decoder func(op qb.Operator, value any) (any, error)

// Field describes a schema field and its capabilities.
type Field struct {
	Name             string
	Aliases          []string
	Filterable       bool
	Sortable         bool
	AllowedOperators []qb.Operator
	Decoder          Decoder
}

// FieldOption customizes a field declaration.
type FieldOption func(*Field)

// Define builds a schema field using developer-friendly defaults.
func Define(name string, opts ...FieldOption) Field {
	field := Field{
		Name:       name,
		Filterable: true,
	}

	for _, opt := range opts {
		opt(&field)
	}

	return field
}

// Aliases adds accepted aliases for the field.
func Aliases(names ...string) FieldOption {
	return func(field *Field) {
		field.Aliases = append(field.Aliases, names...)
	}
}

// Sortable enables sorting for the field.
func Sortable() FieldOption {
	return func(field *Field) {
		field.Sortable = true
	}
}

// DisableFiltering disables filtering for the field.
func DisableFiltering() FieldOption {
	return func(field *Field) {
		field.Filterable = false
	}
}

// Operators restricts the operators allowed for the field.
func Operators(ops ...qb.Operator) FieldOption {
	return func(field *Field) {
		field.AllowedOperators = append([]qb.Operator(nil), ops...)
	}
}

// Decode sets a decoder for the field.
func Decode(decoder Decoder) FieldOption {
	return func(field *Field) {
		field.Decoder = decoder
	}
}

// Schema validates and canonicalizes query fields.
type Schema struct {
	fields  map[string]fieldSpec
	aliases map[string]string
}

type fieldSpec struct {
	name             string
	aliases          []string
	filterable       bool
	sortable         bool
	allowedOperators map[qb.Operator]struct{}
	decoder          Decoder
}

// New constructs a schema from field definitions.
func New(fields ...Field) (Schema, error) {
	s := Schema{
		fields:  make(map[string]fieldSpec, len(fields)),
		aliases: make(map[string]string, len(fields)),
	}

	for _, field := range fields {
		if field.Name == "" {
			return Schema{}, fmt.Errorf("schema: field name cannot be empty")
		}

		if _, exists := s.fields[field.Name]; exists {
			return Schema{}, fmt.Errorf("schema: duplicate field %q", field.Name)
		}

		spec := fieldSpec{
			name:       field.Name,
			aliases:    append([]string(nil), field.Aliases...),
			filterable: field.Filterable,
			sortable:   field.Sortable,
			decoder:    field.Decoder,
		}

		if len(field.AllowedOperators) > 0 {
			spec.allowedOperators = make(map[qb.Operator]struct{}, len(field.AllowedOperators))
			for _, op := range field.AllowedOperators {
				spec.allowedOperators[op] = struct{}{}
			}
		}

		s.fields[field.Name] = spec
		s.aliases[field.Name] = field.Name

		for _, alias := range field.Aliases {
			if alias == "" {
				return Schema{}, fmt.Errorf("schema: field %q contains an empty alias", field.Name)
			}

			if existing, exists := s.aliases[alias]; exists {
				return Schema{}, fmt.Errorf("schema: alias %q already belongs to %q", alias, existing)
			}

			s.aliases[alias] = field.Name
		}
	}

	return s, nil
}

// MustNew constructs a schema or panics.
func MustNew(fields ...Field) Schema {
	schema, err := New(fields...)
	if err != nil {
		panic(err)
	}
	return schema
}

// ResolveFilterField resolves a field alias, checks filtering capability, and
// validates the requested operator.
func (s Schema) ResolveFilterField(field string, op qb.Operator) (string, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return "", err
	}

	if !spec.filterable {
		return "", fmt.Errorf("%w: %s", ErrFilteringNotAllowed, field)
	}

	if len(spec.allowedOperators) > 0 {
		if _, ok := spec.allowedOperators[op]; !ok {
			return "", fmt.Errorf("%w: %s does not allow %s", ErrOperatorNotAllowed, spec.name, op)
		}
	}

	return spec.name, nil
}

// ResolveSortField resolves a field alias and checks sort capability.
func (s Schema) ResolveSortField(field string) (string, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return "", err
	}

	if !spec.sortable {
		return "", fmt.Errorf("%w: %s", ErrSortingNotAllowed, field)
	}

	return spec.name, nil
}

// DecodeValue applies the field decoder when one exists.
func (s Schema) DecodeValue(field string, op qb.Operator, value any) (any, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return nil, err
	}

	if spec.decoder == nil {
		return value, nil
	}

	decoded, err := spec.decoder(op, value)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// Normalize canonicalizes filter and sort fields and validates schema rules.
func (s Schema) Normalize(query qb.Query) (qb.Query, error) {
	normalized := query.Clone()

	if normalized.Filter != nil {
		filter, err := s.normalizeExpr(normalized.Filter)
		if err != nil {
			return qb.Query{}, err
		}
		normalized.Filter = filter
	}

	if len(normalized.Sorts) > 0 {
		sorts := make([]qb.Sort, len(normalized.Sorts))
		for i, sortField := range normalized.Sorts {
			if sortField.Direction == "" {
				sortField.Direction = qb.Asc
			}
			if sortField.Direction != qb.Asc && sortField.Direction != qb.Desc {
				return qb.Query{}, fmt.Errorf("schema: unsupported sort direction %q", sortField.Direction)
			}

			resolvedField, err := s.ResolveSortField(sortField.Field)
			if err != nil {
				return qb.Query{}, err
			}

			sortField.Field = resolvedField
			sorts[i] = sortField
		}
		normalized.Sorts = sorts
	}

	if normalized.Limit != nil && *normalized.Limit < 0 {
		return qb.Query{}, fmt.Errorf("schema: limit cannot be negative")
	}

	if normalized.Offset != nil && *normalized.Offset < 0 {
		return qb.Query{}, fmt.Errorf("schema: offset cannot be negative")
	}

	return normalized, nil
}

// Fields returns the canonical field names defined in the schema.
func (s Schema) Fields() []string {
	names := make([]string, 0, len(s.fields))
	for name := range s.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s Schema) normalizeExpr(expr qb.Expr) (qb.Expr, error) {
	switch typed := expr.(type) {
	case nil:
		return nil, nil
	case qb.Predicate:
		field, err := s.ResolveFilterField(typed.Field, typed.Op)
		if err != nil {
			return nil, err
		}

		typed.Field = field
		return typed, nil
	case qb.Group:
		terms := make([]qb.Expr, len(typed.Terms))
		for i, term := range typed.Terms {
			normalized, err := s.normalizeExpr(term)
			if err != nil {
				return nil, err
			}
			terms[i] = normalized
		}
		typed.Terms = terms
		return typed, nil
	case qb.Negation:
		normalized, err := s.normalizeExpr(typed.Expr)
		if err != nil {
			return nil, err
		}
		typed.Expr = normalized
		return typed, nil
	default:
		return nil, fmt.Errorf("schema: unsupported expression %T", expr)
	}
}

func (s Schema) lookup(field string) (fieldSpec, error) {
	name, ok := s.aliases[field]
	if !ok {
		return fieldSpec{}, fmt.Errorf("%w: %s", ErrUnknownField, field)
	}

	spec, ok := s.fields[name]
	if !ok {
		return fieldSpec{}, fmt.Errorf("%w: %s", ErrUnknownField, field)
	}

	return spec, nil
}
