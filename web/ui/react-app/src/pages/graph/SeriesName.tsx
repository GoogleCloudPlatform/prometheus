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

import React, { FC, useContext } from 'react';
import { useToastContext } from '../../contexts/ToastContext';
import { metricToSeriesName } from '../../utils';

interface SeriesNameProps {
  labels: { [key: string]: string } | null;
  format: boolean;
}

const SeriesName: FC<SeriesNameProps> = ({ labels, format }) => {
  const setClipboardMsg = useToastContext();

  const toClipboard = (e: React.MouseEvent<HTMLSpanElement>) => {
    const copyText = e.currentTarget.innerText || '';
    navigator.clipboard
      .writeText(copyText.trim())
      .then(() => {
        setClipboardMsg(copyText);
      })
      .catch((reason) => {
        console.error(`unable to copy text: ${reason}`);
      });
  };

  const renderFormatted = (): React.ReactElement => {
    const labelNodes: React.ReactElement[] = [];
    let first = true;
    for (const label in labels) {
      if (label === '__name__') {
        continue;
      }

      labelNodes.push(
        <span key={label}>
          {!first && ', '}
          <span className="legend-label-container" onClick={toClipboard} title="Click to copy label matcher">
            <span className="legend-label-name">{label}</span>=<span className="legend-label-value">"{labels[label]}"</span>
          </span>
        </span>
      );

      if (first) {
        first = false;
      }
    }

    return (
      <div>
        <span className="legend-metric-name">{labels ? labels.__name__ : ''}</span>
        <span className="legend-label-brace">{'{'}</span>
        {labelNodes}
        <span className="legend-label-brace">{'}'}</span>
      </div>
    );
  };

  if (labels === null) {
    return <>scalar</>;
  }

  if (format) {
    return renderFormatted();
  }
  // Return a simple text node. This is much faster to scroll through
  // for longer lists (hundreds of items).
  return <>{metricToSeriesName(labels)}</>;
};

export default SeriesName;
