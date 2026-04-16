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

package types

func (m *Any) ProtoSize() (n int)               { return m.Size() }
func (m *Api) ProtoSize() (n int)               { return m.Size() }
func (m *Method) ProtoSize() (n int)            { return m.Size() }
func (m *Mixin) ProtoSize() (n int)             { return m.Size() }
func (m *Duration) ProtoSize() (n int)          { return m.Size() }
func (m *Empty) ProtoSize() (n int)             { return m.Size() }
func (m *FieldMask) ProtoSize() (n int)         { return m.Size() }
func (m *SourceContext) ProtoSize() (n int)     { return m.Size() }
func (m *Struct) ProtoSize() (n int)            { return m.Size() }
func (m *Value) ProtoSize() (n int)             { return m.Size() }
func (m *Value_NullValue) ProtoSize() (n int)   { return m.Size() }
func (m *Value_NumberValue) ProtoSize() (n int) { return m.Size() }
func (m *Value_StringValue) ProtoSize() (n int) { return m.Size() }
func (m *Value_BoolValue) ProtoSize() (n int)   { return m.Size() }
func (m *Value_StructValue) ProtoSize() (n int) { return m.Size() }
func (m *Value_ListValue) ProtoSize() (n int)   { return m.Size() }
func (m *ListValue) ProtoSize() (n int)         { return m.Size() }
func (m *Timestamp) ProtoSize() (n int)         { return m.Size() }
func (m *Type) ProtoSize() (n int)              { return m.Size() }
func (m *Field) ProtoSize() (n int)             { return m.Size() }
func (m *Enum) ProtoSize() (n int)              { return m.Size() }
func (m *EnumValue) ProtoSize() (n int)         { return m.Size() }
func (m *Option) ProtoSize() (n int)            { return m.Size() }
func (m *DoubleValue) ProtoSize() (n int)       { return m.Size() }
func (m *FloatValue) ProtoSize() (n int)        { return m.Size() }
func (m *Int64Value) ProtoSize() (n int)        { return m.Size() }
func (m *UInt64Value) ProtoSize() (n int)       { return m.Size() }
func (m *Int32Value) ProtoSize() (n int)        { return m.Size() }
func (m *UInt32Value) ProtoSize() (n int)       { return m.Size() }
func (m *BoolValue) ProtoSize() (n int)         { return m.Size() }
func (m *StringValue) ProtoSize() (n int)       { return m.Size() }
func (m *BytesValue) ProtoSize() (n int)        { return m.Size() }
