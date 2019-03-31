from django import forms
from mangle.common import config, models, validators


class InstallForm(forms.Form):
    admin_email = forms.CharField(required=True)
    admin_password = forms.CharField(required=True)
    admin_password_confirm = forms.CharField(required=True)
    app_hostname = forms.CharField(required=True)
    app_organization = forms.CharField(required=True)

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

    def clean_admin_password(self):
        """
        Validates the password values match and the password meets the minimum
        complexity requirements.
        :return: str
        """
        if not validators.is_valid_password(self.data["admin_password"]):
            raise forms.ValidationError(
                "The password must be at least 8 characters and contain one "
                "uppercase letter, one lowercase letter, and one digit."
            )
        if self.data["admin_password"] != self.data["admin_password_confirm"]:
            raise forms.ValidationError("The passwords do not match.")
        return self.data["admin_password"]

    def save(self):
        """
        Performs the application installation.
        :return: None
        """
        config.set("app_hostname",
                   self.cleaned_data["app_hostname"])
        config.set("app_organization",
                   self.cleaned_data["app_organization"])
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
        user = models.User(group=group, is_admin=True)
        user.email = self.cleaned_data["admin_email"]
        user.set_password(self.cleaned_data["admin_password"])
        user.save()
        return user


class PasswordForm(forms.Form):
    password = forms.CharField(required=True)
    password_confirm = forms.CharField(required=True)

    def clean_password(self):
        """
        Validates the password values match and the password meets the minimum
        complexity requirements.
        :return: str
        """
        if not validators.is_valid_password(self.data["password"]):
            raise forms.ValidationError(
                "The password must be at least 8 characters and contain one "
                "uppercase letter, one lowercase letter, and one digit."
            )
        if self.data["password"] != self.data["password_confirm"]:
            raise forms.ValidationError("The passwords do not match.")
        return self.data["password"]
