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

// Action callback executed at various stages after all values are populated.
// The application, commands, arguments and flags all have corresponding
// actions.
type Action func(*ParseContext) error

type actionMixin struct {
	actions    []Action
	preActions []Action
}

type actionApplier interface {
	applyActions(*ParseContext) error
	applyPreActions(*ParseContext) error
}

func (a *actionMixin) addAction(action Action) {
	a.actions = append(a.actions, action)
}

func (a *actionMixin) addPreAction(action Action) {
	a.preActions = append(a.preActions, action)
}

func (a *actionMixin) applyActions(context *ParseContext) error {
	for _, action := range a.actions {
		if err := action(context); err != nil {
			return err
		}
	}
	return nil
}

func (a *actionMixin) applyPreActions(context *ParseContext) error {
	for _, preAction := range a.preActions {
		if err := preAction(context); err != nil {
			return err
		}
	}
	return nil
}
