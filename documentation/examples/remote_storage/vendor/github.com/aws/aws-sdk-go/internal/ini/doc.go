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

// Package ini is an LL(1) parser for configuration files.
//
//	Example:
//	sections, err := ini.OpenFile("/path/to/file")
//	if err != nil {
//		panic(err)
//	}
//
//	profile := "foo"
//	section, ok := sections.GetSection(profile)
//	if !ok {
//		fmt.Printf("section %q could not be found", profile)
//	}
//
// Below is the BNF that describes this parser
//  Grammar:
//  stmt -> section | stmt'
//  stmt' -> epsilon | expr
//  expr -> value (stmt)* | equal_expr (stmt)*
//  equal_expr -> value ( ':' | '=' ) equal_expr'
//  equal_expr' -> number | string | quoted_string
//  quoted_string -> " quoted_string'
//  quoted_string' -> string quoted_string_end
//  quoted_string_end -> "
//
//  section -> [ section'
//  section' -> section_value section_close
//  section_value -> number | string_subset | boolean | quoted_string_subset
//  quoted_string_subset -> " quoted_string_subset'
//  quoted_string_subset' -> string_subset quoted_string_end
//  quoted_string_subset -> "
//  section_close -> ]
//
//  value -> number | string_subset | boolean
//  string -> ? UTF-8 Code-Points except '\n' (U+000A) and '\r\n' (U+000D U+000A) ?
//  string_subset -> ? Code-points excepted by <string> grammar except ':' (U+003A), '=' (U+003D), '[' (U+005B), and ']' (U+005D) ?
//
//  SkipState will skip (NL WS)+
//
//  comment -> # comment' | ; comment'
//  comment' -> epsilon | value
package ini
