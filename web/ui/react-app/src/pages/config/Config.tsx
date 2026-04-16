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

import React, { useState, FC } from 'react';
import { Button } from 'reactstrap';
import CopyToClipboard from 'react-copy-to-clipboard';

import { withStatusIndicator } from '../../components/withStatusIndicator';
import { useFetch } from '../../hooks/useFetch';
import { usePathPrefix } from '../../contexts/PathPrefixContext';
import { API_PATH } from '../../constants/constants';

type YamlConfig = { yaml?: string };

interface ConfigContentProps {
  error?: Error;
  data?: YamlConfig;
}

const YamlContent = ({ yaml }: YamlConfig) => <pre className="config-yaml">{yaml}</pre>;
YamlContent.displayName = 'Config';

const ConfigWithStatusIndicator = withStatusIndicator(YamlContent);

export const ConfigContent: FC<ConfigContentProps> = ({ error, data }) => {
  const [copied, setCopied] = useState(false);
  const config = data && data.yaml;
  return (
    <>
      <h2>
        Configuration&nbsp;
        <CopyToClipboard
          text={config ? config : ''}
          onCopy={(_, result) => {
            setCopied(result);
            setTimeout(setCopied, 1500);
          }}
        >
          <Button color="light" disabled={!config}>
            {copied ? 'Copied' : 'Copy to clipboard'}
          </Button>
        </CopyToClipboard>
      </h2>
      <ConfigWithStatusIndicator error={error} isLoading={!config} yaml={config} />
    </>
  );
};

const Config: FC = () => {
  const pathPrefix = usePathPrefix();
  const { response, error } = useFetch<YamlConfig>(`${pathPrefix}/${API_PATH}/status/config`);
  return <ConfigContent error={error} data={response.data} />;
};

export default Config;
