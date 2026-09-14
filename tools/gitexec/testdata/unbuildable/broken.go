// Package unbuildable is a gofmt-clean package that does not compile, so the guard
// step can prove a build failure never reaches the checker and never prints a verdict.
package unbuildable

var _ = undefinedSymbol
