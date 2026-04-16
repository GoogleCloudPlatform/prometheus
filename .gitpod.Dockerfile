# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM gitpod/workspace-full

ENV CUSTOM_NODE_VERSION=16
ENV CUSTOM_GO_VERSION=1.19
ENV GOPATH=$HOME/go-packages
ENV GOROOT=$HOME/go
ENV PATH=$GOROOT/bin:$GOPATH/bin:$PATH

RUN bash -c ". .nvm/nvm.sh && nvm install ${CUSTOM_NODE_VERSION} && nvm use ${CUSTOM_NODE_VERSION} && nvm alias default ${CUSTOM_NODE_VERSION}"

RUN echo "nvm use default &>/dev/null" >> ~/.bashrc.d/51-nvm-fix
RUN curl -fsSL https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz | tar xzs \
    && printf '%s\n' 'export GOPATH=/workspace/go' \
                      'export PATH=$GOPATH/bin:$PATH' > $HOME/.bashrc.d/300-go

