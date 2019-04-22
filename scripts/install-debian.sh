#!/usr/bin/env bash
set -eo pipefail


# Detault installation options for packages that require input
echo iptables-persistent iptables-persistent/autosave_v4 boolean false | sudo debconf-set-selections
echo iptables-persistent iptables-persistent/autosave_v6 boolean false | sudo debconf-set-selections

# Install required OS Packages
apt-get update
apt-get install -y iptables iptables-persistent make nginx openssl python3 \
    redis-server sqlite3 wget

# Ubuntu version-specific steps
if [[ ${OS_VERSION_ID} == "16.04" ]]; then
    # Ubuntu 16.04 ships with OpenVPN 2.3 only so add from official repo
    wget -O - https://swupdate.openvpn.net/repos/repo-public.gpg | apt-key add -
    echo "deb http://build.openvpn.net/debian/openvpn/release/2.4 xenial main" > \
        /etc/apt/sources.list.d/openvpn-aptrepo.list
elif [[ ${OS_VERSION_ID} == "18.04" ]]; then
    # Required by Ubuntu 18.04 for Python3
    apt-get install -y python3-distutils
fi

apt-get update
apt-get install -y openvpn

# Install latest version of PIP
curl https://bootstrap.pypa.io/get-pip.py | python3

# Remove default Nginx site
if [[ -f /etc/nginx/sites-enabled/default ]]; then
    rm /etc/nginx/sites-enabled/default
fi

systemctl enable nginx
systemctl enable redis-server
