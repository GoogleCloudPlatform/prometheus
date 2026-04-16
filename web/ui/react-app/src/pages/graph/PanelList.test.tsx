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
import PanelList, { PanelListContent } from './PanelList';
import Checkbox from '../../components/Checkbox';
import { Button } from 'reactstrap';
import Panel from './Panel';

describe('PanelList', () => {
  it('renders configuration checkboxes', () => {
    [
      { id: 'use-local-time-checkbox', label: 'Use local time', default: false },
      { id: 'query-history-checkbox', label: 'Enable query history', default: false },
      { id: 'autocomplete-checkbox', label: 'Enable autocomplete', default: true },
      { id: 'highlighting-checkbox', label: 'Enable highlighting', default: true },
      { id: 'linter-checkbox', label: 'Enable linter', default: true },
    ].forEach((cb, idx) => {
      const panelList = shallow(<PanelList />);
      const checkbox = panelList.find(Checkbox).at(idx);
      expect(checkbox.prop('id')).toEqual(cb.id);
      expect(checkbox.prop('defaultChecked')).toBe(cb.default);
      expect(checkbox.children().text()).toBe(cb.label);
    });
  });

  it('renders panels', () => {
    const panelList = shallow(<PanelListContent {...({ panels: [{ id: 'foo' }] } as any)} />);
    const panels = panelList.find(Panel);
    expect(panels.length).toBeGreaterThan(0);
  });

  it('renders a button to add a panel', () => {
    const panelList = shallow(<PanelListContent {...({ panels: [] } as any)} />);
    const btn = panelList.find(Button);
    expect(btn.prop('color')).toEqual('primary');
    expect(btn.children().text()).toEqual('Add Panel');
  });
});
