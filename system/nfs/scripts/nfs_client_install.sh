#!/usr/bin/env bash
#
# Copyright 2018, CS Systemes d'Information, http://www.c-s.fr
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
#
# Installs and configures

set -u -o pipefail

function print_error {
    read line file <<<$(caller)
    echo "An error occurred in line $line of file $file:" "{"`sed "${line}q;d" "$file"`"}" >&2
}
trap print_error ERR

function dns_fallback {
    grep nameserver /etc/resolv.conf && return 0
    echo -e "nameserver 1.1.1.1\n" > /tmp/resolv.conf
    sudo cp /tmp/resolv.conf /etc/resolv.conf
    return 0
}

dns_fallback

{{.reserved_BashLibrary}}

echo "Install NFS client"
case $LINUX_KIND in
    debian|ubuntu)
        export DEBIAN_FRONTEND=noninteractive
        touch /var/log/lastlog
        chgrp utmp /var/log/lastlog
        chmod 664 /var/log/lastlog

        sfRetry 3m 5 "sfWaitForApt && apt -y update"
        sfRetry 5m 5 "sfWaitForApt && apt-get install -qqy nfs-common"
        ;;

    rhel|centos)
        yum makecache fast
        yum install -y nfs-utils
        ;;

    *)
        echo "Unsupported OS flavor '$LINUX_KIND'!"
        exit 1
esac
