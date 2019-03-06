from django.shortcuts import redirect
from mangle.common import config
from mangle.web.authentication import oauth2


def redirect_login(request):
    """
    Returns the login URL based on the current authentication backend.
    :return: Any
    """
    backend = config.get("auth_backend", "oauth2")

    if backend == "oauth2":
        url, state = oauth2.get_provider().get_login_url()
        request.session["oauth_state"] = state
        return redirect(url)
