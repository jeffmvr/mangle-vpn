import logging

from django.contrib.auth import authenticate, login, logout
from django.shortcuts import redirect, render
from django.views.decorators.csrf import ensure_csrf_cookie
from mangle.common import config, models
from mangle.web import forms
from mangle.web.decorators import *
from mangle.web.authentication.backend import redirect_login

logger = logging.getLogger(__name__)


@install_required
@credentials_required
@mfa_required
@ensure_csrf_cookie
def show_app(request, *args, **kwargs):
    """
    Renders the web application.
    :return: Response
    """
    return render(request, "index.html")


#######################################
# Installation
#######################################

def show_install(request):
    """
    Renders the application installation page.
    :return: Response
    """
    # do not let the installation run twice
    if config.get_bool("app_installed", False):
        return redirect("/")

    return render(request, "Install.html", {
        "form": request.session.pop("form", {}),
    })


def process_install(request):
    """
    Processes the application installation form.
    :return: Response
    """
    form = forms.InstallForm(request.POST)

    if not form.is_valid():
        save_form(request, form)
        return redirect("/install")

    # save() updates the application settings and creates the administrator
    # user that was defined in the form and log the user in
    admin = form.save()
    login(request, admin)

    # set the ``app_installed`` setting to True to indicate installation has
    # been performed and the application is ready for use before redirecting
    # the user to the main application
    config.set("app_installed", True)
    return redirect("/")


#######################################
# Authentication
#######################################

@install_required
def process_oauth(request):
    """
    Processes an OAuth2 authentication callback request.
    :return: Response
    """
    user = authenticate(request)

    if not user:
        return redirect("/logout")

    login(request, user)
    return redirect("/")


@install_required
def show_login(request):
    """
    Displays the application login page.
    :return: Response
    """
    return redirect_login(request)


@install_required
def process_logout(request):
    """
    Logs the user out of the application.
    :return: Response
    """
    logout(request)
    return redirect_login(request)


#######################################
# Two-Factor Authentication
#######################################

@install_required
@credentials_required
def show_mfa(request):
    """
    Displays the two-factor authentication confirmation page.
    :return: Response
    """
    return render(request, "MfaConfirm.html", {
        "errors": request.session.pop("errors", {})
    })


@install_required
@credentials_required
def show_mfa_setup(request):
    """
    Displays the two-factor authentication setup page.
    :return: Response
    """
    if request.user.mfa_enabled:
        return redirect("/")

    return render(request, "MfaSetup.html", {
        "errors": request.session.pop("errors", {})
    })


@install_required
@credentials_required
def process_mfa(request):
    """
    Verifies the submitted two-factor authentication code for the current user.
    :return: Response
    """
    code = request.POST.get("code")

    if not request.user.verify_mfa_code(code):
        request.session["errors"] = {"code": "Invalid authentication code."}

        models.Event.objects.create(
            name="web.error",
            user=request.user,
            detail="Incorrect two-factor authentication code"
        )

        # if the user doesn't have two-factor authentication enabled on their
        # account then they get redirected to the two-factor setup page
        if not request.user.mfa_enabled:
            return redirect("/mfa/setup")

        return redirect("/mfa")

    # if the user has confirmed their two-factor authentication code then make
    # sure two-factor authentication is enabled for their account
    if not request.user.mfa_enabled:
        request.user.mfa_enabled = True
        request.user.save()

    models.Event.objects.create(
        name="web.login",
        user=request.user,
        detail="Logged in to web application."
    )

    request.session["mfa_confirmed"] = True
    return redirect("/")


#######################################
# Context Processors
#######################################

def base_context_processor(request):
    """
    The base UI context processor.
    :return: dict
    """
    return {
        "organization": config.get("app_organization", "Mangle"),
    }


#######################################
# Helpers
#######################################

def save_form(request, form):
    """
    Saves the given form data and errors in the request session.
    :return: None
    """
    request.session["form"] = {
        "data": form.data,
        "errors": form.errors,
    }
