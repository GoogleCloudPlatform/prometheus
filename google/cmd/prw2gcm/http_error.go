// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ error = httpError{}

type httpError struct {
	error
	code int
}

func (e httpError) HTTPCode() int {
	if e.error == nil || e.code == 0 {
		return http.StatusInternalServerError
	}
	return e.code
}

func newHTTPError(code int, str string) error {
	return &httpError{
		error: errors.New(str), code: code,
	}
}

func newHTTPErrorf(code int, str string, v ...any) error {
	return &httpError{
		error: fmt.Errorf(str, v...), code: code,
	}
}

func newHTTPErrorFromGRPC(err error) error {
	code := http.StatusInternalServerError
	// TODO: Add support for more codes, especially those that shouldn't be retried.
	switch status.Code(err) {
	case codes.OK:
		return nil
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	default:
	}
	return &httpError{
		error: err, code: code,
	}
}

func httpCodeFromError(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	type httpCode interface{ HTTPCode() int }

	if hc, ok := err.(httpCode); ok {
		return hc.HTTPCode(), true
	}
	var hc httpCode
	if errors.As(err, &hc) {
		return hc.HTTPCode(), true
	}
	return 0, false
}

func httpCodeFromErrorOr500(err error) int {
	if c, ok := httpCodeFromError(err); ok {
		return c
	}
	return http.StatusInternalServerError
}

// TODO: There's likely a way to aggregate HTTP codes on read time with errors.Join.
func httpErrJoin(e ...error) error {
	var (
		retCode int
		retErr  error
	)
	for _, err := range e {
		if err == nil {
			continue
		}
		if c, ok := httpCodeFromError(err); ok {
			if retCode < c {
				retCode = c
			}
		}
		retErr = errors.Join(retErr, err)
	}
	if retErr == nil {
		return nil
	}
	if retCode > 0 {
		return newHTTPError(retCode, retErr.Error())
	}
	return retErr
}
