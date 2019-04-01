import logging

from django.conf import settings
from rest_framework import serializers
from mangle.cli.management.commands import install
from mangle.common import config, models, validators
from mangle.common.utils import bash, fs, net


logger = logging.getLogger(__name__)


#######################################
# User
#######################################

class UserDeviceSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.Device
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "name", )


class UserGroupSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.Group
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "name", )


class UserSerializer(serializers.ModelSerializer):
    group = UserGroupSerializer(read_only=True)
    group_id = serializers.UUIDField(required=True)

    class Meta:
        model = models.User
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "email",
                  "group",
                  "group_id",
                  "is_admin",
                  "is_enabled",
                  "last_login",
                  "mfa_enabled",
                  "mfa_enforced",
                  "name", )


class UserInviteSerializer(serializers.Serializer):
    email = serializers.CharField(allow_blank=False, required=True)
    group_id = serializers.UUIDField(required=True)

    def validate_email(self, value):
        """
        Validates each of the e-mail addresses. If an e-mail address is invalid
        then it is skipped without any errors (?).
        :return: List[str]
        """
        emails = []

        for email in value.split():
            if validators.is_email(email):
                emails.append(email.lower())

        return emails

    def validate_group_id(self, value):
        """
        Validates the group ID exists.
        :return: int
        """
        if not models.Group.objects.filter(pk=value).exists():
            raise serializers.ValidationError(
                "The selected group does not exist.")
        return value

    def save(self, **kwargs):
        """
        Creates or updates the user for each e-mail address and returns a list
        containing all of the users that were created (but not updated).
        :return: List[User]
        """
        self.is_valid(True)

        users = []

        # if an e-mail address already exists, then that user is just updated
        # with the group and has their account enabled
        for email in self.validated_data["email"]:
            user = models.User.objects.by_email(email)

            if not user:
                user = models.User(email=email)

                # new users will have a password set
                password = user.reset_password()
                user.temp_password = password
                users.append(user)

            user.group_id = self.validated_data["group_id"]
            user.is_enabled = True
            user.save()

        return users


#######################################
# Group
#######################################

class GroupUserSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.User
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "email",
                  "last_login",
                  "name", )


class GroupSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.Group
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "description",
                  "is_enabled",
                  "max_devices",
                  "mfa_enforced",
                  "name", )


#######################################
# FirewallRule
#######################################

class FirewallRuleSerializer(serializers.ModelSerializer):
    group_id = serializers.UUIDField(required=True)

    class Meta:
        model = models.FirewallRule
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "action",
                  "destination",
                  "group_id",
                  "is_enabled",
                  "port",
                  "protocol", )

    def validate(self, attrs):
        """
        Validates the firewall rule.
        :return: None
        """
        # ports cannot be specified when the protocol is 'all', per iptables
        if attrs["protocol"] == "all" and attrs.get("port"):
            raise serializers.ValidationError({
                "protocol": "Protocol must be TCP or UDP when adding ports."
            })

        return attrs

    def validate_action(self, value):
        """
        Validates the firewall rule action.
        :return: str
        """
        value = value.upper()
        if value not in ("ACCEPT", "DROP"):
            raise serializers.ValidationError("Must be ACCEPT or DROP.")
        return value.upper()

    def validate_destination(self, value):
        """
        Validates the destination is an IPv4 CIDR address.
        :return: str
        """
        if not validators.is_ipv4(value) and not validators.is_cidr(value):
            raise serializers.ValidationError("Must be an IPv4 CIDR address.")
        return value

    def validate_group_id(self, value):
        """
        Validates the selected Group exists.
        :return: int
        """
        if not models.Group.objects.filter(pk=value).exists():
            raise serializers.ValidationError("The group does not exist.")
        return value

    def validate_port(self, value):
        """
        Validates the port is formatted correctly and
        :return: str
        """
        try:
            for port in value.split(","):
                if ":" in port:
                    low, high = map(lambda x: int(x), port.split(":"))
                    if low >= high or low < 1 or high > 65535:
                        raise serializers.ValidationError(
                            "There are one or more invalid port ranges.")
                elif port and not 1 <= int(port) <= 65535:
                    raise serializers.ValidationError(
                        "There are one or more invalid ports.")
            return value
        except ValueError:
            raise serializers.ValidationError(
                "There are one or more invalid ports.")

    def validate_protocol(self, value):
        """
        Validates the protocol is either TCP or UDP.
        :return: str
        """
        if value not in ("all", "tcp", "udp"):
            raise serializers.ValidationError(
                "Protocol must be either All, TCP, or UDP")
        return value.lower()


