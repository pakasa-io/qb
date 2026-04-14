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

		left, err := s.rewriteScalar(predicate.Left, func(field string) (string, error) {
			return s.ResolveFilterField(field, predicate.Op)
		}, qb.StageNormalize)
		if err != nil {
			return nil, err
		}
		predicate.Left = left

		right, err := s.normalizeOperand(predicate.Right, predicatePrimaryField(predicate.Left), predicate.Op)
		if err != nil {
			return nil, err
		}
		predicate.Right = right
		return predicate, nil
	})
	if err != nil {
		return qb.Query{}, err
	}

	if len(normalized.Sorts) > 0 {
		sorts := make([]qb.Sort, len(normalized.Sorts))
		for i, sortExpr := range normalized.Sorts {
			if sortExpr.Direction == "" {
				sortExpr.Direction = qb.Asc
			}
			if sortExpr.Direction != qb.Asc && sortExpr.Direction != qb.Desc {
				return qb.Query{}, qb.NewError(
					fmt.Errorf("unsupported sort direction %q", sortExpr.Direction),
					qb.WithStage(qb.StageNormalize),
					qb.WithCode(qb.CodeInvalidQuery),
					qb.WithField(predicatePrimaryField(sortExpr.Expr)),
				)
			}

			expr, err := s.rewriteScalar(sortExpr.Expr, s.ResolveSortField, qb.StageNormalize)
			if err != nil {
				return qb.Query{}, err
			}

			sortExpr.Expr = expr
			sorts[i] = sortExpr
		}
		normalized.Sorts = sorts
	}

	projections, err := s.rewriteProjections(normalized.Projections, s.ResolveField, qb.StageNormalize)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageNormalize))
	}
	normalized.Projections = projections

	groupBy, err := s.rewriteScalars(normalized.GroupBy, s.ResolveField, qb.StageNormalize)
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

		left, err := s.rewriteScalar(predicate.Left, s.ResolveStorageField, qb.StageRewrite)
		if err != nil {
			return nil, err
		}
		predicate.Left = left

		right, err := s.projectOperand(predicate.Right)
		if err != nil {
			return nil, err
		}
		predicate.Right = right
		return predicate, nil
	})
	if err != nil {
		return qb.Query{}, err
	}

	if len(projected.Sorts) > 0 {
		sorts := make([]qb.Sort, len(projected.Sorts))
		for i, sortExpr := range projected.Sorts {
			expr, err := s.rewriteScalar(sortExpr.Expr, s.ResolveStorageField, qb.StageRewrite)
			if err != nil {
				return qb.Query{}, err
			}

			sortExpr.Expr = expr
			sorts[i] = sortExpr
		}
		projected.Sorts = sorts
	}

	projections, err := s.rewriteProjections(projected.Projections, s.ResolveStorageField, qb.StageRewrite)
	if err != nil {
		return qb.Query{}, qb.WrapError(err, qb.WithDefaultStage(qb.StageRewrite))
	}
	projected.Projections = projections

	groupBy, err := s.rewriteScalars(projected.GroupBy, s.ResolveStorageField, qb.StageRewrite)
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

func (s Schema) normalizeOperand(operand qb.Operand, field string, op qb.Operator) (qb.Operand, error) {
	switch typed := operand.(type) {
	case nil:
		return nil, nil
	case qb.ScalarOperand:
		expr, err := s.normalizeRightScalar(typed.Expr, field, op)
		if err != nil {
			return nil, err
		}
		return qb.ScalarOperand{Expr: expr}, nil
	case qb.ListOperand:
		items := make([]qb.Scalar, len(typed.Items))
		for i, item := range typed.Items {
			expr, err := s.normalizeRightScalar(item, field, op)
			if err != nil {
				return nil, err
			}
			items[i] = expr
		}
		return qb.ListOperand{Items: items}, nil
	default:
		return typed, nil
	}
}

func (s Schema) normalizeRightScalar(expr qb.Scalar, field string, op qb.Operator) (qb.Scalar, error) {
	return qb.RewriteScalar(expr, func(node qb.Scalar) (qb.Scalar, error) {
		switch typed := node.(type) {
		case qb.Ref:
			resolved, err := s.ResolveField(typed.Name)
			if err != nil {
				return nil, qb.WrapError(
					err,
					qb.WithStage(qb.StageNormalize),
					qb.WithField(typed.Name),
					qb.WithOperator(op),
				)
			}
			return qb.F(resolved), nil
		case qb.Literal:
			if field == "" {
				return qb.V(typed.Value), nil
			}
			decoded, err := s.DecodeValue(field, op, typed.Value)
			if err != nil {
				return nil, qb.WrapError(
					err,
					qb.WithStage(qb.StageNormalize),
					qb.WithField(field),
					qb.WithOperator(op),
				)
			}
			return qb.V(decoded), nil
		default:
			return node, nil
		}
	})
}

func (s Schema) projectOperand(operand qb.Operand) (qb.Operand, error) {
	switch typed := operand.(type) {
	case nil:
		return nil, nil
	case qb.ScalarOperand:
		expr, err := s.rewriteScalar(typed.Expr, s.ResolveStorageField, qb.StageRewrite)
		if err != nil {
			return nil, err
		}
		return qb.ScalarOperand{Expr: expr}, nil
	case qb.ListOperand:
		items := make([]qb.Scalar, len(typed.Items))
		for i, item := range typed.Items {
			expr, err := s.rewriteScalar(item, s.ResolveStorageField, qb.StageRewrite)
			if err != nil {
				return nil, err
			}
			items[i] = expr
		}
		return qb.ListOperand{Items: items}, nil
	default:
		return typed, nil
	}
}

func (s Schema) rewriteScalars(values []qb.Scalar, resolver func(string) (string, error), stage qb.ErrorStage) ([]qb.Scalar, error) {
	if len(values) == 0 {
		return nil, nil
	}

	rewritten := make([]qb.Scalar, len(values))
	for i, value := range values {
		expr, err := s.rewriteScalar(value, resolver, stage)
		if err != nil {
			return nil, err
		}
		rewritten[i] = expr
	}

	return rewritten, nil
}

func (s Schema) rewriteProjections(values []qb.Projection, resolver func(string) (string, error), stage qb.ErrorStage) ([]qb.Projection, error) {
	if len(values) == 0 {
		return nil, nil
	}

	rewritten := make([]qb.Projection, len(values))
	for i, value := range values {
		expr, err := s.rewriteScalar(value.Expr, resolver, stage)
		if err != nil {
			return nil, err
		}
		rewritten[i] = qb.Projection{Expr: expr, Alias: value.Alias}
	}

	return rewritten, nil
}

func (s Schema) rewriteScalar(expr qb.Scalar, resolver func(string) (string, error), stage qb.ErrorStage) (qb.Scalar, error) {
	return qb.RewriteScalar(expr, func(node qb.Scalar) (qb.Scalar, error) {
		ref, ok := node.(qb.Ref)
		if !ok {
			return node, nil
		}

		resolved, err := resolver(ref.Name)
		if err != nil {
			return nil, qb.WrapError(
				err,
				qb.WithStage(stage),
				qb.WithField(ref.Name),
			)
		}

		return qb.F(resolved), nil
	})
}

func predicatePrimaryField(expr qb.Scalar) string {
	field, ok := qb.SingleRef(expr)
	if !ok {
		return ""
	}
	return field
}
