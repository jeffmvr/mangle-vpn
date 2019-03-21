import copy
import uuid
import pyotp

from django.contrib.auth.models import AbstractBaseUser
from django.db import models
from django.utils import timezone
from mangle.common import config, iptables, managers, pki


class Model(models.Model):
    """
    Model defines common fields and methods useful to all application models
    and should serve as their base class.
    """
    id = models.UUIDField(default=uuid.uuid4, editable=False, primary_key=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        abstract = True
        ordering = ("id", )

    def __init__(self, *args, **kwargs):
        """
        Creates a shallow copy of the object and stores it in the _original
        property so it can be determined if any fields changed values since it
        was first loaded.
        """
        super().__init__(*args, **kwargs)
        self.original = copy.copy(self)

    @property
    def exists(self):
        """
        Returns whether the model exists in the database.
        :return: bool
        """
        return self.id is not None

    def has_changed(self, field: str):
        """
        Returns whether the given model field valid has changed since the model
        was initially loaded.
        :return: bool
        """
        return getattr(self, field) == getattr(self.original, field)


class User(AbstractBaseUser, Model):
    email = models.CharField(db_index=True, max_length=255, unique=True)
    group = models.ForeignKey("Group", on_delete=models.CASCADE)
    is_admin = models.BooleanField(blank=True, default=False)
    is_enabled = models.BooleanField(blank=True, default=True)
    name = models.CharField(blank=True, default="", max_length=255)
    mfa_enabled = models.BooleanField(blank=True, default=False)
    mfa_secret = models.CharField(blank=True, default="", max_length=255)

    objects = managers.UserManager()

    USERNAME_FIELD = "email"

    class Meta:
        db_table = "users"
        ordering = ("email", )
        default_related_name = "users"

    @property
    def is_active(self):
        """
        Returns whether the user is considered active, taking into account both
        the user and group `is_enabled` values.
        :return: bool
        """
        return self.is_enabled and self.group.is_enabled

    @property
    def mfa_url(self):
        """
        Returns the user's two-factor authenticaiton provisioning URL.
        :return: str
        """
        organization = config.get("app_organization", "Mangle")

        return pyotp.TOTP(self.mfa_secret).provisioning_uri(
            name=self.email,
            issuer_name="{} VPN".format(organization),
        )

    def save(self, *args, **kwargs):
        """
        Sets the user's `mfa_secret` if it has not been set prior to saving.
        :return: None
        """
        if not self.mfa_secret:
            self.reset_mfa()
        super().save(*args, **kwargs)

    def verify_mfa_code(self, code):
        """
        Returns whether the given two-factor authentication code is valid based
        on the user's current `mfa_secret`.
        :return: bool
        """
        return pyotp.TOTP(self.mfa_secret).verify(code)

    def reset_mfa(self, length=16):
        """
        Sets the user's `mfa_secret` value to a random value and marks the user
        as not having setup two-factor authentication.
        :return: None
        """
        self.mfa_secret = pyotp.random_base32(length)
        self.mfa_enabled = False


class Device(Model):
    fingerprint = models.CharField(blank=True, db_index=True, max_length=255)
    last_login = models.DateTimeField(blank=True, null=True)
    name = models.CharField(max_length=32)
    serial = models.CharField(blank=True, default="", max_length=255)
    user = models.ForeignKey("User", on_delete=models.CASCADE)

    objects = managers.DeviceManager()

    class Meta:
        db_table = "devices"
        ordering = ("name", )
        default_related_name = "devices"

    @property
    def common_name(self):
        """
        Returns the device's common name that will be used when generating
        certificates and for connected clients.
        :return: str
        """
        return "{}:{}".format(self.user.email, self.name)

    def create_keypair(self):
        """
        Creates and returns the device's keypair.
        :return: KeyPair
        """
        keypair = pki.create_client_keypair(self.common_name, 3650)
        self.fingerprint = keypair.fingerprint
        self.serial = keypair.crt.serial_number
        self.save()
        return keypair


class RevokedDevice(Model):
    serial = models.CharField(max_length=255)

    class Meta:
        db_table = "revoked_devices"
        ordering = ("created_at", )


class Client(Model):
    common_name = models.CharField(db_index=True, max_length=255, unique=True)
    device = models.OneToOneField("Device", on_delete=models.CASCADE)
    platform = models.CharField(default="", max_length=32)
    remote_ip = models.CharField(max_length=32)
    virtual_ip = models.CharField(db_index=True, max_length=32, unique=True)

    objects = managers.ClientManager()

    class Meta:
        db_table = "clients"
        ordering = ("virtual_ip", )
        default_related_name = "client"

    @property
    def duration(self):
        """
        Returns the number of seconds the client since the client connected.
        :return: int
        """
        return (timezone.now() - self.created_at).seconds

    @property
    def group(self):
        """
        Returns the client user's group.
        :return: Group
        """
        return self.user.group

    @property
    def user(self):
        """
        Returns the client device's user.
        :return: User
        """
        return self.device.user

    def create_firewall_rule(self):
        """
        Creates the client firewall rule.
        :return: bool
        """
        iptables.append_unique_rule(
            "filter",
            "MangleVPN_Clients",
            "-s", self.virtual_ip,
            "-j", self.group.chain,
        )

    def delete_firewall_rule(self):
        """
        Deletes the client firewall rule.
        :return: bool
        """
        iptables.delete_rule(
            "filter",
            "MangleVPN_Clients",
            "-s", self.virtual_ip,
            "-j", self.group.chain,
        )


class Group(Model):
    description = models.TextField(blank=True, default="")
    is_enabled = models.BooleanField(blank=True, default=True)
    max_devices = models.IntegerField(blank=True, default=1)
    name = models.CharField(db_index=True, max_length=32, unique=True)

    objects = managers.GroupManager()

    class Meta:
        db_table = "groups"
        ordering = ("name", )
        default_related_name = "groups"

    @property
    def chain(self):
        """
        Returns the name of the group's firewall chain.
        :return: str
        """
        return "MangleVPN_Group_{}".format(str(self.id).replace("-", ""))[:28]

    @property
    def clients(self):
        """
        Returns all of the group's VPN clients.
        :return: []Client
        """
        return Client.objects.filter(device__user__group_id=self.id).all()

    def create_firewall_chain(self):
        """
        Creates the group firewall chain.
        :return: bool
        """
        iptables.create_chain("filter", self.chain)
        iptables.flush("filter", self.chain)

        # create all of the group's firewall rules
        for rule in self.firewall_rules.order_by("-action"):
            if rule.is_enabled:
                rule.create_firewall_rule()

        # each group will have a default DROP rule at the end
        iptables.append_unique_rule("filter", self.chain, "-j DROP")

    def delete_firewall_chain(self):
        """
        Deletes the group firewall chain.
        :return: bool
        """
        iptables.delete_chain("filter", self.chain)


class FirewallRule(Model):
    action = models.CharField(max_length=255)
    destination = models.CharField(blank=True, default="", max_length=255)
    group = models.ForeignKey("Group", on_delete=models.CASCADE)
    is_enabled = models.BooleanField(blank=True, default=True)
    port = models.CharField(blank=True, default="", max_length=255)
    protocol = models.CharField(blank=True, default="", max_length=255)

    class Meta:
        db_table = "firewall_rules"
        ordering = ("-action", )
        default_related_name = "firewall_rules"

    @property
    def args(self):
        """
        Returns a list containing all of firewall rule iptables arguments.
        :return: List[str]
        """
        args = []

        if self.destination:
            args.extend(["-d", self.destination, ])

        if self.protocol:
            args.extend(["-p", self.protocol, ])

            # ports are only added when a protocol is set
            # ports can be specified in a variety of ways:
            #   - single (80,443)
            #   - range (09:443)
            #   - a combination of both (22,80:443)
            if self.port:
                if "," in self.port or ":" in self.port:
                    args.extend(["--match multiport", "--dports", self.port, ])
                else:
                    args.extend(["--dport", self. port, ])

        args.extend(["-j", self.action, ])
        return args

    def create_firewall_rule(self):
        """
        Creates the firewall rule.
        :return: bool
        """
        return iptables.append_unique_rule("filter", self.group.chain, *self.args)

    def delete_firewall_rule(self):
        """
        Creates the firewall rule.
        :return: bool
        """
        return iptables.delete_rule("filter", self.group.chain, *self.args)


class Event(Model):
    detail = models.TextField(default="")
    name = models.CharField(max_length=255)
    user = models.ForeignKey("User", on_delete=models.CASCADE)

    class Meta:
        db_table = "events"
        ordering = ("-created_at", "name", )
        default_related_name = "events"


class Setting(Model):
    name = models.CharField(db_index=True, max_length=255, unique=True)
    value = models.TextField()

    objects = managers.SettingManager()

    class Meta:
        db_table = "setting"
        ordering = ("name", )
