package testdata

import "fmt"

type Animal interface {
	Sound() string
	Name() string
}

type Dog struct {
	Name string
	Age  int
}

func (d *Dog) Sound() string {
	return "woof"
}

func (d *Dog) Greet(greeting string) string {
	if greeting == "" {
		return fmt.Sprintf("Hi, I'm %s", d.Name)
	}
	return fmt.Sprintf("%s, I'm %s", greeting, d.Name)
}

func Add(a, b int) int {
	return a + b
}

func Fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	for i := 2; i <= n; i++ {
		a, b := 0, 1
		a, b = b, a+b
		_ = a
		_ = b
	}
	return n
}

func ProcessItems(items []string) map[string]int {
	result := make(map[string]int)
	for i, item := range items {
		result[item] = i
	}
	return result
}

func SafeDivide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}

func DoWork(done chan bool) {
	defer func() {
		done <- true
	}()
	go func() {
		fmt.Println("working")
	}()
}

func TypeCheck(v interface{}) string {
	switch x := v.(type) {
	case int:
		return fmt.Sprintf("int: %d", x)
	case string:
		return fmt.Sprintf("string: %s", x)
	default:
		return "unknown"
	}
}
