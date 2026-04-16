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

export interface Labels {
  [key: string]: string;
}

export type Target = {
  discoveredLabels: Labels;
  labels: Labels;
  scrapePool: string;
  scrapeUrl: string;
  globalUrl: string;
  lastError: string;
  lastScrape: string;
  lastScrapeDuration: number;
  health: string;
  scrapeInterval: string;
  scrapeTimeout: string;
};

export interface DroppedTarget {
  discoveredLabels: Labels;
}

export interface ScrapePool {
  upCount: number;
  targets: Target[];
}

export interface ScrapePools {
  [scrapePool: string]: ScrapePool;
}

export const groupTargets = (targets: Target[]): ScrapePools =>
  targets.reduce((pools: ScrapePools, target: Target) => {
    const { health, scrapePool } = target;
    const up = health.toLowerCase() === 'up' ? 1 : 0;
    if (!pools[scrapePool]) {
      pools[scrapePool] = {
        upCount: 0,
        targets: [],
      };
    }
    pools[scrapePool].targets.push(target);
    pools[scrapePool].upCount += up;
    return pools;
  }, {});

export const getColor = (health: string): string => {
  switch (health.toLowerCase()) {
    case 'up':
      return 'success';
    case 'down':
      return 'danger';
    default:
      return 'warning';
  }
};

export interface TargetHealthFilters {
  healthy: boolean;
  unhealthy: boolean;
  unknown: boolean;
}

export const filterTargetsByHealth = (health: string, filters: TargetHealthFilters): boolean => {
  switch (health.toLowerCase()) {
    case 'up':
      return filters.healthy;
    case 'down':
      return filters.unhealthy;
    default:
      return filters.unknown;
  }
};
