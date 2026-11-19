package option

import (
	"github.com/anacrolix/generics"
)

func Map[T, U any](f func(from T) (to U), in generics.Option[T]) (out generics.Option[U]) {
	if in.Ok {
		out = generics.Some(f(in.Value))
	}
	return
}

func AndThen[T, U any](in generics.Option[T], f func(in T) (out generics.Option[U])) (out generics.Option[U]) {
	if in.Ok {
		out = f(in.Value)
	}
	return
}

func FromPtr[T any](in *T) (_ generics.Option[T]) {
	if in != nil {
		return generics.Some(*in)
	}
	return
}
