package generics

type Result[T any] struct {
	Ok  T
	Err error
}

func Err[T any](err error) Result[T] {
	return Result[T]{
		Err: err,
	}
}

func ResultFromTuple[T any](t T, err error) Result[T] {
	return Result[T]{
		Ok:  t,
		Err: err,
	}
}

func (r Result[T]) AsTuple() (T, error) {
	return r.Ok, r.Err
}

func (r Result[T]) Unwrap() T {
	if r.Err != nil {
		panic(r.Err)
	}
	return r.Ok
}

func (r Result[T]) ToOption() Option[T] {
	return Option[T]{
		Ok:    r.Err == nil,
		Value: r.Ok,
	}
}

func (r *Result[T]) SetOk(ok T) {
	r.Ok = ok
	r.Err = nil
}

func (r *Result[T]) SetErr(err error) {
	SetZero(&r.Ok)
	r.Err = err
}

func (r *Result[T]) IsOk() bool {
	return r.Err == nil
}
