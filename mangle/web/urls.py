from django.urls import include, path
from mangle.web import views


urlpatterns = [
    path("", views.show_app),
    path("login", views.show_login),
    path("logout", views.process_logout),
    path("oauth", views.process_oauth),
    path("install", views.show_install),
    path("install/process", views.process_install),
    path("mfa", views.show_mfa),
    path("mfa/process", views.process_mfa),
    path("mfa/setup", views.show_mfa_setup),

    # API
    path("api/", include("mangle.web.api.urls")),
]
