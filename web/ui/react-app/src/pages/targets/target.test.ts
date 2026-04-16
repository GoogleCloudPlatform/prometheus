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

import { sampleApiResponse } from './__testdata__/testdata';
import { groupTargets, Target, ScrapePools, getColor } from './target';

describe('groupTargets', () => {
  const targets: Target[] = sampleApiResponse.data.activeTargets as Target[];
  const targetGroups: ScrapePools = groupTargets(targets);

  it('groups a list of targets by scrape job', () => {
    ['blackbox', 'prometheus/test', 'node_exporter'].forEach((scrapePool) => {
      expect(Object.keys(targetGroups)).toContain(scrapePool);
    });
    Object.keys(targetGroups).forEach((scrapePool: string): void => {
      const ts: Target[] = targetGroups[scrapePool].targets;
      ts.forEach((t: Target) => {
        expect(t.scrapePool).toEqual(scrapePool);
      });
    });
  });

  it('adds upCount during aggregation', () => {
    const testCases: { [key: string]: number } = { blackbox: 3, 'prometheus/test': 1, node_exporter: 1 };
    Object.keys(testCases).forEach((scrapePool: string): void => {
      expect(targetGroups[scrapePool].upCount).toEqual(testCases[scrapePool]);
    });
  });
});

describe('getColor', () => {
  const testCases: { color: string; status: string }[] = [
    { color: 'danger', status: 'down' },
    { color: 'danger', status: 'DOWN' },
    { color: 'warning', status: 'unknown' },
    { color: 'warning', status: 'foo' },
    { color: 'success', status: 'up' },
    { color: 'success', status: 'Up' },
  ];
  testCases.forEach(({ color, status }) => {
    it(`returns ${color} for ${status} status`, () => {
      expect(getColor(status)).toEqual(color);
    });
  });
});
