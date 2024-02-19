FROM debian:buster

# Установка зависимостей
RUN apt-get update && apt-get install -y \
    curl \
    git \
    wget \
    musl-dev \
    crossbuild-essential-arm64 \
 && rm -rf /var/lib/apt/lists/*

# Загрузка и установка Go
ENV GOLANG_VERSION 1.21.6

RUN wget -O go.tgz "https://golang.org/dl/go${GOLANG_VERSION}.linux-amd64.tar.gz" \
    && tar -C /usr/local -xzf go.tgz \
    && rm go.tgz

ENV PATH="/usr/local/go/bin:$PATH"

# Загрузка и распаковка кросс-компилятора
RUN wget -P /root https://musl.cc/aarch64-linux-musl-cross.tgz \
 && tar -xvf /root/aarch64-linux-musl-cross.tgz -C /root

# Настройка переменных окружения для кросс-компиляции
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=arm64 \
    CC=/root//aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc
    

WORKDIR /workdir

