from django.shortcuts import redirect
from mangle.common import config


def credentials_required(func):
    """
    View decorator that redirects the user to the login page if they have to
    authenticate. This does NOT check whether they have confirmed their
    two-factor authentication code.
    :return: Response
    """
    def decorator(request, *args, **kwargs):
        if not request.user.is_authenticated:
            return redirect("/login")
        return func(request, *args, **kwargs)
    return decorator


def mfa_required(func):
    """
    View decorator that redirects the current user to the proper page if they
    have not setup two-factor authentication or have not confirmed their code
    for the current session. Only applies to users who are have two-factor
    authentication required.
    :return: Response
    """
    def decorator(request, *args, **kwargs):
        if request.user.mfa_required:
            if not request.user.mfa_enabled:
                return redirect("/mfa/setup")
            if not request.session.get("mfa_confirmed", False):
                return redirect("/mfa")
        return func(request, *args, **kwargs)
    return decorator


def install_required(func):
    """
    View decorator that redirects the current user to the install page if the
    app has yet to be installed.
    :return: Response
    """
    def decorator(request, *args, **kwargs):
        if not config.get_bool("app_installed", False):
            return redirect("/install")
        return func(request, *args, **kwargs)
    return decorator
