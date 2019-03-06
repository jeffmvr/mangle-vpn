from django.contrib.auth.models import BaseUserManager
from django.db import models
from django.utils import timezone


class BaseManager(models.Manager):
    @property
    def qs(self):
        """
        Returns the manager's queryset.
        :return: QuerySet
        """
        return self.get_queryset()

    def by_pk(self, pk):
        """
        Returns the model with the given primary key.
        :return: BaseModel
        """
        return self.qs.filter(pk=pk).first()


class UserManager(BaseUserManager, BaseManager):
    def by_email(self, email):
        """
        Returns the user with the given e-mail address.
        :return: User
        """
        return self.qs.filter(email__iexact=email).first()

    def get_by_natural_key(self, username):
        """
        Returns the user with the given username. This overrides the auth
        package's BaseUserManager method to allow for case insensitive values.
        :return: User
        """
        return self.get(email__iexact=username)


class PasswordTokenManager(BaseManager):
    def by_user(self, user):
        """
        Returns the PasswordTokens with the given User.
        :return: List[Passwordtoken]
        """
        return self.qs.filter(user=user).all()

    def by_token(self, value):
        """
        Returns the PasswordToken with the given value.
        :return: PasswordToken
        """
        return self.qs.filter(value__iexact=value).first()

    def expired(self):
        """
        Returns all PasswordToken that have expired.
        :return: List[PasswordToken]
        """
        return self.qs.filter(expires_at__lte=timezone.now()).all()


class GroupManager(BaseManager):
    def by_name(self, name):
        """
        Returns the group with the given name.
        :return: Group
        """
        return self.qs.filter(name__iexact=name).first()


class DeviceManager(BaseManager):
    def by_fingerprint(self, fingerprint):
        """
        Returns the device with the given fingerprint.
        :return: Device
        """
        return self.qs.filter(fingerprint__iexact=fingerprint).first()

    def by_serial(self, serial):
        """
        Returns the device with the given serial number.
        :return: Device
        """
        return self.qs.filter(serial__iexact=serial).first()


class ClientManager(BaseManager):
    def by_common_name(self, common_name):
        """
        returns the client with the given common name.
        :return: Client
        """
        return self.qs.filter(common_name__exact=common_name).first()

    def by_user(self, user):
        """
        Returns a queryset containing all clients for the given user.
        :return: QuerySet
        """
        return self.qs.filter(device__user=user)


class SettingManager(BaseManager):
    def by_name(self, name):
        """
        Returns the setting with the given name.
        :return: Setting
        """
        return self.qs.filter(name__iexact=name).first()
