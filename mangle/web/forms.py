from django import forms
from mangle.common import config, models, validators


class InstallForm(forms.Form):
    admin_email = forms.CharField(required=True)
    app_hostname = forms.CharField(required=True)
    app_organization = forms.CharField(required=True)
    oauth2_client_id = forms.CharField(required=True)
    oauth2_client_secret = forms.CharField(required=True)

    def clean_admin_email(self):
        """
        Validates and returns the administrator e-mail address.
        :return: str
        """
        if not validators.is_email(self.data["admin_email"]):
            raise forms.ValidationError("A valid e-mail address is required.")
        return self.data["admin_email"]

    def clean_app_hostname(self):
        """
        Validates and returns the application hostname.
        :return: str
        """
        if (not validators.is_domain(self.data["app_hostname"]) and
                not validators.is_ipv4(self.data["app_hostname"])):
            raise forms.ValidationError(
                "A valid DNS name or IPv4 address is required.")
        return self.data["app_hostname"]

    def save(self):
        """Performs the application installation."""

        # update the application settings
        config.set("app_hostname",
                   self.cleaned_data["app_hostname"])
        config.set("app_organization",
                   self.cleaned_data["app_organization"])
        config.set("oauth2_client_id",
                   self.cleaned_data["oauth2_client_id"])
        config.set("oauth2_client_secret",
                   self.cleaned_data["oauth2_client_secret"])
        config.set("vpn_hostname",
                   self.cleaned_data["app_hostname"])

        # create the default group only if it doesn't already exist
        group = models.Group.objects.by_name("Default")
        if not group:
            group = models.Group.objects.create(
                name="Default",
                description="the default group that contains all users.",
            )

        # create and return the admin user
        return models.User.objects.create(
            email=self.cleaned_data["admin_email"],
            group=group,
            is_admin=True,
        )
