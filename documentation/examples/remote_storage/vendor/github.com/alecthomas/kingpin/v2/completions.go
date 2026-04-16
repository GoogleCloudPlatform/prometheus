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

// HintAction is a function type who is expected to return a slice of possible
// command line arguments.
type HintAction func() []string
type completionsMixin struct {
	hintActions        []HintAction
	builtinHintActions []HintAction
}

func (a *completionsMixin) addHintAction(action HintAction) {
	a.hintActions = append(a.hintActions, action)
}

// Allow adding of HintActions which are added internally, ie, EnumVar
func (a *completionsMixin) addHintActionBuiltin(action HintAction) {
	a.builtinHintActions = append(a.builtinHintActions, action)
}

func (a *completionsMixin) resolveCompletions() []string {
	var hints []string

	options := a.builtinHintActions
	if len(a.hintActions) > 0 {
		// User specified their own hintActions. Use those instead.
		options = a.hintActions
	}

	for _, hintAction := range options {
		hints = append(hints, hintAction()...)
	}
	return hints
}
