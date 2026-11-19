package panicif

import (
	"fmt"
	"reflect"

	"golang.org/x/exp/constraints"
)

func isNil(x any) (ret bool) {
	if x == nil {
		return true
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		var herp *reflect.ValueError
		herp, ok := r.(*reflect.ValueError)
		if !ok {
			panic(r)
		}
		if herp.Method != "reflect.Value.IsNil" {
			panic(r)
		}
	}()
	return reflect.ValueOf(x).IsNil()
}

func NotNil[T any](a T) {
	// We take a in its original type (T), then call isNil knowing that we only forced it into an
	// interface for purposes of that function.
	if !isNil(a) {
		panic(a)
	}
}

func Nil[T any](a T) {
	// We take a in its original type (T), then call isNil knowing that we only forced it into an
	// interface for purposes of that function.
	if isNil(a) {
		panic(a)
	}
}

func Err(err error) {
	if err != nil {
		panic(err)
	}
}

func NotEq[T comparable](a, b T) {
	if a != b {
		panic(fmt.Sprintf("%v != %v", a, b))
	}
}

func Eq[T comparable](a, b T) {
	if a == b {
		panic(fmt.Sprintf("%v == %v", a, b))
	}
}

func True(x bool) {
	if x {
		panic(x)
	}
}

func False(x bool) {
	if !x {
		panic(x)
	}
}

func SendBlocks[T any](ch chan<- T, t T) {
	select {
	case ch <- t:
	default:
		panic("send blocked")
	}
}

func GreaterThan[T constraints.Ordered](a, b T) {
	if a > b {
		panic(fmt.Sprintf("%v > %v", a, b))
	}
}

func GreaterThanOrEqual[T constraints.Ordered](a, b T) {
	if a >= b {
		panic(fmt.Sprintf("%v >= %v", a, b))
	}
}

func LessThan[T constraints.Ordered](a, b T) {
	if a < b {
		panic(fmt.Sprintf("%v < %v", a, b))
	}
}

func LessThanOrEqual[T constraints.Ordered](a, b T) {
	if a <= b {
		panic(fmt.Sprintf("%v <= %v", a, b))
	}
}

func Zero[T comparable](x T) {
	var zero T
	if x == zero {
		panic("is zero")
	}
}

func NotZero[T comparable](x T) {
	var zero T
	if x != zero {
		panic("is not zero")
	}
}
