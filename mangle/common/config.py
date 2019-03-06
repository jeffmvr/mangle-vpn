import os

from django.conf import settings
from mangle.common import models


_settings = {}
"""dict: contains all of the application settings.

This dict will hold an in-memory collection of the application setting that
have been loaded from the database via reload(), preventing the need to run
multiple SELECT queries each time a value is requested.
"""


def all():
    """
    Returns all of the application settings.
    :return: dict
    """
    return _settings


def get(name, default=None):
    """
    Returns the value of the given application setting. If the setting does not
    exist, then the given `default` value is returned.
    :return: Any
    """
    return _settings.get(name, default)


def get_int(name, default=0):
    """
    Returns the value of the given application setting as an int.
    :return: int
    """
    return int(get(name, default))


def get_float(name, default=0.0):
    """
    Returns the value of the given application setting as a float.
    :return: float
    """
    return float(get(name, default))


def get_bool(name, default=False):
    """
    Returns the value of the given application setting as a bool by checking
    for 'truthy' string values.
    :return: bool
    """
    value = get(name, default)

    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value.lower() in ("true", "yes", "1", "on")

    return default


def get_list(name, sep="\n"):
    """
    Returns the value of the given application setting as a list, split by the
    given `sep` string.
    :return: List
    """
    return get(name, "").split(sep)


def has(name):
    """
    Returns whether the given application setting exists.
    :return: bool
    """
    return name in _settings


def set(name, value):
    """
    Sets the value of the given application setting and updates the database
    immediately. If the setting does not exist, then it is created.
    :return: None
    """
    models.Setting.objects.update_or_create({"value": value}, name=name)
    _settings[name] = value


def set_default(name, value):
    """
    Sets the value of the given application setting only if the setting doesn't
    exist. If the setting exists, then this does nothing.
    :return: None
    """
    if name not in _settings:
        set(name, value)


def set_list(name, value, sep="\n"):
    """
    Sets the value of the given application setting by joining the given list
    of `value` by the given `sep` string.
    :return: None
    """
    set(name, sep.join([str(v) for v in value]))


def delete(name):
    """
    Deletes the given application setting.
    :return: None
    """
    models.Setting.objects.filter(name__iexact=name).delete()

    if has(name):
        del _settings[name]


def reload():
    """
    Reloads the application settings from the database.
    :return: None
    """
    for setting in models.Setting.objects.all():
        _settings[setting.name] = setting.value


def url(*paths, **params):
    """
    Returns an application URL with the given paths relative to the application
    base URL and with the given URL parameters.
    :return: str
    """
    hostname = get("app_hostname")
    port = get("app_https_port")

    # the base URL will use HTTPs by default
    value = "https://{}".format(hostname)

    # if the HTTPs port is not the standard 443 value, then append to URL
    if port != "443":
        value += ":{}".format(port)

    # add each of the URL paths
    value += "/" + "/".join(paths)

    # add each of the URL keyword parameters
    for key, val in params.items():
        value += "?" + "{}={}&".format(key, val)
    value = value.rstrip("&")

    return value.lower()
