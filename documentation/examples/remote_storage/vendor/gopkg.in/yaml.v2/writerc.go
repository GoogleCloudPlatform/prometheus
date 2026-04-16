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

package yaml

// Set the writer error and return false.
func yaml_emitter_set_writer_error(emitter *yaml_emitter_t, problem string) bool {
	emitter.error = yaml_WRITER_ERROR
	emitter.problem = problem
	return false
}

// Flush the output buffer.
func yaml_emitter_flush(emitter *yaml_emitter_t) bool {
	if emitter.write_handler == nil {
		panic("write handler not set")
	}

	// Check if the buffer is empty.
	if emitter.buffer_pos == 0 {
		return true
	}

	if err := emitter.write_handler(emitter, emitter.buffer[:emitter.buffer_pos]); err != nil {
		return yaml_emitter_set_writer_error(emitter, "write error: "+err.Error())
	}
	emitter.buffer_pos = 0
	return true
}
