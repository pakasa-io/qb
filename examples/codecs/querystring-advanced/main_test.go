package main

import "testing"

func TestExample(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected main() to panic on unsupported include compilation")
		}
	}()
	main()
}
