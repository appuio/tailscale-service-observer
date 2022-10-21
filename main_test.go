package main

import (
	"testing"

	"github.com/appuio/tailscale-service-observer/tailscaleupdater"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
)

func Test_parseEnv(t *testing.T) {
	tcases := []struct {
		raw      string
		expected []string
	}{
		{raw: "", expected: []string{}},
		{raw: "foo", expected: []string{"foo"}},
		{raw: "foo,", expected: []string{"foo"}},
		{raw: ", foo, ", expected: []string{"foo"}}, {raw: "foo,bar", expected: []string{"foo", "bar"}},
		{raw: "foo, bar", expected: []string{"foo", "bar"}},
		{raw: "foo, bar, ", expected: []string{"foo", "bar"}},
	}

	for _, tc := range tcases {
		parsed := parseEnv(tc.raw)
		assert.Equal(t, tc.expected, parsed)
	}
}

func Test_advertiseAdditionalRoutes(t *testing.T) {
	l := testr.New(t)

	tcases := map[string]struct {
		raw      string
		expected map[string]struct{}
	}{
		"empty": {
			raw:      "",
			expected: map[string]struct{}{},
		},
		"ip": {
			raw: "198.51.100.1",
			expected: map[string]struct{}{
				"198.51.100.1/32": {},
			},
		},
		"multiple_ip": {
			raw: "198.51.100.1, 198.51.100.2",
			expected: map[string]struct{}{
				"198.51.100.1/32": {},
				"198.51.100.2/32": {},
			},
		},
		"prefix": {
			raw: "198.51.100.0/29",
			expected: map[string]struct{}{
				"198.51.100.0/29": {},
			},
		},
		"multiple_prefix": {
			raw: "198.51.100.0/29,198.51.100.128/29",
			expected: map[string]struct{}{
				"198.51.100.0/29":   {},
				"198.51.100.128/29": {},
			},
		},
		"mixed": {
			raw: "198.51.100.1,198.51.100.128/29",
			expected: map[string]struct{}{
				"198.51.100.1/32":   {},
				"198.51.100.128/29": {},
			},
		},
	}

	for _, tc := range tcases {
		tsUpdater := tailscaleupdater.NewUnchecked([]string{"test"}, "foobar", l)
		advertiseAdditionalRoutes(l, tsUpdater, tc.raw)
		assert.Equal(t, tsUpdater.GetRoutes(), tc.expected)
	}
}
