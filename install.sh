#!/usr/bin/env bash
set -eo pipefail


# This script must be run as root
if [[ "$EUID" -ne 0 ]]
  then echo "Please run as root or using sudo."
  exit
fi

CURDIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd)"

# Export OS details for child scripts to use
export OS_ID=$(cat /etc/os-release | grep ^ID= | cut -d "=" -f 2 | tr -d '"')
export OS_VERSION_ID=$(cat /etc/os-release | grep VERSION_ID | cut -d "=" -f 2 | tr -d '"')


#
# Generates and returns a new application secret key.
#
function generateSecretKey() {
    cat /dev/urandom | tr -dc 'abcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*(-_=+)' | fold -w ${1:-50} | head -n 1
}

#
# Creates a directory only if it doesn't exist.
#
function mkdirIfNotExists() {
    if [[ ! -d "$1" ]]; then
        mkdir -p "$1"
    fi
}

# Determine which installation script to run based on the operating system
case ${OS_ID} in
    ubuntu|debian)
        ${CURDIR}/scripts/install-debian.sh
        IPTABLES_SAVE="iptables-save > /etc/iptables/rules.v4"
        ;;
    fedora|centos)
        ${CURDIR}/scripts/install-redhat.sh
        IPTABLES_SAVE="service iptables save"
        ;;
    *)
        echo "Unsupported OS: ${OS_ID}"
        exit 1
        ;;
esac

# Enable IPv4 forwarding (required for OpenVPN)
echo "net.ipv4.ip_forward=1" > /etc/sysctl.conf
sysctl -p

# Flush any existing iptables rules and add rules
# Allows established, SSH, and localhost traffic
iptables -F
iptables -A INPUT -m conntrack --ctstate ESTABLISHED -j ACCEPT
iptables -A INPUT -p tcp --dport 22 -j ACCEPT
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT
iptables -P INPUT DROP
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
eval ${IPTABLES_SAVE}

# Upgrade pip and install application requirements
pip3 install --upgrade pip
pip3 install -r requirements.txt

# Create the application directories
mkdirIfNotExists ${CURDIR}/data/keys
mkdirIfNotExists ${CURDIR}/data/logs
mkdirIfNotExists ${CURDIR}/data/systemd

# Generate the application's secret key
echo "$(generateSecretKey)" > ${CURDIR}/data/keys/secret.key
chmod 600 ${CURDIR}/data/keys/secret.key

# Create database and set proper permissions
python3 manage.py migrate
chown root:root ${CURDIR}/data/mangle.db
chmod 600 ${CURDIR}/data/mangle.db

# perform the application initialization
python3 manage.py install

# Start the web application
systemctl restart mangle-web
