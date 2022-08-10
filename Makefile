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

TARGET=kuscale
GO=go
GO_MODULE=GO111MODULE=on
BIN_DIR=./bin/
CMD_DIR=./cmd/
COMPILE_FLAGS=CGO_ENABLED=0 GOOS=linux GOARCH=amd64
VERSION?=9
KLOG?=5

.PHONY: all clean $(TARGET)

all: $(TARGET)

run:
	./bin/kuscale  --kubeconfig ~/.kube/config -v $(KLOG)

run-docker: $(TARGET) build-docker apply

.PHONY: apply 
apply:
	envsubst < ./deploy/kuscale.yaml | kubectl apply -f -

.PHONY: delete 
delete:
	envsubst < ./deploy/kuscale.yaml | kubectl delete -f -

kuscale:
	$(GO) build -o $(BIN_DIR)$@ $(CMD_DIR)$@
	# $(GO_MODULE) $(COMPILE_FLAGS) $(GO) build -o $(BIN_DIR)$@ $(CMD_DIR)$@

build-get:
	$(GO_MODULE) $(COMPILE_FLAGS) go get -u ./...
	# $(GO_MODULE) $(COMPILE_FLAGS) go get -u k8s.io/client-go@v0.17.2 github.com/googleapis/gnostic@v0.3.1 golang.org/x/net@v0.0.0-20191004110552-13f9640d40b9 ./...

build-docker:
	docker build -t guswns531/kuscale:base-$(VERSION) -f ./build/docker/Dockerfile .

build-base:
	docker build -t guswns531/kuscale:base-$(VERSION) -f ./build/docker-base/Dockerfile .

gemini:
	kubectl apply -f ./deploy/gemini-deploy

gemini-down:
	kubectl delete -f ./deploy/gemini-deploy

kubeshare:
	kubectl apply -f ./deploy/kubeshare-deploy

kubeshare-down:
	kubectl delete -f ./deploy/kubeshare-deploy

watch-pod:
	watch -n 1 kubectl get pods --all-namespaces
	
clean:
	rm -rf $(BIN_DIR)

monitoring:
	kubectl apply -f ./deploy/monitoring/namespace.yaml
	kubectl apply -f ./deploy/monitoring/

monitoring-down:
	kubectl delete -f ./deploy/monitoring/

test3:
	kubectl apply -f ./deploy/test3.yaml

test3d:
	kubectl delete -f ./deploy/test3.yaml