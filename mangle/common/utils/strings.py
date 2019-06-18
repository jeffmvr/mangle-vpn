import math
import os
import random
import string


ALPHANUMERIC = string.ascii_letters + string.digits
"""str: a string that contains only alphanumeric characters which include
lowercase and uppercase ASCII letters (A-z) and digits (0-9).
"""

rng = random.SystemRandom()
"""SystemRandom: a random number generator that uses the underlying OS sources
for generating random numbers.
"""


def random_alphanumeric(length):
    """
    Returns a random alphanumeric string of the given length.
    :return: str
    """
    return "".join([rng.choice(ALPHANUMERIC) for _ in range(length)])


def remove_empty_lines(value):
    """
    Returns the given string value with all empty lines removed.
    :return: str
    """
    return os.linesep.join([s for s in value.splitlines() if s])


def secs_to_hhmmss(value):
    """
    Returns a time string formatted as 'hh mm ss' for the given seconds value.
    :return: str
    """
    value = int(value)

    if value < 60:
        return "{}s".format(value)

    hours = math.floor(value / 3600)
    mins = math.floor((value - hours * 3600) / 60)
    secs = value % 60

    if secs < 10:
        secs = "0{}".format(secs)
    if mins < 10:
        mins = "0{}".format(mins)

    if hours > 0:
        return "{}h 0{}m {}s".format(hours, mins, secs)

    return "{}m {}s".format(mins, secs)
