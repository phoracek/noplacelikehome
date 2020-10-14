#!/usr/bin/env bash
set -euo pipefail

vagrant_dir='/home/vagrant'

if [[ ":${PWD}" == *":${HOME}/code"* ]]; then
    vagrant_dir=${PWD#"${HOME}"}
fi

pushd ${HOME}/code
trap popd EXIT
vagrant ssh -c "cd ${vagrant_dir} && \${SHELL}"