#######################################
# Device
#######################################

class DeviceUserSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.User
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "email", )


#######################################
# Client
#######################################

class ClientDeviceSerializer(serializers.ModelSerializer):
    user = DeviceUserSerializer(read_only=True)

    class Meta:
        model = models.Device
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "name",
                  "user", )


class ClientSerializer(serializers.ModelSerializer):
    device = ClientDeviceSerializer(read_only=True)

    class Meta:
        model = models.Client
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "device",
                  "duration",
                  "platform",
                  "remote_ip",
                  "virtual_ip", )


#######################################
# Event
#######################################

class EventUserSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.User
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "email", )


class EventSerializer(serializers.ModelSerializer):
    user = EventUserSerializer(read_only=True)

    class Meta:
        model = models.Event
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "detail",
                  "name",
                  "user", )


#######################################
# Setting
#######################################

class BaseSettingSerializer(serializers.Serializer):
    @property
    def settings(self):
        """
        Returns all of the serial
        :return:
        """
        settings = {}
        for name in self.fields:
            settings[name] = config.get(name, "")

        settings["interfaces"] = {}

        for iface in net.interface_names("lo", "tun"):
            settings["interfaces"][iface] = net.interface_ip(iface)

        return settings

    def save(self, **kwargs):
        """
        Validates and updates the application settings.
        :return: None
        """
        self.is_valid(True)

        for name, value in self.validated_data.items():
            config.set(name, value)


class AppSettingSerializer(BaseSettingSerializer):
    app_hostname = serializers.CharField(required=True)
    app_http_port = serializers.IntegerField(required=True)
    app_https_port = serializers.IntegerField(required=True)
    app_organization = serializers.CharField(required=True)
    app_ssl_crt = serializers.CharField(allow_blank=True, required=False)
    app_ssl_key = serializers.CharField(allow_blank=True, required=False)

    def validate_app_hostname(self, value):
        """
        Validates and returns the hostname.
        :return: str
        """
        if not validators.is_domain(value) and not validators.is_ipv4(value):
            raise serializers.ValidationError(
                "Hostname must be a valid DNS hostname or IPv4 address."
            )
        return value.lower()

    def validate_app_http_port(self, value):
        """
        Validates and returns the HTTP port.
        :return: int
        """
        if not validators.is_port(value):
            raise serializers.ValidationError("Port must be a valid TCP port.")
        return int(value)

    def validate_app_https_port(self, value):
        """
        Validates and returns the HTTPs port.
        :return: int
        """
        if not validators.is_port(value):
            raise serializers.ValidationError("Port must be a valid TCP port.")
        return int(value)

    def save(self, **kwargs):
        """
        Updates the application settings and SSL certificate and private key
        if provided.
        :return: None
        """
        old_http_port = config.get("app_http_port")
        old_https_port = config.get("app_https_port")

        # the SSL certificate and private key should not be stored in the
        # database as settings so remove them from the validated_data dict
        # prior to saving
        ssl_crt = self.initial_data.pop("app_ssl_crt", None)
        ssl_key = self.initial_data.pop("app_ssl_key", None)

        super().save(**kwargs)

        if ssl_crt and ssl_key:
            fs.write_file(settings.WEB_SSL_CRT_FILE, ssl_crt, 0o600)
            fs.write_file(settings.WEB_SSL_KEY_FILE, ssl_key, 0o600)
            bash.run("systemctl", "reload", "nginx")
        
        if (old_http_port != self.validated_data["app_http_port"] or
                old_https_port != self.validated_data["app_https_port"]):
            install.create_web_vhost()
            bash.run("systemctl", "restart", "nginx")
            bash.run("systemctl", "restart", "mangle-web")


