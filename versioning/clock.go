package versioning

type Relation int

const (
	Lt Relation = iota
	Eq
	Gt
	Con // concurrent
)

type Clock interface {
	Compare(other Clock) Relation
	Merge(other Clock) Clock
	Bump(nodeID string) Clock
}