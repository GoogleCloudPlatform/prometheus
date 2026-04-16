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
import Navigation from './Navbar';
import { NavItem, NavLink } from 'reactstrap';

describe('Navbar should contain console Link', () => {
  it('with non-empty consoleslink', () => {
    const app = shallow(<Navigation consolesLink="/path/consoles" agentMode={false} />);
    expect(
      app.contains(
        <NavItem>
          <NavLink href="/path/consoles">Consoles</NavLink>
        </NavItem>
      )
    ).toBeTruthy();
  });
});

describe('Navbar should not contain consoles link', () => {
  it('with empty string in consolesLink', () => {
    const app = shallow(<Navigation consolesLink={null} agentMode={false} />);
    expect(
      app.contains(
        <NavItem>
          <NavLink>Consoles</NavLink>
        </NavItem>
      )
    ).toBeFalsy();
  });
});
