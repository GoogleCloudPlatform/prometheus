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
import toJson from 'enzyme-to-json';
import { StatusContent } from './Status';

describe('Status', () => {
  describe('Snapshot testing', () => {
    const response: any = [
      {
        startTime: '2019-10-30T22:03:23.247913868+02:00',
        CWD: '/home/boyskila/Desktop/prometheus',
        reloadConfigSuccess: true,
        lastConfigTime: '2019-10-30T22:03:23+02:00',
        corruptionCount: 0,
        goroutineCount: 37,
        GOMAXPROCS: 4,
        GOGC: '',
        GODEBUG: '',
        storageRetention: '15d',
      },
      {
        version: '',
        revision: '',
        branch: '',
        buildUser: '',
        buildDate: '',
        goVersion: 'go1.13.3',
      },
      {
        activeAlertmanagers: [
          { url: 'https://1.2.3.4:9093/api/v1/alerts' },
          { url: 'https://1.2.3.5:9093/api/v1/alerts' },
          { url: 'https://1.2.3.6:9093/api/v1/alerts' },
          { url: 'https://1.2.3.7:9093/api/v1/alerts' },
          { url: 'https://1.2.3.8:9093/api/v1/alerts' },
          { url: 'https://1.2.3.9:9093/api/v1/alerts' },
        ],
        droppedAlertmanagers: [],
      },
    ];
    it('should match table snapshot', () => {
      const wrapper = shallow(<StatusContent data={response} title="Foo" />);
      expect(toJson(wrapper)).toMatchSnapshot();
      jest.restoreAllMocks();
    });
  });
});
