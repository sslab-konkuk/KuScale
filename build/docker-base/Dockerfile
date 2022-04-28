FROM ubuntu:20.04

SHELL ["/bin/bash", "-cu"]

ENV GOLANG_VERSION 1.17.1
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /home/kuscale

COPY . .

RUN apt update && \
    apt install -y g++ wget make iproute2 curl kmod && \
    wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    cd src && \
    make
    # echo hello

CMD ["./bin/main"]

