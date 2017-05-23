package main

import "github.com/AsynkronIT/goring"

func main() {
	q := goring.New(10)

	go func() {
		for range []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1} {
			q.Push(1)
		}
	}()

	for range []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1} {
		q.Length()
	}
}
