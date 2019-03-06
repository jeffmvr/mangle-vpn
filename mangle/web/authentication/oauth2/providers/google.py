from .base import OAuth2Provider


class GoogleOAuth2Provider(OAuth2Provider):
    """
    Provides OAuth2 authentication for Google.
    """
    authorization_url = "https://accounts.google.com/o/oauth2/v2/auth"
    token_url = "https://www.googleapis.com/oauth2/v4/token"
    profile_url = "https://www.googleapis.com/userinfo/v2/me"
    scopes = [
        "https://www.googleapis.com/auth/userinfo.email",
        "https://www.googleapis.com/auth/userinfo.profile",
    ]

    def get_login_url(self):
        """
        Appends the "prompt=select_account" query parameter to force the
        Google authentication page to display the list of accounts.
        :return: (str, str)
        """
        url, state = super().get_login_url()
        return url+"&prompt=select_account", state

    def process_response(self, resp):
        """
        Returns a tuple containing the profile e-mail address and name.
        :return: (str, str)
        """
        return resp["email"], resp["name"]
