import logging

from mangle.common.models import User
from mangle.web.authentication import oauth2


logger = logging.getLogger(__name__)


class OAuth2Backend:
    """
    OAuth2Backend defines an interface for providing user authentication and
    retrieval of user details from an OAuth2 provider.
    """

    def authenticate(self, request):
        """
        Authenticates and returns the User based on the OAuth2 code provided
        in the callback request from the OAuth2 provider.
        :return: User
        """
        if "oauth_state" not in request.session:
            logger.error("auth fail: missing oauth state in session")
            return None
        if "state" not in request.GET:
            logger.error("auth fail: missing oauth state in request")
            return None
        if "code" not in request.GET:
            logger.error("auth fail: missing oauth code in request")
            return None

        # compare the state value stored in the current session with the state
        # value that was provided in the request params to verify the request
        # was made by the same user
        if request.session["oauth_state"] != request.GET["state"]:
            logger.error("auth fail: mismatched oauth state")
            return None

        # Use the current OAuth2 provider to retrieve the User's e-mail address
        # and name so they can be verified and set
        email, name = oauth2.get_provider().get_profile(request.GET["code"])

        # Retrieve the User with the authenticated e-mail address and verify
        # that they exist and are not disabled
        user = User.objects.by_email(email)
        if not user:
            logger.error("auth fail: user does not exist - %s", email)
            return None
        elif not user.is_active:
            logger.error("auth fail: user is inactive - %s", email)
            return None

        if not user.name:
            user.name = name

        user.save()
        return user

    def get_user(self, user_id):
        """
        Returns the user with the given ID.
        :return: User
        """
        return User.objects.filter(pk=user_id).first()
