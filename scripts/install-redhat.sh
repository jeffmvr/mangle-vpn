#!/usr/bin/env bash
set -eo pipefail


# disable firewalld (causes issues with iptables in some instances)
systemctl disable firewalld

# Install OS packages
yum install -y epel-release
yum install -y iptables iptables-services make nginx openssl openvpn redis \
    sqlite wget

# Install Python3 (not available from official Red Hat repos) and symlink binary
yum install -y https://centos7.iuscommunity.org/ius-release.rpm
yum install -y python36u
ln -sf /usr/bin/python3.6 /usr/bin/python3

# Install pip3.6
curl https://bootstrap.pypa.io/get-pip.py | python3.6
ln -sf /usr/bin/pip3.6 /usr/bin/pip3

# Remove the listen directive from the main Nginx config (we need our app to be default)
sed -i '/listen/d' /etc/nginx/nginx.conf
if [[ -f /etc/nginx/conf.d/default ]]; then
    rm /etc/nginx/conf.d/default
fi

# Enable services
chkconfig iptables on
systemctl enable iptables
systemctl enable redis
systemctl start redis
systemctl enable nginx
systemctl start nginx
