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

import React, { Component, ChangeEvent } from 'react';
import { Modal, ModalBody, ModalHeader, Input } from 'reactstrap';
import { Fuzzy, FuzzyResult } from '@nexucis/fuzzy';

const fuz = new Fuzzy({ pre: '<strong>', post: '</strong>', shouldSort: true });

interface MetricsExplorerProps {
  show: boolean;
  updateShow(show: boolean): void;
  metrics: string[];
  insertAtCursor(value: string): void;
}

type MetricsExplorerState = {
  searchTerm: string;
};

class MetricsExplorer extends Component<MetricsExplorerProps, MetricsExplorerState> {
  constructor(props: MetricsExplorerProps) {
    super(props);
    this.state = { searchTerm: '' };
  }
  handleSearchTerm = (event: ChangeEvent<HTMLInputElement>): void => {
    this.setState({ searchTerm: event.target.value });
  };
  handleMetricClick = (query: string): void => {
    this.props.insertAtCursor(query);
    this.props.updateShow(false);
    this.setState({ searchTerm: '' });
  };

  toggle = (): void => {
    this.props.updateShow(!this.props.show);
  };
  render(): JSX.Element {
    return (
      <Modal isOpen={this.props.show} toggle={this.toggle} className="metrics-explorer" scrollable>
        <ModalHeader toggle={this.toggle}>Metrics Explorer</ModalHeader>
        <ModalBody>
          <Input placeholder="Search" value={this.state.searchTerm} type="text" onChange={this.handleSearchTerm} />
          {this.state.searchTerm.length > 0 &&
            fuz
              .filter(this.state.searchTerm, this.props.metrics)
              .map((result: FuzzyResult) => (
                <p
                  key={result.original}
                  className="metric"
                  onClick={this.handleMetricClick.bind(this, result.original)}
                  dangerouslySetInnerHTML={{ __html: result.rendered }}
                ></p>
              ))}
          {this.state.searchTerm.length === 0 &&
            this.props.metrics.map((metric) => (
              <p key={metric} className="metric" onClick={this.handleMetricClick.bind(this, metric)}>
                {metric}
              </p>
            ))}
        </ModalBody>
      </Modal>
    );
  }
}

export default MetricsExplorer;
