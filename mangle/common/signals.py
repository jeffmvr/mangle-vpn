from django.db.models import signals
from django.dispatch import receiver
from mangle.common import models, tasks


@receiver(signals.post_save, sender=models.User)
def user_post_save(instance, **kwargs):
    """
    Handles post-save actions for the given user.
    :return: None
    """
    if not instance.is_active:
        models.Client.objects.by_user(instance).delete()

    # if the user's group has changed then the each client must have their rule
    # deleted and re-created in order to have the new group rules applied
    if instance.has_changed("group_id"):
        for client in models.Client.objects.by_user(instance).all():
            client.delete_firewall_rule()
            client.create_firewall_rule()


@receiver(signals.post_delete, sender=models.Device)
def device_post_delete(instance, **kwargs):
    """
    Handles post-delete actions for the given device.
    :return: None
    """
    if instance.serial:
        models.RevokedDevice.objects.create(serial=instance.serial)


@receiver(signals.post_save, sender=models.Group)
def group_post_save(instance, **kwargs):
    """
    Handles post-save actions for the given group.
    :return: None
    """
    if instance.is_enabled:
        instance.create_firewall_chain()
    else:
        instance.clients.delete()
        instance.delete_firewall_chain()


@receiver(signals.post_delete, sender=models.Group)
def group_post_delete(instance, **kwargs):
    """
    Handles post-delete actions for the given group.
    :return: None
    """
    instance.delete_firewall_chain()


@receiver(signals.post_delete, sender=models.Client)
def client_post_delete(instance, **kwargs):
    """
    Handles post-delete actions for the given client.
    :return: None
    """
    instance.delete_firewall_rule()
    tasks.disconnect_openvpn_client(instance.remote_ip)


@receiver(signals.post_save, sender=models.Client)
def client_post_save(instance, **kwargs):
    """
    Handles post-save actions for the given client.
    :return: None
    """
    instance.create_firewall_rule()


@receiver(signals.post_save, sender=models.FirewallRule)
def firewall_rule_post_save(instance, **kwargs):
    """
    Handles post-save action for the given firewall rule.
    :return: None
    """
    instance.group.create_firewall_chain()


@receiver(signals.post_delete, sender=models.FirewallRule)
def firewall_rule_post_delete(instance, **kwargs):
    """
    Handles post-delete actions for the given firewall rule.
    :return: None
    """
    instance.delete_firewall_rule()
