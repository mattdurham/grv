package testdata

import "fmt"

func Producer(ch chan<- int, count int) {
	for i := 0; i < count; i++ {
		ch <- i
	}
	close(ch)
}

func Consumer(ch <-chan int) int {
	sum := 0
	for v := range ch {
		sum += v
	}
	return sum
}

func Multiplexer(ch1, ch2 <-chan int, out chan<- int, done chan bool) {
	for {
		select {
		case v, ok := <-ch1:
			if !ok {
				return
			}
			out <- v
		case v, ok := <-ch2:
			if !ok {
				return
			}
			out <- v
		case <-done:
			return
		}
	}
}

func Timeout(ch <-chan int, timeout <-chan bool) (int, bool) {
	select {
	case v := <-ch:
		return v, true
	case <-timeout:
		return 0, false
	default:
		fmt.Println("no value ready")
		return 0, false
	}
}

func Pipeline(input []int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, v := range input {
			out <- v * 2
		}
	}()
	return out
}
