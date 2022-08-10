FROM guswns531/kuscale:base-9

SHELL ["/bin/bash", "-cu"]

ENV GOLANG_VERSION 1.17.10
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /home/kuscale

COPY . .

CMD ["sleep", "100000000"]
# CMD ["./bin/kuscale", "-v", "5", "--MonitoringPeriod", "2"]
# CMD ["./bin/kuscale", "--kubeconfig", "~/.kube/config", "-v", "5"]

