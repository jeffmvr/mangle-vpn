from django.http import HttpResponse
from django.utils import timezone
from rest_framework import status, views, viewsets
from rest_framework.response import Response
from mangle.common import config, models, openvpn
from mangle.web.api import authentication, permissions, serializers


class UserView(views.APIView):
    authentication_classes = (authentication.ApiSessionAuthentication, )
    permission_classes = (permissions.UserPermission, )


class UserViewSet(viewsets.GenericViewSet, UserView):
    pass


class ApiInfoView(views.APIView):
    def get(self, request):
        """
        Returns general API information.
        :return: Response
        """
        return Response({
            "app_organization": config.get("app_organization", "Mangle"),
        })


#######################################
# Profile
#######################################

class ProfileView(UserView):
    def get(self, request):
        """
        Returns the current user's profile.
        :return: Response
        """
        serializer = serializers.ProfileSerializer(request.user)
        return Response(serializer.data)


#######################################
# Device
#######################################

class DeviceViewSet(viewsets.mixins.CreateModelMixin,
                    viewsets.mixins.DestroyModelMixin,
                    viewsets.mixins.RetrieveModelMixin,
                    UserViewSet):
    queryset = models.Device.objects.all()
    serializer_class = serializers.ProfileDeviceSerializer

    def create(self, request, *args, **kwargs):
        """
        Creates a new device for the current user.
        :return: Response
        """
        serializer = self.get_serializer(data=request.data)
        serializer.save(request.user)
        return Response(serializer.data, status.HTTP_201_CREATED)

    def retrieve(self, request, pk=None, *args, **kwargs):
        """
        Returns the device OpenVPN client configuration as a file download.
        :return: Response
        """
        device = self.get_object()

        # devices can only be downloaded if their keys have not yet been
        # created and only if the device was created within the last minute
        if device.fingerprint or device.serial:
            return Response(status=status.HTTP_400_BAD_REQUEST)
        if (timezone.now() - device.created_at).seconds > 60:
            return Response(status=status.HTTP_400_BAD_REQUEST)

        # generate the device keypair
        keypair = device.create_keypair()

        # generate the OpenVPN client configuration
        conf = openvpn.client_config(*keypair.pem(), request.GET.get("linux"))

        # return the HTTP response as a file download
        resp = HttpResponse(conf)
        resp["Content-Length"] = len(conf)
        resp["Content-Type"] = "application/force-download"
        resp["Content-Disposition"] = self.get_disposition(device)
        return resp

    def get_disposition(self, device):
        """
        Returns the 'Content-Disposition' header value for the given device
        configuration file download.
        :return: str
        """
        org = config.get("app_organization")
        return 'attachment; filename="{} - {}.ovpn"'.format(org, device.name)
