package generics

import (
	"fmt"
)

// Any functions that include types additional to V must be global and are in the option package.
type Option[V any] struct {
	// Value must be zeroed when Ok is false for deterministic comparability.
	Value V
	// bool is the smallest type, so putting it at the end increases the chance it can be packed
	// with Value.
	Ok bool
}

func (me Option[V]) UnwrapOrZeroValue() (_ V) {
	if me.Ok {
		return me.Value
	}
	return
}

func (me *Option[V]) UnwrapPtr() *V {
	if !me.Ok {
		panic("not set")
	}
	return &me.Value
}

func (me Option[V]) Unwrap() V {
	if !me.Ok {
		panic("not set")
	}
	return me.Value
}

// Deprecated: Use option.AndThen
func (me Option[V]) AndThen(f func(V) Option[V]) Option[V] {
	if me.Ok {
		return f(me.Value)
	}
	return me
}

func (me Option[V]) UnwrapOr(or V) V {
	if me.Ok {
		return me.Value
	} else {
		return or
	}
}

func (me *Option[V]) Set(v V) (prev Option[V]) {
	prev = *me
	me.Ok = true
	me.Value = v
	return
}

func (me *Option[V]) SetNone() {
	me.Ok = false
	me.Value = ZeroValue[V]()
}

func (me *Option[V]) SetFromTuple(v V, ok bool) {
	*me = OptionFromTuple(v, ok)
}

func (me *Option[V]) SetSomeZeroValue() {
	me.Ok = true
	me.Value = ZeroValue[V]()
}

func Some[V any](value V) Option[V] {
	return Option[V]{Ok: true, Value: value}
}

func None[V any]() Option[V] {
	return Option[V]{}
}

func OptionFromTuple[T any](t T, ok bool) Option[T] {
	if ok {
		return Some(t)
	} else {
		return None[T]()
	}
}

func (me Option[V]) String() string {
	if me.Ok {
		return fmt.Sprintf("Some(%v)", me.Value)
	} else {
		return "None"
	}
}

// Returns an Option that is the left Option if it's Some else the right Option.
func (me Option[V]) Or(or Option[V]) Option[V] {
	if me.Ok {
		return me
	}
	return or
}

// Converts the option to an option expressed as a pointer (old-school nonsense).
func (me Option[V]) ToPtr() *V {
	if me.Ok {
		return &me.Value
	}
	return nil
}

func (me Option[V]) AsTuple() (V, bool) {
	return me.Value, me.Ok
}

// Yields value for use as iter.Seq such as in range expression.
func (me Option[V]) Iter(yield func(V) bool) {
	if me.Ok {
		yield(me.Value)
	}
}

func (me Option[V]) Filter(f func(V) bool) Option[V] {
	if me.Ok && !f(me.Value) {
		me.SetNone()
	}
	return me
}
