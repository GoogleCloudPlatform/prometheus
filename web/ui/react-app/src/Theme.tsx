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

import React, { FC, useEffect } from 'react';
import { Form, Button, ButtonGroup } from 'reactstrap';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faMoon, faSun, faAdjust } from '@fortawesome/free-solid-svg-icons';
import { useTheme } from './contexts/ThemeContext';

export const themeLocalStorageKey = 'user-prefers-color-scheme';

export const Theme: FC = () => {
  const { theme } = useTheme();

  useEffect(() => {
    document.body.classList.toggle('bootstrap-dark', theme === 'dark');
    document.body.classList.toggle('bootstrap', theme === 'light');
  }, [theme]);

  return null;
};

export const ThemeToggle: FC = () => {
  const { userPreference, setTheme } = useTheme();

  return (
    <Form className="ml-auto" inline>
      <ButtonGroup size="sm">
        <Button
          color="secondary"
          title="Use light theme"
          active={userPreference === 'light'}
          onClick={() => setTheme('light')}
        >
          <FontAwesomeIcon icon={faSun} className={userPreference === 'light' ? 'text-white' : 'text-dark'} />
        </Button>
        <Button color="secondary" title="Use dark theme" active={userPreference === 'dark'} onClick={() => setTheme('dark')}>
          <FontAwesomeIcon icon={faMoon} className={userPreference === 'dark' ? 'text-white' : 'text-dark'} />
        </Button>
        <Button
          color="secondary"
          title="Use browser-preferred theme"
          active={userPreference === 'auto'}
          onClick={() => setTheme('auto')}
        >
          <FontAwesomeIcon icon={faAdjust} className={userPreference === 'auto' ? 'text-white' : 'text-dark'} />
        </Button>
      </ButtonGroup>
    </Form>
  );
};
