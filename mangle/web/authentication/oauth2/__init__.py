import logging

from mangle.common import config
from .providers.google import GoogleOAuth2Provider


logger = logging.getLogger(__name__)


def get_provider():
    """
    Returns the OAuth2 provider from the application settings.
    :return: OAuth2Provider
    """
    provider = config.get("oauth2_provider", "google")

    if provider == "google":
        return GoogleOAuth2Provider()

    raise ValueError("unknown oauth2 provider: " + provider)
