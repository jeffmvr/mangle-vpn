import ipaddress
import re

from django.core import validators


def is_cidr(value):
    """
    Returns whether the given valus is an IPv4 CIDR address.
    :return: bool
    """
    try:
        ipaddress.ip_network(value, strict=False)
        return True
    except ValueError:
        return False


def is_cidr_network(value):
    """
    Returns whether the given value is an IPv4 CIDR network address.
    :return: bool
    """
    try:
        return ipaddress.ip_network(value, strict=True).prefixlen <= 30
    except ValueError:
        return False


def is_ipv4(value):
    """
    Returns whether the given value is an IPv4 address.
    :return: bool
    """
    try:
        ipaddress.ip_address(value)
        return True
    except ValueError:
        return False


def is_port(value):
    """
    Returns whether the given value is a valid TCP or UDP port number.
    :return: bool
    """
    if isinstance(value, str) and not value.isdigit():
        return False

    return 1 < int(value) < 65535


def is_email(value):
    """
    Returns whether the given value is an e-mail address.
    :return: bool
    """
    try:
        validators.validate_email(value)
        return True
    except validators.ValidationError:
        return False


def is_domain(value):
    """
    Returns whether the givne value is a domain name.
    :return: bool
    """
    return re.match("^([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$", value)


def is_valid_password(value, length=8):
    """
    Returns whether the value is meets the application password complexity
    requirements.
    :return: bool
    """
    has_lower = has_upper = has_digit = False

    for ch in value:
        if ch.islower():
            has_lower = True
        elif ch.isupper():
            has_upper = True
        elif ch.isdigit():
            has_digit = True

    return len(value) >= length and has_lower and has_upper and has_digit
