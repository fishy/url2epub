package main

import (
	"strconv"
	"testing"
)

func TestPrettySize(t *testing.T) {
	for _, c := range []struct {
		size     int
		expected string
	}{
		{
			size:     128,
			expected: "128 B",
		},
		{
			size:     1024,
			expected: "1.0 KiB",
		},
		{
			size:     1000,
			expected: "1.0 KiB",
		},
		{
			size:     1024 * 1024,
			expected: "1.0 MiB",
		},
		{
			size:     1024*1024 - 1,
			expected: "1.0 MiB",
		},
		{
			size:     940 * 1024,
			expected: "940.0 KiB",
		},
		{
			size:     1000 * 1000 * 1000,
			expected: "953.7 MiB",
		},
	} {
		t.Run(strconv.FormatInt(int64(c.size), 10), func(t *testing.T) {
			s := prettySize(c.size)
			if s != c.expected {
				t.Errorf("prettySize(%d) expected %q, got %q", c.size, c.expected, s)
			}
		})
	}
}
