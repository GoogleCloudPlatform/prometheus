// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package require

// TestingT is an interface wrapper around *testing.T
type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
}

type tHelper interface {
	Helper()
}

// ComparisonAssertionFunc is a common function prototype when comparing two values.  Can be useful
// for table driven tests.
type ComparisonAssertionFunc func(TestingT, interface{}, interface{}, ...interface{})

// ValueAssertionFunc is a common function prototype when validating a single value.  Can be useful
// for table driven tests.
type ValueAssertionFunc func(TestingT, interface{}, ...interface{})

// BoolAssertionFunc is a common function prototype when validating a bool value.  Can be useful
// for table driven tests.
type BoolAssertionFunc func(TestingT, bool, ...interface{})

// ErrorAssertionFunc is a common function prototype when validating an error value.  Can be useful
// for table driven tests.
type ErrorAssertionFunc func(TestingT, error, ...interface{})

//go:generate sh -c "cd ../_codegen && go build && cd - && ../_codegen/_codegen -output-package=require -template=require.go.tmpl -include-format-funcs"
