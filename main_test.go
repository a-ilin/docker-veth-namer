// Copyright (C) 2026 Aleksei Ilin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Almost the same as the above, but this one is for single test instead of collection of tests
func setupSuite(testing.TB) func(testing.TB) {
	// Tear down.
	return func(testing.TB) {
		// Reset config to default.
		config = Config{}
	}
}

func TestReplaceMorphemes(t *testing.T) {
	teardownSuite := setupSuite(t)
	defer teardownSuite(t)

	morphConfig := []map[string]string{
		{"admin": "adm"},
		{"a": ""},
		{"o": ""},
	}
	config.Replacements = morphConfig

	testCases := []struct {
		name     string
		expected string
	}{
		{
			name:     "rndm",
			expected: "rndm",
		},
		{
			name:     "random",
			expected: "rndm",
		},
		{
			name:     "admin",
			expected: "adm",
		},
		{
			name:     "radmin",
			expected: "radm",
		},
		{
			name:     "admin123",
			expected: "adm123",
		},
		{
			name:     "radmin123",
			expected: "radm123",
		},
		{
			name:     "radminadmin123",
			expected: "radmadm123",
		},
		{
			name:     "a",
			expected: "",
		},
		{
			name:     "aaa",
			expected: "",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			_ = tc
			result := applyReplacements(tc.name)
			assert.Equal(t, tc.expected, result)
		})
	}
}
