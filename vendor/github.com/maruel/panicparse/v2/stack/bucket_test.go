// Copyright 2018 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stack

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/maruel/panicparse/v2/internal/internaltest"
)

func TestAggregateNotAggressive(t *testing.T) {
	t.Parallel()
	// 2 goroutines with similar but not exact same signature.
	data := []string{
		"panic: runtime error: index out of range",
		"",
		"goroutine 6 [chan receive]:",
		"main.func·001(0x11000000, 2)",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"",
		"goroutine 7 [chan receive]:",
		"main.func·001(0x21000000, 2)",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"",
	}
	s, suffix, err := ScanSnapshot(bytes.NewBufferString(strings.Join(data, "\n")), ioutil.Discard, defaultOpts())
	if err != io.EOF {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected snapshot")
	}
	want := []*Bucket{
		{
			Signature: Signature{
				State: "chan receive",
				Stack: Stack{
					Calls: []Call{
						newCall(
							"main.func·001",
							Args{Values: []Arg{{Value: 0x11000000, IsPtr: true}, {Value: 2}}},
							"/gopath/src/github.com/maruel/panicparse/stack/stack.go",
							72),
					},
				},
			},
			IDs:   []int{6},
			First: true,
		},
		{
			Signature: Signature{
				State: "chan receive",
				Stack: Stack{
					Calls: []Call{
						newCall(
							"main.func·001",
							Args{Values: []Arg{{Value: 0x21000000, Name: "#1", IsPtr: true}, {Value: 2}}},
							"/gopath/src/github.com/maruel/panicparse/stack/stack.go",
							72),
					},
				},
			},
			IDs: []int{7},
		},
	}
	a := s.Aggregate(ExactLines)
	compareBuckets(t, want, a.Buckets)
	if a.Snapshot != s {
		t.Fatal("unexpected snapshot")
	}
	compareString(t, "", string(suffix))
}

func TestAggregateExactMatching(t *testing.T) {
	t.Parallel()
	// 2 goroutines with the exact same signature.
	data := []string{
		"panic: runtime error: index out of range",
		"",
		"goroutine 6 [chan receive]:",
		"main.func·001()",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"created by main.mainImpl",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:74 +0xeb",
		"",
		"goroutine 7 [chan receive]:",
		"main.func·001()",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"created by main.mainImpl",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:74 +0xeb",
		"",
	}
	s, suffix, err := ScanSnapshot(bytes.NewBufferString(strings.Join(data, "\n")), ioutil.Discard, defaultOpts())
	if err != io.EOF {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected snapshot")
	}
	want := []*Bucket{
		{
			Signature: Signature{
				State: "chan receive",
				CreatedBy: Stack{
					Calls: []Call{
						newCall(
							"main.mainImpl",
							Args{},
							"/gopath/src/github.com/maruel/panicparse/stack/stack.go",
							74),
					},
				},
				Stack: Stack{
					Calls: []Call{
						newCall(
							"main.func·001",
							Args{},
							"/gopath/src/github.com/maruel/panicparse/stack/stack.go",
							72),
					},
				},
			},
			IDs:   []int{6, 7},
			First: true,
		},
	}
	compareBuckets(t, want, s.Aggregate(ExactLines).Buckets)
	compareString(t, "", string(suffix))
}

func TestAggregateAggressive(t *testing.T) {
	t.Parallel()
	// 3 goroutines with similar signatures.
	data := []string{
		"panic: runtime error: index out of range",
		"",
		"goroutine 6 [chan receive, 10 minutes]:",
		"main.func·001(0x21000000, 2)",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"",
		"goroutine 7 [chan receive, 50 minutes]:",
		"main.func·001(0x31000000, 2)",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"",
		"goroutine 8 [chan receive, 100 minutes]:",
		"main.func·001(0x41000000, 2)",
		"\t/gopath/src/github.com/maruel/panicparse/stack/stack.go:72 +0x49",
		"",
	}
	s, suffix, err := ScanSnapshot(bytes.NewBufferString(strings.Join(data, "\n")), ioutil.Discard, defaultOpts())
	if err != io.EOF {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected snapshot")
	}
	want := []*Bucket{
		{
			Signature: Signature{
				State:    "chan receive",
				SleepMin: 10,
				SleepMax: 100,
				Stack: Stack{
					Calls: []Call{
						newCall(
							"main.func·001",
							Args{Values: []Arg{{Value: 0x21000000, Name: "*", IsPtr: true}, {Value: 2}}},
							"/gopath/src/github.com/maruel/panicparse/stack/stack.go",
							72),
					},
				},
			},
			IDs:   []int{6, 7, 8},
			First: true,
		},
	}
	compareBuckets(t, want, s.Aggregate(AnyPointer).Buckets)
	compareString(t, "", string(suffix))
}

func BenchmarkAggregate(b *testing.B) {
	b.ReportAllocs()
	s, suffix, err := ScanSnapshot(bytes.NewReader(internaltest.StaticPanicwebOutput()), ioutil.Discard, defaultOpts())
	if err != io.EOF {
		b.Fatal(err)
	}
	if s == nil {
		b.Fatal("missing context")
	}
	if string(suffix) != "" {
		b.Fatalf("unexpected suffix: %q", string(suffix))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buckets := s.Aggregate(AnyPointer).Buckets
		if len(buckets) < 5 {
			b.Fatal("expected more buckets")
		}
	}
}

func compareBuckets(t *testing.T, want, got []*Bucket) {
	helper(t)()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Bucket mismatch (-want +got):\n%s", diff)
	}
}
