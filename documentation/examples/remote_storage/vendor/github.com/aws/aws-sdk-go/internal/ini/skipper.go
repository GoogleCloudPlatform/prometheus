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

package ini

// skipper is used to skip certain blocks of an ini file.
// Currently skipper is used to skip nested blocks of ini
// files. See example below
//
//	[ foo ]
//	nested = ; this section will be skipped
//		a=b
//		c=d
//	bar=baz ; this will be included
type skipper struct {
	shouldSkip bool
	TokenSet   bool
	prevTok    Token
}

func newSkipper() skipper {
	return skipper{
		prevTok: emptyToken,
	}
}

func (s *skipper) ShouldSkip(tok Token) bool {
	// should skip state will be modified only if previous token was new line (NL);
	// and the current token is not WhiteSpace (WS).
	if s.shouldSkip &&
		s.prevTok.Type() == TokenNL &&
		tok.Type() != TokenWS {
		s.Continue()
		return false
	}
	s.prevTok = tok
	return s.shouldSkip
}

func (s *skipper) Skip() {
	s.shouldSkip = true
}

func (s *skipper) Continue() {
	s.shouldSkip = false
	// empty token is assigned as we return to default state, when should skip is false
	s.prevTok = emptyToken
}
