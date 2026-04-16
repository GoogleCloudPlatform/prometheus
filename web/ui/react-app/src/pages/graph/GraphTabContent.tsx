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

import React, { FC } from 'react';
import { Alert } from 'reactstrap';
import Graph from './Graph';
import { QueryParams, ExemplarData } from '../../types/types';
import { isPresent } from '../../utils';

interface GraphTabContentProps {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  data: any;
  exemplars: ExemplarData;
  stacked: boolean;
  useLocalTime: boolean;
  showExemplars: boolean;
  handleTimeRangeSelection: (startTime: number, endTime: number) => void;
  lastQueryParams: QueryParams | null;
  id: string;
}

export const GraphTabContent: FC<GraphTabContentProps> = ({
  data,
  exemplars,
  stacked,
  useLocalTime,
  lastQueryParams,
  showExemplars,
  handleTimeRangeSelection,
  id,
}) => {
  if (!isPresent(data)) {
    return <Alert color="light">No data queried yet</Alert>;
  }
  if (data.result.length === 0) {
    return <Alert color="secondary">Empty query result</Alert>;
  }
  if (data.resultType !== 'matrix') {
    return (
      <Alert color="danger">Query result is of wrong type '{data.resultType}', should be 'matrix' (range vector).</Alert>
    );
  }
  return (
    <Graph
      data={data}
      exemplars={exemplars}
      stacked={stacked}
      useLocalTime={useLocalTime}
      showExemplars={showExemplars}
      handleTimeRangeSelection={handleTimeRangeSelection}
      queryParams={lastQueryParams}
      id={id}
    />
  );
};
