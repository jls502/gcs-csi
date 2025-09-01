FROM alpine
RUN mkdir -p /var/lib/kubelet/pods
RUN mkdir -p /host/etc/workload-identity
RUN mkdir /csifuse-proxy
COPY gcsfuse-3.2.0-1.x86_64.rpm /csifuse-proxy
COPY csifuse-proxy.service /csifuse-proxy
COPY csifuse-proxy /csifuse-proxy
COPY install-csiproxy.sh /csifuse-proxy
COPY driver /usr/local/bin/
COPY synctoken /usr/local/bin/
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /csifuse-proxy/install-csiproxy.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
