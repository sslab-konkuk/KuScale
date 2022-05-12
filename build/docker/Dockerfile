# FROM guswns531/kuscale:base-03
FROM ubuntu:20.04

SHELL ["/bin/bash", "-cu"]

ENV GOLANG_VERSION 1.17.10
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /home/kuscale

COPY . .

RUN apt-get update && \
    apt-get install -y g++ wget make iproute2 curl kmod && \
    wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    make
    # echo hello


CMD ["sleep", "100000000"]
# CMD ["./bin/kuscale", "-v", "5", "--MonitoringPeriod", "2"]
# CMD ["./bin/kuscale", "--kubeconfig", "~/.kube/config", "-v", "5"]

