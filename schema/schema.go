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

		if predicate.Left != nil {
			left, err := s.normalizeFilterExpr(predicate.Left, predicate.Op, qb.StageNormalize)
			if err != nil {
				return nil, err
			}
			predicate.Left = left
			predicate.Field = ""
		} else {
			field, err := s.ResolveFilterField(predicate.Field, predicate.Op)
			if err != nil {
				return nil, qb.WrapError(
					err,
					qb.WithStage(qb.StageNormalize),
					qb.WithField(predicate.Field),
					qb.WithOperator(predicate.Op),
				)
			}
			predicate.Field = field
		}

		value, err := s.normalizePredicateValue(predicate, qb.StageNormalize)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(qb.StageNormalize),
				qb.WithField(predicatePrimaryField(predicate)),
				qb.WithOperator(predicate.Op),
			)
		}

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

	selectExprs, err := s.normalizeExprs(normalized.SelectExprs, s.ResolveField, qb.StageNormalize)
	if err != nil {
		return qb.Query{}, err
	}
	normalized.SelectExprs = selectExprs

	groupBy, err := s.normalizeFields(normalized.GroupBy)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageNormalize))
	}
	normalized.GroupBy = groupBy

	groupExprs, err := s.normalizeExprs(normalized.GroupExprs, s.ResolveField, qb.StageNormalize)
	if err != nil {
		return qb.Query{}, err
	}
	normalized.GroupExprs = groupExprs

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

		if predicate.Left != nil {
			left, err := s.projectExpr(predicate.Left, s.ResolveStorageField, qb.StageRewrite)
			if err != nil {
				return nil, err
			}
			predicate.Left = left
		} else {
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
		}

		value, err := s.projectPredicateValue(predicate.Value, s.ResolveStorageField, qb.StageRewrite)
		if err != nil {
			return nil, err
		}
		predicate.Value = value
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

	selectExprs, err := s.projectExprs(projected.SelectExprs, s.ResolveStorageField, qb.StageRewrite)
	if err != nil {
		return qb.Query{}, err
	}
	projected.SelectExprs = selectExprs

	groupBy, err := s.projectFields(projected.GroupBy)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageRewrite))
	}
	projected.GroupBy = groupBy

	groupExprs, err := s.projectExprs(projected.GroupExprs, s.ResolveStorageField, qb.StageRewrite)
	if err != nil {
		return qb.Query{}, err
	}
	projected.GroupExprs = groupExprs

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

func (s Schema) normalizePredicateValue(predicate qb.Predicate, stage qb.ErrorStage) (any, error) {
	if expr, ok := qb.AsValueExpr(predicate.Value); ok {
		return s.normalizeFilterExpr(expr, predicate.Op, stage)
	}

	field := predicatePrimaryField(predicate)
	if field == "" {
		return qb.CloneValue(predicate.Value), nil
	}

	switch predicate.Op {
	case qb.OpIn, qb.OpNotIn:
		values, ok := anySlice(predicate.Value)
		if !ok {
			return nil, qb.NewError(
				fmt.Errorf("%s requires a list value", predicate.Op),
				qb.WithCode(qb.CodeInvalidValue),
				qb.WithField(field),
				qb.WithOperator(predicate.Op),
			)
		}

		decoded := make([]any, len(values))
		for i, item := range values {
			if expr, ok := qb.AsValueExpr(item); ok {
				rewritten, err := s.normalizeFilterExpr(expr, predicate.Op, stage)
				if err != nil {
					return nil, err
				}
				decoded[i] = rewritten
				continue
			}

			value, err := s.DecodeValue(field, predicate.Op, item)
			if err != nil {
				return nil, err
			}
			decoded[i] = value
		}
		return decoded, nil
	default:
		return s.decodePredicateValue(field, predicate.Op, predicate.Value)
	}
}

func (s Schema) normalizeFilterExpr(expr qb.ValueExpr, op qb.Operator, stage qb.ErrorStage) (qb.ValueExpr, error) {
	return s.rewriteExpr(expr, func(field string) (string, error) {
		return s.ResolveFilterField(field, op)
	}, stage)
}

func (s Schema) normalizeExprs(exprs []qb.ValueExpr, resolver func(string) (string, error), stage qb.ErrorStage) ([]qb.ValueExpr, error) {
	if len(exprs) == 0 {
		return nil, nil
	}

	normalized := make([]qb.ValueExpr, len(exprs))
	for i, expr := range exprs {
		rewritten, err := s.projectExpr(expr, resolver, stage)
		if err != nil {
			return nil, err
		}
		normalized[i] = rewritten
	}
	return normalized, nil
}

func (s Schema) projectExprs(exprs []qb.ValueExpr, resolver func(string) (string, error), stage qb.ErrorStage) ([]qb.ValueExpr, error) {
	return s.normalizeExprs(exprs, resolver, stage)
}

func (s Schema) projectExpr(expr qb.ValueExpr, resolver func(string) (string, error), stage qb.ErrorStage) (qb.ValueExpr, error) {
	return s.rewriteExpr(expr, resolver, stage)
}

func (s Schema) rewriteExpr(expr qb.ValueExpr, resolver func(string) (string, error), stage qb.ErrorStage) (qb.ValueExpr, error) {
	rewritten, err := qb.RewriteValueExpr(expr, func(node qb.ValueExpr) (qb.ValueExpr, error) {
		ref, ok := node.(qb.Ref)
		if !ok {
			return node, nil
		}

		field, err := resolver(string(ref))
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(stage),
				qb.WithField(string(ref)),
			)
		}

		return qb.Field(field), nil
	})
	if err != nil {
		return nil, err
	}

	return rewritten, nil
}

func (s Schema) projectPredicateValue(value any, resolver func(string) (string, error), stage qb.ErrorStage) (any, error) {
	if expr, ok := qb.AsValueExpr(value); ok {
		return s.projectExpr(expr, resolver, stage)
	}

	values, ok := anySlice(value)
	if !ok {
		return qb.CloneValue(value), nil
	}

	projected := make([]any, len(values))
	for i, item := range values {
		if expr, ok := qb.AsValueExpr(item); ok {
			rewritten, err := s.projectExpr(expr, resolver, stage)
			if err != nil {
				return nil, err
			}
			projected[i] = rewritten
			continue
		}
		projected[i] = qb.CloneValue(item)
	}

	return projected, nil
}

func predicatePrimaryField(predicate qb.Predicate) string {
	if predicate.Field != "" {
		return predicate.Field
	}

	if predicate.Left == nil {
		return ""
	}

	field, ok := qb.SingleRef(predicate.Left)
	if !ok {
		return ""
	}

	return field
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
