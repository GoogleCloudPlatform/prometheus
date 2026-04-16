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

import React, { FC, useState } from 'react';
import { Badge, Table } from 'reactstrap';
import { TargetLabels } from './Services';
import { ToggleMoreLess } from '../../components/ToggleMoreLess';
import CustomInfiniteScroll, { InfiniteScrollItemsProps } from '../../components/CustomInfiniteScroll';

interface LabelProps {
  value: TargetLabels[];
  name: string;
}

const formatLabels = (labels: Record<string, string> | string) => {
  return Object.entries(labels).map(([key, value]) => {
    return (
      <div key={key}>
        <Badge color="primary" className="mr-1">
          {`${key}="${value}"`}
        </Badge>
      </div>
    );
  });
};

const LabelsTableContent: FC<InfiniteScrollItemsProps<TargetLabels>> = ({ items }) => {
  return (
    <Table size="sm" bordered hover striped>
      <thead>
        <tr>
          <th>Discovered Labels</th>
          <th>Target Labels</th>
        </tr>
      </thead>
      <tbody>
        {items.map((_, i) => {
          return (
            <tr key={i}>
              <td>{formatLabels(items[i].discoveredLabels)}</td>
              {items[i].isDropped ? (
                <td style={{ fontWeight: 'bold' }}>Dropped</td>
              ) : (
                <td>{formatLabels(items[i].labels)}</td>
              )}
            </tr>
          );
        })}
      </tbody>
    </Table>
  );
};

export const LabelsTable: FC<LabelProps> = ({ value, name }) => {
  const [showMore, setShowMore] = useState(false);

  return (
    <>
      <div>
        <ToggleMoreLess
          event={(): void => {
            setShowMore(!showMore);
          }}
          showMore={showMore}
        >
          <span className="target-head">{name}</span>
        </ToggleMoreLess>
      </div>
      {showMore ? <CustomInfiniteScroll allItems={value} child={LabelsTableContent} /> : null}
    </>
  );
};
