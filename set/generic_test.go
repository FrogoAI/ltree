//go:build unit
// +build unit

package set

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func FuzzGenericDataSetString(f *testing.F) {
	testcasesString := []string{"Hello, world", " ", "!12345"}
	for _, tc := range testcasesString {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, orig string) {
		d := NewGenericDataSet(orig)
		if !d.Contains(orig) {
			t.Errorf("Not exist: %q", orig)
		}
	})
}

func FuzzGenericDataSetInt(f *testing.F) {
	testcasesInt := []int{1, 2, 3}
	for _, tc := range testcasesInt {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, orig int) {
		d := NewGenericDataSet(orig)
		if !d.Contains(orig) {
			t.Errorf("Not exist: %q", orig)
		}
	})
}

func FuzzGenericDataSetFloat(f *testing.F) {
	testcasesFloat := []float64{0.1, 0.2, 0.3}
	for _, tc := range testcasesFloat {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, orig float64) {
		d := NewGenericDataSet(orig)
		if !d.Contains(orig) {
			t.Errorf("Not exist: %f", orig)
		}
	})
}

func TestGenericDataSet(t *testing.T) {
	stringTestCases := map[string]struct {
		keys             []string
		shouldContain    []string
		shouldnotContain []string
	}{
		"empty": {
			keys:             []string{},
			shouldContain:    []string{},
			shouldnotContain: []string{" ", "abc"},
		},
		"valid": {
			keys: []string{
				"one",
				"two",
				"three",
			},
			shouldContain:    []string{"one", "two", "three"},
			shouldnotContain: []string{"four", "", " ", ","},
		},
	}

	for exampleName, example := range stringTestCases {
		example := example
		t.Run(exampleName, func(t *testing.T) {
			set := NewGenericDataSet(example.keys...)

			if len(example.shouldContain) != set.Count() {
				t.Fatal("Invalid set length")
			}

			for _, key := range example.shouldContain {
				if !set.Contains(key) {
					t.Fatalf("expected set: %v to contain: %s", set, key)
				}
			}

			for _, key := range example.shouldnotContain {
				if set.Contains(key) {
					t.Fatalf("expected set: %v to not contain: %s", set, key)
				}
			}

			res := set.ToSlice()
			sort.Slice(example.keys, func(i, j int) bool {
				return strings.Compare(example.keys[i], example.keys[j]) <= 0
			})
			sort.Slice(res, func(i, j int) bool {
				return strings.Compare(res[i], res[j]) <= 0
			})

			if !reflect.DeepEqual(res, example.keys) {
				t.Fatalf("Unexpected result %v != %v", res, example.keys)
			}
		})
	}
}

func TestGenericOrderedDataSetStrings(t *testing.T) {
	testcases := map[string]struct {
		keys             []string
		shouldContain    []string
		shouldnotContain []string
	}{
		"empty": {
			keys:             []string{},
			shouldContain:    []string{},
			shouldnotContain: []string{" ", "abc"},
		},
		"valid": {
			keys: []string{
				"two",
				"one",
				"three",
			},
			shouldContain:    []string{"one", "two", "three"},
			shouldnotContain: []string{"four", "", " ", ","},
		},
	}

	for exampleName, example := range testcases {
		example := example
		t.Run(exampleName, func(t *testing.T) {
			set := NewGenericOrderedDataSet(example.keys...)

			if len(example.shouldContain) != set.Count() {
				t.Fatalf("Invalid set length: %d != %d", len(example.shouldContain), set.Count())
			}

			for _, key := range example.shouldContain {
				if !set.Contains(key) {
					t.Fatalf("expected set: %v to contain: %s", set, key)
				}
			}

			for _, key := range example.shouldnotContain {
				if set.Contains(key) {
					t.Fatalf("expected set: %v to not contain: %s", set, key)
				}
			}

			res := set.ToSlice()
			if !reflect.DeepEqual(res, example.keys) {
				t.Fatalf("Unexpected result %v != %v", res, example.keys)
			}
		})
	}
}

func TestGenericDataSet_Union(t *testing.T) {
	tt := []struct {
		name string
		set  GenericDataSet[string]
		u    GenericDataSet[string]
		want GenericDataSet[string]
	}{
		{
			name: "both are empty",
			set:  GenericDataSet[string]{},
			u:    GenericDataSet[string]{},
			want: GenericDataSet[string]{},
		},
		{
			name: "first not empty - second is empty",
			set:  NewGenericDataSet("a", "b", "c"),
			u:    GenericDataSet[string]{},
			want: NewGenericDataSet("a", "b", "c"),
		},
		{
			name: "a b c + d e f",
			set:  NewGenericDataSet("a", "b", "c"),
			u:    NewGenericDataSet("d", "e", "f"),
			want: NewGenericDataSet("a", "b", "c", "d", "e", "f"),
		},
		{
			name: "a b c + a b c",
			set:  NewGenericDataSet("a", "b", "c"),
			u:    NewGenericDataSet("a", "b", "c"),
			want: NewGenericDataSet("a", "b", "c"),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tc.set.Union(tc.u)
			if !reflect.DeepEqual(tc.set, tc.want) {
				t.Fatalf("Unexpected result %v != %v", tc.set, tc.want)
			}
		})
	}
}
