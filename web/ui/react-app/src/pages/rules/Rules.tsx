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

import React, { FC } from 'react';
import { useFetch } from '../../hooks/useFetch';
import { withStatusIndicator } from '../../components/withStatusIndicator';
import { RulesMap, RulesContent } from './RulesContent';
import { usePathPrefix } from '../../contexts/PathPrefixContext';
import { API_PATH } from '../../constants/constants';

const RulesWithStatusIndicator = withStatusIndicator(RulesContent);

const Rules: FC = () => {
  const pathPrefix = usePathPrefix();
  const { response, error, isLoading } = useFetch<RulesMap>(`${pathPrefix}/${API_PATH}/rules`);

  return <RulesWithStatusIndicator response={response} error={error} isLoading={isLoading} />;
};

export default Rules;
