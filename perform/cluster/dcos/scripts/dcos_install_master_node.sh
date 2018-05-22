#!/usr/bin/env bash
#
# Installs and configure a master node
# This script must be executed on server to configure as master node

# Installs and configures everything needed on any node
{{.IncludeInstallCommons}}

# Installs graphical environment
yum install -y tigervnc-server

# Installs SafeScale containers
curl http://{{.BootstrapIP}}:{{.BootstrapPort}}/docker/guacamole.tar.gz 2>/dev/null | docker image load
curl http://{{.BootstrapIP}}:{{.BootstrapPort}}/docker/proxy.tar.gz 2>/dev/null | docker image load

# Get install script from bootstrap server
mkdir /tmp/dcos && cd /tmp/dcos
curl -O http://{{.BootstrapIP}}:{{.BootstrapPort}}/dcos_install.sh || exit 1

# Launch installation
bash dcos_install.sh master
retcode=$?

#  Do some cleanup
#rm -rf /tmp/dcos

exit $retcode