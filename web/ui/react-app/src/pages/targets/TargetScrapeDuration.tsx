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
import { Tooltip } from 'reactstrap';
import 'css.escape';
import { humanizeDuration } from '../../utils';

export interface TargetScrapeDurationProps {
  duration: number;
  interval: string;
  timeout: string;
  idx: number;
  scrapePool: string;
}

const TargetScrapeDuration: FC<TargetScrapeDurationProps> = ({ duration, interval, timeout, idx, scrapePool }) => {
  const [scrapeTooltipOpen, setScrapeTooltipOpen] = useState<boolean>(false);
  const id = `scrape-duration-${scrapePool}-${idx}`;

  return (
    <>
      <div id={id} className="scrape-duration-container">
        {humanizeDuration(duration * 1000)}
      </div>
      <Tooltip
        isOpen={scrapeTooltipOpen}
        toggle={() => setScrapeTooltipOpen(!scrapeTooltipOpen)}
        target={CSS.escape(id)}
        placement={'right-end'}
        style={{ maxWidth: 'none', textAlign: 'left' }}
      >
        <Fragment>
          <span>Interval: {interval}</span>
          <br />
        </Fragment>
        <Fragment>
          <span>Timeout: {timeout}</span>
        </Fragment>
      </Tooltip>
    </>
  );
};

export default TargetScrapeDuration;
