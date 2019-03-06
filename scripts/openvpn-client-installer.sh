#!/usr/bin/env bash
set -eo pipefail

APT_KEY_URL="https://swupdate.openvpn.net/repos/repo-public.gpg"
UBUNTU_NAME=$(cat /etc/lsb-release | grep DISTRIB_CODENAME | cut -d "=" -f 2)

# downlaod and add the OpenVPN key and add the proper repo
wget -O - ${APT_KEY_URL} | apt-key add -
echo "deb http://build.openvpn.net/debian/openvpn/release/2.4 ${UBUNTU_NAME} main" \
    > /etc/apt/sources.list.d/openvpn-aptrepo.list

# install the OpenVPN and resolvconf packages
apt-get update
apt-get install -y openvpn resolvconf
