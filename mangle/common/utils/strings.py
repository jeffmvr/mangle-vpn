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
