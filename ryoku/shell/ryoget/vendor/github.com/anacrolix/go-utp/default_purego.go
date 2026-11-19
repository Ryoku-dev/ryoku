//go:build !cgo || purego || purego_utp

package utp

// Default is the implementation this binary was built with: the pure Go one here, because the
// build set the purego or purego_utp tag, or disabled cgo. Otherwise it would be LibUtp. A program
// that wants to override the build's choice can assign to it before making any sockets.
var Default = PureGo
