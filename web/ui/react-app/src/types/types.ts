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

import { Alert, RuleState } from '../pages/alerts/AlertContents';

export interface Metric {
  [key: string]: string;
}

export interface Histogram {
  count: string;
  sum: string;
  buckets?: [number, string, string, string][];
}

export interface Exemplar {
  labels: { [key: string]: string };
  value: string;
  timestamp: number;
}

export interface QueryParams {
  startTime: number;
  endTime: number;
  resolution: number;
}

export type Rule = {
  alerts: Alert[];
  annotations: Record<string, string>;
  duration: number;
  keepFiringFor: number;
  evaluationTime: string;
  health: string;
  labels: Record<string, string>;
  lastError?: string;
  lastEvaluation: string;
  name: string;
  query: string;
  state: RuleState;
  type: string;
};

export interface WALReplayData {
  min: number;
  max: number;
  current: number;
}

export interface WALReplayStatus {
  data?: WALReplayData;
}

export type ExemplarData = Array<{ seriesLabels: Metric; exemplars: Exemplar[] }> | undefined;
