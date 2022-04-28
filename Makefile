# Copyright 2022 Hyeon-Jun Jang, SSLab, Konkuk University
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

TARGET=main
GO=go
GO_MODULE=GO111MODULE=on
BIN_DIR=./bin/
CMD_DIR=./bin/
COMPILE_FLAGS=CGO_ENABLED=0 GOOS=linux GOARCH=amd64

.PHONY: all clean $(TARGET)

all: $(TARGET)

main:
	$(GO_MODULE) $(COMPILE_FLAGS) $(GO) build -o $(BIN_DIR)$@ $(CMD_DIR)$@

kubeshare:
	kubectl apply -f ./deploy/kubeshare-deploy

kubeshare-down:
	kubectl delete -f ./deploy/kubeshare-deploy

clean:
	rm -rf $(BIN_DIR)
