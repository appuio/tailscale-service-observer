package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseNamespaces(t *testing.T) {
	tcases := []struct {
		raw      string
		expected []string
	}{
		{raw: "foo", expected: []string{"foo"}},
		{raw: "foo,", expected: []string{"foo"}},
		{raw: ", foo, ", expected: []string{"foo"}},
		{raw: "foo,bar", expected: []string{"foo", "bar"}},
		{raw: "foo, bar", expected: []string{"foo", "bar"}},
		{raw: "foo, bar, ", expected: []string{"foo", "bar"}},
	}

	for _, tc := range tcases {
		parsed := parseNamespaces(tc.raw)
		assert.Equal(t, tc.expected, parsed)
	}
}
