

__MAJOR__ = 0
"""Increment when making incompatible API changes."""

__MINOR__ = 3
"""Increment when adding backwards compatible functionality."""

__PATCH__ = 0
"""Increment when making backwards compatible bug fixes."""


def version():
    """
    Returns the full version number.
    :return: str
    """
    return "{}.{}.{}".format(__MAJOR__, __MINOR__, __PATCH__)
