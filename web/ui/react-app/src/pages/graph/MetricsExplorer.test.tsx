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
import { mount, ReactWrapper } from 'enzyme';
import MetricsExplorer from './MetricsExplorer';
import { Input } from 'reactstrap';

describe('MetricsExplorer', () => {
  const spyInsertAtCursor = jest.fn().mockImplementation((value: string) => {
    value = value;
  });
  const metricsExplorerProps = {
    show: true,
    updateShow: (show: boolean): void => {
      show = show;
    },
    metrics: ['go_test_1', 'prometheus_test_1'],
    insertAtCursor: spyInsertAtCursor,
  };

  let metricsExplorer: ReactWrapper;
  beforeEach(() => {
    metricsExplorer = mount(<MetricsExplorer {...metricsExplorerProps} />);
  });

  it('renders an Input[type=text]', () => {
    const input = metricsExplorer.find(Input);
    expect(input.prop('type')).toEqual('text');
  });

  it('lists all metrics in props', () => {
    const metrics = metricsExplorer.find('.metric');
    expect(metrics).toHaveLength(metricsExplorerProps.metrics.length);
  });

  it('filters metrics with search', () => {
    const input = metricsExplorer.find(Input);
    input.simulate('change', { target: { value: 'go' } });
    const metrics = metricsExplorer.find('.metric');
    expect(metrics).toHaveLength(1);
  });

  it('handles click on metric', () => {
    const metric = metricsExplorer.find('.metric').at(0);
    metric.simulate('click');
    expect(metricsExplorerProps.insertAtCursor).toHaveBeenCalled();
  });
});
