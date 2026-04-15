package qb

import "strings"

// CastTo constructs a cast expression using a canonical logical type name.
func CastTo(value any, typeName string) Cast {
	return Cast{
		Expr: scalarFromAny(value),
		Type: strings.TrimSpace(typeName),
	}
}

func (r Ref) Cast(typeName string) Cast     { return CastTo(r, typeName) }
func (l Literal) Cast(typeName string) Cast { return CastTo(l, typeName) }
func (c Call) Cast(typeName string) Cast    { return CastTo(c, typeName) }
func (c Cast) Cast(typeName string) Cast    { return CastTo(c, typeName) }

func (c Cast) As(alias string) Projection { return Project(c).As(alias) }
