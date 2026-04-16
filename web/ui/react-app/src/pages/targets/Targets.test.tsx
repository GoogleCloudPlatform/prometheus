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

import React from 'react';
import { shallow, mount, ReactWrapper } from 'enzyme';
import { act } from 'react-dom/test-utils';
import Targets from './Targets';
import ScrapePoolList from './ScrapePoolList';
import { FetchMock } from 'jest-fetch-mock/types';
import { scrapePoolsSampleAPI } from './__testdata__/testdata';

describe('Targets', () => {
  beforeEach(() => {
    fetchMock.resetMocks();
  });

  let targets: ReactWrapper;
  let mock: FetchMock;

  describe('Header', () => {
    const targets = shallow(<Targets />);
    const h2 = targets.find('h2');
    it('renders a header', () => {
      expect(h2.text()).toEqual('Targets');
    });
    it('renders exactly one header', () => {
      const h2 = targets.find('h2');
      expect(h2).toHaveLength(1);
    });
  });

  it('renders a scrape pool list', async () => {
    mock = fetchMock.mockResponseOnce(JSON.stringify(scrapePoolsSampleAPI));
    await act(async () => {
      targets = mount(<Targets />);
    });
    expect(mock).toHaveBeenCalledWith('/api/v1/scrape_pools', {
      cache: 'no-store',
      credentials: 'same-origin',
    });
    targets.update();

    const scrapePoolList = targets.find(ScrapePoolList);
    expect(scrapePoolList).toHaveLength(1);
  });
});
