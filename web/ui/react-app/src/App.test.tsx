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
import App from './App';
import Navigation from './Navbar';
import { Container } from 'reactstrap';
import { Route } from 'react-router-dom';
import {
  AgentPage,
  AlertsPage,
  ConfigPage,
  FlagsPage,
  RulesPage,
  ServiceDiscoveryPage,
  StatusPage,
  TargetsPage,
  TSDBStatusPage,
  PanelListPage,
} from './pages';

describe('App', () => {
  const app = shallow(<App consolesLink={null} agentMode={false} ready={false} />);

  it('navigates', () => {
    expect(app.find(Navigation)).toHaveLength(1);
  });
  it('routes', () => {
    [
      AgentPage,
      AlertsPage,
      ConfigPage,
      FlagsPage,
      RulesPage,
      ServiceDiscoveryPage,
      StatusPage,
      TargetsPage,
      TSDBStatusPage,
      PanelListPage,
    ].forEach((component) => {
      const c = app.find(component);
      expect(c).toHaveLength(1);
    });
    expect(app.find(Route)).toHaveLength(10);
    expect(app.find(Container)).toHaveLength(1);
  });
});
