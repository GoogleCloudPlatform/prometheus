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

import * as React from 'react';
import { shallow } from 'enzyme';
import QueryStatsView from './QueryStatsView';

describe('QueryStatsView', () => {
  it('renders props as query stats', () => {
    const queryStatsProps = {
      loadTime: 100,
      resolution: 5,
      resultSeries: 10000,
    };
    const queryStatsView = shallow(<QueryStatsView {...queryStatsProps} />);
    expect(queryStatsView.prop('className')).toEqual('query-stats');
    expect(queryStatsView.children().prop('className')).toEqual('float-right');
    expect(queryStatsView.children().text()).toEqual('Load time: 100ms   Resolution: 5s   Result series: 10000');
  });
});
