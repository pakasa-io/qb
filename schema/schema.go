package schema

import (
	"errors"
	"fmt"
	"reflect"
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
	StorageName      string
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
		Name:        name,
		StorageName: name,
		Filterable:  true,
	}

	for _, opt := range opts {
		opt(&field)
	}

	return field
}

// Storage sets the storage-facing field identifier used by adapters.
func Storage(name string) FieldOption {
	return func(field *Field) {
		field.StorageName = name
	}
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
	storageName      string
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
			name:        field.Name,
			storageName: field.StorageName,
			aliases:     append([]string(nil), field.Aliases...),
			filterable:  field.Filterable,
			sortable:    field.Sortable,
			decoder:     field.Decoder,
		}

		if spec.storageName == "" {
			spec.storageName = field.Name
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
		return "", qb.WrapError(
			err,
			qb.WithCode(qb.CodeUnknownField),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	if !spec.filterable {
		return "", qb.NewError(
			fmt.Errorf("%w: %s", ErrFilteringNotAllowed, field),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	if len(spec.allowedOperators) > 0 {
		if _, ok := spec.allowedOperators[op]; !ok {
			return "", qb.NewError(
				fmt.Errorf("%w: %s does not allow %s", ErrOperatorNotAllowed, spec.name, op),
				qb.WithCode(qb.CodeUnsupportedOperator),
				qb.WithField(spec.name),
				qb.WithOperator(op),
			)
		}
	}

	return spec.name, nil
}

// ResolveSortField resolves a field alias and checks sort capability.
func (s Schema) ResolveSortField(field string) (string, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return "", qb.WrapError(
			err,
			qb.WithCode(qb.CodeUnknownField),
			qb.WithField(field),
		)
	}

	if !spec.sortable {
		return "", qb.NewError(
			fmt.Errorf("%w: %s", ErrSortingNotAllowed, field),
			qb.WithCode(qb.CodeInvalidQuery),
			qb.WithField(field),
		)
	}

	return spec.name, nil
}

// ResolveStorageField resolves a field alias to its storage-facing identifier.
func (s Schema) ResolveStorageField(field string) (string, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return "", qb.WrapError(
			err,
			qb.WithCode(qb.CodeUnknownField),
			qb.WithField(field),
		)
	}

	return spec.storageName, nil
}

// ResolveField resolves a field alias to its canonical API-facing identifier.
func (s Schema) ResolveField(field string) (string, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return "", qb.WrapError(
			err,
			qb.WithCode(qb.CodeUnknownField),
			qb.WithField(field),
		)
	}

	return spec.name, nil
}

// DecodeValue applies the field decoder when one exists.
func (s Schema) DecodeValue(field string, op qb.Operator, value any) (any, error) {
	spec, err := s.lookup(field)
	if err != nil {
		return nil, qb.WrapError(
			err,
			qb.WithCode(qb.CodeUnknownField),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}

	if spec.decoder == nil {
		return value, nil
	}

	decoded, err := spec.decoder(op, value)
	if err != nil {
		return nil, qb.NewError(
			err,
			qb.WithCode(qb.CodeInvalidValue),
			qb.WithField(field),
			qb.WithOperator(op),
		)
	}
	return decoded, nil
}

// Normalize canonicalizes filter and sort fields and validates schema rules.
func (s Schema) Normalize(query qb.Query) (qb.Query, error) {
	normalized, err := qb.RewriteQuery(query, func(expr qb.Expr) (qb.Expr, error) {
		predicate, ok := expr.(qb.Predicate)
		if !ok {
			return expr, nil
		}

		field, err := s.ResolveFilterField(predicate.Field, predicate.Op)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(qb.StageNormalize),
				qb.WithField(predicate.Field),
				qb.WithOperator(predicate.Op),
			)
		}

		value, err := s.decodePredicateValue(field, predicate.Op, predicate.Value)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(qb.StageNormalize),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		predicate.Field = field
		predicate.Value = value
		return predicate, nil
	})
	if err != nil {
		return qb.Query{}, err
	}

	if len(normalized.Sorts) > 0 {
		sorts := make([]qb.Sort, len(normalized.Sorts))
		for i, sortField := range normalized.Sorts {
			if sortField.Direction == "" {
				sortField.Direction = qb.Asc
			}
			if sortField.Direction != qb.Asc && sortField.Direction != qb.Desc {
				return qb.Query{}, qb.NewError(
					fmt.Errorf("unsupported sort direction %q", sortField.Direction),
					qb.WithStage(qb.StageNormalize),
					qb.WithCode(qb.CodeInvalidQuery),
					qb.WithField(sortField.Field),
				)
			}

			resolvedField, err := s.ResolveSortField(sortField.Field)
			if err != nil {
				return qb.Query{}, qb.WrapError(
					err,
					qb.WithStage(qb.StageNormalize),
					qb.WithField(sortField.Field),
				)
			}

			sortField.Field = resolvedField
			sorts[i] = sortField
		}
		normalized.Sorts = sorts
	}

	selects, err := s.normalizeFields(normalized.Selects)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageNormalize))
	}
	normalized.Selects = selects

	groupBy, err := s.normalizeFields(normalized.GroupBy)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageNormalize))
	}
	normalized.GroupBy = groupBy

	if _, _, err := normalized.ResolvedPagination(); err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageNormalize))
	}

	return normalized, nil
}

