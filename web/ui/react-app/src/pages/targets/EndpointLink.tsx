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
import { Badge } from 'reactstrap';

export interface EndpointLinkProps {
  endpoint: string;
  globalUrl: string;
}

const EndpointLink: FC<EndpointLinkProps> = ({ endpoint, globalUrl }) => {
  let url: URL;
  let search = '';
  let invalidURL = false;
  try {
    url = new URL(endpoint);
  } catch (err: unknown) {
    // In cases of IPv6 addresses with a Zone ID, URL may not be parseable.
    // See https://github.com/prometheus/prometheus/issues/9760
    // In this case, we attempt to prepare a synthetic URL with the
    // same query parameters, for rendering purposes.
    invalidURL = true;
    if (endpoint.indexOf('?') > -1) {
      search = endpoint.substring(endpoint.indexOf('?'));
    }
    url = new URL('http://0.0.0.0' + search);
  }

  const { host, pathname, protocol, searchParams }: URL = url;
  const params = Array.from(searchParams.entries());
  const displayLink = invalidURL ? endpoint.replace(search, '') : `${protocol}//${host}${pathname}`;
  return (
    <>
      <a href={globalUrl}>{displayLink}</a>
      {params.length > 0 ? <br /> : null}
      {params.map(([labelName, labelValue]: [string, string]) => {
        return (
          <Badge color="primary" className="mr-1" key={`${labelName}/${labelValue}`}>
            {`${labelName}="${labelValue}"`}
          </Badge>
        );
      })}
    </>
  );
};

export default EndpointLink;
