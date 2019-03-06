import logging
import socket

from django.conf import settings
from django.template.loader import render_to_string
from mangle.common import config, pki
from mangle.common.utils import bash, net, strings


logger = logging.getLogger(__name__)


def create_server_keys():
    """
    Creates and sets the OpenVPN server keys.
    :return: None
    """
    crt, key = pki.create_server_keypair("OpenVPN Server", 3650).pem()
    config.set("vpn_crt", crt)
    config.set("vpn_key", key)
    config.set("vpn_dh_params", pki.create_dh_params(2048))
    config.set("vpn_tls_auth_key", create_tls_auth_key())


def start():
    """
    Starts the OpenVPN server.
    :return: bool
    """
    return bash.run("systemctl", "start", "mangle-vpn")


def stop():
    """
    Stops the OpenVPN server.
    :return: bool
    """
    return bash.run("systemctl", "stop", "mangle-vpn")


def restart():
    """
    Restarts the OpenVPN server.
    :return: bool
    """
    return bash.run("systemctl", "restart", "mangle-vpn")


def is_running():
    """
    Returns whether the OpenVPN server is running.
    :return: bool
    """
    return bash.run("systemctl", "status", "mangle-vpn")


def create_tls_auth_key():
    """
    Generates and returns a new OpenVPN TLS auth key.
    :return: str
    """
    code, out, err = bash.run_output("openvpn --genkey --secret /dev/stdout")
    return out


def server_config():
    """
    Returns the OpenVPN server configuration generated from the application
    settings.
    :return: str
    """
    conf = render_to_string("openvpn/server.conf", {
        "root_dir": settings.BASE_DIR,
        "bind_address": net.interface_ip(config.get("vpn_interface")),
        "bind_port": config.get("vpn_port"),
        "ca_crt": config.get("ca_crt"),
        "crl_file": settings.PKI_CRL_FILE,
        "dh_params": config.get("vpn_dh_params"),
        "domain": config.get("domain"),
        "log_file": settings.OPENVPN_LOG_FILE,
        "managment_socket": settings.OPENVPN_MANAGEMENT_SOCKET,
        "nameservers": config.get_list("vpn_nameservers"),
        "protocol": config.get("vpn_protocol"),
        "routes": config.get_list("vpn_routes"),
        "server_crt": config.get("vpn_crt"),
        "server_key": config.get("vpn_key"),
        "status_file": settings.OPENVPN_STATUS_FILE,
        "subnet": config.get("vpn_subnet"),
        "tls_auth_key": config.get("vpn_tls_auth_key"),
    })
    return strings.remove_empty_lines(conf)


def client_config(crt, key, is_linux=False):
    """
    Returns an OpenVPN client configuration for the given client certificate
    and private key. If `is_linux` is True then the proper DNS scripts commands
    are added.
    :return: str
    """
    conf = render_to_string("openvpn/client.conf", {
        "ca_crt": config.get("ca_crt"),
        "client_crt": crt,
        "client_key": key,
        "hostname": config.get("vpn_hostname"),
        "is_linux": is_linux,
        "port": config.get("vpn_port"),
        "protocol": config.get("vpn_protocol"),
        "tls_auth_key": config.get("vpn_tls_auth_key"),
    })
    return strings.remove_empty_lines(conf)


def kill_client(name):
    """
    Kills the OpenVPN client connection with the given name.
    :return: None
    """
    with management() as m:
        m.run("kill", name)


def management():
    """
    Returns the application's OpenVPN management interface.
    :return: Management
    """
    return Management(settings.OPENVPN_MANAGEMENT_SOCKET)


class Management:
    """
    Management is a class that exposes an interface for interacting with the
    OpenVPN management UNIX socket.
    """
    def __init__(self, path):
        self.path = path
        self.sock = None

    def __enter__(self):
        """
        Returns the Management object when used as context manager.
        :return: Management
        """
        self.connect()
        self._recv()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """
        Closes the socket connection when used as context manager.
        :return: None
        """
        self.quit()

    def run(self, *args):
        """
        Sends the given command to the management socket and returns the data
        that was received in response. This will
        :return: str
        """
        self._send(*args)
        return self._recv()

    def quit(self):
        """
        Sends the quit command and closes the socket connection.
        :return: None
        """
        self._send("quit")
        self.sock.close()

    def connect(self):
        """
        Initializes the socket connection.
        :return: None
        """
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)

        try:
            sock.connect(self.path)
            sock.settimeout(1)
            self.sock = sock
        except socket.error as exc:
            logger.error("failed to open managmenet socket")
            logger.error(exc)
            raise ValueError("failed to open management socket")

    def _recv(self):
        """
        Returns the data received from the socket. This will continue to read
        in 1024 byte chunks until an '\r\n' is read which indicates the end of
        the data stream.
        :return: str
        """
        data = ""
        while True:
            data += self.sock.recv(1024).decode("utf-8")
            if data.endswith("\r\n"):
                break
        return data

    def _send(self, *args):
        """
        Sends the given command to the management interface.
        :return:
        """
        self.sock.sendall(bytes(" ".join(args) + "\r\n", "utf-8"))
