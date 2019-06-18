import logging
import os
import pathlib
import sys

from django.conf import settings
from django.template.loader import render_to_string
from django.utils import timezone
from mangle.cli.command import BaseCommand
from mangle.common import config, iptables, models, openvpn
from mangle.common.utils import net, strings


logger = logging.getLogger("cli")


class Command(BaseCommand):
    help = "performs the initial application installation"

    def add_arguments(self, parser):
        parser.add_argument("action", help="the web action to perform")
        parser.add_argument("args", default=list(), nargs="?")

    def handle(self, *args, **options):
        action = options["action"]

        if action == "pre-start":
            vpn_pre_start()
        elif action == "post-start":
            vpn_post_start()
        elif action == "post-stop":
            vpn_post_stop()
        elif action == "client-authenticate":
            vpn_client_authenticate()
        elif action == "client-connect":
            vpn_client_connect()
        elif action == "client-disconnect":
            vpn_client_disconnect()
        else:
            self.exit("unknown action: %s", action)


######################################
# Actions
######################################

def vpn_pre_start():
    """
    Generates the OpenVPN server configuration file.
    :return: None
    """
    with open(settings.OPENVPN_CONFIG_FILE, "w") as f:
        f.write(openvpn.server_config())

    # this ensures the CRL file itself exists, regardless of whether it's an
    # actual CRL and stops OpenVPN from failing to start when the file is gone
    pathlib.Path(settings.PKI_CRL_FILE).touch()


def vpn_post_start():
    """
    Creates the OpenVPN firewall rules.
    :return: None
    """
    vpn_post_stop()
    iptables.create_chain("filter", "MangleVPN")
    iptables.create_chain("filter", "MangleVPN_Clients")

    rules = render_rules()
    for rule in rules.split("\n"):
        iptables.run(rule)

    for group in models.Group.objects.filter(is_enabled=True).all():
        group.create_firewall_chain()

    config.set("vpn_firewall_rules", rules)
    config.set("vpn_restart_pending", False)


def vpn_post_stop():
    """
    Deletes the OpenVPN firewall rules.
    :return: None
    """
    rules = config.get("vpn_firewall_rules", "")

    for rule in rules.split("\n"):
        iptables.run(rule.replace("-A", "-D"))

    # delete the application chains
    iptables.delete_chain("filter", "MangleVPN")
    iptables.delete_chain("filter", "MangleVPN_Clients")

    config.delete("vpn_firewall_rules")

    # delete all of the group chains
    for group in models.Group.objects.filter(is_enabled=True).all():
        group.delete_firewall_chain()

    # delete all clients to avoid potential stale clients persisting through
    # dirty restarts or reboots
    models.Client.objects.all().delete()


def vpn_client_authenticate():
    """
    Handles OpenVPN client user authentication.
    :return: None
    """
    username = os.environ["username"]
    password = os.environ["password"]

    user = models.User.objects.by_email(username)

    if not user:
        print("user with username {} not found".format(username))
        sys.exit(1)

    if not user.is_active:
        models.Event.objects.create(
            name="vpn.error",
            user=user,
            detail="User is not currently active.",
        )
        sys.exit(1)

    if user.mfa_required and not user.verify_mfa_code(password):
        models.Event.objects.create(
            name="vpn.error",
            user=user,
            detail="User two-factor authentication code invalid.",
        )
        sys.exit(1)


def vpn_client_connect():
    """
    Handles OpenVPN clients connecting (after authentications).
    :return: None
    """
    common_name = os.environ["common_name"]
    platform = os.environ["IV_PLAT"]
    fingerprint = os.environ["tls_digest_0"]
    virtual_ip = os.environ["ifconfig_pool_remote_ip"]
    remote_ip = os.environ["trusted_ip"]
    trusted_port = os.environ["trusted_port"]

    device = models.Device.objects.by_fingerprint(fingerprint)

    if not device:
        print("unknown device with fingerprint: {}".format(fingerprint))
        sys.exit(1)

    # create the device client
    models.Client.objects.create(
        common_name=common_name,
        device=device,
        platform=platform,
        remote_ip=remote_ip + ":" + trusted_port,
        virtual_ip=virtual_ip,
    )

    # create the user event
    models.Event.objects.create(
        name="vpn.connect",
        user=device.user,
        detail="Device {} connected from {}".format(device.name, remote_ip),
    )

    device.last_login = timezone.now()
    device.save()


def vpn_client_disconnect():
    """
    Handles OpenVPN clients disconnecting.
    :return: None
    """
    common_name = os.environ["common_name"]
    remote_ip = os.environ["trusted_ip"]

    client = models.Client.objects.by_common_name(common_name)
    if client:
        models.Event.objects.create(
            name="vpn.disconnect",
            user=client.device.user,
            detail="Device {} disconnected from {} after {}".format(
                client.device.name,
                remote_ip,
                strings.secs_to_hhmmss(client.duration),
            ),
        )

        client.delete()


def render_rules():
    """
    Returns the base OpenVPN rules.
    :return: str
    """
    return render_to_string(
        template_name="firewall/vpn.rules",
        context={
            "interface": config.get("vpn_interface"),
            "nat_interface": config.get("vpn_nat_interface"),
            "local_addrs": net.ip_addresses(),
            "port": config.get("vpn_port"),
            "protocol": config.get("vpn_protocol"),
            "nameservers": config.get_list("vpn_nameservers"),
            "subnet": config.get("vpn_subnet"),
        },
    )
