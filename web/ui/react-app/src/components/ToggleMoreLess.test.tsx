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
import { Button } from 'reactstrap';
import { ToggleMoreLess } from './ToggleMoreLess';

describe('ToggleMoreLess', () => {
  const showMoreValue = false;
  const defaultProps = {
    event: (): void => {
      tggleBtn.setProps({ showMore: !showMoreValue });
    },
    showMore: showMoreValue,
  };
  const tggleBtn = shallow(<ToggleMoreLess {...defaultProps} />);

  it('renders a show more btn at start', () => {
    const btn = tggleBtn.find(Button);
    expect(btn).toHaveLength(1);
    expect(btn.prop('color')).toEqual('primary');
    expect(btn.prop('size')).toEqual('xs');
    expect(btn.render().text()).toEqual('show more');
  });

  it('renders a show less btn if clicked', () => {
    tggleBtn.find(Button).simulate('click');
    expect(tggleBtn.find(Button).render().text()).toEqual('show less');
  });
});
