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

package kingpin

import (
	"os"
	"regexp"
)

var (
	envVarValuesSeparator = "\r?\n"
	envVarValuesTrimmer   = regexp.MustCompile(envVarValuesSeparator + "$")
	envVarValuesSplitter  = regexp.MustCompile(envVarValuesSeparator)
)

type envarMixin struct {
	envar   string
	noEnvar bool
}

func (e *envarMixin) HasEnvarValue() bool {
	return e.GetEnvarValue() != ""
}

func (e *envarMixin) GetEnvarValue() string {
	if e.noEnvar || e.envar == "" {
		return ""
	}
	return os.Getenv(e.envar)
}

func (e *envarMixin) GetSplitEnvarValue() []string {
	envarValue := e.GetEnvarValue()
	if envarValue == "" {
		return []string{}
	}

	// Split by new line to extract multiple values, if any.
	trimmed := envVarValuesTrimmer.ReplaceAllString(envarValue, "")

	return envVarValuesSplitter.Split(trimmed, -1)
}
