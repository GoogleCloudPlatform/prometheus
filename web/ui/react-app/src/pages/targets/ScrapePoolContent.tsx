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
import { getColor, Target } from './target';
import { Badge, Table } from 'reactstrap';
import TargetLabels from './TargetLabels';
import styles from './ScrapePoolPanel.module.css';
import { formatRelative } from '../../utils';
import { now } from 'moment';
import TargetScrapeDuration from './TargetScrapeDuration';
import EndpointLink from './EndpointLink';
import CustomInfiniteScroll, { InfiniteScrollItemsProps } from '../../components/CustomInfiniteScroll';

const columns = ['Endpoint', 'State', 'Labels', 'Last Scrape', 'Scrape Duration', 'Error'];

interface ScrapePoolContentProps {
  targets: Target[];
}

const ScrapePoolContentTable: FC<InfiniteScrollItemsProps<Target>> = ({ items }) => {
  return (
    <Table className={styles.table} size="sm" bordered hover striped>
      <thead>
        <tr key="header">
          {columns.map((column) => (
            <th key={column}>{column}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {items.map((target, index) => (
          <tr key={index}>
            <td className={styles.endpoint}>
              <EndpointLink endpoint={target.scrapeUrl} globalUrl={target.globalUrl} />
            </td>
            <td className={styles.state}>
              <Badge color={getColor(target.health)}>{target.health.toUpperCase()}</Badge>
            </td>
            <td className={styles.labels}>
              <TargetLabels
                discoveredLabels={target.discoveredLabels}
                labels={target.labels}
                scrapePool={target.scrapePool}
                idx={index}
              />
            </td>
            <td className={styles['last-scrape']}>{formatRelative(target.lastScrape, now())}</td>
            <td className={styles['scrape-duration']}>
              <TargetScrapeDuration
                duration={target.lastScrapeDuration}
                scrapePool={target.scrapePool}
                idx={index}
                interval={target.scrapeInterval}
                timeout={target.scrapeTimeout}
              />
            </td>
            <td className={styles.errors}>
              {target.lastError ? <span className="text-danger">{target.lastError}</span> : null}
            </td>
          </tr>
        ))}
      </tbody>
    </Table>
  );
};

export const ScrapePoolContent: FC<ScrapePoolContentProps> = ({ targets }) => {
  return <CustomInfiniteScroll allItems={targets} child={ScrapePoolContentTable} />;
};
