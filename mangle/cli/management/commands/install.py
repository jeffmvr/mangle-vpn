import multiprocessing

from django.conf import settings
from django.template.loader import render_to_string
from mangle.cli.command import BaseCommand
from mangle.common import config, openvpn, pki, tasks
from mangle.common.utils import bash, fs, net


class Command(BaseCommand):
    help = "performs the initial application installation"

    def handle(self, *args, **options):
        """
        Runs the application installation.
        :return: None
        """
        self.title()
        self.header("Running the Mangle VPN installer...")
        pki.create_certificate_authority()
        self.ok("  - certificate authority keys created.")
        tasks.create_crl()
        self.ok("  - crl created.")
        openvpn.create_server_keys()
        self.ok("  - openvpn keys created.")
        create_web_keys()
        self.ok("  - ssl keys created.")
        create_web_vhost()
        self.ok("  - nginx vhost created.")
        create_web_unit()
        self.ok("  - web systemd unit created.")
        create_vpn_unit()
        self.ok("  - vpn systemd unit created.")
        create_tasks_unit()
        self.ok("  - tasks systemd unit created.")
        bash.run("systemctl", "daemon-reload")
        self.newline()
        self.ok("Installation finished!")
        self.print(
            "Please visit the web UI at https://{} to complete the application "
            "setup process", net.ip_addresses()[0],
        )
        self.newline()


######################################
# Tasks
######################################

def create_tasks_unit():
    """
    Creates the 'mangle-tasks' systemd service unit file.
    :return: None
    """
    _create_unit_from_template(
        path=settings.SYSTEMD_TASKS_FILE,
        template="install/systemd/mangle-tasks.service",
        data={
            "root_dir": settings.BASE_DIR,
        })


######################################
# VPN
######################################

def create_vpn_unit():
    """
    Creates the 'mangle-vpn' systemd service unit file.
    :return: None
    """
    _create_unit_from_template(
        path=settings.SYSTEMD_VPN_FILE,
        template="install/systemd/mangle-vpn.service",
        data={
            "openvpn": which("openvpn"),
            "ovpn_config": settings.OPENVPN_CONFIG_FILE,
            "root_dir": settings.BASE_DIR,
        })


######################################
# Web
######################################

def create_web_keys():
    """
    Creates the SSL certificate, private key, and Diffie-Hellman parameters.
    :return: None
    """
    crt, key = pki.create_keypair("vpn", 3650, True, True).pem()
    fs.write_file(settings.WEB_SSL_CRT_FILE, crt, 0o600)
    fs.write_file(settings.WEB_SSL_KEY_FILE, key, 0o600)
    fs.write_file(settings.WEB_SSL_DH_FILE, pki.create_dh_params(2048), 0o600)


def create_web_unit():
    """
    Creates the 'mangle-web' systemd service unit file.
    :return: None
    """
    _create_unit_from_template(
        path=settings.SYSTEMD_WEB_FILE,
        template="install/systemd/mangle-web.service",
        data={
            "access_log": settings.WEB_ACCESS_LOG_FILE,
            "error_log": settings.WEB_ERROR_LOG_FILE,
            "gunicorn": which("gunicorn"),
            "root_dir": settings.BASE_DIR,
            "socket": settings.WEB_WSGI_SOCKET,
            "workers": multiprocessing.cpu_count() * 2,
        })


def create_web_vhost():
    """
    Creates the web application Nginx virtual host file.
    :return: None
    """
    content = render_to_string("install/nginx/mangle-web.conf", {
        "hostname": config.get("app_hostname"),
        "http_port": config.get("app_http_port", 80),
        "https_port": config.get("app_https_port", 443),
        "root_dir": settings.BASE_DIR,
        "ssl_crt": settings.WEB_SSL_CRT_FILE,
        "ssl_key": settings.WEB_SSL_KEY_FILE,
        "ssl_dh": settings.WEB_SSL_DH_FILE,
        "wsgi_socket": settings.WEB_WSGI_SOCKET,
    })

    fs.write_file(settings.WEB_VHOST_FILE, content, 0o755)
    bash.run("systemctl", "restart", "nginx")


######################################
# Helpers
######################################

def _create_unit_from_template(path, template, data=None):
    """
    Creates and enables a systemd unit service file at the given path for the
    given Django template.
    :return: None
    """
    fs.write_file(path, render_to_string(template, data), 0o755)
    bash.run("systemctl", "enable", path)


def which(name):
    """
    Returns the path of the executable with the given name.
    :return: str
    """
    code, out, err = bash.run_output("which", name)
    return out
