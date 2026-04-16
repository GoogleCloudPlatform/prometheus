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

import $ from 'jquery';
import React, { Component } from 'react';
import { Button, Input, InputGroup, InputGroupAddon } from 'reactstrap';

import moment from 'moment-timezone';

import 'tempusdominus-core';
import 'tempusdominus-bootstrap-4';
import 'tempusdominus-bootstrap-4/build/css/tempusdominus-bootstrap-4.min.css';

import { dom, library } from '@fortawesome/fontawesome-svg-core';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faArrowDown,
  faArrowUp,
  faCalendarCheck,
  faChevronLeft,
  faChevronRight,
  faTimes,
} from '@fortawesome/free-solid-svg-icons';

library.add(faChevronLeft, faChevronRight, faCalendarCheck, faArrowUp, faArrowDown, faTimes);
// Sadly needed to also replace <i> within the date picker, since it's not a React component.
dom.watch();

interface TimeInputProps {
  time: number | null; // Timestamp in milliseconds.
  useLocalTime: boolean;
  range: number; // Range in seconds.
  placeholder: string;
  onChangeTime: (time: number | null) => void;
}

class TimeInput extends Component<TimeInputProps> {
  private timeInputRef = React.createRef<HTMLInputElement>();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private $time: any = null;

  getBaseTime = (): number => {
    return this.props.time || moment().valueOf();
  };

  calcShiftRange = (): number => this.props.range / 2;

  increaseTime = (): void => {
    const time = this.getBaseTime() + this.calcShiftRange();
    this.props.onChangeTime(time);
  };

  decreaseTime = (): void => {
    const time = this.getBaseTime() - this.calcShiftRange();
    this.props.onChangeTime(time);
  };

  clearTime = (): void => {
    this.props.onChangeTime(null);
  };

  timezone = (): string => {
    return this.props.useLocalTime ? moment.tz.guess() : 'UTC';
  };

  componentDidMount(): void {
    if (!this.timeInputRef.current) {
      return;
    }
    this.$time = $(this.timeInputRef.current);

    this.$time.datetimepicker({
      icons: {
        today: 'fas fa-calendar-check',
      },
      buttons: {
        //showClear: true,
        showClose: true,
        showToday: true,
      },
      sideBySide: true,
      format: 'YYYY-MM-DD HH:mm:ss',
      locale: 'en',
      timeZone: this.timezone(),
      defaultDate: this.props.time,
    });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    this.$time.on('change.datetimepicker', (e: any) => {
      // The end time can also be set by dragging a section on the graph,
      // and that value will have decimal places.
      if (e.date && e.date.valueOf() !== Math.trunc(this.props.time?.valueOf() || NaN)) {
        this.props.onChangeTime(e.date.valueOf());
      }
    });
  }

  componentWillUnmount(): void {
    this.$time.datetimepicker('destroy');
  }

  componentDidUpdate(prevProps: TimeInputProps): void {
    const { time, useLocalTime } = this.props;
    if (prevProps.time !== time) {
      this.$time.datetimepicker('date', time ? moment(time) : null);
    }
    if (prevProps.useLocalTime !== useLocalTime) {
      this.$time.datetimepicker('options', { timeZone: this.timezone(), defaultDate: null });
    }
  }

  render(): JSX.Element {
    return (
      <InputGroup className="time-input" size="sm">
        <InputGroupAddon addonType="prepend">
          <Button title="Decrease time" onClick={this.decreaseTime}>
            <FontAwesomeIcon icon={faChevronLeft} fixedWidth />
          </Button>
        </InputGroupAddon>

        <Input
          placeholder={this.props.placeholder}
          innerRef={this.timeInputRef}
          onFocus={() => this.$time.datetimepicker('show')}
          onBlur={() => this.$time.datetimepicker('hide')}
          onKeyDown={(e) => ['Escape', 'Enter'].includes(e.key) && this.$time.datetimepicker('hide')}
        />

        {/* CAUTION: While the datetimepicker also has an option to show a 'clear' button,
            that functionality is broken, so we create an external solution instead. */}
        {this.props.time && (
          <InputGroupAddon addonType="append">
            <Button outline className="clear-time-btn" title="Clear time" onClick={this.clearTime}>
              <FontAwesomeIcon icon={faTimes} fixedWidth />
            </Button>
          </InputGroupAddon>
        )}

        <InputGroupAddon addonType="append">
          <Button title="Increase time" onClick={this.increaseTime}>
            <FontAwesomeIcon icon={faChevronRight} fixedWidth />
          </Button>
        </InputGroupAddon>
      </InputGroup>
    );
  }
}

export default TimeInput;
