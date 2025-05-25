package streaming

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_frameJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "append newline when missing",
			input:    []byte("{\"k\":\"v\"}"),
			expected: []byte("{\"k\":\"v\"}\n"),
		},
		{
			name:     "preserve when newline present",
			input:    []byte("{\"k\":1}\n"),
			expected: []byte("{\"k\":1}\n"),
		},
		{
			name:     "empty payload",
			input:    []byte(``),
			expected: []byte("\n"),
		},
	}

	for _, tc := range testCases {
		actual := frameJSON(tc.input)
		assert.EqualValues(t, tc.expected, actual, tc.name)
	}
}
