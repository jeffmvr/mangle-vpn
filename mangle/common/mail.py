from django.conf import settings
from django.template.loader import render_to_string
from mangle.common import config, tasks


def send(recipient, subject, body):
    """
    Sends an e-mail with the given details by calling an application task.
    :return: bool
    """
    sender = config.get("smtp_reply_address", config.get("smtp_username"))
    tasks.send_email(recipient, subject, body, sender)
    return True


def send_template(recipient, subject, template, data=None):
    """
    Sends an e-mail rendered from the template at the given path.
    :return: None
    """
    send(recipient, subject, render_to_string(template, data))


def init():
    """
    Sets the Django SMTP settings from the application settings and returns
    whether the application is ready to send e-mails.
    :return: None
    """
    config.reload()

    if not is_configured():
        return False

    settings.EMAIL_BACKEND = 'django.core.mail.backends.smtp.EmailBackend'
    settings.EMAIL_USE_TLS = config.get("smtp_tls", True)
    settings.EMAIL_HOST = config.get("smtp_host")
    settings.EMAIL_PORT = config.get("smtp_port")
    settings.EMAIL_HOST_USER = config.get("smtp_username")
    settings.EMAIL_HOST_PASSWORD = config.get("smtp_password")
    return True


def is_configured():
    """
    Returns whether the application settings are configured to send e-mails.
    :return: bool
    """
    return not (not config.get("smtp_host") or
                not config.get("smtp_port") or
                not config.get("smtp_username") or
                not config.get("smtp_password"))
