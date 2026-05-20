// Package typesdata provides fixed Go source fixtures for go/packages-based tests.
// This module has its own go.mod so go/packages can load it as a real package.
package typesdata

import "fmt"

// Animal is an interface with a single method.
type Animal interface {
	Sound() string
}

// Dog is a struct that implements Animal.
type Dog struct {
	Name string
}

// Sound implements Animal.
func (d *Dog) Sound() string {
	return fmt.Sprintf("%s says woof", d.Name)
}

// Cat implements Animal.
type Cat struct {
	Name string
}

// Sound implements Animal.
func (c *Cat) Sound() string {
	return fmt.Sprintf("%s says meow", c.Name)
}

// Helper is a plain function that calls Sound on an Animal.
func Helper(a Animal) string {
	return a.Sound()
}

// Add adds two integers.
func Add(x, y int) int {
	return x + y
}

// UseAdd calls Add internally.
func UseAdd() int {
	return Add(1, 2)
}
