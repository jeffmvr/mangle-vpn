import logging
import time

from django.conf import settings
from django.http.response import JsonResponse
from rest_framework import filters, status, views, viewsets
from rest_framework.decorators import action
from rest_framework.response import Response
from mangle.common import config, mail, models, openvpn
from mangle.common.utils import bash
from mangle.web.api import authentication
from mangle.web.api.admin import permissions, serializers


logger = logging.getLogger(__name__)


class AdminView(views.APIView):
    authentication_classes = (authentication.ApiSessionAuthentication, )
    permission_classes = (permissions.AdminPermission, )
    filter_backends = (filters.SearchFilter, )


class AdminViewSet(AdminView, viewsets.GenericViewSet):
    pass


#######################################
# User
#######################################

class UserAdminViewSet(AdminView, viewsets.ModelViewSet):
    queryset = models.User.objects.all()
    serializer_class = serializers.UserSerializer
    search_fields = ("email", "name", "group__name", )

    def create(self, request, *args, **kwargs):
        """
        Creates a user account for all of the submitted e-mail addresses.
        :return: Response
        """
        serializer = serializers.UserInviteSerializer(data=request.data)

        users = serializer.save()

        # contains the e-mail and password that is returned to the UI
        emails_passwords = []

        # send e-mail notifications if desired
        for user in users:
            emails_passwords.append({
                "email": user.email,
                "password": user.temp_password,
            })

            if request.data.get("notify", False):
                organization = config.get("app_organization")

                mail.send_template(
                    recipient=user.email,
                    subject="Welcome to the {} VPN!".format(organization),
                    template="mail/Welcome.html",
                    data={
                        "email": user.email,
                        "organization": organization,
                        "password": user.temp_password,
                        "url": config.url(),
                    }
                )

        return Response(emails_passwords, status=status.HTTP_201_CREATED)

    @action(["DELETE"], detail=True)
    def mfa(self, request, pk=None):
        """
        Resets the user's two-factor authentication code and status. This will
        force them to re-add two-factor authentication on their account when
        they login next.
        :return: Response
        """
        user = self.get_object()
        user.reset_mfa()
        user.save()
        return Response(status=status.HTTP_204_NO_CONTENT)

    @action(["DELETE"], detail=True)
    def password(self, request, pk=None):
        """
        Resets the user's password and forces them to change it.
        :return: Response
        """
        user = self.get_object()
        password = user.reset_password()
        user.save()

        # send e-mail notifying user
        mail.send_template(
            recipient=user.email,
            subject="{} VPN Password Reset".format(config.get("app_organization")),
            template="mail/Password.html",
            data={
                "email": user.email,
                "organization": config.get("app_organization"),
                "password": password,
                "url": config.url(),
            }
        )

        return Response({"password": password})

    @action(["GET"], detail=True)
    def devices(self, request, pk=None):
        """
        Returns all of the user devices.
        :return: Response
        """
        user = self.get_object()
        serializer = serializers.UserDeviceSerializer(user.devices, many=True)
        return Response(serializer.data)

    @action(["PUT"], detail=True)
    def mfa(self, request, pk=None):
        """
        Resets the user's two-factor authentication secret and status.
        :return: Response
        """
        user = self.get_object()
        user.reset_mfa()
        user.save()
        return Response(status=status.HTTP_204_NO_CONTENT)


#######################################
# Group
#######################################

class GroupAdminViewSet(AdminViewSet, viewsets.ModelViewSet):
    queryset = models.Group.objects.all()
    serializer_class = serializers.GroupSerializer
    search_fields = ("name", )

    @action(["GET"], detail=False)
    def all(self, request, *args, **kwargs):
        """
        Returns all of the groups suitable for populating a dropdown.
        :return: Response
        """
        serializer = serializers.UserGroupSerializer(self.queryset, many=True)
        return Response(serializer.data)

    @action(["GET"], detail=True)
    def firewall(self, request, *args, **kwargs):
        """
        Returns all of the group's firewall rules.
        :return: Response
        """
        rules = self.get_object().firewall_rules

        serializer = serializers.FirewallRuleSerializer(rules, many=True)
        return Response(serializer.data)

    @action(["GET"], detail=True)
    def users(self, request, *args, **kwargs):
        """
        Returns all of the group's users.
        :return: Response
        """
        users = self.get_object().users

        serializer = serializers.GroupUserSerializer(users, many=True)
        return Response(serializer.data)


