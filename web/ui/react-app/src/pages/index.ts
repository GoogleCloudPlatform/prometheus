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

import Agent from './agent/Agent';
import Alerts from './alerts/Alerts';
import Config from './config/Config';
import Flags from './flags/Flags';
import Rules from './rules/Rules';
import ServiceDiscovery from './serviceDiscovery/Services';
import Status from './status/Status';
import Targets from './targets/Targets';
import PanelList from './graph/PanelList';
import TSDBStatus from './tsdbStatus/TSDBStatus';
import { withStartingIndicator } from '../components/withStartingIndicator';

const AgentPage = withStartingIndicator(Agent);
const AlertsPage = withStartingIndicator(Alerts);
const ConfigPage = withStartingIndicator(Config);
const FlagsPage = withStartingIndicator(Flags);
const RulesPage = withStartingIndicator(Rules);
const ServiceDiscoveryPage = withStartingIndicator(ServiceDiscovery);
const StatusPage = withStartingIndicator(Status);
const TSDBStatusPage = withStartingIndicator(TSDBStatus);
const TargetsPage = withStartingIndicator(Targets);
const PanelListPage = withStartingIndicator(PanelList);

// prettier-ignore
export {
  AgentPage,
  AlertsPage,
  ConfigPage,
  FlagsPage,
  RulesPage,
  ServiceDiscoveryPage,
  StatusPage,
  TSDBStatusPage,
  TargetsPage,
  PanelListPage
};
