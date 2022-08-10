# FROM guswns531/kuscale:base-03
FROM ubuntu:20.04

SHELL ["/bin/bash", "-cu"]

ENV GOLANG_VERSION 1.17.10
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /home/kuscale

COPY . .

RUN apt-get update && \
    apt install -y bison build-essential cmake flex git libedit-dev libllvm7 llvm-7-dev libclang-7-dev python zlib1g-dev libelf-dev libfl-dev python3-distutils
    
RUN git clone https://github.com/iovisor/bcc.git && \
    cd bcc  && \
    git checkout v0.24.0  && \
    mkdir build  && \
    cd build  && \
    cmake ..  && \
    make  && \
    make install  && \ 
    cmake -DPYTHON_CMD=python3 .. # build python3 binding  &&\
    pushd src/python/  && \
    make  && \
    make install  && \
    popd 


RUN apt-get update && \
    apt-get install -y g++ wget make iproute2 curl kmod && \
    wget -nv -O - https://storage.googleapis.com/golang/go1.17.10.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    make
    # echo hello


CMD ["sleep", "100000000"]
# CMD ["./bin/kuscale", "-v", "5", "--MonitoringPeriod", "2"]
# CMD ["./bin/kuscale", "--kubeconfig", "~/.kube/config", "-v", "5"]

