package missinggo

import "reflect"

// Deprecated: Use max builtin.
func Max(_less interface{}, vals ...interface{}) interface{} {
	ret := reflect.ValueOf(vals[0])
	retType := ret.Type()
	less := reflect.ValueOf(_less)
	for _, _v := range vals[1:] {
		v := reflect.ValueOf(_v).Convert(retType)
		out := less.Call([]reflect.Value{ret, v})
		if out[0].Bool() {
			ret = v
		}
	}
	return ret.Interface()
}

// Deprecated: Use max builtin.
func MaxInt(first int64, rest ...interface{}) int64 {
	return Max(func(l, r interface{}) bool {
		return l.(int64) < r.(int64)
	}, append([]interface{}{first}, rest...)...).(int64)
}

// Deprecated: Use min builtin.
func MinInt(first interface{}, rest ...interface{}) int64 {
	ret := reflect.ValueOf(first).Int()
	for _, _i := range rest {
		i := reflect.ValueOf(_i).Int()
		if i < ret {
			ret = i
		}
	}
	return ret
}