// ToStorage normalizes the query and rewrites public field names to their
// storage-facing identifiers.
func (s Schema) ToStorage(query qb.Query) (qb.Query, error) {
	normalized, err := s.Normalize(query)
	if err != nil {
		return qb.Query{}, err
	}

	projected, err := qb.RewriteQuery(normalized, func(expr qb.Expr) (qb.Expr, error) {
		predicate, ok := expr.(qb.Predicate)
		if !ok {
			return expr, nil
		}

		field, err := s.ResolveStorageField(predicate.Field)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(qb.StageRewrite),
				qb.WithField(predicate.Field),
				qb.WithOperator(predicate.Op),
			)
		}

		predicate.Field = field
		return predicate, nil
	})
	if err != nil {
		return qb.Query{}, err
	}

	if len(projected.Sorts) > 0 {
		sorts := make([]qb.Sort, len(projected.Sorts))
		for i, sortField := range projected.Sorts {
			storageField, err := s.ResolveStorageField(sortField.Field)
			if err != nil {
				return qb.Query{}, qb.WrapError(
					err,
					qb.WithStage(qb.StageRewrite),
					qb.WithField(sortField.Field),
				)
			}

			sortField.Field = storageField
			sorts[i] = sortField
		}
		projected.Sorts = sorts
	}

	selects, err := s.projectFields(projected.Selects)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageRewrite))
	}
	projected.Selects = selects

	groupBy, err := s.projectFields(projected.GroupBy)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageRewrite))
	}
	projected.GroupBy = groupBy

	return projected, nil
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

func (s Schema) decodePredicateValue(field string, op qb.Operator, value any) (any, error) {
	switch op {
	case qb.OpIn, qb.OpNotIn:
		values, ok := anySlice(value)
		if !ok {
			return nil, qb.NewError(
				fmt.Errorf("%s requires a list value", op),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(op),
			)
		}

		decoded := make([]any, len(values))
		for i, item := range values {
			decodedValue, err := s.DecodeValue(field, op, item)
			if err != nil {
				return nil, err
			}
			decoded[i] = decodedValue
		}
		return decoded, nil
	case qb.OpIsNull, qb.OpNotNull:
		return nil, nil
	default:
		return s.DecodeValue(field, op, value)
	}
}

func anySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return append([]any(nil), typed...), true
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = item
		}
		return out, true
	default:
		if typed == nil {
			return nil, false
		}

		rv := reflect.ValueOf(typed)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return nil, false
		}

		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return nil, false
		}

		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
}

func (s Schema) normalizeFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	normalized := make([]string, len(fields))
	for i, field := range fields {
		resolved, err := s.ResolveField(field)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithField(field),
			)
		}
		normalized[i] = resolved
	}
	return normalized, nil
}

func (s Schema) projectFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	projected := make([]string, len(fields))
	for i, field := range fields {
		resolved, err := s.ResolveStorageField(field)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithField(field),
			)
		}
		projected[i] = resolved
	}
	return projected, nil
}
