from rest_framework.authentication import exceptions, SessionAuthentication


class CredentialAuthentication(SessionAuthentication):
    """
    Validates that the current user has logged in with their credentials but
    does not check for two-factor authentication.
    """
    def authenticate(self, request):
        user = super().authenticate(request)

        if isinstance(user, tuple):
            user = user[0]

        if not user or not user.is_authenticated:
            raise exceptions.AuthenticationFailed("NotLoggedIn")

        return user


class ApiSessionAuthentication(CredentialAuthentication):
    """
    Validates that the current user has two-factor authentication enabled and
    has confirmed their current two-factor authentication code.
    """
    def authenticate(self, request):
        user = super().authenticate(request)

        if user.mfa_required:
            if not user.mfa_enabled:
                raise exceptions.AuthenticationFailed("MfaNotEnabled")
            if not request.session.get("mfa_confirmed", False):
                raise exceptions.AuthenticationFailed("MfaNotConfirmed")

        return user, None
