package main

import "testing"

func TestSplitCSV(t *testing.T) {
	cases := []struct{ in string; want []string }{
		{"a,b,c", []string{"a","b","c"}},
		{" a , b , c ", []string{"a","b","c"}},
		{"a,,c", []string{"a","c"}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitCSV(c.in)
		if len(got) != len(c.want) { t.Fatalf("%q -> %v, want %v", c.in, got, c.want) }
		for i := range got {
			if got[i] != c.want[i] { t.Fatalf("%q -> %v, want %v", c.in, got, c.want) }
		}
	}
}
