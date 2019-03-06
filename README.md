# Mangle VPN
an OpenVPN management web application for mere mortals.

## Overview
Mangle VPN is an OpenVPN management application that allows administrators an
easy way to offer self-service VPN access to users. It is heavily inspired by
the OpenVPN Access Server.

The backend application is written in Python 3.6 and uses the Django web 
framework backed by an SQLite database, and requires OpenVPN 2.4+.

The frontend is uses Vue.js and the Semantic UI CSS framework.

## Getting Started
*The application should be installed on a fresh machine that is dedicated to 
running nothing but the web application and OpenVPN server.*

## Features
- Self-service OpenVPN device provisioning
- User groups with group specific firewall rules
- Certificate authority management
- E-mail notifications
- Two-factor authentication
- OAuth2 authentication
- Web based administration
- Real-time control of OpenVPN clients

### Installation
#### Notes
Before beginning the installation, please note:
- You must have **Git** installed
- If **SELinux** is enabled, you must provide your own policies
- It is recommended that you install the application in **/opt**, but if 
installing in a home directory, ensure permissions on **all** parent
directories allow for the root user to read

Clone the repository and run the install script:
```bash
$ git clone https://github.com/jeffmvr/mangle-vpn.git
$ cd mangle-vpn
$ sudo ./install.sh
``` 
Once the installation script has finished, please navigate to the frontend
application in your browser to perform the initial setup. 

### Authentication
Authentication is performed using one of the supported OAuth2 providers list
below, which means you must first obtain an OAuth2 client ID and secret before 
you will be allowed to login.

**Supported Providers:**
- Google

## Configuration

### SSL
By default, a self-signed SSL certificate is created for the web application. 
While this is OK for testing purposes, you will want to use a valid SSL
certificate signed by a trusted certificate authority.

The **Settings** page in the administration section provides you with the
ability to copy/paste your own SSL certificates.

### Firewall
The application manages all of the firewall rules via iptables for both the web
application and OpenVPN server (based on your application settings), and sets 
the following default policies and rules:
```bash
-P INPUT DROP
-P OUTPUT ACCEPT
-P FORWARD ACCEPT
-A INPUT -m conntrack --ctstate ESTABLISHED -j ACCEPT
-A INPUT -p tcp --dport 22 -j ACCEPT
```

### Application Data
All application data, including the database, logs, and application keys are
stored in the <app-path>/data directory. This is the only directory required
when backing up the machine.

## Starting/Stopping
The web application and OpenVPN server can be controlled using standard systemd
services: 
```bash
$ sudo systemctl [start|stop|restart|status] mangle-web.service
$ sudo systemctl [start|stop|restart|status] mangle-vpn.service
```

In addition, there is also a tasks service that is running which handles various
background tasks, such as sending e-mails and generating the OpenVPN CRL and
is started/stopped alongside the mangle-web service:
```bash
$ sudo systemctl [start|stop|restart|status] mangle-tasks.service
```

## Updating
You can pull the latest release code by running the following command:
```bash
$ sudo make update
```
This will restart the web application but **does not restart the OpenVPN
server**.

## Security
While much effort has been put forth to ensure the application is as secure as
possible, best practices should always be followed in order to harden the local 
machine.

## Todo List
The following items are in no particular order and represent features that are
to be added in the future.
- [ ] Support additional OAuth2 providers
- [ ] Support LDAP authentication
- [ ] Group specific device expiration time limits
- [x] Add web-based setup
- [x] Support setting specific operations (instead of all settings at once)
- [ ] REST API for developer consumption
- [ ] Update application from web UI
- [ ] Automate the configuration and use of Let's Encrypt from web UI
- [ ] User Linux installation scripts
- [x] Add license
