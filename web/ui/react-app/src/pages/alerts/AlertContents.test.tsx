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
import { shallow } from 'enzyme';
import AlertsContent from './AlertContents';

describe('AlertsContent', () => {
  const defaultProps = {
    groups: [],
    statsCount: {
      inactive: 0,
      pending: 0,
      firing: 0,
    },
  };
  const wrapper = shallow(<AlertsContent {...defaultProps} />);

  it('matches a snapshot', () => {
    expect(wrapper).toMatchSnapshot();
  });

  [
    { selector: '#inactive-toggler', propName: 'inactive' },
    { selector: '#pending-toggler', propName: 'pending' },
    { selector: '#firing-toggler', propName: 'firing' },
  ].forEach((testCase) => {
    it(`toggles the ${testCase.propName} checkbox from true to false when clicked and back to true when clicked again`, () => {
      expect(wrapper.find(testCase.selector).prop('checked')).toBe(true);
      wrapper.find(testCase.selector).simulate('change', { target: { checked: false } });
      expect(wrapper.find(testCase.selector).prop('checked')).toBe(false);
      wrapper.find(testCase.selector).simulate('change', { target: { checked: true } });
      expect(wrapper.find(testCase.selector).prop('checked')).toBe(true);
    });
  });

  it('toggles the "annotations" checkbox from false to true when clicked and back to false when clicked again', () => {
    expect(wrapper.find('#show-annotations-toggler').prop('checked')).toBe(false);
    wrapper.find('#show-annotations-toggler').simulate('change', { target: { checked: true } });
    expect(wrapper.find('#show-annotations-toggler').prop('checked')).toBe(true);
    wrapper.find('#show-annotations-toggler').simulate('change', { target: { checked: false } });
    expect(wrapper.find('#show-annotations-toggler').prop('checked')).toBe(false);
  });
});
