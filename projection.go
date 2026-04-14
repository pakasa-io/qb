package qb

// Projection describes a selected scalar expression and its optional alias.
type Projection struct {
	Expr  Scalar
	Alias string
}

// Project wraps a scalar expression as a projection.
func Project(expr Scalar) Projection {
	return Projection{Expr: CloneScalar(expr)}
}

// Clone returns a deep copy of the projection.
func (p Projection) Clone() Projection {
	return Projection{
		Expr:  CloneScalar(p.Expr),
		Alias: p.Alias,
	}
}

// As assigns an alias to a projection.
func (p Projection) As(alias string) Projection {
	p.Alias = alias
	return p
}

func cloneProjections(values []Projection) []Projection {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]Projection, len(values))
	for i, value := range values {
		cloned[i] = value.Clone()
	}
	return cloned
}
