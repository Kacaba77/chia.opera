FROM registry.gitlab.com/cmmarslender/kubectl-helm:v3 AS kubectl

FROM ghcr.io/chia-network/chia:latest

ENV keys="none"

COPY --from=kubectl /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY start.sh /start.sh
CMD ["/bin/bash", "/start.sh"]