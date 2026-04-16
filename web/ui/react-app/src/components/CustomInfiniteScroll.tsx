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

import { ComponentType, useEffect, useState } from 'react';
import InfiniteScroll from 'react-infinite-scroll-component';

const initialNumberOfItemsDisplayed = 50;

export interface InfiniteScrollItemsProps<T> {
  items: T[];
}

interface CustomInfiniteScrollProps<T> {
  allItems: T[];
  child: ComponentType<InfiniteScrollItemsProps<T>>;
}

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
const CustomInfiniteScroll = <T,>({ allItems, child }: CustomInfiniteScrollProps<T>) => {
  const [items, setItems] = useState<T[]>(allItems.slice(0, 50));
  const [index, setIndex] = useState<number>(initialNumberOfItemsDisplayed);
  const [hasMore, setHasMore] = useState<boolean>(allItems.length > initialNumberOfItemsDisplayed);
  const Child = child;

  useEffect(() => {
    setItems(allItems.slice(0, initialNumberOfItemsDisplayed));
    setHasMore(allItems.length > initialNumberOfItemsDisplayed);
  }, [allItems]);

  const fetchMoreData = () => {
    if (items.length === allItems.length) {
      setHasMore(false);
    } else {
      const newIndex = index + initialNumberOfItemsDisplayed;
      setIndex(newIndex);
      setItems(allItems.slice(0, newIndex));
    }
  };

  return (
    <InfiniteScroll
      next={fetchMoreData}
      hasMore={hasMore}
      loader={<h4>loading...</h4>}
      dataLength={items.length}
      height={items.length > 25 ? '75vh' : ''}
    >
      <Child items={items} />
    </InfiniteScroll>
  );
};

export default CustomInfiniteScroll;
