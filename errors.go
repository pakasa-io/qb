package qb

import (
	"errors"
	"strings"
)

// ErrorStage identifies where a failure occurred.
type ErrorStage string

const (
	StageParse     ErrorStage = "parse"
	StageNormalize ErrorStage = "normalize"
	StageRewrite   ErrorStage = "rewrite"
	StageCompile   ErrorStage = "compile"
	StageApply     ErrorStage = "apply"
)

// ErrorCode describes the failure category.
type ErrorCode string

const (
	CodeInvalidInput        ErrorCode = "invalid_input"
	CodeInvalidValue        ErrorCode = "invalid_value"
	CodeInvalidQuery        ErrorCode = "invalid_query"
	CodeUnknownField        ErrorCode = "unknown_field"
	CodeUnsupportedOperator ErrorCode = "unsupported_operator"
	CodeUnsupportedFunction ErrorCode = "unsupported_function"
	CodeUnsupportedFeature  ErrorCode = "unsupported_feature"
)

// Error carries machine-readable diagnostics while remaining compatible with
// standard Go error handling.
type Error struct {
	Stage    ErrorStage
	Code     ErrorCode
	Path     string
	Field    string
	Operator Operator
	Function string
	Err      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	parts := make([]string, 0, 5)
	if e.Stage != "" {
		parts = append(parts, string(e.Stage))
	}
	if e.Code != "" {
		parts = append(parts, string(e.Code))
	}
	if e.Path != "" {
		parts = append(parts, "path="+e.Path)
	}
	if e.Field != "" {
		parts = append(parts, "field="+e.Field)
	}
	if e.Operator != "" {
		parts = append(parts, "op="+string(e.Operator))
	}
	if e.Function != "" {
		parts = append(parts, "fn="+e.Function)
	}

	prefix := strings.Join(parts, " ")
	if e.Err == nil {
		return prefix
	}
	if prefix == "" {
		return e.Err.Error()
	}
	return prefix + ": " + e.Err.Error()
}

// Unwrap exposes the underlying error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorOption mutates a structured error.
type ErrorOption func(*Error)

// WithStage sets the error stage.
func WithStage(stage ErrorStage) ErrorOption {
	return func(err *Error) {
		err.Stage = stage
	}
}

// WithDefaultStage sets the stage only when it has not already been assigned.
func WithDefaultStage(stage ErrorStage) ErrorOption {
	return func(err *Error) {
		if err.Stage == "" {
			err.Stage = stage
		}
	}
}

// WithCode sets the error code.
func WithCode(code ErrorCode) ErrorOption {
	return func(err *Error) {
		err.Code = code
	}
}

// WithDefaultCode sets the code only when it has not already been assigned.
func WithDefaultCode(code ErrorCode) ErrorOption {
	return func(err *Error) {
		if err.Code == "" {
			err.Code = code
		}
	}
}

// WithPath sets the input/query path associated with the error.
func WithPath(path string) ErrorOption {
	return func(err *Error) {
		err.Path = path
	}
}

// WithField sets the field associated with the error.
func WithField(field string) ErrorOption {
	return func(err *Error) {
		err.Field = field
	}
}

// WithOperator sets the operator associated with the error.
func WithOperator(op Operator) ErrorOption {
	return func(err *Error) {
		err.Operator = op
	}
}

// WithFunction sets the function associated with the error.
func WithFunction(name string) ErrorOption {
	return func(err *Error) {
		err.Function = name
	}
}

// NewError creates a new structured error.
func NewError(err error, opts ...ErrorOption) error {
	if err == nil {
		return nil
	}

	diagnostic := &Error{Err: err}
	for _, opt := range opts {
		opt(diagnostic)
	}
	return diagnostic
}

// WrapError augments an existing error with structured metadata.
func WrapError(err error, opts ...ErrorOption) error {
	if err == nil {
		return nil
	}

	var diagnostic *Error
	if errors.As(err, &diagnostic) {
		clone := *diagnostic
		for _, opt := range opts {
			opt(&clone)
		}
		return &clone
	}

	return NewError(err, opts...)
}

// UnsupportedFunction creates a structured unsupported-function error.
func UnsupportedFunction(stage ErrorStage, dialect string, function string) error {
	return NewError(
		errors.New("function is not supported by dialect "+dialect),
		WithStage(stage),
		WithCode(CodeUnsupportedFunction),
		WithFunction(function),
	)
}
