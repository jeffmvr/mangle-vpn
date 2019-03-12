from django.urls import include, path
from rest_framework import routers
from mangle.web.api.admin import views


router = routers.DefaultRouter()
router.register("clients", views.ClientAdminViewSet)
router.register("devices", views.DeviceAdminViewSet)
router.register("firewall", views.FirewallAdminViewSet)
router.register("groups", views.GroupAdminViewSet)
router.register("events", views.EventAdminViewSet)
router.register("users", views.UserAdminViewSet)
router.register("openvpn", views.OpenVPNViewSet, base_name="openvpn")

urlpatterns = [
    path("", include(router.urls)),

    # Settings
    path("settings/app", views.AppSettingView.as_view()),
    path("settings/mail", views.MailSettingView.as_view()),
    path("settings/mail/test", views.MailSettingTestView.as_view()),
    path("settings/auth", views.AuthSettingView.as_view()),
    path("settings/vpn", views.VPNSettingView.as_view()),

    # Update
    path("update", views.UpdateAppView.as_view()),
]
