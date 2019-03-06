from django.apps import AppConfig
from django.db.utils import OperationalError
from django.template import defaultfilters
from mangle.common.utils import net


class CommonConfig(AppConfig):
    name = "mangle.common"

    def ready(self):
        from mangle.common import config, signals

        # set default application settings
        try:
            interfaces = net.interface_names("lo", "tun")

            config.reload()
            config.set_default("app_installed", False)
            config.set_default("app_organization", "Mangle")
            config.set_default("app_http_port", 80)
            config.set_default("app_https_port", 443)
            config.set_default("pki_key_size", 4096)
            config.set_default("smtp_reply_address", "Mangle VPN")
            config.set_default("vpn_interface", interfaces[0])
            config.set_default("vpn_nat_interface", interfaces[0])
            config.set_default("vpn_port", 1194)
            config.set_default("vpn_protocol", "udp")
            config.set_default("vpn_subnet", "172.25.0.0/16")
        except OperationalError:
            # an OperationalError is raised when attempting to load the
            # application settings before the DB has been created, which will
            # happen when calling ``makemigrations`` or ``migrate``
            pass

        # register template filters
        defaultfilters.register.filter("expand_cidr", net.expand_cidr)


default_app_config = "mangle.common.CommonConfig"
