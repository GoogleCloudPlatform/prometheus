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
import { Alert } from 'reactstrap';
import { GraphTabContent } from './GraphTabContent';

describe('GraphTabContent', () => {
  it('renders an alert if data result type is different than "matrix"', () => {
    const props: any = {
      data: { resultType: 'invalid', result: [{}] },
      stacked: false,
      queryParams: {
        startTime: 1572100210000,
        endTime: 1572100217898,
        resolution: 10,
      },
      color: 'danger',
      children: `Query result is of wrong type '`,
    };
    const graph = shallow(<GraphTabContent {...props} />);
    const alert = graph.find(Alert);
    expect(alert.prop('color')).toEqual(props.color);
    expect(alert.childAt(0).text()).toEqual(props.children);
  });

  it('renders an alert if data result empty', () => {
    const props: any = {
      data: {
        resultType: 'matrix',
        result: [],
      },
      color: 'secondary',
      children: 'Empty query result',
      stacked: false,
      queryParams: {
        startTime: 1572100210000,
        endTime: 1572100217898,
        resolution: 10,
      },
    };
    const graph = shallow(<GraphTabContent {...props} />);
    const alert = graph.find(Alert);
    expect(alert.prop('color')).toEqual(props.color);
    expect(alert.childAt(0).text()).toEqual(props.children);
  });
});
