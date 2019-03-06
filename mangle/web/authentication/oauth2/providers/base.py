import abc
import logging

from requests_oauthlib import OAuth2Session
from mangle.common import config


logger = logging.getLogger(__name__)


class OAuth2Provider:
    """
    OAuth2Provider defines methods and properties for making OAuth2 requests
    to an OAuth2 provider and should be used as the base class for supported
    OAuth2 providers.
    """
    authorization_url = None
    token_url = None
    profile_url = None
    scopes = None

    def __init__(self):
        self.logger = logger

    @property
    def client_id(self):
        """
        Returns the OAuth2 client ID.
        :return: str
        """
        return config.get("oauth2_client_id")

    @property
    def client_secret(self):
        """
        Returns the OAuth2 client secret.
        :return: str
        """
        return config.get("oauth2_client_secret")

    @property
    def redirect_uri(self):
        """
        Returns the OAuth2 redirect URI.
        :return: str
        """
        return config.url("oauth")

    @abc.abstractmethod
    def process_response(self, resp):
        """
        This method must be implemented by child classes and parses the
        response returned by the OAuth2 provider, returning the user's e-mail
        address.
        :return: Tuple[str,str]
        """
        pass

    def get_login_url(self):
        """
        Returns the URL to the OAuth2 provider login page and state.
        :return: (str, str)
        """
        return self.get_session().authorization_url(self.authorization_url)

    def get_session(self):
        """
        Returns an OAuth2 session.
        :return: OAuth2Session
        """
        return OAuth2Session(
            client_id=self.client_id,
            redirect_uri=self.redirect_uri,
            scope=self.scopes,
        )

    def get_profile(self, code):
        """
        Returns the OAuth2 profile e-mail address and full name.
        :return: (str, str)
        """
        session = self.get_session()

        # use the OAuth2 session to fetch the access token that will be used
        # to retrieve the user's profile details
        session.fetch_token(
            code=code,
            client_secret=self.client_secret,
            token_url=self.token_url,
        )

        # request the user profile
        resp = session.get(self.profile_url)

        # the only acceptable response status code is 200
        if resp.status_code != 200:
            logger.error("oauth2 failed: unable to retrieve oauth2 profile")
            logger.error(resp.text)
            return None

        try:
            return self.process_response(resp.json())
        except ValueError:
            logger.error("oauth2 failed: unabled to decode json response")
            return None
