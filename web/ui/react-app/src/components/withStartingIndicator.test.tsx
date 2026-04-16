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
import { WALReplayData } from '../types/types';
import { StartingContent } from './withStartingIndicator';
import { Progress } from 'reactstrap';

describe('Starting', () => {
  describe('progress bar', () => {
    it('does not show when replay not started', () => {
      const status: WALReplayData = {
        min: 0,
        max: 0,
        current: 0,
      };
      const starting = shallow(<StartingContent status={status} isUnexpected={false} />);
      const progress = starting.find(Progress);
      expect(progress).toHaveLength(0);
    });

    it('shows progress bar when max is not 0', () => {
      const status: WALReplayData = {
        min: 0,
        max: 1,
        current: 0,
      };
      const starting = shallow(<StartingContent status={status} isUnexpected={false} />);
      const progress = starting.find(Progress);
      expect(progress).toHaveLength(1);
    });

    it('renders progress correctly', () => {
      const status: WALReplayData = {
        min: 0,
        max: 20,
        current: 1,
      };
      const starting = shallow(<StartingContent status={status} isUnexpected={false} />);
      const progress = starting.find(Progress);
      expect(progress.prop('value')).toBe(2);
      expect(progress.prop('min')).toBe(0);
      expect(progress.prop('max')).toBe(21);
    });

    it('shows done when replay done', () => {
      const status: WALReplayData = {
        min: 0,
        max: 20,
        current: 20,
      };
      const starting = shallow(<StartingContent status={status} isUnexpected={false} />);
      const progress = starting.find(Progress);
      expect(progress.prop('value')).toBe(21);
      expect(progress.prop('color')).toBe('success');
    });
  });
});
