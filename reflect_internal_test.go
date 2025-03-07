package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBoolPointerFromRequest(t *testing.T) {
	TRUE := true
	FALSE := false
	STRING := "hello"
	t.Parallel()
	tests := map[string]struct {
		obj any
		err string
		exp *bool
	}{
		"ExpectPointerError": {
			obj: struct{ Url string }{
				Url: "http://localhost:8080",
			},
			err: "expected pointer to struct { Url string }, got struct",
		},
		"ExpectPointerToStructError": {
			obj: &STRING,
			err: "expected pointer to struct, got pointer to string",
		},
		"MissingStreamField": {
			obj: &struct{ Url string }{
				Url: "http://localhost:8080",
			},
			err: "missing Stream field in struct { Url string }",
		},
		"ExpectedPointerToBool": {
			obj: &struct{ Stream bool }{
				Stream: true,
			},
			err: "expected *bool, got bool",
		},
		"ExpectedPointerToBoolString": {
			obj: &struct{ Stream *string }{
				Stream: &STRING,
			},
			err: "expected *bool, got *string",
		},
		"SuccessOnNilStream": {
			obj: &struct{ Stream *bool }{
				Stream: nil,
			},
		},
		"SuccessOnTrueStream": {
			obj: &struct{ Stream *bool }{
				Stream: &TRUE,
			},
			exp: &TRUE,
		},
		"SuccessOnFalseStream": {
			obj: &struct{ Stream *bool }{
				Stream: &FALSE,
			},
			exp: &FALSE,
		},
	}
	for name, tc := range tests {
		o := tc.obj
		e := tc.err
		exp := tc.exp
		t.Run(name, func(t *testing.T) {
			b, err := extractBoolPointerFromRequest(o)
			if e != "" {
				assert.EqualError(t, err, e)
			} else {
				assert.NoError(t, err)
			}
			if exp != nil {
				assert.Equal(t, *exp, *b)
			}
		})
	}
}
