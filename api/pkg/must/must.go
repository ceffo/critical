// Package must turns "cannot fail" errors into panics.
//
// Panicking is acceptable only where an error is a programming mistake that
// should stop the program immediately: tests, package-level variables and
// constants initialised from literals, and one-off setup helpers. Never use it
// on input that comes from users, files or the network; return the error.
package must

// Do returns v, panicking if err is not nil. It is meant to wrap a call that
// returns (T, error): must.Do(formula.Parse("3+level")).
func Do[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
