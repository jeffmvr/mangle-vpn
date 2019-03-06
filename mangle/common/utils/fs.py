import logging
import os


logger = logging.getLogger(__name__)


def read_file(path, offset=0, size=-1):
    """
    Returns the contents from the file at the given path. If an `offset` value
    is given, then begins reading from that byte offset. If a `size` value is
    given, then only reads up to that number of bytes.=
    :return: str
    """
    with open(path, "r") as f:
        f.seek(offset)
        return f.read(size)


def append_file(path, content, perms=None):
    """
    Appends the given content to the end of the file at the given path and sets
    the given file permissions. If the file does not exist, then it is created.
    :return: None
    """
    with open(path, "a") as f:
        f.write(content)

    if perms and isinstance(perms, int):
        os.chmod(path, perms)


def write_file(path, content, perms=None):
    """
    Writes the given content to the file at the given path and sets the given
    file permissions. If the file does not exist, then it is created. If it
    does exist, the current contents are overwritten.
    :return: None
    """
    with open(path, "w") as f:
        f.write(content)

    if perms and isinstance(perms, int):
        os.chmod(path, perms)
