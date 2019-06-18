from rest_framework import serializers
from mangle.common import models


#######################################
# Profile
#######################################

class ProfileDeviceSerializer(serializers.ModelSerializer):
    os = serializers.CharField(allow_blank=False, required=True, write_only=True)

    class Meta:
        model = models.Device
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "last_login",
                  "name",
                  "os", )

    def save(self, user, **kwargs):
        """
        Saves the Device with the given User.
        :return: Device
        """
        self.is_valid(raise_exception=True)

        # validate the user hasn't reached the group limit on devices
        if user.devices.count() >= user.group.max_devices:
            raise serializers.ValidationError({
                "name": "You cannot create any more devices."
            })

        # create and save the Device
        self.instance = models.Device.objects.create(
            name=self.validated_data["name"],
            user=user,
        )

        return self.instance

    def validate_name(self, value):
        """
        Strips invalid characters from and returns the Device name. OpenVPN has
        trouble with names that with non-ASCII characters.
        :return:
        """
        return value.encode("ascii", errors="ignore").decode()

    def validate_os(self, value):
        """
        Validates and returns the device's operating system.
        :return: str
        """
        if value not in ("windows", "macos", "linux"):
            raise serializers.ValidationError("Unknown operating system.")
        return value


class ProfileGroupSerializer(serializers.ModelSerializer):
    class Meta:
        model = models.Group
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "max_devices",
                  "name", )


class ProfileSerializer(serializers.ModelSerializer):
    devices = ProfileDeviceSerializer(many=True, read_only=True)
    group = ProfileGroupSerializer(read_only=True)

    class Meta:
        model = models.User
        fields = ("id",
                  "created_at",
                  "updated_at",
                  "devices",
                  "email",
                  "group",
                  "is_admin",
                  "is_enabled",
                  "last_login",
                  "mfa_enabled",
                  "mfa_enforced",
                  "mfa_required", )
