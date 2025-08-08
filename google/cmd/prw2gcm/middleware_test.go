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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectPOSTMethodProjectID(t *testing.T) {
	var got string
	mux := http.NewServeMux()
	mux.Handle(pathPrefix, detectPOSTMethodProjectID(func(_ http.ResponseWriter, r *http.Request) {
		got = getProjectID(r.Context())
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("wrong path", func(t *testing.T) {
		u := fmt.Sprintf("%s/v1/NOT/my-project1/location/global/prometheus/api/v1/write", srv.URL)
		r, err := srv.Client().Post(u, "application/x-www-form-urlencoded", http.NoBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, r.StatusCode)
	})
	t.Run("wrong project id", func(t *testing.T) {
		u := fmt.Sprintf("%s/v1/projects/gs§fs§f1/location/global/prometheus/api/v1/write", srv.URL)
		r, err := srv.Client().Post(u, "application/x-www-form-urlencoded", http.NoBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, r.StatusCode)
	})
	t.Run("project id", func(t *testing.T) {
		u := fmt.Sprintf("%s/v1/projects/my-project1/location/global/prometheus/api/v1/write", srv.URL)
		r, err := srv.Client().Post(u, "application/x-www-form-urlencoded", http.NoBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, r.StatusCode)
		require.Equal(t, "my-project1", got)
	})
}