class AuthSettingSerializer(BaseSettingSerializer):
    oauth2_provider = serializers.CharField(allow_blank=True, required=False)
    oauth2_client_id = serializers.CharField(allow_blank=True, required=False)
    oauth2_client_secret = serializers.CharField(allow_blank=True, required=False)

    def validate_oauth2_client_id(self, value):
        """
        Validates the OAuth2 client ID.
        :return: str
        """
        if self.initial_data["oauth2_provider"] != "none" and not value:
            raise serializers.ValidationError("This field is required.")
        return value

    def validate_oauth2_client_secret(self, value):
        """
        Validates the OAuth2 client secret.
        :return: str
        """
        if self.initial_data["oauth2_provider"] != "none" and not value:
            raise serializers.ValidationError("This field is required.")
        return value

    def validate(self, attrs):
        """
        Removes any OAuth2 settings if provider is None.
        :return: str
        """
        if attrs["oauth2_provider"] == "none":
            attrs["oauth2_client_id"] = ""
            attrs["oauth2_client_secret"] = ""
        return attrs


class MailSettingSerializer(BaseSettingSerializer):
    smtp_host = serializers.CharField(allow_blank=True, required=False)
    smtp_port = serializers.IntegerField(required=False)
    smtp_username = serializers.CharField(allow_blank=True, required=False)
    smtp_password = serializers.CharField(allow_blank=True, required=False)
    smtp_reply_address = serializers.CharField(allow_blank=True, required=False)

    def validate_smtp_host(self, value):
        """
        Validates and returns the hostname.
        :return: str
        """
        if not validators.is_domain(value) and not validators.is_ipv4(value):
            raise serializers.ValidationError(
                "Hostname must be a valid DNS hostname or IPv4 address."
            )
        return value.lower()

    def validate_smtp_port(self, value):
        """
        Validates and returns the HTTP port.
        :return: int
        """
        if not validators.is_port(value):
            raise serializers.ValidationError(
                "Port must be a valid TCP or UDP port."
            )
        return int(value)


class VpnSettingSerializer(BaseSettingSerializer):
    vpn_domain = serializers.CharField(allow_blank=True, required=False)
    vpn_hostname = serializers.CharField(required=True)
    vpn_interface = serializers.CharField(required=True)
    vpn_nameservers = serializers.CharField(allow_blank=True, required=False)
    vpn_nat_interface = serializers.CharField(required=True)
    vpn_port = serializers.IntegerField(required=True)
    vpn_protocol = serializers.CharField(required=True)
    vpn_routes = serializers.CharField(allow_blank=True, required=False)
    vpn_subnet = serializers.CharField(required=True)

    def validate_vpn_hostname(self, value):
        """
        Validates and returns the VPN hostname.
        :param value:
        :return:
        """
        if not validators.is_domain(value) and not validators.is_ipv4(value):
            raise serializers.ValidationError(
                "Hostname must be a valid DNS hostname or IPv4 address."
            )
        return value.lower()

    def validate_vpn_interface(self, value):
        """
        Validates and returns the VPN interface.
        :return: str
        """
        if value not in net.interface_names():
            raise serializers.ValidationError("The interface does not exist.")
        return value

    def validate_vpn_nameservers(self, value):
        """
        Validates and returns the VPN nameservers.
        :return: str
        """
        if not value:
            return ""

        for nameserver in value.split("\n"):
            if not validators.is_ipv4(nameserver):
                raise serializers.ValidationError(
                    "There are one or more invalid DNS servers."
                )
        return value

    def validate_vpn_nat_interface(self, value):
        """
        Validates and returns the VPN NAT interface.
        :return: str
        """
        if value not in net.interface_names():
            raise serializers.ValidationError("The interface does not exist.")
        return value

    def validate_vpn_port(self, value):
        """
        Validates and returns the VPN port.
        :return: int
        """
        if not validators.is_port(value):
            raise serializers.ValidationError(
                "Port must be a valid TCP or UDP port."
            )
        return int(value)

    def validate_vpn_protocol(self, value):
        """
        Validates and returns the VPN protocol.
        :return: str
        """
        if value not in ("tcp", "udp"):
            raise serializers.ValidationError("Protocol must be TCP or UDP.")
        return value.lower()

    def validate_vpn_routes(self, value):
        """
        Validates and returns the VPN routes.
        :return: str
        """
        if not value:
            return ""

        for route in value.split("\n"):
            if not validators.is_cidr(route) and not validators.is_ipv4(route):
                raise serializers.ValidationError(
                    "There are one or more invalid routes."
                )
        return value

    def validate_vpn_subnet(self, value):
        """
        Validates and returns the VPN subnet.
        :return: str
        """
        if not validators.is_cidr(value):
            raise serializers.ValidationError(
                "Subnet must be an IPv4 CIDR address."
            )
        return value
