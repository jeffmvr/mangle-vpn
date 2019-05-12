import logging

from django.conf import settings
from django.core.mail import EmailMultiAlternatives
from huey import crontab
from huey.contrib.djhuey import task, db_periodic_task
from mangle.common import mail, models, openvpn, pki


logger = logging.getLogger(__name__)


@db_periodic_task(crontab(minute=0, hour="*"))
def create_crl():
    """
    Creates and writes the application certificate revocation list file.
    :return: None
    """
    serials = []

    # create a list that contains all of the revoked certificate serial numbers
    for revoked in models.RevokedDevice.objects.all():
        serials.append(revoked.serial)

    with open(settings.PKI_CRL_FILE, "w") as f:
        f.write(pki.create_crl(*serials))

    logger.info("CRL generated for {} revoked certs".format(len(serials)))


@task()
def send_email(recipient, subject, body, sender):
    """
    Sends the given application e-mail.
    :return: None
    """
    if not mail.init():
        logger.error("email failed: SMTP not configured")

    msg = EmailMultiAlternatives(subject, body, sender, (recipient, ))
    msg.attach_alternative(body, "text/html")

    if msg.send() == 0:
        logger.error("email failed: %s - '%s'", recipient, subject)
    else:
        logger.info("email sent: %s - '%s'", recipient, subject)


@task()
def disconnect_openvpn_client(client):
    """
    Kills the OpenVPN client connnection with the given address.
    :return: None
    """
    with openvpn.Management(settings.OPENVPN_MANAGEMENT_SOCKET) as m:
        m.run("kill", client)

    logger.info("disconnected openvpn client %s", client)
