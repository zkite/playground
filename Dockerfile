# Используйте официальный образ Go как базовый
FROM golang:latest

# Установите кросс-компилятор для arm64
RUN apt-get update && \
    apt-get install -y crossbuild-essential-arm64

# Установите переменные окружения для кросс-компиляции
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=arm64 \
    CC=aarch64-linux-gnu-gcc

# Установите рабочую директорию
WORKDIR /workdir
