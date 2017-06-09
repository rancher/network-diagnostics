FROM rancher/agent-base:v0.3.0
RUN apt-get update \
    && apt-get install -y zip unzip \
    && mkdir -p /logs
ENV RANCHER_CLI_VERSION v0.6.1
RUN curl -sLf https://github.com/rancher/cli/releases/download/${RANCHER_CLI_VERSION}/rancher-linux-amd64-${RANCHER_CLI_VERSION}.tar.gz | tar xvzf - -C /tmp --strip-components=1 \
    && mv /tmp/rancher-${RANCHER_CLI_VERSION}/rancher /usr/bin \
    && rm -rf /tmp/rancher-${RANCHER_CLI_VERSION} \
    && chmod +x /usr/bin/rancher
COPY start.sh setup-cli.sh network-support logs-collector.sh /usr/bin/
CMD ["start.sh"]
