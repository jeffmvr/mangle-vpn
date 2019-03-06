from django.urls import include, path
from rest_framework import routers
from mangle.web.api import views


router = routers.DefaultRouter()
router.register("devices", views.DeviceViewSet)


urlpatterns = [
    path("", include(router.urls)),
    path("info", views.ApiInfoView.as_view()),
    path("profile", views.ProfileView.as_view()),

    # Admin
    path("admin/", include("mangle.web.api.admin.urls")),
]
