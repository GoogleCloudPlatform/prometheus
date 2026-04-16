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

import { parser } from '../dist/index.es.js';
import { fileTests } from '@lezer/generator/dist/test';

import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

let caseDir = path.dirname(fileURLToPath(import.meta.url))
for (const file of fs.readdirSync(caseDir)) {
    if (!/\.txt$/.test(file)) continue;

    const name = /^[^.]*/.exec(file)[0];
    describe(name, () => {
        for (const { name, run } of fileTests(fs.readFileSync(path.join(caseDir, file), 'utf8'), file)) it(name, () => run(parser));
    });
}