#######################################
# FirewallRule
#######################################

class FirewallAdminViewSet(AdminViewSet, viewsets.ModelViewSet):
    queryset = models.FirewallRule.objects.all()
    serializer_class = serializers.FirewallRuleSerializer


#######################################
# Device
#######################################

class DeviceAdminViewSet(AdminViewSet, viewsets.mixins.DestroyModelMixin):
    queryset = models.Device.objects.all()
    serializer_class = serializers.UserDeviceSerializer


#######################################
# Client
#######################################

class ClientAdminViewSet(AdminViewSet,
                         viewsets.mixins.ListModelMixin,
                         viewsets.mixins.DestroyModelMixin):
    queryset = models.Client.objects.all()
    serializer_class = serializers.ClientSerializer
    search_fields = ("remote_ip",
                     "virtual_ip",
                     "device__name",
                     "device__user__email",
                     "device__user__name",
                     "device__user__group__name", )


#######################################
# Event
#######################################

class EventAdminViewSet(AdminViewSet, viewsets.mixins.ListModelMixin):
    queryset = models.Event.objects.all()
    serializer_class = serializers.EventSerializer
    search_fields = ("name",
                     "user__email",
                     "user__name",
                     "user__group__name", )


#######################################
# OpenVPN
#######################################

class OpenVPNViewSet(AdminViewSet):
    def list(self, request, *args, **kwargs):
        """
        Returns the status of the OpenVPN server.
        :return: Response
        """
        return Response({
            "restart_pending": config.get_bool("vpn_restart_pending", False),
            "status": openvpn.is_running(),
        })

    @action(["GET"], detail=False)
    def toggle(self, request, *args, **kwargs):
        """
        Starts or stops the OpenVPN server depending on it's current state.
        :return: Response
        """
        if openvpn.is_running():
            openvpn.stop()
        else:
            openvpn.start()

        # sleep here for a very short amount of time to give the OpenVPN server
        # process time to start/stop and allows an accurate status response
        time.sleep(0.5)
        return self.list(request)

    @action(["GET"], detail=False)
    def restart(self, request, *args, **kwargs):
        """
        Restarts the OpenVPN server.
        :return: Response
        """
        openvpn.restart()
        time.sleep(0.5)
        return self.list(request)


#######################################
# Setting
#######################################

class BaseSettingView(AdminView):
    serializer_class = None

    def get(self, request, *args, **kwargs):
        """
        Returns the application settings.
        :return: Response
        """
        return JsonResponse(self.serializer_class().settings)

    def put(self, request, *args, **kwargs):
        """
        Updates the application settings.
        :return: Response
        """
        serializer = self.serializer_class(data=request.data)
        serializer.save()
        return Response(status=status.HTTP_204_NO_CONTENT)


class AppSettingView(BaseSettingView):
    serializer_class = serializers.AppSettingSerializer


class MailSettingView(BaseSettingView):
    serializer_class = serializers.MailSettingSerializer


class MailSettingTestView(AdminView):
    def post(self, request):
        """
        Sends a test e-mail to the submitted e-mail address.
        :return: Response
        """
        email = request.data.get("email")

        mail.send(email, "Mangle VPN Test E-mail", "It Works!")
        return Response(status=status.HTTP_204_NO_CONTENT)


class AuthSettingView(BaseSettingView):
    serializer_class = serializers.AuthSettingSerializer


class VPNSettingView(BaseSettingView):
    serializer_class = serializers.VpnSettingSerializer


#######################################
# App Update
#######################################

class UpdateAppView(AdminView):
    def post(self, request):
        """
        Updates the application to the latest code.
        :return: Response
        """
        bash.run("make", "update", cwd=settings.BASE_DIR)
        return Response(status=status.HTTP_204_NO_CONTENT)

