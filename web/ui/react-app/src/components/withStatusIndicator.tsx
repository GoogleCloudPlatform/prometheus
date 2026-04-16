/**
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { FC, ComponentType } from 'react';
import { Alert } from 'reactstrap';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faSpinner } from '@fortawesome/free-solid-svg-icons';

interface StatusIndicatorProps {
  error?: Error;
  isLoading?: boolean;
  customErrorMsg?: JSX.Element;
  componentTitle?: string;
}

export const withStatusIndicator =
  <T extends Record<string, any>>( // eslint-disable-line @typescript-eslint/no-explicit-any
    Component: ComponentType<T>
  ): FC<StatusIndicatorProps & T> =>
  ({ error, isLoading, customErrorMsg, componentTitle, ...rest }) => {
    if (error) {
      return (
        <Alert color="danger">
          {customErrorMsg ? (
            customErrorMsg
          ) : (
            <>
              <strong>Error:</strong> Error fetching {componentTitle || Component.displayName}: {error.message}
            </>
          )}
        </Alert>
      );
    }

    if (isLoading) {
      return (
        <FontAwesomeIcon
          size="3x"
          icon={faSpinner}
          spin
          className="position-absolute"
          style={{ transform: 'translate(-50%, -50%)', top: '50%', left: '50%' }}
        />
      );
    }
    return <Component {...(rest as T)} />;
  };
