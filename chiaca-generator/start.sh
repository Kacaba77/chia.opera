#!/usr/bin/env bash

cd ${CHIA_ROOT}/config/ssl/ca
kubectl create secret generic ${SECRET_NAME} --namespace ${NAMESPACE} --from-file=chia_ca.crt --from-file=chia_ca.key --from-file=private_ca.crt --from-file=private_ca.key
