FROM gcr.io/distroless/base

ENV APISERVER_CERT_LOCATION /vkubelet-boc-crt.pem
ENV APISERVER_KEY_LOCATION /vkubelet-boc-key.pem
ENV KUBELET_PORT 10250

# Use the pre-built binary in "bin/virtual-kubelet".
COPY ./virtual-kubelet /virtual-kubelet
# Copy the configuration file for the mock provider.
COPY ./vkubelet-boc-cfg.json /vkubelet-boc-cfg.json
# Copy the certificate for the HTTPS server.
COPY ./vkubelet-boc-crt.pem /vkubelet-boc-crt.pem
# Copy the private key for the HTTPS server.
COPY ./vkubelet-boc-key.pem /vkubelet-boc-key.pem

CMD ["/virtual-kubelet"]
