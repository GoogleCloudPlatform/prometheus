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
import { Progress, Alert } from 'reactstrap';

import { useFetchReadyInterval } from '../hooks/useFetch';
import { WALReplayData } from '../types/types';
import { usePathPrefix } from '../contexts/PathPrefixContext';
import { useReady } from '../contexts/ReadyContext';

interface StartingContentProps {
  isUnexpected: boolean;
  status?: WALReplayData;
}

export const StartingContent: FC<StartingContentProps> = ({ status, isUnexpected }) => {
  if (isUnexpected) {
    return (
      <Alert color="danger">
        <strong>Error:</strong> Server is not responding
      </Alert>
    );
  }

  return (
    <div className="text-center m-3">
      <div className="m-4">
        <h2>Starting up...</h2>
        {status && status.max > 0 ? (
          <div>
            <p>
              Replaying WAL ({status.current}/{status.max})
            </p>
            <Progress
              animated
              value={status.current - status.min + 1}
              min={status.min}
              max={status.max - status.min + 1}
              color={status.max === status.current ? 'success' : undefined}
              style={{ width: '10%', margin: 'auto' }}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
};

export const withStartingIndicator =
  <T extends Record<string, unknown>>(Page: ComponentType<T>): FC<T> =>
  ({ ...rest }) => {
    const pathPrefix = usePathPrefix();
    const { ready, walReplayStatus, isUnexpected } = useFetchReadyInterval(pathPrefix);
    const staticReady = useReady();

    if (staticReady || ready || isUnexpected) {
      return <Page {...(rest as T)} />;
    }

    return <StartingContent isUnexpected={isUnexpected} status={walReplayStatus.data} />;
  };
