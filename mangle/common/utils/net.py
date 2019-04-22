import ipaddress
import logging
import os
import socket
import netifaces

from mangle.common.utils import bash


logger = logging.getLogger(__name__)


def ip_addresses():
    """
    Returns a list containing all of the local machine's IP addresses.
    :return: List[str]
    """
    addrs = []
    for iface in interface_names("lo"):
        addrs.append(interface_ip(iface))
    return addrs


def interface_names(*ignore):
    """
    Returns a list the contains all of the network interface names for the
    local machine, ignoring any names that are prefixed from any values given
    in the `ignore` arg.
    :return: List[str]
    """
    ifaces = []

    for iface in netifaces.interfaces():
        if not ignore or not iface.startswith(tuple(ignore)):
            if interface_ip(iface):
                ifaces.append(iface)

    return ifaces


def interface_ip(iface):
    """
    Returns the IPv4 address for the given network interface. If the address
    does not exist, then an empty string is returned.
    :return: str
    """
    try:
        return netifaces.ifaddresses(iface)[netifaces.AF_INET][0]["addr"]
    except (IndexError, KeyError):
        logger.error("failed to get IP for iface: %s", iface)
        return ""


def hostname():
    """
    Returns the hostname of the local machine.
    :return: str
    """
    return socket.getfqdn()


def nameservers():
    """
    Returns a list the contains the DNS servers set for the local machine by
    parsing the '/etc/resolv.conf' file.
    :return: List[str]
    """
    values = []

    with open("/etc/resolv.conf", "r") as f:
        for line in f.read().split("\n"):
            if line.startswith("nameserver"):
                values.append(line.split()[1])

    return values


def expand_cidr(value):
    """
    Returns a string that contains both the IPv4 address and IPv4 subnet mask
    in dotted notation parsed from the given IPv4 CIDR address. If the value
    cannot be parsed (e.g. invalid), then an empty string is returned.
    :return: str
    """
    try:
        ip = ipaddress.ip_network(value)
        return ip.network_address.exploded + " " + ip.netmask.exploded
    except ValueError:
        logger.error("failed to expand CIDR value: %s", value)
        return ""


def get_interface_stats(iface):
    """
    Returns the statistics for the given network interface.
    :return: Dict
    """
    base = "/sys/class/net/{}/statistics/".format(iface)

    try:
        with open(os.path.join(base, "rx_bytes"), "r") as f:
            rx_bytes = f.read()
        with open(os.path.join(base, "tx_bytes"), "r") as f:
            tx_bytes = f.read()

        return {
            "rx": int(rx_bytes),
            "tx": int(tx_bytes),
        }
    except FileNotFoundError:
        logger.error("failed to parse stats for iface: %s", iface)
        return {}
