#!/bin/sh

# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xe

HOST_CMD="nsenter --mount=/proc/1/ns/mnt"

DISTRIBUTION=$($HOST_CMD cat /etc/os-release | grep ^ID= | cut -d'=' -f2 | tr -d '"')
ARCH=$($HOST_CMD uname -m)
echo "Linux distribution: $DISTRIBUTION, Arch: $ARCH"

if [ ! -d "/host/etc/workload-identity" ]; then
  echo "/host/etc/workload-identity not exist, create it"
  mkdir /host/etc/workload-identity
fi

cp /etc/workload-identity/* /host/etc/workload-identity

# install csifuse-proxy and gcsfuse if needed
echo "start install gcsfuse-3.2.0-1 for ${DISTRIBUTION}...."
if [ "${DISTRIBUTION}" = "rocky" ]
then
  cp /csifuse-proxy/gcsfuse-3.2.0-1.x86_64.rpm /host/etc/
  $HOST_CMD dnf install -y /etc/gcsfuse-3.2.0-1.x86_64.rpm
  $HOST_CMD rm -f /etc/gcsfuse-3.2.0-1.x86_64.rpm
fi

echo "copy csifuse-proxy to /usr/bin/ ...."
if [ ! -f "/host/usr/bin/csifuse-proxy" ];then
  cp /csifuse-proxy/csifuse-proxy /host/usr/bin/csifuse-proxy --force
  chmod +x /host/usr/bin/csifuse-proxy
fi

echo "copy csifuse-proxy.service ...."
if [ ! -f "/host/usr/lib/systemd/system/csifuse-proxy.service" ];then
  mkdir -p /host/usr/lib/systemd/system
  cp /csifuse-proxy/csifuse-proxy.service /host/usr/lib/systemd/system/csifuse-proxy.service
fi

echo "reload/start csifuse-proxy.service ...."
$HOST_CMD systemctl daemon-reload
$HOST_CMD systemctl enable csifuse-proxy.service
$HOST_CMD systemctl restart csifuse-proxy.service
