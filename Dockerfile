FROM bitnami/minideb:latest

ENV USER=agent
ENV CGO_ENABLED=0

# Install a comprehensive set of development tools and languages.
# This allows for a versatile development environment capable of
# handling a wide range of programming tasks.
RUN install_packages \
    sudo \
    ca-certificates \
    git \
    ssh \
    curl \
    libasound2-dev \
    && useradd -m ${USER} -s /bin/bash && \
    echo "${USER} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USER} && \
    chmod 0440 /etc/sudoers.d/${USER}

ARG TARGETARCH
RUN if [ "${TARGETARCH}" = "arm64" ]; then \
	ARCH=arm64; \
	else \
	ARCH=amd64; \
	fi && \
	curl -LO https://golang.org/dl/go1.24.2.linux-${ARCH}.tar.gz && \
	tar -C /usr/local -xzf go1.24.2.linux-${ARCH}.tar.gz && \
	rm go1.24.2.linux-${ARCH}.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOFLAGS=-buildvcs=false

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download 

COPY . .
RUN go build -o a2a-go

USER ${USER}
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# WORKDIR /home/${USER}
# RUN echo "Host github.com\n\tStrictHostKeyChecking no\n" >> /home/${USER}/.ssh/config

ENTRYPOINT ["/app/a2a-go"]