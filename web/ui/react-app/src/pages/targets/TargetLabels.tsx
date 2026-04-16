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

import React, { FC, Fragment, useState } from 'react';
import { Badge, Tooltip } from 'reactstrap';
import 'css.escape';
import styles from './TargetLabels.module.css';

interface Labels {
  [key: string]: string;
}

export interface TargetLabelsProps {
  discoveredLabels: Labels;
  labels: Labels;
  idx: number;
  scrapePool: string;
}

const formatLabels = (labels: Labels): string[] => Object.keys(labels).map((key) => `${key}="${labels[key]}"`);

const TargetLabels: FC<TargetLabelsProps> = ({ discoveredLabels, labels, idx, scrapePool }) => {
  const [tooltipOpen, setTooltipOpen] = useState(false);

  const toggle = (): void => setTooltipOpen(!tooltipOpen);
  const id = `series-labels-${scrapePool}-${idx}`;

  return (
    <>
      <div id={id} className="series-labels-container">
        {Object.keys(labels).map((labelName) => {
          return (
            <Badge color="primary" className="mr-1" key={labelName}>
              {`${labelName}="${labels[labelName]}"`}
            </Badge>
          );
        })}
      </div>
      <Tooltip
        isOpen={tooltipOpen}
        target={CSS.escape(id)}
        toggle={toggle}
        placement={'right-end'}
        style={{ maxWidth: 'none', textAlign: 'left' }}
      >
        <b>Before relabeling:</b>
        {formatLabels(discoveredLabels).map((s: string, labelIndex: number) => (
          <Fragment key={labelIndex}>
            <br />
            <span className={styles.discovered}>{s}</span>
          </Fragment>
        ))}
      </Tooltip>
    </>
  );
};

export default TargetLabels;
