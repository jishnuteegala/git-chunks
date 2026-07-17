package main

import "testing"

func files(sizes ...int64) []File {
	fs := make([]File, len(sizes))
	for i, s := range sizes {
		fs[i] = File{Path: string(rune('a' + i)), Size: s}
	}
	return fs
}

func lens(chunks [][]File) []int {
	out := make([]int, len(chunks))
	for i, c := range chunks {
		out[i] = len(c)
	}
	return out
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestChunkByMaxFiles(t *testing.T) {
	chunks := chunkFiles(files(1, 1, 1, 1, 1), 2, 0)
	if got := lens(chunks); !equal(got, []int{2, 2, 1}) {
		t.Fatalf("got %v, want [2 2 1]", got)
	}
}

func TestChunkByMaxSize(t *testing.T) {
	chunks := chunkFiles(files(40, 40, 40), 0, 100)
	if got := lens(chunks); !equal(got, []int{2, 1}) {
		t.Fatalf("got %v, want [2 1]", got)
	}
}

func TestOversizedFileGetsOwnChunk(t *testing.T) {
	chunks := chunkFiles(files(10, 500, 10), 0, 100)
	if got := lens(chunks); !equal(got, []int{1, 1, 1}) {
		t.Fatalf("got %v, want [1 1 1]", got)
	}
}

func TestCombinedCriteria(t *testing.T) {
	chunks := chunkFiles(files(60, 60, 1, 1, 1), 3, 100)
	if got := lens(chunks); !equal(got, []int{1, 3, 1}) {
		t.Fatalf("got %v, want [1 3 1]", got)
	}
}

func TestNoCriteriaSingleChunk(t *testing.T) {
	chunks := chunkFiles(files(1, 2, 3), 0, 0)
	if got := lens(chunks); !equal(got, []int{3}) {
		t.Fatalf("got %v, want [3]", got)
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"50M":     50 << 20,
		"50MB":    50 << 20,
		"500K":    500 << 10,
		"1G":      1 << 30,
		"1048576": 1048576,
		"1.5M":    1572864,
	}
	for input, want := range cases {
		var s Size
		if err := s.Set(input); err != nil {
			t.Fatalf("Set(%q): %v", input, err)
		}
		if int64(s) != want {
			t.Fatalf("Set(%q) = %d, want %d", input, int64(s), want)
		}
	}
	var s Size
	if err := s.Set("abc"); err == nil {
		t.Fatal("Set(abc) should fail")
	}
}
