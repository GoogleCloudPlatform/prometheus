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
	"path/filepath"
)

var (
	// CommandLine is the default Kingpin parser.
	CommandLine = New(filepath.Base(os.Args[0]), "")
	// Global help flag. Exposed for user customisation.
	HelpFlag = CommandLine.HelpFlag
	// Top-level help command. Exposed for user customisation. May be nil.
	HelpCommand = CommandLine.HelpCommand
	// Global version flag. Exposed for user customisation. May be nil.
	VersionFlag = CommandLine.VersionFlag
	// Whether to file expansion with '@' is enabled.
	EnableFileExpansion = true
)

// Command adds a new command to the default parser.
func Command(name, help string) *CmdClause {
	return CommandLine.Command(name, help)
}

// Flag adds a new flag to the default parser.
func Flag(name, help string) *FlagClause {
	return CommandLine.Flag(name, help)
}

// Arg adds a new argument to the top-level of the default parser.
func Arg(name, help string) *ArgClause {
	return CommandLine.Arg(name, help)
}

// Parse and return the selected command. Will call the termination handler if
// an error is encountered.
func Parse() string {
	selected := MustParse(CommandLine.Parse(os.Args[1:]))
	if selected == "" && CommandLine.cmdGroup.have() {
		Usage()
		CommandLine.terminate(0)
	}
	return selected
}

// Errorf prints an error message to stderr.
func Errorf(format string, args ...interface{}) {
	CommandLine.Errorf(format, args...)
}

// Fatalf prints an error message to stderr and exits.
func Fatalf(format string, args ...interface{}) {
	CommandLine.Fatalf(format, args...)
}

// FatalIfError prints an error and exits if err is not nil. The error is printed
// with the given prefix.
func FatalIfError(err error, format string, args ...interface{}) {
	CommandLine.FatalIfError(err, format, args...)
}

// FatalUsage prints an error message followed by usage information, then
// exits with a non-zero status.
func FatalUsage(format string, args ...interface{}) {
	CommandLine.FatalUsage(format, args...)
}

// FatalUsageContext writes a printf formatted error message to stderr, then
// usage information for the given ParseContext, before exiting.
func FatalUsageContext(context *ParseContext, format string, args ...interface{}) {
	CommandLine.FatalUsageContext(context, format, args...)
}

// Usage prints usage to stderr.
func Usage() {
	CommandLine.Usage(os.Args[1:])
}

// Set global usage template to use (defaults to DefaultUsageTemplate).
func UsageTemplate(template string) *Application {
	return CommandLine.UsageTemplate(template)
}

// MustParse can be used with app.Parse(args) to exit with an error if parsing fails.
func MustParse(command string, err error) string {
	if err != nil {
		Fatalf("%s, try --help", err)
	}
	return command
}

// Version adds a flag for displaying the application version number.
func Version(version string) *Application {
	return CommandLine.Version(version)
}
